// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// cloudsql-proxy can be used as a proxy to Cloud SQL databases. It supports
// connecting to many instances and authenticating via different means.
// Specifically, a list of instances may be provided on the command line, in
// GCE metadata (for VMs), or provided during connection time via a
// FUSE-mounted directory. See flags for a more specific explanation.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/certs"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/fuse"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/limits"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/util"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	goauth "golang.org/x/oauth2/google"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

var (
	version        = flag.Bool("version", false, "Print the version of the proxy and exit")
	verbose        = flag.Bool("verbose", true, "If false, verbose output such as information about when connections are created/closed without error are suppressed")
	quiet          = flag.Bool("quiet", false, "Disable log messages")
	logDebugStdout = flag.Bool("log_debug_stdout", false, "If true, log messages that are not errors will output to stdout instead of stderr")

	refreshCfgThrottle = flag.Duration("refresh_config_throttle", proxy.DefaultRefreshCfgThrottle, "If set, this flag specifies the amount of forced sleep between successive API calls in order to protect client API quota. Minimum allowed value is "+minimumRefreshCfgThrottle.String())
	checkRegion        = flag.Bool("check_region", false, `If specified, the 'region' portion of the connection string is required for
Unix socket-based connections.`)

	// Settings for how to choose which instance to connect to.
	dir      = flag.String("dir", "", "Directory to use for placing Unix sockets representing database instances")
	projects = flag.String("projects", "", `Open sockets for each Cloud SQL Instance in the projects specified
(comma-separated list)`)
	instances = flag.String("instances", "", `Comma-separated list of fully qualified instances (project:region:name)
to connect to. If the name has the suffix '=tcp:port', a TCP server is opened
on the specified port to proxy to that instance. Otherwise, one socket file per
instance is opened in 'dir'. You may use INSTANCES environment variable
for the same effect. Using both will use value from flag, Not compatible with -fuse`)
	instanceSrc = flag.String("instances_metadata", "", `If provided, it is treated as a path to a metadata value which
is polled for a comma-separated list of instances to connect to. For example,
to use the instance metadata value named 'cloud-sql-instances' you would
provide 'instance/attributes/cloud-sql-instances'. Not compatible with -fuse`)
	useFuse = flag.Bool("fuse", false, `Mount a directory at 'dir' using FUSE for accessing instances. Note that the
directory at 'dir' must be empty before this program is started.`)
	fuseTmp = flag.String("fuse_tmp", defaultTmp, `Used as a temporary directory if -fuse is set. Note that files in this directory
can be removed automatically by this program.`)

	// Settings for limits
	maxConnections = flag.Uint64("max_connections", 0, `If provided, the maximum number of connections to establish before refusing new connections. Defaults to 0 (no limit)`)
	fdRlimit       = flag.Uint64("fd_rlimit", limits.ExpectedFDs, `Sets the rlimit on the number of open file descriptors for the proxy to the provided value. If set to zero, disables attempts to set the rlimit. Defaults to a value which can support 4K connections to one instance`)
	termTimeout    = flag.Duration("term_timeout", 0, "When set, the proxy will wait for existing connections to close before terminating. Any connections that haven't closed after the timeout will be dropped")

	// Settings for authentication.
	token     = flag.String("token", "", "When set, the proxy uses this Bearer token for authorization.")
	tokenFile = flag.String("credential_file", "", `If provided, this json file will be used to retrieve Service Account credentials.
You may set the GOOGLE_APPLICATION_CREDENTIALS environment variable for the same effect.`)
	ipAddressTypes = flag.String("ip_address_types", "PUBLIC,PRIVATE", "Default to be 'PUBLIC,PRIVATE'. Options: a list of strings separated by ',', e.g. 'PUBLIC,PRIVATE' ")

	skipInvalidInstanceConfigs = flag.Bool("skip_failed_instance_config", false, `Setting this flag will allow you to prevent the proxy from terminating when
	some instance configurations could not be parsed and/or are unavailable.`)

	// Setting to choose what API to connect to
	host = flag.String("host", "", "When set, the proxy uses this host as the base API path. Example: https://sqladmin.googleapis.com")
)

const (
	minimumRefreshCfgThrottle = time.Second

	port = 3307
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `
The Cloud SQL Proxy allows simple, secure connectivity to Google Cloud SQL. It
is a long-running process that opens local sockets (either TCP or Unix sockets)
according to the parameters passed to it. A local application connects to a
Cloud SQL instance by using the corresponding socket.


Authorization:
  * On Google Compute Engine, the default service account is used.
    The Cloud SQL API must be enabled for the VM.

  * When the gcloud command-line tool is installed on the local machine, the
    "active account" is used for authentication. Run 'gcloud auth list' to see
    which accounts are installed on your local machine and
    'gcloud config list account' to view the active account.

  * To configure the proxy using a service account, pass the -credential_file
    parameter or set the GOOGLE_APPLICATION_CREDENTIALS environment variable.
    This will override gcloud or GCE (Google Compute Engine) credentials,
    if they exist.

General:
  -quiet
    Disable log messages (e.g. when new connections are established).
    WARNING: this option disables ALL logging output (including connection
    errors), which will likely make debugging difficult. The -quiet flag takes
    precedence over the -verbose flag.
  -log_debug_stdout
    When explicitly set to true, verbose and info log messages will be directed
	to stdout as opposed to the default stderr.
  -verbose
    When explicitly set to false, disable log messages that are not errors nor
    first-time startup messages (e.g. when new connections are established).

Connection:
  -instances
    To connect to a specific list of instances, set the instances parameter
    to a comma-separated list of instance connection strings. For example:

           -instances=my-project:my-region:my-instance

    For connectivity over TCP, you must specify a tcp port as part of the
    instance string. For example, the following example opens a loopback TCP
    socket on port 3306, which will be proxied to connect to the instance
    'my-instance' in project 'my-project':

            -instances=my-project:my-region:my-instance=tcp:3306

     When connecting over TCP, the -instances parameter is required.

    To set a custom socket name, you can specify it as part of the instance
	string.  The following example opens a unix socket in the directory
	specified by -dir, which will be proxied to connect to the instance
    'my-instance' in project 'my-project':

            -instances=my-project:my-region:my-instance=unix:custom-socket-name

	To override the -dir parameter, specify an absolute path as shown in the
	following example:

            -instances=my-project:my-region:my-instance=unix:/my/custom/sql-socket

     Supplying INSTANCES environment variable achieves the same effect.  One can
     use that to keep k8s manifest files constant across multiple environments

  -instances_metadata
     When running on GCE (Google Compute Engine) you can avoid the need to
     specify the list of instances on the command line by using the Metadata
     server. This parameter specifies a path to a metadata value which is then
     interpreted as a list of instances in the exact same way as the -instances
     parameter. Updates to the metadata value will be observed and acted on by
     the Proxy.

  -projects
    To direct the proxy to allow connections to all instances in specific
    projects, set the projects parameter:

       -projects=my-project

  -fuse
    If your local environment has FUSE installed, you can specify the -fuse
    flag to avoid the requirement to specify instances in advance. With FUSE,
    any attempts to open a Unix socket in the directory specified by -dir
    automatically creates that socket and connects to the corresponding
    instance.

  -dir
    When using Unix sockets (the default for systems which support them), the
    Proxy places the sockets in the directory specified by the -dir parameter.

Automatic instance discovery:
   If the Google Cloud SQL is installed on the local machine and no instance
   connection flags are specified, the proxy connects to all instances in the
   gcloud tool's active project. Run 'gcloud config list project' to
   display the active project.


Information for all flags:
`)
		flag.VisitAll(func(f *flag.Flag) {
			usage := strings.Replace(f.Usage, "\n", "\n    ", -1)
			fmt.Fprintf(os.Stderr, "  -%s\n    %s\n\n", f.Name, usage)
		})
	}
}

var defaultTmp = filepath.Join(os.TempDir(), "cloudsql-proxy-tmp")

const defaultVersionString = "NO_VERSION_SET"

var versionString = defaultVersionString

// userAgentFromVersionString returns an appropriate user agent string for
// identifying this proxy process, or a blank string if versionString was not
// set to an interesting value.
func userAgentFromVersionString() string {
	if versionString == defaultVersionString {
		return ""
	}

	// Example versionString (see build.sh):
	//    version 1.05; sha 0f69d99588991aba0879df55f92562f7e79d7ca1 built Mon May  2 17:57:05 UTC 2016
	//
	// We just want the part before the semicolon.
	semi := strings.IndexByte(versionString, ';')
	if semi == -1 {
		return ""
	}
	return "cloud_sql_proxy " + versionString[:semi]
}

const accountErrorSuffix = `Please create a new VM with Cloud SQL access (scope) enabled under "Identity and API access". Alternatively, create a new "service account key" and specify it using the -credential_file parameter`

func checkFlags(onGCE bool) error {
	if !onGCE {
		if *instanceSrc != "" {
			return errors.New("-instances_metadata unsupported outside of Google Compute Engine")
		}
		return nil
	}

	if *token != "" || *tokenFile != "" || os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		return nil
	}

	// Check if gcloud credentials are available and if so, skip checking the GCE VM service account scope.
	_, err := util.GcloudConfig()
	if err == nil {
		return nil
	}

	scopes, err := metadata.Scopes("default")
	if err != nil {
		if _, ok := err.(metadata.NotDefinedError); ok {
			return errors.New("no service account found for this Compute Engine VM. " + accountErrorSuffix)
		}
		return fmt.Errorf("error checking scopes: %T %v | %+v", err, err, err)
	}

	ok := false
	for _, sc := range scopes {
		if sc == proxy.SQLScope || sc == "https://www.googleapis.com/auth/cloud-platform" {
			ok = true
			break
		}
	}
	if !ok {
		return errors.New(`the default Compute Engine service account is not configured with sufficient permissions to access the Cloud SQL API from this VM. ` + accountErrorSuffix)
	}
	return nil
}

func authenticatedClientFromPath(ctx context.Context, f string) (*http.Client, error) {
	all, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("invalid json file %q: %v", f, err)
	}
	// First try and load this as a service account config, which allows us to see the service account email:
	if cfg, err := goauth.JWTConfigFromJSON(all, proxy.SQLScope); err == nil {
		logging.Infof("using credential file for authentication; email=%s", cfg.Email)
		return cfg.Client(ctx), nil
	}

	cred, err := goauth.CredentialsFromJSON(ctx, all, proxy.SQLScope)
	if err != nil {
		return nil, fmt.Errorf("invalid json file %q: %v", f, err)
	}
	logging.Infof("using credential file for authentication; path=%q", f)
	return oauth2.NewClient(ctx, cred.TokenSource), nil
}

func authenticatedClient(ctx context.Context) (*http.Client, error) {
	if *tokenFile != "" {
		return authenticatedClientFromPath(ctx, *tokenFile)
	} else if tok := *token; tok != "" {
		src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
		return oauth2.NewClient(ctx, src), nil
	} else if f := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); f != "" {
		return authenticatedClientFromPath(ctx, f)
	}

	// If flags or env don't specify an auth source, try either gcloud or application default
	// credentials.
	src, err := util.GcloudTokenSource(ctx)
	if err != nil {
		src, err = goauth.DefaultTokenSource(ctx, proxy.SQLScope)
	}
	if err != nil {
		return nil, err
	}

	return oauth2.NewClient(ctx, src), nil
}

func stringList(s string) []string {
	spl := strings.Split(s, ",")
	if len(spl) == 1 && spl[0] == "" {
		return nil
	}
	return spl
}

func listInstances(ctx context.Context, cl *http.Client, projects []string) ([]string, error) {
	if len(projects) == 0 {
		// No projects requested.
		return nil, nil
	}

	sql, err := sqladmin.New(cl)
	if err != nil {
		return nil, err
	}
	if *host != "" {
		sql.BasePath = *host
	}

	ch := make(chan string)
	var wg sync.WaitGroup
	wg.Add(len(projects))
	for _, proj := range projects {
		proj := proj
		go func() {
			err := sql.Instances.List(proj).Pages(ctx, func(r *sqladmin.InstancesListResponse) error {
				for _, in := range r.Items {
					// The Proxy is only support on Second Gen
					if in.BackendType == "SECOND_GEN" {
						ch <- fmt.Sprintf("%s:%s:%s", in.Project, in.Region, in.Name)
					}
				}
				return nil
			})
			if err != nil {
				logging.Errorf("Error listing instances in %v: %v", proj, err)
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	var ret []string
	for x := range ch {
		ret = append(ret, x)
	}
	if len(ret) == 0 {
		return nil, fmt.Errorf("no Cloud SQL Instances found in these projects: %v", projects)
	}
	return ret, nil
}

func gcloudProject() ([]string, error) {
	cfg, err := util.GcloudConfig()
	if err != nil {
		return nil, err
	}
	if cfg.Configuration.Properties.Core.Project == "" {
		return nil, fmt.Errorf("gcloud has no active project, you can set it by running `gcloud config set project <project>`")
	}
	return []string{cfg.Configuration.Properties.Core.Project}, nil
}

// Main executes the main function of the proxy, allowing it to be called from tests.
//
// Setting timeout to a value greater than 0 causes the process to panic after
// that amount of time. This is to sidestep an issue where sending a Signal to
// the process (via the SSH library) doesn't seem to have an effect, and
// closing the SSH session causes the process to get leaked. This timeout will
// at least cause the proxy to exit eventually.
func Main(timeout time.Duration) {
	if timeout > 0 {
		go func() {
			time.Sleep(timeout)
			panic("timeout exceeded")
		}()
	}
	main()
}

func main() {
	flag.Parse()

	if *version {
		fmt.Println("Cloud SQL Proxy:", versionString)
		return
	}

	if *logDebugStdout {
		logging.LogDebugToStdout()
	}

	if !*verbose {
		logging.LogVerboseToNowhere()
	}

	if *quiet {
		log.Println("Cloud SQL Proxy logging has been disabled by the -quiet flag. All messages (including errors) will be suppressed.")
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	// Split the input ipAddressTypes to the slice of string
	ipAddrTypeOptsInput := strings.Split(*ipAddressTypes, ",")

	if *fdRlimit != 0 {
		if err := limits.SetupFDLimits(*fdRlimit); err != nil {
			logging.Infof("failed to setup file descriptor limits: %v", err)
		}
	}

	if *host != "" && !strings.HasSuffix(*host, "/") {
		logging.Errorf("Flag host should always end with /")
		flag.PrintDefaults()
		return
	}

	// TODO: needs a better place for consolidation
	// if instances is blank and env var INSTANCES is supplied use it
	if envInstances := os.Getenv("INSTANCES"); *instances == "" && envInstances != "" {
		*instances = envInstances
	}

	instList := stringList(*instances)
	projList := stringList(*projects)
	// TODO: it'd be really great to consolidate flag verification in one place.
	if len(instList) == 0 && *instanceSrc == "" && len(projList) == 0 && !*useFuse {
		var err error
		projList, err = gcloudProject()
		if err == nil {
			logging.Infof("Using gcloud's active project: %v", projList)
		} else if gErr, ok := err.(*util.GcloudError); ok && gErr.Status == util.GcloudNotFound {
			log.Fatalf("gcloud is not in the path and -instances and -projects are empty")
		} else {
			log.Fatalf("unable to retrieve the active gcloud project and -instances and -projects are empty: %v", err)
		}
	}

	onGCE := metadata.OnGCE()
	if err := checkFlags(onGCE); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	client, err := authenticatedClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	ins, err := listInstances(ctx, client, projList)
	if err != nil {
		log.Fatal(err)
	}
	instList = append(instList, ins...)
	cfgs, err := CreateInstanceConfigs(*dir, *useFuse, instList, *instanceSrc, client, *skipInvalidInstanceConfigs)
	if err != nil {
		log.Fatal(err)
	}

	// We only need to store connections in a ConnSet if FUSE is used; otherwise
	// it is not efficient to do so.
	var connset *proxy.ConnSet

	// Initialize a source of new connections to Cloud SQL instances.
	var connSrc <-chan proxy.Conn
	if *useFuse {
		connset = proxy.NewConnSet()
		c, fuse, err := fuse.NewConnSrc(*dir, *fuseTmp, connset)
		if err != nil {
			log.Fatalf("Could not start fuse directory at %q: %v", *dir, err)
		}
		connSrc = c
		defer fuse.Close()
	} else {
		updates := make(chan string)
		if *instanceSrc != "" {
			go func() {
				for {
					err := metadata.Subscribe(*instanceSrc, func(v string, ok bool) error {
						if ok {
							updates <- v
						}
						return nil
					})
					if err != nil {
						logging.Errorf("Error on receiving new instances from metadata: %v", err)
					}
					time.Sleep(5 * time.Second)
				}
			}()
		}

		c, err := WatchInstances(*dir, cfgs, updates, client)
		if err != nil {
			log.Fatal(err)
		}
		connSrc = c
	}

	refreshCfgThrottle := *refreshCfgThrottle
	if refreshCfgThrottle < minimumRefreshCfgThrottle {
		refreshCfgThrottle = minimumRefreshCfgThrottle
	}
	logging.Infof("Ready for new connections")

	proxyClient := &proxy.Client{
		Port:           port,
		MaxConnections: *maxConnections,
		Certs: certs.NewCertSourceOpts(client, certs.RemoteOpts{
			APIBasePath:    *host,
			IgnoreRegion:   !*checkRegion,
			UserAgent:      userAgentFromVersionString(),
			IPAddrTypeOpts: ipAddrTypeOptsInput,
		}),
		Conns:              connset,
		RefreshCfgThrottle: refreshCfgThrottle,
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-signals
		logging.Infof("Received TERM signal. Waiting up to %s before terminating.", *termTimeout)

		err := proxyClient.Shutdown(*termTimeout)

		if err == nil {
			os.Exit(0)
		}
		os.Exit(2)
	}()

	proxyClient.Run(connSrc)
}

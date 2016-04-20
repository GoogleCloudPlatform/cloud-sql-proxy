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
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/certs"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/fuse"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	goauth "golang.org/x/oauth2/google"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	"google.golang.org/cloud/compute/metadata"
)

var (
	version = flag.Bool("version", false, "Print the version of the proxy and exit")

	checkRegion = flag.Bool("check_region", false, `If specified, the 'region' portion of the connection string is required for
UNIX socket-based connections.`)

	// Settings for how to choose which instance to connect to.
	dir      = flag.String("dir", "", "Directory to use for placing UNIX sockets representing database instances")
	projects = flag.String("projects", "", `Open sockets for each Cloud SQL Instance in the projects specified
(comma-separated list)`)
	instances = flag.String("instances", "", `Comma-separated list of fully qualified instances (project:region:name)
to connect to. If the name has the suffix '=tcp:port', a TCP server is opened
on the specified port to proxy to that instance. Otherwise, one socket file per
instance is opened in 'dir'. Not compatible with -fuse`)
	instanceSrc = flag.String("instances_metadata", "", `If provided, it is treated as a path to a metadata value which
is polled for a comma-separated list of instances to connect to. For example,
to use the instance metadata value named 'cloud-sql-instances' you would
provide 'instance/attributes/cloud-sql-instances'. Not compatible with -fuse`)
	useFuse = flag.Bool("fuse", false, `Mount a directory at 'dir' using FUSE for accessing instances. Note that the
directory at 'dir' must be empty before this program is started.`)
	fuseTmp = flag.String("fuse_tmp", defaultTmp, `Used as a temporary directory if -fuse is set. Note that files in this directory
can be removed automatically by this program.`)

	// Settings for authentication.
	token     = flag.String("token", "", "When set, the proxy uses this Bearer token for authorization.")
	tokenFile = flag.String("credential_file", "", `If provided, this json file will be used to retrieve Service Account credentials.
You may set the GOOGLE_APPLICATION_CREDENTIALS environment variable for the same effect.`)
)

const (
	host = "https://www.googleapis.com/sql/v1beta4/"
	port = 3307
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `
The Cloud SQL Proxy allows simple, secure connectivity to Google Cloud SQL. It
is a long-running process that opens local sockets (either TCP or UNIX sockets)
according to the flags passed to it. A local application connects to a Cloud
SQL instance by using the corresponding socket.


Authorization:
  * On Google Compute Engine, the default service account is used.
    The Cloud SQL API must be enabled for the VM.

  * When gcloud is installed on the local machine, the "active account" is used
    for authentication. Run 'gcloud auth list' to see which accounts are
    installed on your local machine and 'gcloud config list account' to view
    the active account.

  * To configure the proxy using a service account, pass the -credential_file
    flag or set the GOOGLE_APPLICATION_CREDENTIALS environment variable. This
    will override gcloud or GCE credentials (if they exist).


Connection:
  -instances
    To connect to a specific list of instances, set the instances flag to a
    comma-separated list of instance connection strings. For example:

           -instances my-project:my-region:my-instance

    For connectivity over TCP, you must specify a tcp port as part of the
    instance string. For example, the following example opens a loopback TCP
    socket on port 3306, which will be proxied to connect to the instance
    'my-instance' in project 'my-project':

            -instances my-project:my-region:my-instance=tcp:3306

     When connecting over TCP, the -instances flag is required.

  -projects
    To direct the proxy to connect to all instances in a specific project, set
    the projects flag:

       -projects my-project

  -fuse
    If your local environment has FUSE installed, you can specify the -fuse
    flag to avoid the requirement to specify instances in advance. With FUSE,
    any attempts to open a UNIX socket in the directory specified by -dir
    automatically creates that socket and connects to the corresponding
    instance.

  -dir
    When using UNIX sockets (the default for systems which support them), the
    Proxy places the sockets in the directory specified by the -dir flag.


Automatic instance discovery:
    If gcloud is installed on the local machine and no instance connection flags
    are specified, the proxy connects to all instances in the gcloud active
    project, Run 'gcloud config list project' to display the active project.


Information for all flags:
`)
		flag.VisitAll(func(f *flag.Flag) {
			usage := strings.Replace(f.Usage, "\n", "\n    ", -1)
			fmt.Fprintf(os.Stderr, "  -%s\n    %s\n\n", f.Name, usage)
		})
	}
}

const SQLScope = "https://www.googleapis.com/auth/sqlservice.admin"

var defaultTmp = filepath.Join(os.TempDir(), "cloudsql-proxy-tmp")

// See https://github.com/GoogleCloudPlatform/gcloud-golang/issues/194
func onGCE() bool {
	res, err := http.Get("http://metadata.google.internal")
	if err != nil {
		return false
	}
	return res.Header.Get("Metadata-Flavor") == "Google"
}

var versionString = "NO_VERSION_SET"

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

	scopes, err := metadata.Scopes("default")
	if err != nil {
		if _, ok := err.(metadata.NotDefinedError); ok {
			return errors.New("no service account found for this Compute Engine VM. " + accountErrorSuffix)
		}
		return fmt.Errorf("error checking scopes: %T %v | %+v", err, err, err)
	}

	ok := false
	for _, sc := range scopes {
		if sc == SQLScope || sc == "https://www.googleapis.com/auth/cloud-platform" {
			ok = true
			break
		}
	}
	if !ok {
		return errors.New(`the default Compute Engine service account is not configured with sufficient permissions to access the Cloud SQL API from this VM. ` + accountErrorSuffix)
	}
	return nil
}

func authenticatedClient(ctx context.Context) (*http.Client, error) {
	if f := *tokenFile; f != "" {
		all, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("invalid json file %q: %v", f, err)
		}
		cfg, err := goauth.JWTConfigFromJSON(all, SQLScope)
		if err != nil {
			return nil, fmt.Errorf("invalid json file %q: %v", f, err)
		}
		return cfg.Client(ctx), nil
	} else if tok := *token; tok != "" {
		src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
		return oauth2.NewClient(ctx, src), nil
	}

	return goauth.DefaultClient(ctx, SQLScope)
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
				log.Printf("Error listing instances in %v: %v", proj, err)
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

func gcloudProject() []string {
	buf := new(bytes.Buffer)
	cmd := exec.Command("gcloud", "--format", "json", "config", "list", "core/project")
	cmd.Stdout = buf

	if err := cmd.Run(); err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			// gcloud not installed; ignore the error
			return nil
		}
		log.Printf("Error detecting gcloud project: %v", err)
		return nil
	}

	var data struct {
		Core struct {
			Project string
		}
	}

	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		log.Printf("Failed to unmarshal bytes from gcloud: %v", err)
		log.Printf("   gcloud returned:\n%s", buf)
		return nil
	}

	log.Printf("Using gcloud's active project: %v", data.Core.Project)
	return []string{data.Core.Project}
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

	instList := stringList(*instances)
	projList := stringList(*projects)
	// TODO: it'd be really great to consolidate flag verification in one place.
	if len(instList) == 0 && *instanceSrc == "" && len(projList) == 0 && !*useFuse {
		projList = gcloudProject()
	}

	onGCE := onGCE()
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

	cfgs, err := CreateInstanceConfigs(*dir, *useFuse, instList, *instanceSrc)
	if err != nil {
		log.Fatal(err)
	}

	// All active connections are stored in this variable.
	connset := proxy.NewConnSet()

	// Initialize a source of new connections to Cloud SQL instances.
	var connSrc <-chan proxy.Conn
	if *useFuse {
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
						log.Print(err)
					}
					time.Sleep(5 * time.Second)
				}
			}()
		}

		c, err := WatchInstances(*dir, cfgs, updates)
		if err != nil {
			log.Fatal(err)
		}
		connSrc = c
	}

	log.Print("Ready for new connections")

	(&proxy.Client{
		Port:  port,
		Certs: certs.NewCertSource(host, client, *checkRegion),
		Conns: connset,
	}).Run(connSrc)
}

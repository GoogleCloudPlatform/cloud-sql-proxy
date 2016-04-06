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

	host = flag.String("host", "https://www.googleapis.com/sql/v1beta4/", "API endpoint to use")

	port = flag.Int("remote_port", 3307, "Port to use when connecting to instances")

	checkRegion = flag.Bool("check_region", false, "If specified, the 'region' portion of the connection string is required for connections. If this is false and a region is not specified only a log is printed.")

	// Settings for how to choose which instance to connect to.
	dir         = flag.String("dir", "", "Directory to use for placing UNIX sockets representing database instances")
	projects    = flag.String("projects", "", "Open sockets for each Cloud SQL Instance in the projects specified here (comma-separated list)")
	instances   = flag.String("instances", "", "Comma-separated list of fully qualified instances (project:region:name) to connect to. If the name has the suffix '=tcp:port', a TCP server is opened on the specified port to proxy to that instance. Otherwise, one socket file per instance is opened in 'dir'; ignored if -fuse is set")
	instanceSrc = flag.String("instances_metadata", "", "If provided, it is treated as a path to a metadata value which is polled for a comma-separated list of instances to connect to; ignored if -fuse is set. For example, to use the instance metadata value named 'cloud-sql-instances' you would provide 'instance/attributes/cloud-sql-instances'.")
	useFuse     = flag.Bool("fuse", false, "Mount a directory at 'dir' using FUSE for accessing instances. Note that the directory at 'dir' must be empty before this program is started.")
	fuseTmp     = flag.String("fuse_tmp", defaultTmp, "Used as a temporary directory if -fuse is set. Note that files in this directory can be removed automatically by this program.")

	// Settings for authentication.
	token     = flag.String("token", "", "By default, requests are authorized under the identity of the default service account. Setting this flag causes requests to include this Bearer token instead.")
	tokenFile = flag.String("credential_file", "", "If provided, this json file will be used to retrieve Service Account credentials; you may also set the GOOGLE_APPLICATION_CREDENTIALS environment variable to avoid the need to pass this flag.")
)

const sqlScope = "https://www.googleapis.com/auth/sqlservice.admin"

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

func checkFlags(onGCE bool) error {
	if !onGCE {
		if *instanceSrc != "" {
			return errors.New("-instances_metadata unsupported outside of Google Compute Engine")
		}
		return nil
	}
	scopes, err := metadata.Scopes("default")
	if err != nil {
		return fmt.Errorf("error checking scopes: %v", err)
	}

	ok := false
	for _, sc := range scopes {
		if sc == "https://www.googleapis.com/auth/sqladmin" || sc == "https://www.googleapis.com/auth/cloud-platform" {
			ok = true
			break
		}
	}
	if !ok {
		return errors.New(`the default Compute Engine service account is not configured with sufficient permissions to access the Cloud SQL API from this VM. Please create a new VM with Cloud SQL access (scope) enabled under "Identity and API access". Alternatively, create a new "service account key" and specify it using the -credentials_file parameter`)
	}
	return nil
}

func authenticatedClient(ctx context.Context) (*http.Client, error) {
	if f := *tokenFile; f != "" {
		all, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("invalid json file %q: %v", f, err)
		}
		cfg, err := goauth.JWTConfigFromJSON(all, sqlScope)
		if err != nil {
			return nil, fmt.Errorf("invalid json file %q: %v", f, err)
		}
		return cfg.Client(ctx), nil
	} else if tok := *token; tok != "" {
		src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
		return oauth2.NewClient(ctx, src), nil
	}

	return goauth.DefaultClient(ctx, sqlScope)
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
	return []string{data.Core.Project}
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

	if *dir != "" {
		log.Print("Socket prefix: " + *dir)
	}

	src, err := certs.NewCertSource(*host, client, *checkRegion)
	if err != nil {
		log.Fatal(err)
	}

	(&proxy.Client{
		Port:  *port,
		Certs: src,
		Conns: connset,
	}).Run(connSrc)
}

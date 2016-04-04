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
	"path/filepath"
	"strings"
	"time"

	"github.com/Carrotman42/cloudsql-proxy/gcp/auth"
	"github.com/Carrotman42/cloudsql-proxy/proxy/certs"
	"github.com/Carrotman42/cloudsql-proxy/proxy/fuse"
	"github.com/Carrotman42/cloudsql-proxy/proxy/proxy"

	"golang.org/x/net/context"
	goauth "golang.org/x/oauth2/google"
	"google.golang.org/cloud/compute/metadata"
)

var (
	version = flag.Bool("version", false, "Print the version of the proxy and exit")

	host = flag.String("host", "https://www.googleapis.com/sql/v1beta4/", "API endpoint to use")

	port = flag.Int("remote_port", 3307, "Port to use when connecting to instances")

	checkRegion = flag.Bool("check_region", false, "If specified, the 'region' portion of the connection string is required for connections. If this is false and a region is not specified only a log is printed.")

	// Settings for how to choose which instance to connect to.
	dir         = flag.String("dir", "", "Directory to use for placing instance sockets. Exactly what ends up in this directory depends on other flags.")
	instances   = flag.String("instances", "", "Comma-separated list of fully qualified instances (project:name) to connect to. If the name has the suffix '=tcp:port', a TCP server is opened on the specified port to proxy to that instance. Otherwise, one socket file per instance is opened in 'dir'; ignored if -fuse is set")
	instanceSrc = flag.String("instances_metadata", "", "If provided, it is treated as a path to a metadata value which is polled for a comma-separated list of instances to connect to; ignored if -fuse is set. For example, to use the instance metadata value named 'cloud-sql-instances' you would provide 'instance/attributes/cloud-sql-instances'.")
	useFuse     = flag.Bool("fuse", false, "Mount a directory at 'dir' using FUSE for accessing instances. Note that the directory at 'dir' must be empty before this program is started.")
	fuseTmp     = flag.String("fuse_tmp", defaultTmp, "Used as a temporary directory if -fuse is set. Note that files in this directory can be removed automatically by this program.")

	// Settings for authentication.
	token     = flag.String("token", "", "By default, requests are authorized under the identity of the default service account. Setting this flag causes requests to include this Bearer token instead.")
	tokenFile = flag.String("credential_file", "", "If provided, this json file will be used to retrieve Service Account credentials; you may also set the GOOGLE_APPLICATION_CREDENTIALS environment variable to avoid the need to pass this flag.")
)

const sqlScope = "https://www.googleapis.com/auth/sqlservice.admin"

var defaultTmp = filepath.Join(os.TempDir(), "cloudsql-proxy-tmp")

// See https://github.com/Carrotman42/gcloud-golang/issues/194
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

func main() {
	flag.Parse()

	if *version {
		fmt.Println("Cloud SQL Proxy:", versionString)
		return
	}

	onGCE := onGCE()
	if err := checkFlags(onGCE); err != nil {
		log.Fatal(err)
	}

	credentialFile := *tokenFile
	// Use the environment variable only if the flag hasn't been set.
	if credentialFile == "" {
		credentialFile = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}

	var client *http.Client
	if credentialFile != "" {
		all, err := ioutil.ReadFile(credentialFile)
		if err != nil {
			log.Fatalf("invalid json file %q: %v", credentialFile, err)
		}
		cfg, err := goauth.JWTConfigFromJSON(all, sqlScope)
		if err != nil {
			log.Fatalf("invalid json file %q: %v", credentialFile, err)
		}
		client = auth.NewClientFrom(cfg.TokenSource(context.Background()))
	} else if *token != "" || onGCE {
		// Passing token == "" causes the GCE metadata server to be used.
		client = auth.NewAuthenticatedClient(*token)
	} else {
		log.Fatal("No authentication method available! When not running on Google Compute Engine, provide the -credential_file flag.")
	}

	cfgs, err := CreateInstanceConfigs(*dir, *useFuse, strings.Split(*instances, ","), *instanceSrc)
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

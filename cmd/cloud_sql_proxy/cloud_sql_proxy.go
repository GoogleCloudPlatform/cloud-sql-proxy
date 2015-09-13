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
	"os"
	"path/filepath"
	"strings"

	"gcp/auth"
	"gcp/metadata"
	"proxy/certs"
	"proxy/fuse"
	"proxy/proxy"

	"flag"
	
	"log"
)

var (
	host = flag.String("host", "https://www.googleapis.com/sql/v1beta4/", "API endpoint to use")

	port = flag.Int("remote_port", 3307, "Port to use when connecting to instances")

	// Settings for how to choose which instance to connect to.
	dir         = flag.String("dir", "", "Directory to use for placing instance sockets. Exactly what ends up in this directory depends on other flags.")
	instances   = flag.String("instances", "", "Comma-separated list of fully qualified instances (project:name) to connect to. If the name has the suffix '=tcp:port', a TCP server is opened on the specified port to proxy to that instance. Otherwise, one socket file per instance is opened in 'dir'; ignored if -fuse is set")
	instanceSrc = flag.String("instances_metadata", "", "If provided, it is treated as a path to a metadata value which is polled for a comma-separated list of instances to connect to; ignored if -fuse is set. For example, to use the instance metadata value named 'cloud-sql-instances' you would provide 'instance/attributes/cloud-sql-instances'.")
	useFuse     = flag.Bool("fuse", false, "Mount a directory at 'dir' using FUSE for accessing instances. Note that the directory at 'dir' must be empty before this program is started.")
	fuseTmp     = flag.String("fuse_tmp", defaultTmp, "Used as a temporary directory if -fuse is set. Note that files in this directory can be removed automatically by this program.")

	// Settings for authentication.
	token = flag.String("token", "", "By default, requests are authorized under the identity of the default service account. Setting this flag causes requests to include this Bearer token instead.")
)

var defaultTmp = filepath.Join(os.TempDir(), "cloudsql-proxy-tmp")

func main() {
	flag.Parse()

	instances := strings.Split(*instances, ",")
	if len(instances) == 1 && instances[0] == "" {
		instances = nil
	}
	if err := Check(*dir, *useFuse, instances, *instanceSrc); err != nil {
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
		var updates <-chan string
		if *instanceSrc != "" {
			c, err := metadata.Subscribe(*instanceSrc)
			if err != nil {
				log.Fatal(err)
			}
			updates = c
		}

		c, err := WatchInstances(*dir, instances, updates)
		if err != nil {
			log.Fatal(err)
		}
		connSrc = c
	}

	// Passing token == "" causes the GCE metadata server to be used.
	client := auth.NewAuthenticatedClient(*token)

	log.Print("Socket prefix: " + *dir)

	proxy.Client{
		Port:  *port,
		Certs: certs.NewCertSource(*host, client),
		Conns: connset,
	}.Run(connSrc)
}

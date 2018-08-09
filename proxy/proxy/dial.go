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

package proxy

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/certs"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
)

const port = 3307

var dialClient struct {
	// This client is initialized in Init/InitClient/InitDefault and read in Dial.
	c *Client
	sync.Mutex
}

// Dial returns a net.Conn connected to the Cloud SQL Instance specified. The
// format of 'instance' is "project-name:region:instance-name".
//
// If one of the Init functions hasn't been called yet, InitDefault is called.
//
// This is a network-level function; consider looking in the dialers
// subdirectory for more convenience functions related to actually logging into
// your database.
func Dial(instance string) (net.Conn, error) {
	dialClient.Lock()
	c := dialClient.c
	dialClient.Unlock()
	if c == nil {
		if err := InitDefault(context.Background()); err != nil {
			return nil, fmt.Errorf("default proxy initialization failed; consider calling proxy.Init explicitly: %v", err)
		}
		// InitDefault initialized the client.
		dialClient.Lock()
		c = dialClient.c
		dialClient.Unlock()
	}

	return c.Dial(instance)
}

// Dialer is a convenience type to model the standard 'Dial' function.
type Dialer func(net, addr string) (net.Conn, error)

// Init must be called before Dial is called. This is a more flexible version
// of InitDefault, but allows you to set more fields.
//
// The http.Client is used to authenticate API requests.
// The connset parameter is optional.
// If the dialer is nil, net.Conn is used.
func Init(auth *http.Client, connset *ConnSet, dialer Dialer) {
	dialClient.Lock()
	dialClient.c = &Client{
		Port:   port,
		Certs:  certs.NewCertSource("https://www.googleapis.com/sql/v1beta4/", auth, true),
		Conns:  connset,
		Dialer: dialer,
	}
	dialClient.Unlock()
}

// InitClient is similar to Init, but allows you to specify the Client
// directly.
func InitClient(c Client) {
	dialClient.Lock()
	dialClient.c = &c
	dialClient.Unlock()
}

// InitDefault attempts to initialize the Dial function using application
// default credentials.
func InitDefault(ctx context.Context) error {
	cl, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/sqlservice.admin")
	if err != nil {
		return err
	}
	Init(cl, nil, nil)
	return nil
}

// Copyright 2022 Google LLC
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
	"context"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
)

var errFUSENotSupported = errors.New("FUSE is not supported on Windows")

// UnixAddress returns the Unix socket for a given instance in the provided
// directory, by replacing all colons in the instance's name with periods.
func UnixAddress(dir, inst string) string {
	inst2 := strings.ReplaceAll(inst, ":", ".")
	return filepath.Join(dir, inst2)
}

// socketMount is a tcp/unix socket that listens for a Cloud SQL instance.
type socketMount struct {
	inst     string
	listener net.Listener
	dialOpts []cloudsqlconn.DialOption
}

// Client proxies connections from a local client to the remote server side
// proxy for multiple Cloud SQL instances.
type Client struct {
	// connCount tracks the number of all open connections from the Client to
	// all Cloud SQL instances.
	connCount uint64

	// maxConns is the maximum number of allowed connections tracked by
	// connCount. If not set, there is no limit.
	maxConns uint64

	dialer cloudsql.Dialer

	// mnts is a list of all mounted sockets for this client
	mnts []*socketMount

	// waitOnClose is the maximum duration to wait for open connections to close
	// when shutting down.
	waitOnClose time.Duration

	logger cloudsql.Logger

	// fuseDir is never set on Windows. All FUSE-related behavior is enabled
	// when this value is set on Linux and Darwin versions.
	fuseDir string
}

func configureFUSE(c *Client, conf *Config) (*Client, error)         { return nil, errFUSENotSupported }
func (c *Client) fuseMounts() []*socketMount                         { return nil }
func (c *Client) serveFuse(ctx context.Context, notify func()) error { return errFUSENotSupported }
func (c *Client) unmountFUSEMounts(_ MultiErr) MultiErr              { return nil }
func (c *Client) waitForFUSEMounts()                                 {}

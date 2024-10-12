// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import "github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"

// Option is a function that configures a Command.
type Option func(*Command)

// WithLogger overrides the default logger.
func WithLogger(l cloudsql.Logger) Option {
	return func(c *Command) {
		c.logger = l
	}
}

// WithDialer configures the Command to use the provided dialer to connect to
// Cloud SQL instances.
func WithDialer(d cloudsql.Dialer) Option {
	return func(c *Command) {
		c.dialer = d
	}
}

// WithFuseDir mounts a directory at the path using FUSE to access Cloud SQL
// instances.
func WithFuseDir(dir string) Option {
	return func(c *Command) {
		c.conf.FUSEDir = dir
	}
}

// WithFuseTempDir sets the temp directory where Unix sockets are created with
// FUSE
func WithFuseTempDir(dir string) Option {
	return func(c *Command) {
		c.conf.FUSETempDir = dir
	}
}

// WithMaxConnections sets the maximum allowed number of connections. Default
// is no limit.
func WithMaxConnections(max uint64) Option {
	return func(c *Command) {
		c.conf.MaxConnections = max
	}
}

// WithUserAgent sets additional user agents for Admin API tracking and should
// be a space separated list of additional user agents, e.g.
// cloud-sql-proxy-operator/0.0.1,other-agent/1.0.0
func WithUserAgent(agent string) Option {
	return func(c *Command) {
		c.conf.OtherUserAgents = agent
	}
}

// WithAutoIP enables legacy behavior of v1 and will try to connect to first IP
// address returned by the SQL Admin API. In most cases, this flag should not
// be used. Prefer default of public IP or use --private-ip instead.`
func WithAutoIP() Option {
	return func(c *Command) {
		c.conf.AutoIP = true
	}
}

// WithQuietLogging configures the Proxy to log error messages only.
func WithQuietLogging() Option {
	return func(c *Command) {
		c.conf.Quiet = true
	}
}

// WithDebugLogging configures the Proxy to log debug level messages.
func WithDebugLogging() Option {
	return func(c *Command) {
		c.conf.DebugLogs = true
	}
}

// WithLazyRefresh configures the Proxy to refresh connection info on an
// as-needed basis when the cached copy has expired.
func WithLazyRefresh() Option {
	return func(c *Command) {
		c.conf.LazyRefresh = true
	}
}

// WithConnRefuseNotify configures the Proxy to start a goroutine and run the
// given notify callback function in the event of a connection refuse.
func WithConnRefuseNotify(n func(string)) Option {
       return func(c *Command) {
               c.connRefuseNotify = n
       }
}

// Copyright 2021 Google LLC
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

package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/cloudsql"
	"github.com/spf13/cobra"
)

// InstanceConnConfig holds the configuration for an individual instance
// connection.
type InstanceConnConfig struct {
	// Name is the instance connection name.
	Name string
	// Addr is the address on which to bind a listener for the instance.
	Addr string
	// Port is the port on which to bind a listener for the instance.
	Port int
	// UnixSocket is the directory where a Unix socket will be created,
	// connected to the Cloud SQL instance. If set, takes precedence over Addr
	// and Port.
	UnixSocket string
	// IAMAuthN enables automatic IAM DB Authentication for the instance.
	// Postgres-only. If it is nil, the value was not specified.
	IAMAuthN *bool
}

// Config contains all the configuration provided by the caller.
type Config struct {
	// Token is the Bearer token used for authorization.
	Token string

	// CredentialsFile is the path to a service account key.
	CredentialsFile string

	// GcloudAuth set whether to use Gcloud's config helper to retrieve a
	// token for authentication.
	GcloudAuth bool

	// Addr is the address on which to bind all instances.
	Addr string

	// Port is the initial port to bind to. Subsequent instances bind to
	// increments from this value.
	Port int

	// UnixSocket is the directory where Unix sockets will be created,
	// connected to any Instances. If set, takes precedence over Addr and Port.
	UnixSocket string

	// IAMAuthN enables automatic IAM DB Authentication for all instances.
	// Postgres-only.
	IAMAuthN bool

	// MaxConnections are the maximum number of connections the Client may
	// establish to the Cloud SQL server side proxy before refusing additional
	// connections. A zero-value indicates no limit.
	MaxConnections uint32

	// WaitOnClose sets the duration to wait for connections to close before
	// shutting down. Not setting this field means to close immediately
	// regardless of any open connections.
	WaitOnClose time.Duration

	// Instances are configuration for individual instances. Instance
	// configuration takes precedence over global configuration.
	Instances []InstanceConnConfig

	// Dialer specifies the dialer to use when connecting to Cloud SQL
	// instances.
	Dialer cloudsql.Dialer

	// DialerOpts specifies the opts to use when creating a new dialer. This
	// value is ignored when a Dialer has been set.
	DialerOpts []cloudsqlconn.Option
}

type portConfig struct {
	global    int
	postgres  int
	mysql     int
	sqlserver int
}

func newPortConfig(global int) *portConfig {
	return &portConfig{
		global:    global,
		postgres:  5432,
		mysql:     3306,
		sqlserver: 1433,
	}
}

// nextPort returns the next port based on the initial global value.
func (c *portConfig) nextPort() int {
	p := c.global
	c.global++
	return p
}

func (c *portConfig) nextDBPort(version string) int {
	switch {
	case strings.HasPrefix(version, "MYSQL"):
		p := c.mysql
		c.mysql++
		return p
	case strings.HasPrefix(version, "POSTGRES"):
		p := c.postgres
		c.postgres++
		return p
	case strings.HasPrefix(version, "SQLSERVER"):
		p := c.sqlserver
		c.sqlserver++
		return p
	default:
		// Unexpected engine version, use global port setting instead.
		return c.nextPort()
	}
}

// Client proxies connections from a local client to the remote server side
// proxy for multiple Cloud SQL instances.
type Client struct {
	cmd    *cobra.Command
	dialer cloudsql.Dialer

	// mnts is a list of all mounted sockets for this client
	mnts []*socketMount

	// connCount tracks the number of all open connections from the Client to
	// all Cloud SQL instances.
	connCount uint32

	// maxConns is the maximum number of allowed connections tracked by
	// connCount. If not set, there is no limit.
	maxConns uint32

	// waitOnClose is the maximum duration to wait for open connections to close
	// when shutting down.
	waitOnClose time.Duration
}

// NewClient completes the initial setup required to get the proxy to a "steady" state.
func NewClient(ctx context.Context, d cloudsql.Dialer, cmd *cobra.Command, conf *Config) (*Client, error) {
	var mnts []*socketMount
	for _, inst := range conf.Instances {
		// Initiate refresh operation
		go func(name string) { d.EngineVersion(ctx, name) }(inst.Name)
	}
	pc := newPortConfig(conf.Port)
	for _, inst := range conf.Instances {
		version, err := d.EngineVersion(ctx, inst.Name)
		if err != nil {
			return nil, err
		}

		var (
			// network is one of "tcp" or "unix"
			network string
			// address is either a TCP host port, or a Unix socket
			address string
		)
		// IF
		//   a global Unix socket directory is NOT set AND
		//   an instance-level Unix socket is NOT set
		//   (e.g.,  I didn't set a Unix socket globally or for this instance)
		// OR
		//   an instance-level TCP address or port IS set
		//   (e.g., I'm overriding any global settings to use TCP for this
		//   instance)
		// use a TCP listener.
		// Otherwise, use a Unix socket.
		if (conf.UnixSocket == "" && inst.UnixSocket == "") ||
			(inst.Addr != "" || inst.Port != 0) {
			network = "tcp"

			a := conf.Addr
			if inst.Addr != "" {
				a = inst.Addr
			}

			var np int
			switch {
			case inst.Port != 0:
				np = inst.Port
			case conf.Port != 0:
				np = pc.nextPort()
			default:
				np = pc.nextDBPort(version)
			}

			address = net.JoinHostPort(a, fmt.Sprint(np))
		} else {
			network = "unix"

			dir := conf.UnixSocket
			if dir == "" {
				dir = inst.UnixSocket
			}
			if _, err := os.Stat(dir); err != nil {
				return nil, err
			}
			address = UnixAddress(dir, inst.Name)
			// When setting up a listener for Postgres, create address as a
			// directory, and use the Postgres-specific socket name
			// .s.PGSQL.5432.
			if strings.HasPrefix(version, "POSTGRES") {
				// Make the directory only if it hasn't already been created.
				if _, err := os.Stat(address); err != nil {
					if err = os.Mkdir(address, 0777); err != nil {
						return nil, err
					}
				}
				address = UnixAddress(address, ".s.PGSQL.5432")
			}
		}

		var opts []cloudsqlconn.DialOption
		if inst.IAMAuthN != nil {
			opts = append(opts, cloudsqlconn.WithDialIAMAuthN(*inst.IAMAuthN))
		}
		m := &socketMount{inst: inst.Name, dialOpts: opts}
		addr, err := m.listen(ctx, network, address)
		if err != nil {
			for _, m := range mnts {
				m.close()
			}
			return nil, fmt.Errorf("[%v] Unable to mount socket: %v", inst.Name, err)
		}

		cmd.Printf("[%s] Listening on %s\n", inst.Name, addr.String())
		mnts = append(mnts, m)
	}
	c := &Client{
		mnts:        mnts,
		cmd:         cmd,
		dialer:      d,
		maxConns:    conf.MaxConnections,
		waitOnClose: conf.WaitOnClose,
	}
	return c, nil
}

// Serve starts proxying connections for all configured instances using the
// associated socket.
func (c *Client) Serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	exitCh := make(chan error)
	for _, m := range c.mnts {
		go func(mnt *socketMount) {
			err := c.serveSocketMount(ctx, mnt)
			if err != nil {
				select {
				// Best effort attempt to send error.
				// If this send fails, it means the reading goroutine has
				// already pulled a value out of the channel and is no longer
				// reading any more values. In other words, we report only the
				// first error.
				case exitCh <- err:
				default:
					return
				}
			}
		}(m)
	}
	return <-exitCh
}

// Close triggers the proxyClient to shutdown.
func (c *Client) Close() error {
	// First, close all open socket listeners to prevent additional connections.
	for _, m := range c.mnts {
		_ = m.close() // TODO this returns an error (if the listener doesn't close)
	}
	// Next, close the dialer to prevent any future refreshes.
	_ = c.dialer.Close() // TODO this returns an error (which is always nil right now)
	if c.waitOnClose == 0 {
		return nil
	}
	timeout := time.After(c.waitOnClose)
	tick := time.Tick(100 * time.Millisecond)
	for {
		select {
		case <-tick:
			if atomic.LoadUint32(&c.connCount) > 0 {
				continue
			}
		case <-timeout:
		}
		break
	}
	open := atomic.LoadUint32(&c.connCount)
	if open > 0 {
		return fmt.Errorf("%d connection(s) still open after waiting %v", open, c.waitOnClose)
	}
	return nil
}

// serveSocketMount persistently listens to the socketMounts listener and proxies connections to a
// given Cloud SQL instance.
func (c *Client) serveSocketMount(ctx context.Context, s *socketMount) error {
	for {
		cConn, err := s.Accept()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				c.cmd.PrintErrf("[%s] Error accepting connection: %v\n", s.inst, err)
				// For transient errors, wait a small amount of time to see if it resolves itself
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return err
		}
		// handle the connection in a separate goroutine
		go func() {
			c.cmd.Printf("[%s] accepted connection from %s\n", s.inst, cConn.RemoteAddr())

			count := atomic.AddUint32(&c.connCount, 1)
			defer atomic.AddUint32(&c.connCount, ^uint32(0))

			if c.maxConns > 0 && count > c.maxConns {
				c.cmd.Printf("max connections (%v) exceeded, refusing new connection\n", c.maxConns)
				cConn.Close()
				return
			}

			// give a max of 30 seconds to connect to the instance
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			sConn, err := c.dialer.Dial(ctx, s.inst, s.dialOpts...)
			if err != nil {
				c.cmd.Printf("[%s] failed to connect to instance: %v\n", s.inst, err)
				cConn.Close()
				return
			}
			c.proxyConn(s.inst, cConn, sConn)
		}()
	}
}

// socketMount is a tcp/unix socket that listens for a Cloud SQL instance.
type socketMount struct {
	inst     string
	dialOpts []cloudsqlconn.DialOption
	mu       sync.Mutex
	listener net.Listener
}

func (s *socketMount) Accept() (net.Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil, fmt.Errorf("[%s] mount doesn't have a listener set", s.inst)
	}
	return s.listener.Accept()
}

// listen causes a socketMount to create a Listener at the specified network address.
func (s *socketMount) listen(ctx context.Context, network string, address string) (net.Addr, error) {
	lc := net.ListenConfig{KeepAlive: 30 * time.Second}
	l, err := lc.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.listener = l
	s.mu.Unlock()
	return s.listener.Addr(), nil
}

// close stops the mount from listening for any more connections
func (s *socketMount) close() error {
	err := s.listener.Close()
	s.mu.Lock()
	s.listener = nil
	s.mu.Unlock()
	return err
}

// proxyConn sets up a bidirectional copy between two open connections
func (c *Client) proxyConn(inst string, client, server net.Conn) {
	// only allow the first side to give an error for terminating a connection
	var o sync.Once
	cleanup := func(errDesc string, isErr bool) {
		o.Do(func() {
			client.Close()
			server.Close()
			if isErr {
				c.cmd.PrintErrln(errDesc)
			} else {
				c.cmd.Println(errDesc)
			}
		})
	}

	// copy bytes from client to server
	go func() {
		buf := make([]byte, 8*1024) // 8kb
		for {
			n, cErr := client.Read(buf)
			var sErr error
			if n > 0 {
				_, sErr = server.Write(buf[:n])
			}
			switch {
			case cErr == io.EOF:
				cleanup(fmt.Sprintf("[%s] client closed the connection", inst), false)
				return
			case cErr != nil:
				cleanup(fmt.Sprintf("[%s] connection aborted - error reading from client: %v", inst, cErr), true)
				return
			case sErr == io.EOF:
				cleanup(fmt.Sprintf("[%s] instance closed the connection", inst), false)
				return
			case sErr != nil:
				cleanup(fmt.Sprintf("[%s] connection aborted - error writing to instance: %v", inst, cErr), true)
				return
			}
		}
	}()

	// copy bytes from server to client
	buf := make([]byte, 8*1024) // 8kb
	for {
		n, sErr := server.Read(buf)
		var cErr error
		if n > 0 {
			_, cErr = client.Write(buf[:n])
		}
		switch {
		case sErr == io.EOF:
			cleanup(fmt.Sprintf("[%s] instance closed the connection", inst), false)
			return
		case sErr != nil:
			cleanup(fmt.Sprintf("[%s] connection aborted - error reading from instance: %v", inst, sErr), true)
			return
		case cErr == io.EOF:
			cleanup(fmt.Sprintf("[%s] client closed the connection", inst), false)
			return
		case cErr != nil:
			cleanup(fmt.Sprintf("[%s] connection aborted - error writing to client: %v", inst, sErr), true)
			return
		}
	}
}

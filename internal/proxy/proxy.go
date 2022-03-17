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
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/cloudsqlconn"
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
}

// Config contains all the configuration provided by the caller.
type Config struct {
	// Addr is the address on which to bind all instances.
	Addr string

	// Port is the first port to bind to. Subsequent ports will increment from
	// this value.
	Port int

	// Instances are configuration for individual instances. Instance
	// configuration takes precedence over global configuration.
	Instances []InstanceConnConfig
}

// Client represents the state of the current instantiation of the proxy.
type Client struct {
	cmd    *cobra.Command
	dialer *cloudsqlconn.Dialer

	// mnts is a list of all mounted sockets for this client
	mnts []*socketMount
}

type portConfig struct {
	mu        sync.Mutex
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
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.global
	c.global++
	return p
}

func (c *portConfig) nextDBPort(version string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
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

// NewClient completes the initial setup required to get the proxy to a "steady" state.
func NewClient(ctx context.Context, d *cloudsqlconn.Dialer, cmd *cobra.Command, conf *Config) (*Client, error) {
	pc := newPortConfig(conf.Port)
	type mountRes struct {
		mnt *socketMount
		err error
	}
	mntCh := make(chan mountRes, len(conf.Instances))
	for _, inst := range conf.Instances {
		go func(i InstanceConnConfig) {
			m := &socketMount{inst: i.Name}
			a := conf.Addr
			if i.Addr != "" {
				a = i.Addr
			}
			version, err := d.EngineVersion(ctx, i.Name)
			if err != nil {
				mntCh <- mountRes{err: err}
				return
			}
			var np int
			switch {
			case i.Port != 0:
				np = i.Port
			case conf.Port != 0:
				np = pc.nextPort()
			default:
				np = pc.nextDBPort(version)
			}
			addr, err := m.listen(ctx, "tcp", net.JoinHostPort(a, fmt.Sprint(np)))
			if err != nil {
				err := fmt.Errorf("[%v] Unable to mount socket: %v", i.Name, err)
				mntCh <- mountRes{err: err}
				return
			}
			cmd.Printf("[%s] Listening on %s\n", i.Name, addr.String())
			mntCh <- mountRes{mnt: m}
		}(inst)
	}
	var (
		mnts []*socketMount
		err  error
	)
	for i := 0; i < len(conf.Instances); i++ {
		res := <-mntCh
		if res.err != nil {
			err = res.err
			continue
		}
		mnts = append(mnts, res.mnt)
	}
	if err != nil {
		for _, m := range mnts {
			m.close()
		}
		return nil, err
	}

	return &Client{mnts: mnts, cmd: cmd, dialer: d}, nil
}

// Serve listens on the mounted ports and beging proxying the connections to the instances.
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
func (c *Client) Close() {
	defer c.dialer.Close()
	for _, m := range c.mnts {
		m.close()
	}
}

// serveSocketMount persistently listens to the socketMounts listener and proxies connections to a
// given Cloud SQL instance.
func (c *Client) serveSocketMount(ctx context.Context, s *socketMount) error {
	if s.listener == nil {
		return fmt.Errorf("[%s] mount doesn't have a listener set", s.inst)
	}
	for {
		cConn, err := s.listener.Accept()
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

			// give a max of 30 seconds to connect to the instance
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			sConn, err := c.dialer.Dial(ctx, s.inst)
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
	listener net.Listener
}

// listen causes a socketMount to create a Listener at the specified network address.
func (s *socketMount) listen(ctx context.Context, network string, host string) (net.Addr, error) {
	lc := net.ListenConfig{KeepAlive: 30 * time.Second}
	l, err := lc.Listen(ctx, network, host)
	if err != nil {
		return nil, err
	}
	s.listener = l
	return s.listener.Addr(), nil
}

// close stops the mount from listening for any more connections
func (s *socketMount) close() error {
	err := s.listener.Close()
	s.listener = nil
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

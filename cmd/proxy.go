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

package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/spf13/cobra"
)

// proxyClient represents the state of the current instantiation of the proxy.
type proxyClient struct {
	cmd    *cobra.Command
	dialer *cloudsqlconn.Dialer

	// mnts is a list of all mounted sockets for this client
	mnts []*socketMount
	// serveCtx is used for serving data
	serveCtx    context.Context
	serveCancel context.CancelFunc
}

// newProxyClient completes the initial setup required to get the proxy to a "steady" state.
func newProxyClient(ctx context.Context, cmd *cobra.Command, args []string) (*proxyClient, error) {
	dialer, err := cloudsqlconn.NewDialer(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing dialer: %v", err)
	}
	pc := &proxyClient{cmd: cmd, dialer: dialer}

	port := 5000 // TODO: figure out better port allocation strategy
	for i, inst := range args {
		m := newSocketMount(dialer, inst)
		addr, err := m.listen(ctx, "tcp4", net.JoinHostPort("127.0.0.1", fmt.Sprint(port+i)))
		if err != nil {
			return nil, fmt.Errorf("[%s] Unable to mount socket: %v", inst, err)
		}
		cmd.Printf("[%s] Listening on %s\n", inst, addr.String())
		pc.mnts = append(pc.mnts, m)
	}

	return pc, nil
}

// serve listens on the mounted ports and beging proxying the connections to the instances.
func (pc *proxyClient) serve(ctx context.Context) error {
	pc.serveCtx, pc.serveCancel = context.WithCancel(ctx)
	defer pc.serveCancel()
	exitCh := make(chan error)
	for _, m := range pc.mnts {
		go func(mnt *socketMount) {
			err := mnt.serve(ctx)
			if err != nil {
				exitCh <- err
				return
			}
		}(m)
	}
	return <-exitCh
}

// close triggers the proxyClient to shutdown.
func (pc *proxyClient) close() {
	defer pc.dialer.Close()
	defer pc.serveCancel()
	for _, m := range pc.mnts {
		m.close()
	}
}

// socketMount is a tcp/unix socket that listens for a Cloud SQL instance. It should
// only be created with newSocketMount.
type socketMount struct {
	dialer *cloudsqlconn.Dialer
	inst   string

	listener net.Listener
}

// newSocketMount creates a new socketMount struct for a specific Cloud SQL instance.
func newSocketMount(dialer *cloudsqlconn.Dialer, inst string) *socketMount {
	return &socketMount{
		dialer: dialer,
		inst:   inst,
	}
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

// serve persistently listens to the socketMounts listener and proxies connections to a
// given Cloud SQL instance.
func (s *socketMount) serve(ctx context.Context) error {
	if s.listener == nil {
		return fmt.Errorf("[%s] mount doesn't have a listener set", s.inst)
	}
	for {
		cConn, err := s.listener.Accept()
		if err != nil {
			log.Printf("[%s] Error accepting connection: %v\n", s.inst, err)
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				// For transient errors, wait a small amount of time to see if it resolves itself
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return err
		}
		// handle the connection in a separate goroutine
		go func() {
			log.Printf("[%s] accepted connection from %s\n", s.inst, cConn.RemoteAddr())
			sConn, err := s.dialer.Dial(ctx, s.inst)
			if err != nil {
				log.Printf("[%s] failed to connect to instance: %v\n", s.inst, err)
				cConn.Close()
				return
			}
			proxyConn(s.inst, cConn, sConn)
		}()
	}
}

// close stops the mount from listening for any more connections
func (s *socketMount) close() error {
	err := s.listener.Close()
	s.listener = nil
	return err
}

// proxyConn sets up a bidirectional copy between two open connections
func proxyConn(inst string, client, server net.Conn) {
	// only allow the first side to give an error for terminating a connection
	var o sync.Once
	cleanup := func(errDesc string) {
		o.Do(func() {
			log.Printf(errDesc + "\n")
			client.Close()
			server.Close()
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
				cleanup(fmt.Sprintf("[%s] client closed the connection", inst))
				return
			case cErr != nil:
				cleanup(fmt.Sprintf("[%s] connection aborted - error reading from client: %v", inst, cErr))
				return
			case sErr == io.EOF:
				cleanup(fmt.Sprintf("[%s] instance closed the connection", inst))
				return
			case sErr != nil:
				cleanup(fmt.Sprintf("[%s] connection aborted - error writing to instance: %v", inst, cErr))
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
			cleanup(fmt.Sprintf("[%s] instance closed the connection", inst))
			return
		case sErr != nil:
			cleanup(fmt.Sprintf("[%s] connection aborted - error reading from instance: %v", inst, sErr))
			return
		case cErr == io.EOF:
			cleanup(fmt.Sprintf("[%s] client closed the connection", inst))
			return
		case cErr != nil:
			cleanup(fmt.Sprintf("[%s] connection aborted - error writing to client: %v", inst, sErr))
			return
		}
	}
}

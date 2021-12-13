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
)

type socketMount struct {
	dialer cloudsqlconn.Dialer
	inst   string

	listener net.Listener
}

func newSocketMount(dialer cloudsqlconn.Dialer, inst string) *socketMount {
	return &socketMount{
		dialer: dialer,
		inst:   inst,
	}
}

func (s *socketMount) Listen(ctx context.Context, network string, host string) (net.Addr, error) {
	lc := net.ListenConfig{KeepAlive: 30 * time.Second}
	l, err := lc.Listen(ctx, network, host)
	if err != nil {
		return nil, err
	}
	s.listener = l
	return s.listener.Addr(), nil
}

func (s *socketMount) Serve(ctx context.Context) error {
	if s.listener == nil {
		return fmt.Errorf("socket isn't mounted")
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
				return
			}
			proxyConn(s.inst, cConn, sConn)
		}()
	}
}

// Close stops the mount from listening for any more connections
func (s *socketMount) Close() error {
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
		buf := make([]byte, 0x2000) // 8kb
		for {
			n, cErr := client.Read(buf)
			var sErr error
			if n > 0 {
				_, sErr = server.Write(buf[:n])
			}
			switch {
			case cErr == io.EOF:
				cleanup(fmt.Sprintf("[%s] client closed the connection", inst))
			case cErr != nil:
				cleanup(fmt.Sprintf("[%s] connection aborted - error reading from client: %v", inst, cErr))
			case sErr == io.EOF:
				cleanup(fmt.Sprintf("[%s] instance closed the connection", inst))
			case sErr != nil:
				cleanup(fmt.Sprintf("[%s] connection aborted - error writing to instance: %v", inst, cErr))
			}
		}
	}()

	// copy bytes from server to client
	buf := make([]byte, 0x2000) // 8kb
	for {
		n, sErr := server.Read(buf)
		var cErr error
		if n > 0 {
			_, cErr = client.Write(buf[:n])
		}
		switch {
		case sErr == io.EOF:
			cleanup(fmt.Sprintf("[%s] instance closed the connection", inst))
		case sErr != nil:
			cleanup(fmt.Sprintf("[%s] connection aborted - error reading from instance: %v", inst, sErr))
		case cErr == io.EOF:
			cleanup(fmt.Sprintf("[%s] client closed the connection", inst))
		case cErr != nil:
			cleanup(fmt.Sprintf("[%s] connection aborted - error writing to client: %v", inst, sErr))
		}
	}
}

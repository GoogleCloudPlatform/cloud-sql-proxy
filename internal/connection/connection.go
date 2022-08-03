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

package connection

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/cloudsql"
)

// AcceptAndHandle runs in a loop accepting connections from a listener and
// starting goroutines to handle the copying of bytes to the remote Cloud SQL
// instance.
func AcceptAndHandle(ctx context.Context, ln net.Listener, l cloudsql.Logger, d cloudsql.Dialer, counter *Counter, instance string, opts ...cloudsqlconn.DialOption) error {
	for {
		cConn, err := ln.Accept()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				l.Errorf("[%s] Error accepting connection: %v", instance, err)
				// For transient errors, wait a small amount of time to see if it resolves itself
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return err
		}
		// handle the connection in a separate goroutine
		go func() {
			l.Infof("[%s] accepted connection from %s", instance, cConn.RemoteAddr())

			// A client has established a connection to the local socket. Before
			// we initiate a connection to the Cloud SQL backend, increment the
			// connection counter. If the total number of connections exceeds
			// the maximum, refuse to connect and close the client connection.
			dec, err := counter.Inc()
			if err != nil {
				_, max := counter.Count()
				l.Infof("max connections (%v) exceeded, refusing new connection", max)
				_ = cConn.Close()
				return
			}
			defer dec()

			// give a max of 30 seconds to connect to the instance
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			sConn, err := d.Dial(ctx, instance, opts...)
			if err != nil {
				l.Errorf("[%s] failed to connect to instance: %v", instance, err)
				cConn.Close()
				return
			}
			Copy(l, instance, cConn, sConn)
		}()
	}
}

// Copy sets up a bidirectional copy between two open connections
func Copy(l cloudsql.Logger, inst string, client, server net.Conn) {
	// only allow the first side to give an error for terminating a connection
	var o sync.Once
	cleanup := func(errDesc string, isErr bool) {
		o.Do(func() {
			client.Close()
			server.Close()
			if isErr {
				l.Errorf(errDesc)
			} else {
				l.Infof(errDesc)
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

var errMaxExceeded = errors.New("max connections exceeded")

// NewCounter initializes a Counter.
func NewCounter(max uint64) *Counter {
	return &Counter{max: max}
}

// Counter tracks a count next to a maximum value.
type Counter struct {
	// count tracks the number of all open connections from the Client to
	// all Cloud SQL instances.
	count uint64

	// maxConns is the maximum number of allowed connections tracked by
	// connCount. If not set, there is no limit.
	max uint64
}

// IsZero reports whether the existing connection count is zero.
func (c *Counter) IsZero() bool {
	return atomic.LoadUint64(&c.count) == 0
}

// Count reports the number of open connections, and the maximum configured
// connections.
func (c *Counter) Count() (uint64, uint64) {
	return atomic.LoadUint64(&c.count), c.max
}

// Inc increases the count. Callers should cal the cleanup function to decrement
// the count.
func (c *Counter) Inc() (func(), error) {
	count := atomic.AddUint64(&c.count, 1)
	cleanup := func() { atomic.AddUint64(&c.count, ^uint64(0)) }

	if c.max > 0 && count > c.max {
		return func() {}, errMaxExceeded
	}
	return cleanup, nil
}

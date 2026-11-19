// Copyright 2023 Google LLC
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

package cmd

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWaitCommandFlags(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Use a read deadline to produce read error
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		// Read client request first.
		io.ReadAll(conn)
		// Write a generic 200 response back.
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	}()

	_, err = invokeProxyCommand([]string{
		"wait",
		"--http-address", host,
		"--http-port", port,
		"--max=1s",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitCommandFails(t *testing.T) {
	_, err := invokeProxyCommand([]string{
		"wait",
		// assuming default host and port
		"--max=100ms",
	})
	if err == nil {
		t.Fatal("wait should fail when endpoint does not respond")
	}
}

// bodyTrackingTransport returns a fixed response for every request and
// remembers the bodies it handed out so tests can check they were closed.
type bodyTrackingTransport struct {
	status int
	bodies []*trackedBody
}

func (t *bodyTrackingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	b := &trackedBody{Reader: strings.NewReader("not ready")}
	t.bodies = append(t.bodies, b)
	return &http.Response{
		StatusCode: t.status,
		Body:       b,
		Header:     make(http.Header),
	}, nil
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

func TestWaitCommandClosesResponseBody(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{"proxy is ready", http.StatusOK},
		{"proxy is not ready", http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := &bodyTrackingTransport{status: tc.status}
			prev := http.DefaultClient.Transport
			http.DefaultClient.Transport = rt
			defer func() { http.DefaultClient.Transport = prev }()

			_, _ = invokeProxyCommand([]string{
				"wait",
				"--http-address", "127.0.0.1",
				"--http-port", "9090",
				"--max=100ms",
			})

			if len(rt.bodies) == 0 {
				t.Fatal("wait did not poll the startup endpoint")
			}
			for i, b := range rt.bodies {
				if !b.closed {
					t.Errorf("poll %d: response body was left open", i)
				}
			}
		})
	}
}

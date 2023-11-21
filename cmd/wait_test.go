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
	"net"
	"testing"
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
		"--max=500ms",
	})
	if err == nil {
		t.Fatal("wait should fail when endpoint does not respond")
	}
}

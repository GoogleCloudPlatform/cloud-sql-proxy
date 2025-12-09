// Copyright 2025 Google LLC
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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShutdownCommand(t *testing.T) {
	shutdownCh := make(chan bool, 1)
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("want = POST, got = %v", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		shutdownCh <- true
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	_, err = invokeProxyCommand([]string{
		"shutdown",
		"--admin-port", port,
	})
	if err != nil {
		t.Fatalf("invokeProxyCommand failed: %v", err)
	}

	select {
	case <-shutdownCh:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("server did not receive shutdown request")
	}
}

func TestShutdownCommandFails(t *testing.T) {
	_, err := invokeProxyCommand([]string{
		"shutdown",
		// assuming default host and port
		"--wait=100ms",
	})
	if err == nil {
		t.Fatal("shutdown should fail when endpoint does not respond")
	}
}

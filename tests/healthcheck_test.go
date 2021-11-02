// Copyright 2021 Google LLC All Rights Reserved.
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

// healthcheck_test.go provides some helpers for end to end health check server tests.
package tests

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

const (
	readinessPath = "/readiness"
	testPort      = "8090"
)

// singleInstanceDial verifies that when a proxy client serves the given instance, the readiness
// endpoint serves http.StatusOK.
func singleInstanceDial(t *testing.T, connName string, port int) {
	ctx := context.Background()

	var args []string
	args = append(args, fmt.Sprintf("-instances=%s=tcp6:%d", connName, port), "-use_http_health_check")

	// Start the proxy.
	p, err := StartProxy(ctx, args...)
	if err != nil {
		t.Fatalf("unable to start proxy: %v", err)
	}
	defer p.Close()
	output, err := p.WaitForServe(ctx)
	if err != nil {
		t.Fatalf("unable to verify proxy was serving: %s \n %s", err, output)
	}

	resp, err := http.Get("http://[::1]:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want %v, got %v", http.StatusOK, resp.StatusCode)
	}
}

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

package healthcheck_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"syscall"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/cmd/cloud_sql_proxy/internal/healthcheck"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
)

const (
	livenessPath  = "/liveness"
	readinessPath = "/readiness"
	testPort      = "8090"
)

// Test to verify that when the proxy client is up, the liveness endpoint writes 200.
func TestLiveness(t *testing.T) {
	s, err := healthcheck.NewServer(&proxy.Client{}, testPort)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + livenessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Got status code %v instead of 200", resp.StatusCode)
	}
}

// Test to verify that 1. when startup has NOT finished, the readiness endpoint writes 500.
// 2. when startup HAS finished (and MaxConnections limit not specified), the readiness
// endpoint writes 200.
func TestStartup(t *testing.T) {
	cases := []struct {
		finishedStartup bool
		statusCode int
	}{
		{
			finishedStartup: false,
			statusCode: 500,
		},
		{
			finishedStartup: true,
			statusCode: 200,
		},
	}

	for _, c := range cases {
		func() {
			s, err := healthcheck.NewServer(&proxy.Client{}, testPort)
			if err != nil {
				t.Fatalf("Could not initialize health check: %v", err)
			}
			defer s.Close(context.Background())
		
			if c.finishedStartup == true {
				s.NotifyStarted() // Simulate the proxy client completing startup.
			}
		
			resp, err := http.Get("http://localhost:" + testPort + readinessPath)
			if err != nil {
				t.Fatalf("HTTP GET failed: %v", err)
			}
			if resp.StatusCode != c.statusCode {
				t.Errorf("Got status code %v instead of %v", resp.StatusCode, c.statusCode)
			}
		}()
	}
}

// Test to verify that when startup has finished, but MaxConnections has been reached,
// the readiness endpoint writes 500.
func TestMaxConnectionsReached(t *testing.T) {
	c := &proxy.Client{
		MaxConnections: 1,
	}
	s, err := healthcheck.NewServer(c, testPort)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())

	s.NotifyStarted()
	c.ConnectionsCounter = c.MaxConnections // Simulate reaching the limit for maximum number of connections

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("Got status code %v instead of 500", resp.StatusCode)
	}
}

// Test to verify that after closing a healthcheck, its liveness endpoint serves
// an error.
func TestCloseHealthCheck(t *testing.T) {
	s, err := healthcheck.NewServer(&proxy.Client{}, testPort)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + livenessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Got status code %v instead of 200", resp.StatusCode)
	}

	err = s.Close(context.Background())
	if err != nil {
		t.Fatalf("Failed to close health check: %v", err)
	}

	_, err = http.Get("http://localhost:" + testPort + livenessPath)
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("HTTP GET did not give a 'connection refused' error after closing health check")
	}
}

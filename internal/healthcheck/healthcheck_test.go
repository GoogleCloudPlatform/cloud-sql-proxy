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
	"net/http"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/internal/healthcheck"
)

const (
	livenessPath = "/liveness"
	readinessPath = "/readiness"
	testPort = "8090"
)

// Test to verify that when the proxy client is up, the liveness endpoint writes 200.
func TestLiveness(t *testing.T) {
	hc, err := healthcheck.NewHealthCheck(&proxy.Client{}, testPort)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v\n", err)
	}
	defer hc.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + livenessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Got status code %v instead of 200\n", resp.StatusCode)
	}
}

// Test to verify that when startup has not finished, the readiness endpoint writes 500.
func TestStartupFail(t *testing.T) {
	hc, err := healthcheck.NewHealthCheck(&proxy.Client{}, testPort)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v\n", err)
	}
	defer hc.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v\n", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("Got status code %v instead of 500\n", resp.StatusCode)
	}
}

// Test to verify that when startup has finished, and MaxConnections has not been reached,
// the readiness endpoint writes 200.
func TestStartupPass(t *testing.T) {
	hc, err := healthcheck.NewHealthCheck(&proxy.Client{}, testPort)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v\n", err)
	}
	defer hc.Close(context.Background())

	// Simulate the proxy client completing startup.
	hc.NotifyStarted()

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Got status code %v instead of 200\n", resp.StatusCode)
	}
}

// Test to verify that when startup has finished, but MaxConnections has been reached,
// the readiness endpoint writes 500.
func TestMaxConnectionsReached(t *testing.T) {
	proxyClient := &proxy.Client{
		MaxConnections: 1,
	}
	hc, err := healthcheck.NewHealthCheck(proxyClient, testPort)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v\n", err)
	}
	defer hc.Close(context.Background())

	hc.NotifyStarted()
	proxyClient.ConnectionsCounter = proxyClient.MaxConnections // Simulate reaching the limit for maximum number of connections

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v\n", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("Got status code %v instead of 500\n", resp.StatusCode)
	}
}

// Test to verify that after closing a healthcheck, its liveness endpoint serves
// an error.
func TestCloseHealthCheck(t *testing.T) {
	hc, err := healthcheck.NewHealthCheck(&proxy.Client{}, testPort)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v\n", err)
	}
	defer hc.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + livenessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Got status code %v instead of 200\n", resp.StatusCode)
	}

	err = hc.Close(context.Background())
	if err != nil {
		t.Fatalf("Failed to close health check: %v\n", err)
	}

	_, err = http.Get("http://localhost:" + testPort + livenessPath)
	if err == nil { // If NO error
		t.Fatalf("HTTP GET did not fail after closing health check\n")
	}
}

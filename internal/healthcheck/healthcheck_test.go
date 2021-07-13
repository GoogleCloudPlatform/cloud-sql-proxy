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
	testPort = "9090"
)

// Test to verify that when the proxy client is up, the liveness endpoint writes 200.
func TestLiveness(t *testing.T) {
	proxyClient := &proxy.Client{}
	hc := healthcheck.NewHealthCheck(proxyClient, testPort)
	defer hc.Close(context.Background()) // Close health check upon exiting the test.

	resp, err := http.Get("http://localhost:" + testPort + livenessPath)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}
}

// Test to verify that when startup has not finished, the readiness endpoint writes 500.
func TestStartupFail(t *testing.T) {
	proxyClient := &proxy.Client{}
	hc := healthcheck.NewHealthCheck(proxyClient, testPort)
	defer hc.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("got status code %v instead of 500", resp.StatusCode)
	}
}

// Test to verify that when startup has finished, and MaxConnections has not been reached,
// the readiness endpoint writes 200.
func TestStartupPass(t *testing.T) {
	proxyClient := &proxy.Client{}
	hc := healthcheck.NewHealthCheck(proxyClient, testPort)
	defer hc.Close(context.Background())

	// Simulate the proxy client completing startup.
	hc.NotifyStarted()

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}
}

// Test to verify that when startup has finished, but MaxConnections has been reached,
// the readiness endpoint writes 500.
func TestMaxConnectionsReached(t *testing.T) {
	proxyClient := &proxy.Client{
		MaxConnections: 10,
	}
	hc := healthcheck.NewHealthCheck(proxyClient, testPort)
	defer hc.Close(context.Background())

	hc.NotifyStarted()
	proxyClient.ConnectionsCounter = proxyClient.MaxConnections // Simulate reaching the limit for maximum number of connections

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("got status code %v instead of 500", resp.StatusCode)
	}
}

// Test to verify that after closing a healthcheck, its liveness endpoint serves
// an error.
func TestCloseHealthCheck(t *testing.T) {
	proxyClient := &proxy.Client{}
	hc := healthcheck.NewHealthCheck(proxyClient, testPort)

	resp, err := http.Get("http://localhost:" + testPort + livenessPath)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}

	hc.Close(context.Background())

	_, err = http.Get("http://localhost:" + testPort + livenessPath)
	if err == nil { // If NO error
		t.Fatal(err)
	}
}

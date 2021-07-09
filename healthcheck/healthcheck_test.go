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

package healthcheck

import (
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
)
 
const (
	livenessURL = "http://localhost:8080/liveness"
	readinessURL = "http://localhost:8080/readiness"
)

func TestLiveness(t *testing.T) {
	proxyClient := &proxy.Client{}
	hc := NewHealthCheck(proxyClient)
	defer hc.Close() // Close health check upon exiting the test.

	time.Sleep(100 * time.Millisecond) // Wait for ListenAndServe to begin to avoid flaky tests.

	resp, err := http.Get(livenessURL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}
}

func TestStartupFail(t *testing.T) {
	proxyClient := &proxy.Client{}
	hc := NewHealthCheck(proxyClient)
	defer hc.Close()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(readinessURL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("got status code %v instead of 500", resp.StatusCode)
	}
}

func TestStartupPass(t *testing.T) {
	proxyClient := &proxy.Client{}
	hc := NewHealthCheck(proxyClient)
	defer hc.Close()

	time.Sleep(100 * time.Millisecond)

	hc.NotifyReadyForConnections()

	resp, err := http.Get(readinessURL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}
}

func TestMaxConnectionsReached(t *testing.T) {
	proxyClient := &proxy.Client{
		MaxConnections: 10,
	}
	hc := NewHealthCheck(proxyClient)
	defer hc.Close()

	time.Sleep(100 * time.Millisecond)

	hc.NotifyReadyForConnections()

	resp, err := http.Get(readinessURL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}

	proxyClient.ConnectionsCounter = proxyClient.MaxConnections // Simulate reaching the limit for maximum number of connections

	resp, err = http.Get(readinessURL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("got status code %v instead of 500", resp.StatusCode)
	}
}

func TestCloseHealthCheck(t *testing.T) {
	proxyClient := &proxy.Client{}
	hc := NewHealthCheck(proxyClient)
	defer hc.Close()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(livenessURL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}

	hc.Close()

	_, err = http.Get(livenessURL)
	if err == nil { // If NO error
		t.Fatal(err)
	}
}

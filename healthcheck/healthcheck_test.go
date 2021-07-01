// Copyright 2015 Google Inc. All Rights Reserved.
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

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
)

func newClient(mc uint64) *proxy.Client {
	return &proxy.Client{
		MaxConnections: mc,
	}
}

func TestLiveness(t *testing.T) {
	proxyClient := newClient(0)
	InitHealthCheck(proxyClient)

	resp, err := http.Get("http://localhost:8080/liveness")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}
}

func TestBadStartup(t *testing.T) {
	proxyClient := newClient(0)
	InitHealthCheck(proxyClient)

	resp, err := http.Get("http://localhost:8080/readiness")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("got status code %v instead of 500", resp.StatusCode)
	}
}

func TestSuccessfulStartup(t *testing.T) {
	proxyClient := newClient(0)
	hc := InitHealthCheck(proxyClient)
	NotifyReadyForConnections(hc)

	resp, err := http.Get("http://localhost:8080/readiness")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}
}

func TestMaxConnections(t *testing.T) {
	proxyClient := newClient(10) // MaxConnections == 10
	hc := InitHealthCheck(proxyClient)
	NotifyReadyForConnections(hc)

	resp, err := http.Get("http://localhost:8080/readiness")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}

	proxyClient.ConnectionsCounter = proxyClient.MaxConnections // Simulate reaching the limit for maximum number of connections

	resp, err = http.Get("http://localhost:8080/readiness")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 500 {
		t.Errorf("got status code %v instead of 500", resp.StatusCode)
	}
}

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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/cmd/cloud_sql_proxy/internal/healthcheck"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/proxy/proxy"
)

const (
	startupPath   = "/startup"
	livenessPath  = "/liveness"
	readinessPath = "/readiness"
	testPort      = "8090"
)

type fakeCertSource struct{}

func (cs *fakeCertSource) Local(instance string) (tls.Certificate, error) {
	return tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: time.Date(9999, 0, 0, 0, 0, 0, 0, time.UTC),
		},
	}, nil
}

func (cs *fakeCertSource) Remote(instance string) (cert *x509.Certificate, addr, name, version string, err error) {
	return &x509.Certificate{}, "fake address", "fake name", "fake version", nil
}

type failingCertSource struct{}

func (cs *failingCertSource) Local(instance string) (tls.Certificate, error) {
	return tls.Certificate{}, errors.New("failed")
}

func (cs *failingCertSource) Remote(instance string) (cert *x509.Certificate, addr, name, version string, err error) {
	return nil, "", "", "", errors.New("failed")
}

// Test to verify that when the proxy client is up, the liveness endpoint writes http.StatusOK.
func TestLivenessPasses(t *testing.T) {
	s, err := healthcheck.NewServer(&proxy.Client{}, testPort, nil)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + livenessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want %v, got %v", http.StatusOK, resp.StatusCode)
	}
}

func TestLivenessFails(t *testing.T) {
	c := &proxy.Client{
		Certs: &failingCertSource{},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, errors.New("error")
		},
	}
	// ensure cache has errored config
	_, err := c.Dial("proj:region:instance")
	if err == nil {
		t.Fatalf("expected Dial to fail, but it succeeded")
	}

	s, err := healthcheck.NewServer(c, testPort, []string{"proj:region:instance"})
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + livenessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()
	want := http.StatusServiceUnavailable
	if got := resp.StatusCode; got != want {
		t.Errorf("want %v, got %v", want, got)
	}
}

// Test to verify that when startup HAS finished (and MaxConnections limit not specified),
// the startup and readiness endpoints write http.StatusOK.
func TestStartupPass(t *testing.T) {
	s, err := healthcheck.NewServer(&proxy.Client{}, testPort, nil)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())

	// Simulate the proxy client completing startup.
	s.NotifyStarted()

	resp, err := http.Get("http://localhost:" + testPort + startupPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%v: want %v, got %v", startupPath, http.StatusOK, resp.StatusCode)
	}

	resp, err = http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%v: want %v, got %v", readinessPath, http.StatusOK, resp.StatusCode)
	}
}

// Test to verify that when startup has NOT finished, the startup and readiness endpoints write
// http.StatusServiceUnavailable.
func TestStartupFail(t *testing.T) {
	s, err := healthcheck.NewServer(&proxy.Client{}, testPort, nil)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + startupPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("%v: want %v, got %v", startupPath, http.StatusOK, resp.StatusCode)
	}

	resp, err = http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("%v: want %v, got %v", readinessPath, http.StatusOK, resp.StatusCode)
	}
}

// Test to verify that when startup has finished, but MaxConnections has been reached,
// the readiness endpoint writes http.StatusServiceUnavailable.
func TestMaxConnectionsReached(t *testing.T) {
	c := &proxy.Client{
		MaxConnections: 1,
	}
	s, err := healthcheck.NewServer(c, testPort, nil)
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
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("want %v, got %v", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

// Test to verify that when dialing instance(s) returns an error, the readiness endpoint
// writes http.StatusServiceUnavailable.
func TestDialFail(t *testing.T) {
	tests := map[string]struct {
		insts []string
	}{
		"Single instance":    {insts: []string{"project:region:instance"}},
		"Multiple instances": {insts: []string{"project:region:instance-1", "project:region:instance-2", "project:region:instance-3"}},
	}

	c := &proxy.Client{
		Certs: &fakeCertSource{},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, errors.New("error")
		},
	}

	for name, test := range tests {
		func() {
			s, err := healthcheck.NewServer(c, testPort, test.insts)
			if err != nil {
				t.Fatalf("%v: Could not initialize health check: %v", name, err)
			}
			defer s.Close(context.Background())
			s.NotifyStarted()

			resp, err := http.Get("http://localhost:" + testPort + readinessPath)
			if err != nil {
				t.Fatalf("%v: HTTP GET failed: %v", name, err)
			}
			if resp.StatusCode != http.StatusServiceUnavailable {
				t.Errorf("want %v, got %v", http.StatusServiceUnavailable, resp.StatusCode)
			}
		}()
	}
}

// Test to verify that after closing a healthcheck, its liveness endpoint serves
// an error.
func TestCloseHealthCheck(t *testing.T) {
	s, err := healthcheck.NewServer(&proxy.Client{}, testPort, nil)
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())

	resp, err := http.Get("http://localhost:" + testPort + livenessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want %v, got %v", http.StatusOK, resp.StatusCode)
	}

	err = s.Close(context.Background())
	if err != nil {
		t.Fatalf("Failed to close health check: %v", err)
	}

	_, err = http.Get("http://localhost:" + testPort + livenessPath)
	if err == nil {
		t.Fatalf("HTTP GET did not return error after closing health check server.")
	}
}

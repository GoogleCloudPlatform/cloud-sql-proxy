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

	"github.com/GoogleCloudPlatform/cloudsql-proxy/cmd/cloud_sql_proxy/internal/healthcheck"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
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

// Test to verify that when the proxy client is up, the liveness endpoint writes http.StatusOK.
func TestLiveness(t *testing.T) {
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
		t.Errorf("Got status code %v instead of %v", resp.StatusCode, http.StatusOK)
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
		t.Errorf("%v returned status code %v instead of %v", startupPath, resp.StatusCode, http.StatusOK)
	}

	resp, err = http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%v returned status code %v instead of %v", readinessPath, resp.StatusCode, http.StatusOK)
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
		t.Errorf("%v returned status code %v instead of %v", startupPath, resp.StatusCode, http.StatusServiceUnavailable)
	}

	resp, err = http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("%v returned status code %v instead of %v", readinessPath, resp.StatusCode, http.StatusServiceUnavailable)
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
		t.Errorf("Got status code %v instead of %v", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

// Test to verify that when a client has one instance and dialing it returns an error,
// the readiness endpoint writes http.StatusServiceUnavailable.
func TestSingleInstanceFail(t *testing.T) {
	c := &proxy.Client{
		Certs: &fakeCertSource{},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, errors.New("error")
		},
	}
	s, err := healthcheck.NewServer(c, testPort, []string{"instance-name"})
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())
	s.NotifyStarted()

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Got status code %v instead of %v", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

// Test to verify that when a client has multiple instances and dialing them returns an error,
// the readiness endpoint writes http.StatusServiceUnavailable.
func TestMultiInstanceFail(t *testing.T) {
	c := &proxy.Client{
		Certs: &fakeCertSource{},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, errors.New("error")
		},
	}
	s, err := healthcheck.NewServer(c, testPort, []string{"instance-1", "instance-2", "instance-3"})
	if err != nil {
		t.Fatalf("Could not initialize health check: %v", err)
	}
	defer s.Close(context.Background())

	s.NotifyStarted()

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Got status code %v instead of %v", resp.StatusCode, http.StatusServiceUnavailable)
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
		t.Errorf("Got status code %v instead of %v", resp.StatusCode, http.StatusOK)
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

// Copyright 2022 Google LLC
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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/healthcheck"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/log"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
)

var (
	logger    = log.NewStdLogger(os.Stdout, os.Stdout)
	proxyHost = "127.0.0.1"
	proxyPort = 9000
)

func proxyAddr() string {
	return fmt.Sprintf("%s:%d", proxyHost, proxyPort)
}

func dialTCP(t *testing.T, addr string) net.Conn {
	for i := 0; i < 10; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			return conn
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("failed to dial %v", addr)
	return nil
}

type fakeDialer struct {
	isClosed bool
}

func (f *fakeDialer) Dial(_ context.Context, _ string, _ ...cloudsqlconn.DialOption) (net.Conn, error) {
	if f.isClosed {
		return nil, errors.New("closed")
	}
	conn, _ := net.Pipe()
	return conn, nil
}

func (*fakeDialer) EngineVersion(_ context.Context, _ string) (string, error) {
	return "POSTGRES_14", nil
}

func (f *fakeDialer) Close() error {
	if f.isClosed {
		return errors.New("closed")
	}
	f.isClosed = true
	return nil
}

func newProxyWithParams(t *testing.T, maxConns uint64, dialer cloudsql.Dialer, instances []proxy.InstanceConnConfig) *proxy.Client {
	c := &proxy.Config{
		Addr:           proxyHost,
		Port:           proxyPort,
		Instances:      instances,
		MaxConnections: maxConns,
	}
	p, err := proxy.NewClient(context.Background(), dialer, logger, c, nil)
	if err != nil {
		t.Fatalf("proxy.NewClient: %v", err)
	}
	return p
}

func newTestProxyWithMaxConns(t *testing.T, maxConns uint64) *proxy.Client {
	return newProxyWithParams(t, maxConns, &fakeDialer{}, []proxy.InstanceConnConfig{
		{Name: "proj:region:pg"},
	})
}

func newTestProxy(t *testing.T) *proxy.Client {
	return newProxyWithParams(t, 0, &fakeDialer{}, []proxy.InstanceConnConfig{{Name: "proj:region:pg"}})
}

func TestHandleStartupWhenNotNotified(t *testing.T) {
	p := newTestProxy(t)
	defer func() {
		if err := p.Close(); err != nil {
			t.Logf("failed to close proxy client: %v", err)
		}
	}()
	check := healthcheck.NewCheck(p, logger)

	rec := httptest.NewRecorder()
	check.HandleStartup(rec, &http.Request{URL: &url.URL{}})

	// Startup is not complete because the Check has not been notified of the
	// proxy's startup.
	resp := rec.Result()
	if got, want := resp.StatusCode, http.StatusServiceUnavailable; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}
}

func TestHandleStartupWhenNotified(t *testing.T) {
	p := newTestProxy(t)
	defer func() {
		if err := p.Close(); err != nil {
			t.Logf("failed to close proxy client: %v", err)
		}
	}()
	check := healthcheck.NewCheck(p, logger)

	check.NotifyStarted()

	rec := httptest.NewRecorder()
	check.HandleStartup(rec, &http.Request{URL: &url.URL{}})

	resp := rec.Result()
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}
}

func TestHandleReadinessWhenNotNotified(t *testing.T) {
	p := newTestProxy(t)
	defer func() {
		if err := p.Close(); err != nil {
			t.Logf("failed to close proxy client: %v", err)
		}
	}()
	check := healthcheck.NewCheck(p, logger)

	rec := httptest.NewRecorder()
	check.HandleReadiness(rec, &http.Request{URL: &url.URL{}})

	resp := rec.Result()
	if got, want := resp.StatusCode, http.StatusServiceUnavailable; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}
}

func TestHandleReadinessWhenStopped(t *testing.T) {
	p := newTestProxy(t)
	defer func() {
		if err := p.Close(); err != nil {
			t.Logf("failed to close proxy client: %v", err)
		}
	}()
	check := healthcheck.NewCheck(p, logger)

	check.NotifyStarted() // The Proxy has started.
	check.NotifyStopped() // And now the Proxy is shutting down.

	rec := httptest.NewRecorder()
	check.HandleReadiness(rec, &http.Request{URL: &url.URL{}})

	resp := rec.Result()
	if got, want := resp.StatusCode, http.StatusServiceUnavailable; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}
}

func TestHandleReadinessForMaxConns(t *testing.T) {
	p := newTestProxyWithMaxConns(t, 1)
	defer func() {
		if err := p.Close(); err != nil {
			t.Logf("failed to close proxy client: %v", err)
		}
	}()
	started := make(chan struct{})
	check := healthcheck.NewCheck(p, logger)
	go p.Serve(context.Background(), func() {
		check.NotifyStarted()
		close(started)
	})
	select {
	case <-started:
		// proxy has started
	case <-time.After(10 * time.Second):
		t.Fatal("proxy has not started after 10 seconds")
	}

	conn := dialTCP(t, proxyAddr())
	defer conn.Close()

	// The proxy calls the dialer in a separate goroutine. So wait for that
	// goroutine to run before asserting on the readiness response.
	waitForConnect := func(t *testing.T, wantCode int) *http.Response {
		for i := 0; i < 10; i++ {
			rec := httptest.NewRecorder()
			check.HandleReadiness(rec, &http.Request{URL: &url.URL{}})
			resp := rec.Result()
			if resp.StatusCode == wantCode {
				return resp
			}
			time.Sleep(time.Second)
		}
		t.Fatalf("failed to receive status code = %v", wantCode)
		return nil
	}
	resp := waitForConnect(t, http.StatusServiceUnavailable)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !strings.Contains(string(body), "max connections") {
		t.Fatalf("want max connections error, got = %v", string(body))
	}
}
func TestHandleReadinessForMinReady(t *testing.T) {
	p := newTestProxy(t)
	defer func() {
		p.Close()
	}()
	started := make(chan struct{})
	check := healthcheck.NewCheck(p, logger)
	go p.Serve(context.Background(), func() {
		check.NotifyStarted()
		close(started)
	})
	select {
	case <-started:
		// proxy has started
	case <-time.After(10 * time.Second):
		t.Fatal("proxy has not started after 10 seconds")
	}

	conn := dialTCP(t, proxyAddr())
	defer conn.Close()

	minReadyTest := func(t *testing.T, minReady int, wantCode int) *http.Response {
		rec := httptest.NewRecorder()
		check.HandleReadiness(rec, &http.Request{URL: &url.URL{RawQuery: fmt.Sprintf("min-ready=%d", minReady)}})
		resp := rec.Result()
		if resp.StatusCode == wantCode {
			return resp
		}
		t.Fatalf("failed to receive status code = %v: %v", wantCode, resp.StatusCode)
		return nil
	}

	var resp *http.Response

	resp = minReadyTest(t, 0, http.StatusOK)
	resp = minReadyTest(t, 1, http.StatusOK)
	resp = minReadyTest(t, 2, http.StatusBadRequest)

	conn.Close()
	if err := p.Close(); err != nil {
		t.Logf("failed to close proxy client: %v", err)
	}

	resp = minReadyTest(t, 1, http.StatusServiceUnavailable)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !strings.Contains(string(body), "min ready") {
		t.Fatalf("want max connections error, got = %v", string(body))
	}
}

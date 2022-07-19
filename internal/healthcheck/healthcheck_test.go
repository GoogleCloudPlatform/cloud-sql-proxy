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
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/healthcheck"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/log"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
)

var (
	logger    = log.NewStdLogger(os.Stdout, os.Stdout)
	proxyHost = "127.0.0.1"
	proxyPort = 5000
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

type fakeDialer struct{}

func (*fakeDialer) Dial(ctx context.Context, inst string, opts ...cloudsqlconn.DialOption) (net.Conn, error) {
	conn, _ := net.Pipe()
	return conn, nil
}

func (*fakeDialer) EngineVersion(ctx context.Context, inst string) (string, error) {
	return "POSTGRES_14", nil
}

func (*fakeDialer) Close() error {
	return nil
}

type errorDialer struct {
	fakeDialer
}

func (*errorDialer) Dial(ctx context.Context, inst string, opts ...cloudsqlconn.DialOption) (net.Conn, error) {
	return nil, errors.New("errorDialer always errors")
}

func newProxyWithParams(t *testing.T, maxConns uint64, dialer cloudsql.Dialer) *proxy.Client {
	c := &proxy.Config{
		Addr: proxyHost,
		Port: proxyPort,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg"},
		},
		MaxConnections: maxConns,
	}
	p, err := proxy.NewClient(context.Background(), dialer, logger, c)
	if err != nil {
		t.Fatalf("proxy.NewClient: %v", err)
	}
	return p
}

func newTestProxyWithMaxConns(t *testing.T, maxConns uint64) *proxy.Client {
	return newProxyWithParams(t, maxConns, &fakeDialer{})
}

func newTestProxyWithDialer(t *testing.T, d cloudsql.Dialer) *proxy.Client {
	return newProxyWithParams(t, 0, d)
}

func newTestProxy(t *testing.T) *proxy.Client {
	return newProxyWithParams(t, 0, &fakeDialer{})
}

func TestHandleStartupWhenNotNotified(t *testing.T) {
	p := newTestProxy(t)
	defer p.Close()
	check := healthcheck.NewCheck(p, logger)

	rec := httptest.NewRecorder()
	check.HandleStartup(rec, &http.Request{})

	// Startup is not complete because the Check has not been notified of the
	// proxy's startup.
	resp := rec.Result()
	if got, want := resp.StatusCode, http.StatusServiceUnavailable; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}
}

func TestHandleStartupWhenNotified(t *testing.T) {
	p := newTestProxy(t)
	defer p.Close()
	check := healthcheck.NewCheck(p, logger)

	check.NotifyStarted()

	rec := httptest.NewRecorder()
	check.HandleStartup(rec, &http.Request{})

	resp := rec.Result()
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}
}

func TestHandleReadinessWhenNotNotified(t *testing.T) {
	p := newTestProxy(t)
	defer p.Close()
	check := healthcheck.NewCheck(p, logger)

	rec := httptest.NewRecorder()
	check.HandleReadiness(rec, &http.Request{})

	resp := rec.Result()
	if got, want := resp.StatusCode, http.StatusServiceUnavailable; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}
}

func TestHandleReadinessForMaxConns(t *testing.T) {
	p := newTestProxyWithMaxConns(t, 1)
	defer p.Close()
	check := healthcheck.NewCheck(p, logger)
	go p.Serve(context.Background(), check.NotifyStarted)

	conn := dialTCP(t, proxyAddr())
	defer conn.Close()

	// The proxy calls the dialer in a separate goroutine. So wait for that
	// goroutine to run before asserting on the readiness response.
	waitForConnect := func(t *testing.T, wantCode int) *http.Response {
		for i := 0; i < 10; i++ {
			rec := httptest.NewRecorder()
			check.HandleReadiness(rec, &http.Request{})
			resp := rec.Result()
			if resp.StatusCode == wantCode {
				return resp
			}
			time.Sleep(500 * time.Millisecond)
		}
		return nil
	}
	resp := waitForConnect(t, http.StatusServiceUnavailable)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !strings.Contains(string(body), "max connections") {
		t.Fatalf("want max connections error, got = %v", string(body))
	}
}

func TestHandleReadinessWithConnectionProblems(t *testing.T) {
	p := newTestProxyWithDialer(t, &errorDialer{}) // error dialer will error on dial
	defer p.Close()
	check := healthcheck.NewCheck(p, logger)
	check.NotifyStarted()

	rec := httptest.NewRecorder()
	check.HandleReadiness(rec, &http.Request{})

	resp := rec.Result()
	if got, want := resp.StatusCode, http.StatusServiceUnavailable; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if want := "errorDialer"; !strings.Contains(string(body), want) {
		t.Fatalf("want substring with = %q, got = %v", want, string(body))
	}
}

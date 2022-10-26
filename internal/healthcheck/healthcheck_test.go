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
	"sync/atomic"
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

type fakeDialer struct{}

func (*fakeDialer) Dial(_ context.Context, _ string, _ ...cloudsqlconn.DialOption) (net.Conn, error) {
	conn, _ := net.Pipe()
	return conn, nil
}

func (*fakeDialer) EngineVersion(_ context.Context, _ string) (string, error) {
	return "POSTGRES_14", nil
}

func (*fakeDialer) Close() error {
	return nil
}

type flakeyDialer struct {
	dialCount uint64
	fakeDialer
}

// Dial fails on odd calls and succeeds on even calls.
func (f *flakeyDialer) Dial(_ context.Context, _ string, _ ...cloudsqlconn.DialOption) (net.Conn, error) {
	c := atomic.AddUint64(&f.dialCount, 1)
	if c%2 == 0 {
		conn, _ := net.Pipe()
		return conn, nil
	}
	return nil, errors.New("flakey dialer fails on odd calls")
}

type errorDialer struct {
	fakeDialer
}

func (*errorDialer) Dial(_ context.Context, _ string, _ ...cloudsqlconn.DialOption) (net.Conn, error) {
	return nil, errors.New("errorDialer always errors")
}

func newProxyWithParams(t *testing.T, maxConns uint64, dialer cloudsql.Dialer, instances []proxy.InstanceConnConfig) *proxy.Client {
	c := &proxy.Config{
		Addr:           proxyHost,
		Port:           proxyPort,
		Instances:      instances,
		MaxConnections: maxConns,
	}
	p, err := proxy.NewClient(context.Background(), dialer, logger, c)
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

func newTestProxyWithDialer(t *testing.T, d cloudsql.Dialer) *proxy.Client {
	return newProxyWithParams(t, 0, d, []proxy.InstanceConnConfig{{Name: "proj:region:pg"}})
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

func TestHandleReadinessWithConnectionProblems(t *testing.T) {
	p := newTestProxyWithDialer(t, &errorDialer{}) // error dialer will error on dial
	defer func() {
		if err := p.Close(); err != nil {
			t.Logf("failed to close proxy client: %v", err)
		}
	}()
	check := healthcheck.NewCheck(p, logger)
	check.NotifyStarted()

	rec := httptest.NewRecorder()
	check.HandleReadiness(rec, &http.Request{URL: &url.URL{}})

	resp := rec.Result()
	if got, want := resp.StatusCode, http.StatusServiceUnavailable; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if want := "errorDialer"; !strings.Contains(string(body), want) {
		t.Fatalf("want substring with = %q, got = %v", want, string(body))
	}
}

func TestReadinessWithMinReady(t *testing.T) {
	tcs := []struct {
		desc       string
		minReady   string
		wantStatus int
		dialer     cloudsql.Dialer
	}{
		{
			desc:       "when min ready is zero",
			minReady:   "0",
			wantStatus: http.StatusBadRequest,
			dialer:     &fakeDialer{},
		},
		{
			desc:       "when min ready is less than zero",
			minReady:   "-1",
			wantStatus: http.StatusBadRequest,
			dialer:     &fakeDialer{},
		},
		{
			desc:       "when only one instance must be ready",
			minReady:   "1",
			wantStatus: http.StatusOK,
			dialer:     &flakeyDialer{}, // fails on first call, succeeds on second
		},
		{
			desc:       "when all instances must be ready",
			minReady:   "2",
			wantStatus: http.StatusServiceUnavailable,
			dialer:     &errorDialer{},
		},
		{
			desc:       "when min ready is greater than the number of instances",
			minReady:   "3",
			wantStatus: http.StatusBadRequest,
			dialer:     &fakeDialer{},
		},
		{
			desc:       "when min ready is bogus",
			minReady:   "bogus",
			wantStatus: http.StatusBadRequest,
			dialer:     &fakeDialer{},
		},
		{
			desc:       "when min ready is not set",
			minReady:   "",
			wantStatus: http.StatusOK,
			dialer:     &fakeDialer{},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			p := newProxyWithParams(t, 0,
				tc.dialer,
				[]proxy.InstanceConnConfig{
					{Name: "p:r:instance-1"},
					{Name: "p:r:instance-2"},
				},
			)
			defer func() {
				if err := p.Close(); err != nil {
					t.Logf("failed to close proxy client: %v", err)
				}
			}()

			check := healthcheck.NewCheck(p, logger)
			check.NotifyStarted()
			u, err := url.Parse(fmt.Sprintf("/readiness?min-ready=%s", tc.minReady))
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			check.HandleReadiness(rec, &http.Request{URL: u})

			resp := rec.Result()
			if got, want := resp.StatusCode, tc.wantStatus; got != want {
				t.Fatalf("want = %v, got = %v", want, got)
			}
		})
	}
}

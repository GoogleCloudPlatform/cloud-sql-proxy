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

package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

const instance = "project:region:instance"

var (
	sentinelError = errors.New("sentinel error")
	forever       = time.Date(9999, 0, 0, 0, 0, 0, 0, time.UTC)
)

type fakeCerts struct {
	sync.Mutex
	called int
}

type blockingCertSource struct {
	values     map[string]*fakeCerts
	validUntil time.Time
}

func (cs *blockingCertSource) Local(instance string) (tls.Certificate, error) {
	v, ok := cs.values[instance]
	if !ok {
		return tls.Certificate{}, fmt.Errorf("test setup failure: unknown instance %q", instance)
	}
	v.Lock()
	v.called++
	v.Unlock()

	// Returns a cert which is valid forever.
	return tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: cs.validUntil,
		},
	}, nil
}

func (cs *blockingCertSource) Remote(instance string) (cert *x509.Certificate, addr, name, version string, err error) {
	return &x509.Certificate{}, "fake address", "fake name", "fake version", nil
}

func newCertSource(certs *fakeCerts, expiration time.Time) CertSource {
	return &blockingCertSource{
		values: map[string]*fakeCerts{
			instance: certs,
		},
		validUntil: expiration,
	}
}

func newClient(cs CertSource) *Client {
	return &Client{
		Certs: cs,
		Dialer: func(string, string) (net.Conn, error) {
			return nil, sentinelError
		},
	}
}

func TestContextDialer(t *testing.T) {
	cs := newCertSource(&fakeCerts{}, forever)
	c := newClient(cs)

	c.ContextDialer = func(context.Context, string, string) (net.Conn, error) {
		return nil, sentinelError
	}
	c.Dialer = func(string, string) (net.Conn, error) {
		return nil, fmt.Errorf("this dialer should not be used when ContextDialer is set")
	}

	if _, err := c.DialContext(context.Background(), instance); err != sentinelError {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClientCache(t *testing.T) {
	b := &fakeCerts{}
	c := newClient(newCertSource(b, forever))

	for i := 0; i < 5; i++ {
		if _, err := c.Dial(instance); err != sentinelError {
			t.Errorf("unexpected error: %v", err)
		}
	}

	b.Lock()
	if b.called != 1 {
		t.Errorf("called %d times, want called 1 time", b.called)
	}
	b.Unlock()
}

func TestInvalidateConfigCache(t *testing.T) {
	srv := httptest.NewTLSServer(nil)
	defer srv.Close()
	b := &fakeCerts{}
	c := &Client{
		Certs: newCertSource(b, forever),
		Dialer: func(string, string) (net.Conn, error) {
			return net.Dial(
				srv.Listener.Addr().Network(),
				srv.Listener.Addr().String(),
			)
		},
	}
	c.cachedCfg(context.Background(), instance)
	if needsRefresh(c.cfgCache[instance], DefaultRefreshCfgBuffer) {
		t.Error("cached config expected to be valid")
	}
	_, err := c.Dial(instance)
	if err == nil {
		t.Errorf("c.Dial(%q) expected to fail with handshake error", instance)
	}
	if !needsRefresh(c.cfgCache[instance], DefaultRefreshCfgBuffer) {
		t.Error("cached config expected to be invalidated after handshake error")
	}
}

func TestValidClient(t *testing.T) {
	someErr := errors.New("error")
	openCh := make(chan struct{})
	closedCh := make(chan struct{})
	close(closedCh)

	equalErrors := func(a, b []*InvalidError) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i].instance != b[i].instance {
				return false
			}
			if a[i].err != b[i].err {
				return false
			}
			if a[i].hasTLS != b[i].hasTLS {
				return false
			}
		}
		return true
	}

	testCases := []struct {
		desc  string
		cache map[string]cacheEntry
		want  []*InvalidError
	}{
		{
			desc:  "when the cache has only valid entries",
			cache: map[string]cacheEntry{"proj:region:inst": cacheEntry{cfg: &tls.Config{}, done: closedCh}},
			want:  nil,
		},
		{
			desc:  "when the cache has invalid TLS entries",
			cache: map[string]cacheEntry{"proj:region:inst": cacheEntry{done: closedCh}},
			want:  []*InvalidError{&InvalidError{instance: "proj:region:inst", hasTLS: false}},
		},
		{
			desc:  "when the cache has errored entries",
			cache: map[string]cacheEntry{"proj:region:inst": cacheEntry{err: someErr, done: closedCh}},
			want:  []*InvalidError{&InvalidError{instance: "proj:region:inst", hasTLS: false, err: someErr}},
		},
		{
			desc:  "when the cache has an entry with an in-progress refresh",
			cache: map[string]cacheEntry{"proj:region:inst": cacheEntry{err: someErr, done: openCh}},
			want:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			client := &Client{cfgCache: tc.cache}
			if got := client.InvalidInstances(); !equalErrors(got, tc.want) {
				t.Errorf("want = %v, got = %v", tc.want, got)
			}
		})
	}
}

func TestConcurrentRefresh(t *testing.T) {
	b := &fakeCerts{}
	c := newClient(newCertSource(b, forever))

	ch := make(chan error)
	b.Lock()

	const numDials = 20

	for i := 0; i < numDials; i++ {
		go func() {
			_, err := c.Dial(instance)
			ch <- err
		}()
	}

	b.Unlock()

	for i := 0; i < numDials; i++ {
		if err := <-ch; err != sentinelError {
			t.Errorf("unexpected error: %v", err)
		}
	}
	b.Lock()
	if b.called != 1 {
		t.Errorf("called %d times, want called 1 time", b.called)
	}
	b.Unlock()
}

func TestMaximumConnectionsCount(t *testing.T) {
	certSource := &blockingCertSource{
		values:     map[string]*fakeCerts{},
		validUntil: forever,
	}
	c := newClient(certSource)

	const maxConnections = 10
	c.MaxConnections = maxConnections
	var dials uint64
	firstDialExited := make(chan struct{})
	c.Dialer = func(string, string) (net.Conn, error) {
		atomic.AddUint64(&dials, 1)
		// Wait until the first dial fails to ensure the max connections count
		// is reached by a concurrent dialer
		<-firstDialExited
		return nil, sentinelError
	}

	// Build certSource.values before creating goroutines to avoid concurrent map read and map write
	const numConnections = maxConnections + 1
	instanceNames := make([]string, numConnections)
	for i := 0; i < numConnections; i++ {
		// Vary instance name to bypass config cache and avoid second call to Client.tryConnect() in Client.Dial()
		instanceName := fmt.Sprintf("%s-%d", instance, i)
		certSource.values[instanceName] = &fakeCerts{}
		instanceNames[i] = instanceName
	}

	var wg sync.WaitGroup
	var firstDialOnce sync.Once
	for _, instanceName := range instanceNames {
		wg.Add(1)
		go func(instanceName string) {
			defer wg.Done()

			conn := Conn{
				Instance: instanceName,
				Conn:     &dummyConn{},
			}
			c.handleConn(context.Background(), conn)

			firstDialOnce.Do(func() { close(firstDialExited) })
		}(instanceName)
	}

	wg.Wait()

	switch {
	case dials > maxConnections:
		t.Errorf("client should have refused to dial new connection on %dth attempt when the maximum of %d connections was reached (%d dials)", numConnections, maxConnections, dials)
	case dials == maxConnections:
		t.Logf("client has correctly refused to dial new connection on %dth attempt when the maximum of %d connections was reached (%d dials)\n", numConnections, maxConnections, dials)
	case dials < maxConnections:
		t.Errorf("client should have dialed exactly the maximum of %d connections (%d connections, %d dials)", maxConnections, numConnections, dials)
	}
}

func TestShutdownTerminatesEarly(t *testing.T) {
	cs := newCertSource(&fakeCerts{}, forever)
	c := newClient(cs)
	// Ensure the dialer returns no error.
	c.Dialer = func(string, string) (net.Conn, error) {
		return nil, nil
	}

	shutdown := make(chan bool, 1)
	go func() {
		c.Shutdown(1)
		shutdown <- true
	}()
	shutdownFinished := false
	// In case the code is actually broken and the client doesn't shut down quickly, don't cause the test to hang until it times out.
	select {
	case <-time.After(100 * time.Millisecond):
	case shutdownFinished = <-shutdown:
	}
	if !shutdownFinished {
		t.Errorf("shutdown should have completed quickly because there are no active connections")
	}
}

func TestRefreshTimer(t *testing.T) {
	timeToExpire := 2 * time.Second
	certCreated := time.Now()
	cs := newCertSource(&fakeCerts{}, certCreated.Add(timeToExpire))
	c := newClient(cs)

	c.RefreshCfgThrottle = 20 * time.Millisecond
	c.RefreshCfgBuffer = time.Second

	// Call Dial to cache the cert.
	if _, err := c.Dial(instance); err != sentinelError {
		t.Fatalf("Dial(%s) failed: %v", instance, err)
	}
	c.cacheL.Lock()
	cfg, ok := c.cfgCache[instance]
	c.cacheL.Unlock()
	if !ok {
		t.Fatalf("expected instance to be cached")
	}

	time.Sleep(timeToExpire - time.Since(certCreated))
	// Check if cert was refreshed in the background, without calling Dial again.
	c.cacheL.Lock()
	newCfg, ok := c.cfgCache[instance]
	c.cacheL.Unlock()
	if !ok {
		t.Fatalf("expected instance to be cached")
	}
	if !newCfg.lastRefreshed.After(cfg.lastRefreshed) {
		t.Error("expected cert to be refreshed.")
	}
}

func TestSyncAtomicAlignment(t *testing.T) {
	// The sync/atomic pkg has a bug that requires the developer to guarantee
	// 64-bit alignment when using 64-bit functions on 32-bit systems.
	c := &Client{}
	if a := unsafe.Offsetof(c.ConnectionsCounter); a%64 != 0 {
		t.Errorf("Client.ConnectionsCounter is not aligned: want %v, got %v", 0, a)
	}
}

type invalidRemoteCertSource struct{}

func (cs *invalidRemoteCertSource) Local(instance string) (tls.Certificate, error) {
	return tls.Certificate{}, nil
}

func (cs *invalidRemoteCertSource) Remote(instance string) (*x509.Certificate, string, string, string, error) {
	return nil, "", "", "", sentinelError
}

func TestRemoteCertError(t *testing.T) {
	c := newClient(&invalidRemoteCertSource{})

	_, err := c.DialContext(context.Background(), instance)
	if err != sentinelError {
		t.Errorf("expected sentinel error, got %v", err)
	}

}

func TestParseInstanceConnectionName(t *testing.T) {
	// SplitName has its own tests and is not specifically tested here.
	table := []struct {
		in           string
		wantErrorStr string
	}{
		{"proj:region:my-db", ""},
		{"proj:region:my-db=options", ""},
		{"proj=region=my-db", "invalid instance argument: must be either form - `<instance_connection_string>` or `<instance_connection_string>=<options>`; invalid arg was \"proj=region=my-db\""},
		{"projregionmy-db", "invalid instance connection string: must be in the form `project:region:instance-name`; invalid name was \"projregionmy-db\""},
	}

	for _, test := range table {
		_, _, _, _, gotError := ParseInstanceConnectionName(test.in)
		var gotErrorStr string
		if gotError != nil {
			gotErrorStr = gotError.Error()
		}
		if gotErrorStr != test.wantErrorStr {
			t.Errorf("ParseInstanceConnectionName(%q): got \"%v\" for error, want \"%v\"", test.in, gotErrorStr, test.wantErrorStr)
		}
	}
}

type localhostCertSource struct {
}

func (c localhostCertSource) Local(instance string) (tls.Certificate, error) {
	return tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: forever,
		},
	}, nil
}

func (c localhostCertSource) Remote(instance string) (cert *x509.Certificate, addr, name, version string, err error) {
	return &x509.Certificate{}, "localhost", "fake name", "fake version", nil
}

var _ CertSource = &localhostCertSource{}

func TestClientHandshakeCanceled(t *testing.T) {
	errorIsDeadlineOrTimeout := func(err error) bool {
		if errors.Is(err, context.Canceled) {
			return true
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return true
		}
		if strings.Contains(err.Error(), "i/o timeout") {
			// We should use os.ErrDeadlineExceeded exceeded here,
			// but it is not present in Go versions below 1.15.
			return true
		}
		return false
	}

	withTestHarness := func(t *testing.T, f func(port int)) {
		// serverShutdown is closed to free the server
		// goroutine that is holding up the client request.
		serverShutdown := make(chan struct{})

		l, err := tls.Listen(
			"tcp",
			":",
			&tls.Config{
				GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
					// Make the client wait forever to handshake.
					<-serverShutdown
					return nil, errors.New("some error")
				},
			})
		if err != nil {
			t.Fatalf("tls.Listen: %v", err)
		}

		port := l.Addr().(*net.TCPAddr).Port

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				conn, err := l.Accept()
				if err != nil {
					// Below Go 1.16, we have to string match here.
					// https://golang.org/doc/go1.16#net
					if !strings.Contains(err.Error(), "use of closed network connection") {
						t.Errorf("l.Accept: %v", err)
					}
					return
				}

				_, _ = ioutil.ReadAll(conn) // Trigger the handshake.
				_ = conn.Close()
			}
		}()

		f(port)
		close(serverShutdown) // Free the server thread.
		_ = l.Close()
		wg.Wait()
	}

	validateError := func(t *testing.T, err error) {
		if err == nil {
			t.Fatal("nil error unexpected")
		}
		if !errorIsDeadlineOrTimeout(err) {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	newClient := func(port int) *Client {
		return &Client{
			Port:  port,
			Certs: &localhostCertSource{},
		}
	}

	// Makes it to Handshake.
	t.Run("with timeout", func(t *testing.T) {
		withTestHarness(t, func(port int) {
			c := newClient(port)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			_, err := c.DialContext(ctx, instance)
			validateError(t, err)
		})
	})

	t.Run("when liveness check is called on invalidated config", func(t *testing.T) {
		withTestHarness(t, func(port int) {
			c := newClient(port)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			_, err := c.DialContext(ctx, instance)
			if err == nil {
				t.Fatal("expected DialContext to fail, got no error")
			}

			invalid := c.InvalidInstances()
			if gotLen := len(invalid); gotLen != 1 {
				t.Fatalf("invalid instance want = 1, got = %v", gotLen)
			}
			got := invalid[0]
			if got.err == nil {
				t.Fatal("want invalid instance error, got nil")
			}
		})
	})

	// Makes it to Handshake.
	// Same as the above but the context doesn't have a deadline,
	// it is canceled manually after a while.
	t.Run("canceled after a while, no deadline", func(t *testing.T) {
		withTestHarness(t, func(port int) {
			c := newClient(port)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			time.AfterFunc(3*time.Second, cancel)

			_, err := c.DialContext(ctx, instance)
			validateError(t, err)
		})

	})

	// Doesn't make it to Handshake.
	t.Run("with short timeout", func(t *testing.T) {
		withTestHarness(t, func(port int) {
			c := newClient(port)

			ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
			defer cancel()

			_, err := c.DialContext(ctx, instance)
			validateError(t, err)
		})
	})

	// Doesn't make it to Handshake.
	t.Run("canceled without timeout", func(t *testing.T) {
		withTestHarness(t, func(port int) {
			c := newClient(port)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := c.DialContext(ctx, instance)
			validateError(t, err)
		})
	})
}

func TestConnectingWithInvalidConfig(t *testing.T) {
	c := &Client{}

	_, err := c.tryConnect(context.Background(), "", "myinstance", &tls.Config{})
	if err != ErrUnexpectedFailure {
		t.Fatalf("wanted ErrUnexpectedFailure, got = %v", err)
	}
}

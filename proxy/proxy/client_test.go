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
	"net"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

const instance = "instance-name"

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
			c.handleConn(conn)

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

func TestValidate(t *testing.T) {
	// SplitName has its own tests and is not specifically tested here.
	table := []struct {
		in           string
		wantValid    bool
		wantErrorStr string
	}{
		{"proj:region:my-db", true, ""},
		{"proj:region:my-db=options", true, ""},
		{"proj=region=my-db", false, "invalid instance argument: must be either form - `<instance_connection_string>` or `<instance_connection_string>=<options>`; invalid arg was \"proj=region=my-db\""},
		{"projregionmy-db", false, "invalid instance connection string: must be in the form `project:region:instance-name`; invalid name was \"projregionmy-db\""},
	}

	for _, test := range table {
		_, _, _, _, gotError := ParseInstanceConnectionName(test.in)
		var gotErrorStr string
		if gotError != nil {
			gotErrorStr = gotError.Error()
		}
		if gotErrorStr != test.wantErrorStr {
			t.Errorf("Validate(%q): got \"%v\" for error, want \"%v\"", test.in, gotError, test.wantErrorStr)
		}
	}
}

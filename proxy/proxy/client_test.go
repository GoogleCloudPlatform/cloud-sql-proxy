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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

const instance = "instance-name"

var (
	errFakeDial = errors.New("this error is returned by the dialer")
	forever     = time.Date(9999, 0, 0, 0, 0, 0, 0, time.UTC)
)

type fakeCerts struct {
	sync.Mutex
	called int
}

type blockingCertSource struct {
	values     map[string]*fakeCerts
	validUntil time.Time
	server     *tls.Certificate
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
	// Refresh the server certificate if necessary.
	server, err := cs.ServerCertificate()
	if err != nil {
		return nil, "", "", "", err
	}

	scert, err := x509.ParseCertificate(server.Certificate[0])
	if err != nil {
		return nil, "", "", "", fmt.Errorf("unable to parse certifcate data: %v", err)
	}

	return scert, "fake address", "fake name", "fake version", nil
}

func (cs *blockingCertSource) ServerCertificate() (*tls.Certificate, error) {
	// Generate a new self signed server certificate.
	if cs.server == nil {
		var err error
		cs.server, err = cs.selfSigned()
		if err != nil {
			return nil, err
		}
	}

	return cs.server, nil
}

func (cs *blockingCertSource) selfSigned() (*tls.Certificate, error) {
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "fake name",
			Organization: []string{"fake company"},
		},
		NotAfter:              cs.validUntil,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create server certificate: %v", err)
	}

	return &tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}, nil
}

func TestContextDialer(t *testing.T) {
	b := &fakeCerts{}
	c := &Client{
		Certs: &blockingCertSource{
			map[string]*fakeCerts{
				instance: b,
			},
			forever,
			nil,
		},
		ContextDialer: func(context.Context, string, string) (net.Conn, error) {
			return nil, errFakeDial
		},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, fmt.Errorf("this dialer should't be used when ContextDialer is set")
		},
	}

	if _, err := c.DialContext(context.Background(), instance); err != errFakeDial {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClientCache(t *testing.T) {
	b := &fakeCerts{}
	c := &Client{
		Certs: &blockingCertSource{
			map[string]*fakeCerts{
				instance: b,
			},
			forever,
			nil,
		},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, errFakeDial
		},
	}

	for i := 0; i < 5; i++ {
		if _, err := c.Dial(instance); err != errFakeDial {
			t.Errorf("unexpected error: %v", err)
		}
	}

	b.Lock()
	if b.called != 1 {
		t.Errorf("called %d times, want called 1 time", b.called)
	}
	b.Unlock()
}

func TestConcurrentRefresh(t *testing.T) {
	b := &fakeCerts{}
	c := &Client{
		Certs: &blockingCertSource{
			map[string]*fakeCerts{
				instance: b,
			},
			forever,
			nil,
		},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, errFakeDial
		},
	}

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
		if err := <-ch; err != errFakeDial {
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
	const maxConnections = 10
	const numConnections = maxConnections + 1
	var dials uint64 = 0

	b := &fakeCerts{}
	certSource := blockingCertSource{
		map[string]*fakeCerts{},
		forever,
		nil,
	}
	firstDialExited := make(chan struct{})
	c := &Client{
		Certs: &certSource,
		Dialer: func(string, string) (net.Conn, error) {
			atomic.AddUint64(&dials, 1)

			// Wait until the first dial fails to ensure the max connections count is reached by a concurrent dialer
			<-firstDialExited

			return nil, errFakeDial
		},
		MaxConnections: maxConnections,
	}

	// Build certSource.values before creating goroutines to avoid concurrent map read and map write
	instanceNames := make([]string, numConnections)
	for i := 0; i < numConnections; i++ {
		// Vary instance name to bypass config cache and avoid second call to Client.tryConnect() in Client.Dial()
		instanceName := fmt.Sprintf("%s-%d", instance, i)
		certSource.values[instanceName] = b
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
				Conn:     newDummyConn(nil, nil, nil),
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
	b := &fakeCerts{}
	c := &Client{
		Certs: &blockingCertSource{
			map[string]*fakeCerts{
				instance: b,
			},
			forever,
			nil,
		},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, nil
		},
	}
	shutdown := make(chan bool, 1)
	go func() {
		c.Shutdown(1, false, 0)
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

func TestShutdownDrainsIdleConnections(t *testing.T) {
	b := &fakeCerts{}
	cs := &blockingCertSource{
		map[string]*fakeCerts{
			instance: b,
		},
		forever,
		nil,
	}
	c := &Client{
		Certs: cs,
		Conns: NewConnSet(),
	}

	cert, err := cs.ServerCertificate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	cfg := &tls.Config{
		Certificates:       []tls.Certificate{*cert},
		InsecureSkipVerify: true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.Dialer = func(string, string) (net.Conn, error) {
		localReader, localWriter := io.Pipe()
		remoteReader, remoteWriter := io.Pipe()

		conn := tls.Server(newDummyConn(ctx, remoteReader, localWriter), cfg)
		go func() {
			defer conn.Close()

			buf := make([]byte, 4096)
			for {
				if _, err := conn.Read(buf); err != nil {
					if err == io.EOF {
						break
					}

					t.Errorf("unexpected error: %v", err)
				}
			}
		}()

		return newDummyConn(ctx, localReader, remoteWriter), nil
	}

	for i := 0; i < 10; i++ {
		go c.handleConn(Conn{
			Instance: instance,
			Conn:     newDummyConn(nil, nil, nil),
		})
	}

	time.Sleep(50 * time.Millisecond)

	shutdown := make(chan bool, 1)
	go func() {
		c.Shutdown(time.Second, true, 50*time.Millisecond)
		shutdown <- true
	}()
	shutdownFinished := false

	select {
	case <-time.After(200 * time.Millisecond):
	case shutdownFinished = <-shutdown:
	}

	if !shutdownFinished {
		t.Errorf("shutdown should have completed quickly because there are no active connections")
	}
}

func TestRefreshTimer(t *testing.T) {
	timeToExpire := 5 * time.Second
	b := &fakeCerts{}
	certCreated := time.Now()
	c := &Client{
		Certs: &blockingCertSource{
			map[string]*fakeCerts{
				instance: b,
			},
			certCreated.Add(timeToExpire),
			nil,
		},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, errFakeDial
		},
		RefreshCfgThrottle: 20 * time.Millisecond,
		RefreshCfgBuffer:   time.Second,
	}
	// Call Dial to cache the cert.
	if _, err := c.Dial(instance); err != errFakeDial {
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
	// The sync/atomic pkg has a bug that requires the developer to guarantee 64-bit alignment when using 64-bit functions on 32-bit systems.
	c := &Client{}
	if a := unsafe.Offsetof(c.ConnectionsCounter); a%64 != 0 {
		t.Errorf("Client.ConnectionsCounter is not aligned: want %v, got %v", 0, a)
	}
}

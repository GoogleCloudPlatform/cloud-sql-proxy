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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func (cs *blockingCertSource) Remote(instance string) (cert *x509.Certificate, addr, name string, err error) {
	return &x509.Certificate{}, "fake address", "fake name", nil
}

func TestClientCache(t *testing.T) {
	b := &fakeCerts{}
	c := &Client{
		Certs: &blockingCertSource{
			map[string]*fakeCerts{
				instance: b,
			},
			forever,
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

func TestRefreshTimer(t *testing.T) {
	refreshCertBuffer = time.Millisecond * 10
	timeToExpire := time.Millisecond * 500
	b := &fakeCerts{}
	c := &Client{
		Certs: &blockingCertSource{
			map[string]*fakeCerts{
				instance: b,
			},
			time.Now().Add(timeToExpire),
		},
		Dialer: func(string, string) (net.Conn, error) {
			return nil, errFakeDial
		},
		RefreshCfgThrottle: 20 * time.Millisecond,
	}

	// Call Dial to cache the cert.
	if _, err := c.Dial(instance); err != errFakeDial {
		t.Errorf("unexpected error: %v", err)
	}

	c.cacheL.Lock()
	cached, ok := c.cfgCache[instance]
	c.cacheL.Unlock()
	if !ok {
		t.Error("expected instance to be cached")
	}
	waitTil := time.After(timeToExpire + (10 * time.Millisecond))
loop:
	for {
		select {
		case <-waitTil:
			break loop
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Verify cert was refreshed in the background, without calling Dial again.
	c.cacheL.Lock()
	refreshed, ok := c.cfgCache[instance]
	c.cacheL.Unlock()
	if !ok {
		t.Error("expected instance to be cached")
	}
	if !refreshed.lastRefreshed.After(cached.lastRefreshed) {
		t.Error("expected cert to be refreshed.")
	}
}

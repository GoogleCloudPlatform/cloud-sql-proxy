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
	"os"
	"os/exec"
	"io/ioutil"
	"strings"
	"syscall"
)

const instance = "instance-name"

var errFakeDial = errors.New("this error is returned by the dialer")

type fakeCerts struct {
	sync.Mutex
	called int
}

type blockingCertSource struct {
	values map[string]*fakeCerts
}

func (cs *blockingCertSource) Local(instance string) (tls.Certificate, error) {
	v, ok := cs.values[instance]
	if !ok {
		return tls.Certificate{}, fmt.Errorf("test setup failure: unknown instance %q", instance)
	}
	v.Lock()
	v.called++
	v.Unlock()

	validUntil, _ := time.Parse("2006", "9999")
	// Returns a cert which is valid forever.
	return tls.Certificate{
		Leaf: &x509.Certificate{
			NotAfter: validUntil,
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
			}},
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
			}},
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
		map[string]*fakeCerts{}}
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

func TestGracefulTermination(t *testing.T) {
	if os.Getenv("BE_TERMINATOR") == "1" {
		var dials uint64 = 0
		t.Logf("dials")
		b := &fakeCerts{}
		certSource := blockingCertSource{
			map[string]*fakeCerts{}}
		firstDialExited := make(chan struct{})
		c := &Client{
			Certs: &certSource,
			Dialer: func(string, string) (net.Conn, error) {
				atomic.AddUint64(&dials, 1)

				// Wait until the first dial fails to make sure we get to the point where connections are refused
				<-firstDialExited

				return nil, errFakeDial
			},
			TerminationGracePeriod: time.Second * 1,
		}
		go c.closeOnSigterm()

		// 10 connections with a 200 sleep should be a good enough spread if we wait 1 seconds before sending the
		// sigterm signal - we want to see some refused connections
		numConnections := 10
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
			time.Sleep(time.Millisecond * 200)
		}

		wg.Wait()
	}

	// Start the actual test in a different subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestGracefulTermination")
	cmd.Env = append(os.Environ(), "BE_TERMINATOR=1")
	stdout, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second * 1)

	//syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
	cmd.Process.Signal(syscall.SIGTERM)

	// Check that the log fatal message is what we expected
	gotBytes, _ := ioutil.ReadAll(stdout)
	got := string(gotBytes)

	expectedStrings := []string{
		"received SIGTERM, waiting",
		"refusing connection - received termination signal", // expect at least some refused connections
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(got, expected) {
			t.Fatalf("Unexpected log message. Got %s but should contain %s", got, expected)
		}
	}

	// Check that the program exited with success (0)
	err := cmd.Wait()
	if err != nil {
		t.Fatalf("Process ran with err %v, want exit status 0", err)
	}

}
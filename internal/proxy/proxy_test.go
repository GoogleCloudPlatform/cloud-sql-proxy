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

package proxy_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/log"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
)

var testLogger = log.NewStdLogger(os.Stdout, os.Stdout)

type fakeDialer struct {
	mu                 sync.Mutex
	dialCount          int
	engineVersionCount int
	instances          []string
}

func (*fakeDialer) Close() error {
	return nil
}

func (f *fakeDialer) dialAttempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dialCount
}

func (f *fakeDialer) engineVersionAttempts() int { //nolint:unused
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.engineVersionCount
}

func (f *fakeDialer) dialedInstances() []string { //nolint:unused
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.instances...)
}

func (f *fakeDialer) Dial(_ context.Context, inst string, _ ...cloudsqlconn.DialOption) (net.Conn, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dialCount++
	f.instances = append(f.instances, inst)
	c1, _ := net.Pipe()
	return c1, nil
}

func (f *fakeDialer) EngineVersion(_ context.Context, inst string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.engineVersionCount++
	switch {
	case strings.Contains(inst, "pg"):
		return "POSTGRES_14", nil
	case strings.Contains(inst, "mysql"):
		return "MYSQL_8_0", nil
	case strings.Contains(inst, "sqlserver"):
		return "SQLSERVER_2019_STANDARD", nil
	case strings.Contains(inst, "fakeserver"):
		return "", fmt.Errorf("non existing server")
	default:
		return "POSTGRES_14", nil
	}
}

type errorDialer struct {
	fakeDialer
}

func (*errorDialer) Dial(_ context.Context, _ string, _ ...cloudsqlconn.DialOption) (net.Conn, error) {
	return nil, errors.New("errorDialer returns error on Dial")
}

func (*errorDialer) Close() error {
	return errors.New("errorDialer returns error on Close")
}

func createTempDir(t *testing.T) (string, func()) {
	testDir, err := os.MkdirTemp("", "*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return testDir, func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("failed to cleanup temp dir: %v", err)
		}
	}
}

func TestClientInitialization(t *testing.T) {
	ctx := context.Background()
	testDir, cleanup := createTempDir(t)
	testUnixSocketPath := path.Join(testDir, "db")
	testUnixSocketPathPg := path.Join(testDir, "db", ".s.PGSQL.5432")

	defer cleanup()

	tcs := []struct {
		desc          string
		in            *proxy.Config
		wantTCPAddrs  []string
		wantUnixAddrs []string
	}{
		{
			desc: "multiple instances",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 50000,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg},
					{Name: mysql},
					{Name: sqlserver},
				},
			},
			wantTCPAddrs: []string{"127.0.0.1:50000", "127.0.0.1:50001", "127.0.0.1:50002"},
		},
		{
			desc: "with instance address",
			in: &proxy.Config{
				Addr: "1.1.1.1", // bad address, binding shouldn't happen here.
				Port: 50003,
				Instances: []proxy.InstanceConnConfig{
					{Addr: "0.0.0.0", Name: pg},
				},
			},
			wantTCPAddrs: []string{"0.0.0.0:50003"},
		},
		{
			desc: "IPv6 support",
			in: &proxy.Config{
				Addr: "::1",
				Port: 50004,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg},
				},
			},
			wantTCPAddrs: []string{"[::1]:50004"},
		},
		{
			desc: "with instance port",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 50005,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg, Port: 60000},
				},
			},
			wantTCPAddrs: []string{"127.0.0.1:60000"},
		},
		{
			desc: "with global port and instance port",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 50006,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg},
					{Name: mysql, Port: 60001},
					{Name: sqlserver},
				},
			},
			wantTCPAddrs: []string{
				"127.0.0.1:50006",
				"127.0.0.1:60001",
				"127.0.0.1:50007",
			},
		},
		{
			desc: "with incrementing automatic port selection",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Instances: []proxy.InstanceConnConfig{
					{Name: pg},
					{Name: pg2},
					{Name: mysql},
					{Name: mysql2},
					{Name: sqlserver},
					{Name: sqlserver2},
				},
			},
			wantTCPAddrs: []string{
				"127.0.0.1:5432",
				"127.0.0.1:5433",
				"127.0.0.1:3306",
				"127.0.0.1:3307",
				"127.0.0.1:1433",
				"127.0.0.1:1434",
			},
		},
		{
			desc: "with a Unix socket",
			in: &proxy.Config{
				UnixSocket: testDir,
				Instances: []proxy.InstanceConnConfig{
					{Name: mysql},
				},
			},
			wantUnixAddrs: []string{
				filepath.Join(testDir, mysql),
			},
		},
		{
			desc: "with a global TCP host port and an instance Unix socket",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 50008,
				Instances: []proxy.InstanceConnConfig{
					{Name: mysql, UnixSocket: testDir},
				},
			},
			wantUnixAddrs: []string{
				filepath.Join(testDir, mysql),
			},
		},
		{
			desc: "with a global Unix socket and an instance TCP port",
			in: &proxy.Config{
				Addr:       "127.0.0.1",
				UnixSocket: testDir,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg, Port: 50009},
				},
			},
			wantTCPAddrs: []string{
				"127.0.0.1:50009",
			},
		},
		{
			desc: "with a Unix socket for Postgres",
			in: &proxy.Config{
				UnixSocket: testDir,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg},
				},
			},
			wantUnixAddrs: []string{
				filepath.Join(testDir, pg, ".s.PGSQL.5432"),
			},
		},
		{
			desc: "with a Unix socket path per instance",
			in: &proxy.Config{
				Instances: []proxy.InstanceConnConfig{
					{Name: mysql, UnixSocketPath: testUnixSocketPath},
				},
			},
			wantUnixAddrs: []string{
				filepath.Join(testUnixSocketPath),
			},
		},
		{
			desc: "with a Unix socket path overriding Unix socket",
			in: &proxy.Config{
				UnixSocket: testDir,
				Instances: []proxy.InstanceConnConfig{
					{Name: mysql, UnixSocketPath: testUnixSocketPath},
				},
			},
			wantUnixAddrs: []string{
				filepath.Join(testUnixSocketPath),
			},
		},
		{
			desc: "with a Unix socket path per pg instance",
			in: &proxy.Config{
				Instances: []proxy.InstanceConnConfig{
					{Name: pg, UnixSocketPath: testUnixSocketPath},
				},
			},
			wantUnixAddrs: []string{
				filepath.Join(testUnixSocketPathPg),
			},
		},
		{
			desc: "with a Unix socket path per pg instance and explicit pg path suffix",
			in: &proxy.Config{
				Instances: []proxy.InstanceConnConfig{
					{Name: pg, UnixSocketPath: testUnixSocketPathPg},
				},
			},
			wantUnixAddrs: []string{
				filepath.Join(testUnixSocketPathPg),
			},
		},
		{
			desc: "with TCP port for non functional instance",
			in: &proxy.Config{
				Instances: []proxy.InstanceConnConfig{
					{Name: "proj:region:fakeserver", Port: 50010},
				},
			},
			wantTCPAddrs: []string{
				"127.0.0.1:50010",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c, err := proxy.NewClient(ctx, &fakeDialer{}, testLogger, tc.in, nil)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					t.Logf("failed to close client: %v", err)
				}
			}()
			for _, addr := range tc.wantTCPAddrs {
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					t.Fatalf("want error = nil, got = %v", err)
				}
				err = conn.Close()
				if err != nil {
					t.Logf("failed to close connection: %v", err)
				}
			}

			for _, addr := range tc.wantUnixAddrs {
				verifySocketPermissions(t, addr)

				conn, err := net.Dial("unix", addr)
				if err != nil {
					t.Fatalf("want error = nil, got = %v", err)
				}
				err = conn.Close()
				if err != nil {
					t.Logf("failed to close connection: %v", err)
				}
			}
		})
	}
}

func TestClientLimitsMaxConnections(t *testing.T) {
	d := &fakeDialer{}
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 50011,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg"},
		},
		MaxConnections: 1,
	}
	callbackGot := 0
	connRefuseNotify := func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		callbackGot++
	}
	c, err := proxy.NewClient(context.Background(), d, testLogger, in, connRefuseNotify)
	if err != nil {
		t.Fatalf("proxy.NewClient error: %v", err)
	}
	defer c.Close()
	go c.Serve(context.Background(), func() {})

	conn1, err1 := net.Dial("tcp", "127.0.0.1:50011")
	if err1 != nil {
		t.Fatalf("net.Dial error: %v", err1)
	}
	defer conn1.Close()

	conn2, err2 := net.Dial("tcp", "127.0.0.1:50011")
	if err2 != nil {
		t.Fatalf("net.Dial error: %v", err1)
	}
	defer conn2.Close()

	wantEOF := func(t *testing.T, conns ...net.Conn) {
		for _, c := range conns {
			// Set a read deadline so any open connections will error on an i/o
			// timeout instead of hanging indefinitely.
			c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			_, err := c.Read(make([]byte, 1))
			if err == io.EOF {
				return
			}
		}
		t.Fatal("neither connection returned an io.EOF")
	}

	// either conn1 or conn2 should be closed
	// it doesn't matter which is closed
	wantEOF(t, conn1, conn2)

	want := 1
	if got := d.dialAttempts(); got != want {
		t.Fatalf("dial attempts did not match expected, want = %v, got = %v", want, got)
	}

	if callbackGot == 0 {
		t.Fatal("connRefuseNotifyCallback is not called")
	}
}

func tryTCPDial(t *testing.T, addr string) net.Conn {
	attempts := 10
	var (
		conn net.Conn
		err  error
	)
	for i := 0; i < attempts; i++ {
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		// Give the socket some time to finish the connection.
		time.Sleep(100 * time.Millisecond)
		return conn
	}

	t.Fatalf("failed to dial in %v attempts: %v", attempts, err)
	return nil
}

func TestClientCloseWaitsForActiveConnections(t *testing.T) {
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 50012,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg"},
		},
		WaitOnClose: 1 * time.Second,
	}
	c, err := proxy.NewClient(context.Background(), &fakeDialer{}, testLogger, in, nil)
	if err != nil {
		t.Fatalf("proxy.NewClient error: %v", err)
	}
	go c.Serve(context.Background(), func() {})

	conn := tryTCPDial(t, "127.0.0.1:50012")
	defer conn.Close()

	if err := c.Close(); err == nil {
		t.Fatal("c.Close should error, got = nil")
	}
}

func TestClientClosesCleanly(t *testing.T) {
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 50013,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:reg:inst"},
		},
	}
	c, err := proxy.NewClient(context.Background(), &fakeDialer{}, testLogger, in, nil)
	if err != nil {
		t.Fatalf("proxy.NewClient error want = nil, got = %v", err)
	}
	go c.Serve(context.Background(), func() {})

	conn := tryTCPDial(t, "127.0.0.1:50013")
	_ = conn.Close()

	if err := c.Close(); err != nil {
		t.Fatalf("c.Close() error = %v", err)
	}
}

func TestClosesWithError(t *testing.T) {
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 50014,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:reg:inst"},
		},
	}
	c, err := proxy.NewClient(context.Background(), &errorDialer{}, testLogger, in, nil)
	if err != nil {
		t.Fatalf("proxy.NewClient error want = nil, got = %v", err)
	}
	go c.Serve(context.Background(), func() {})

	conn := tryTCPDial(t, "127.0.0.1:50014")
	defer conn.Close()

	if err = c.Close(); err == nil {
		t.Fatal("c.Close() should error, got nil")
	}
}

func TestMultiErrorFormatting(t *testing.T) {
	tcs := []struct {
		desc string
		in   proxy.MultiErr
		want string
	}{
		{
			desc: "with one error",
			in:   proxy.MultiErr{errors.New("woops")},
			want: "woops",
		},
		{
			desc: "with many errors",
			in:   proxy.MultiErr{errors.New("woops"), errors.New("another error")},
			want: "woops, another error",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			if got := tc.in.Error(); got != tc.want {
				t.Errorf("want = %v, got = %v", tc.want, got)
			}
		})
	}
}

func TestClientInitializationWorksRepeatedly(t *testing.T) {
	// The client creates a Unix socket on initial startup and does not remove
	// it on shutdown. This test ensures the existing socket does not cause
	// problems for a second invocation.
	ctx := context.Background()
	testDir, cleanup := createTempDir(t)
	defer cleanup()

	in := &proxy.Config{
		UnixSocket: testDir,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg"},
		},
	}

	c, err := proxy.NewClient(ctx, &fakeDialer{}, testLogger, in, nil)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}
	c.Close()

	c, err = proxy.NewClient(ctx, &fakeDialer{}, testLogger, in, nil)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}
	c.Close()
}

func TestClientNotifiesCallerOnServe(t *testing.T) {
	ctx := context.Background()
	in := &proxy.Config{
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg"},
		},
	}
	c, err := proxy.NewClient(ctx, &fakeDialer{}, testLogger, in, nil)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}
	done := make(chan struct{})
	notify := func() { close(done) }

	go c.Serve(ctx, notify)

	verifyNotification := func(t *testing.T, ch <-chan struct{}) {
		for i := 0; i < 10; i++ {
			select {
			case <-ch:
				return
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
		t.Fatal("channel should have been closed but was not")
	}
	verifyNotification(t, done)
}

func TestClientConnCount(t *testing.T) {
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 50015,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg"},
		},
		MaxConnections: 10,
	}

	c, err := proxy.NewClient(context.Background(), &fakeDialer{}, testLogger, in, nil)
	if err != nil {
		t.Fatalf("proxy.NewClient error: %v", err)
	}
	defer c.Close()
	go c.Serve(context.Background(), func() {})

	gotOpen, gotMax := c.ConnCount()
	if gotOpen != 0 {
		t.Fatalf("want 0 open connections, got = %v", gotOpen)
	}
	if gotMax != 10 {
		t.Fatalf("want 10 max connections, got = %v", gotMax)
	}

	conn := tryTCPDial(t, "127.0.0.1:50015")
	defer conn.Close()

	verifyOpen := func(t *testing.T, want uint64) {
		var got uint64
		for i := 0; i < 10; i++ {
			got, _ = c.ConnCount()
			if got == want {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("open connections, want = %v, got = %v", want, got)
	}
	verifyOpen(t, 1)
}

func TestCheckConnections(t *testing.T) {
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 50016,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg"},
		},
	}
	d := &fakeDialer{}
	c, err := proxy.NewClient(context.Background(), d, testLogger, in, nil)
	if err != nil {
		t.Fatalf("proxy.NewClient error: %v", err)
	}
	defer c.Close()
	go c.Serve(context.Background(), func() {})

	n, err := c.CheckConnections(context.Background())
	if err != nil {
		t.Fatalf("CheckConnections failed: %v", err)
	}
	if want, got := len(in.Instances), n; want != got {
		t.Fatalf("CheckConnections number of connections: want = %v, got = %v", want, got)
	}

	if want, got := 1, d.dialAttempts(); want != got {
		t.Fatalf("dial attempts: want = %v, got = %v", want, got)
	}

	in = &proxy.Config{
		Addr: "127.0.0.1",
		Port: 60002,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg1"},
			{Name: "proj:region:pg2"},
		},
	}
	ed := &errorDialer{}
	c, err = proxy.NewClient(context.Background(), ed, testLogger, in, nil)
	if err != nil {
		t.Fatalf("proxy.NewClient error: %v", err)
	}
	defer c.Close()
	go c.Serve(context.Background(), func() {})

	n, err = c.CheckConnections(context.Background())
	if err == nil {
		t.Fatal("CheckConnections should have failed, but did not")
	}
	if want, got := len(in.Instances), n; want != got {
		t.Fatalf("CheckConnections number of connections: want = %v, got = %v", want, got)
	}
}

func TestRunConnectionCheck(t *testing.T) {
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 50017,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg"},
		},
		RunConnectionTest: true,
	}
	d := &fakeDialer{}
	c, err := proxy.NewClient(context.Background(), d, testLogger, in, nil)
	if err != nil {
		t.Fatalf("proxy.NewClient error: %v", err)
	}
	defer func(c *proxy.Client) {
		err := c.Close()
		if err != nil {
			t.Log(err)
		}
	}(c)
	go func() {
		// Serve alone without any connections will still verify that the
		// provided instances are reachable.
		_ = c.Serve(context.Background(), func() {})
	}()

	verifyDialAttempts := func() error {
		var tries int
		for {
			tries++
			if tries == 10 {
				return errors.New("failed to verify dial tries after 10 tries")
			}
			if got := d.dialAttempts(); got > 0 {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	if err := verifyDialAttempts(); err != nil {
		t.Fatal(err)
	}

}

func TestProxyInitializationWithFailedUnixSocket(t *testing.T) {
	ctx := context.Background()
	testDir, _ := createTempDir(t)
	testUnixSocketPath := path.Join(testDir, "db")

	tcs := []struct {
		desc string
		in   *proxy.Config
	}{
		{
			desc: "with unix socket for non functional instance",
			in: &proxy.Config{
				Instances: []proxy.InstanceConnConfig{
					{
						Name:           "proj:region:fakeserver",
						UnixSocketPath: testUnixSocketPath,
					},
				},
			},
		},
		{
			desc: "without TCP port or unix socket for non functional instance",
			in: &proxy.Config{
				Instances: []proxy.InstanceConnConfig{
					{Name: "proj:region:fakeserver"},
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := proxy.NewClient(ctx, &fakeDialer{}, testLogger, tc.in, nil)
			if err == nil {
				t.Fatalf("want non nil error, got = %v", err)
			}
		})
	}
}

func TestProxyMultiInstances(t *testing.T) {
	ctx := context.Background()
	testDir, _ := createTempDir(t)
	testUnixSocketPath := path.Join(testDir, "db")

	tcs := []struct {
		desc        string
		in          *proxy.Config
		wantSuccess bool
	}{
		{
			desc: "with tcp socket and unix for non functional instance",
			in: &proxy.Config{
				Instances: []proxy.InstanceConnConfig{
					{
						Name:           "proj:region:fakeserver",
						UnixSocketPath: testUnixSocketPath,
					},
					{Name: mysql, Port: 3306},
				},
			},
			wantSuccess: false,
		},
		{
			desc: "with two tcp socket instances and conflicting ports",
			in: &proxy.Config{
				Instances: []proxy.InstanceConnConfig{
					{Name: "proj:region:fakeserver", Port: 60003},
					{Name: mysql, Port: 60003},
				},
			},
			wantSuccess: false,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := proxy.NewClient(ctx, &fakeDialer{}, testLogger, tc.in, nil)
			if tc.wantSuccess != (err == nil) {
				t.Fatalf("want return = %v, got = %v", tc.wantSuccess, err == nil)
			}
		})
	}
}

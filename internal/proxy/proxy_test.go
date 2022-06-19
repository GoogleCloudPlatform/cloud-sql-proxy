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
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
	"github.com/spf13/cobra"
)

type fakeDialer struct {
	mu        sync.Mutex
	dialCount int
}

func (*fakeDialer) Close() error {
	return nil
}

func (f *fakeDialer) dialAttempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dialCount
}

func (f *fakeDialer) Dial(ctx context.Context, inst string, opts ...cloudsqlconn.DialOption) (net.Conn, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dialCount++
	c1, _ := net.Pipe()
	return c1, nil
}

func (*fakeDialer) EngineVersion(_ context.Context, inst string) (string, error) {
	switch {
	case strings.Contains(inst, "pg"):
		return "POSTGRES_14", nil
	case strings.Contains(inst, "mysql"):
		return "MYSQL_8_0", nil
	case strings.Contains(inst, "sqlserver"):
		return "SQLSERVER_2019_STANDARD", nil
	default:
		return "POSTGRES_14", nil
	}
}

func createTempDir(t *testing.T) (string, func()) {
	testDir, err := ioutil.TempDir("", "*")
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
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg},
					{Name: mysql},
					{Name: sqlserver},
				},
			},
			wantTCPAddrs: []string{"127.0.0.1:5000", "127.0.0.1:5001", "127.0.0.1:5002"},
		},
		{
			desc: "with instance address",
			in: &proxy.Config{
				Addr: "1.1.1.1", // bad address, binding shouldn't happen here.
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Addr: "0.0.0.0", Name: pg},
				},
			},
			wantTCPAddrs: []string{"0.0.0.0:5000"},
		},
		{
			desc: "IPv6 support",
			in: &proxy.Config{
				Addr: "::1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg},
				},
			},
			wantTCPAddrs: []string{"[::1]:5000"},
		},
		{
			desc: "with instance port",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg, Port: 6000},
				},
			},
			wantTCPAddrs: []string{"127.0.0.1:6000"},
		},
		{
			desc: "with global port and instance port",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg},
					{Name: mysql, Port: 6000},
					{Name: sqlserver},
				},
			},
			wantTCPAddrs: []string{
				"127.0.0.1:5000",
				"127.0.0.1:6000",
				"127.0.0.1:5001",
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
				Port: 5000,
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
					{Name: pg, Port: 5000},
				},
			},
			wantTCPAddrs: []string{
				"127.0.0.1:5000",
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
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c, err := proxy.NewClient(ctx, &fakeDialer{}, &cobra.Command{}, tc.in)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}
			defer c.Close()
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
	in := &proxy.Config{
		Addr: "127.0.0.1",
		Port: 5000,
		Instances: []proxy.InstanceConnConfig{
			{Name: "proj:region:pg"},
		},
		MaxConnections: 1,
	}
	d := &fakeDialer{}
	c, err := proxy.NewClient(context.Background(), d, &cobra.Command{}, in)
	if err != nil {
		t.Fatalf("proxy.NewClient error: %v", err)
	}
	defer c.Close()
	go c.Serve(context.Background())

	conn1, err1 := net.Dial("tcp", "127.0.0.1:5000")
	if err1 != nil {
		t.Fatalf("net.Dial error: %v", err1)
	}
	defer conn1.Close()

	conn2, err2 := net.Dial("tcp", "127.0.0.1:5000")
	if err2 != nil {
		t.Fatalf("net.Dial error: %v", err1)
	}
	defer conn2.Close()

	// try to read to check if the connection is closed
	// wait only a second for the result (since nothing is writing to the
	// socket)
	conn2.SetReadDeadline(time.Now().Add(time.Second))
	_, rErr := conn2.Read(make([]byte, 1))
	if rErr != io.EOF {
		t.Fatalf("conn.Read should return io.EOF, got = %v", rErr)
	}

	want := 1
	if got := d.dialAttempts(); got != want {
		t.Fatalf("dial attempts did not match expected, want = %v, got = %v", want, got)
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
	c, err := proxy.NewClient(ctx, &fakeDialer{}, &cobra.Command{}, in)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}
	c.Close()

	c, err = proxy.NewClient(ctx, &fakeDialer{}, &cobra.Command{}, in)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}
	c.Close()
}

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

//go:build !windows

package proxy_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
)

var (
	pg         = "proj:region:pg"
	pg2        = "proj:region:pg2"
	mysql      = "proj:region:mysql"
	mysql2     = "proj:region:mysql2"
	sqlserver  = "proj:region:sqlserver"
	sqlserver2 = "proj:region:sqlserver2"
)

func verifySocketPermissions(t *testing.T, addr string) {
	fi, err := os.Stat(addr)
	if err != nil {
		t.Fatalf("os.Stat(%v): %v", addr, err)
	}
	if fm := fi.Mode(); fm != 0777|os.ModeSocket {
		t.Fatalf("file mode: want = %v, got = %v", 0777|os.ModeSocket, fm)
	}
}

func TestFuseClosesGracefully(t *testing.T) {
	c, err := proxy.NewClient(
		context.Background(), nil, testLogger,
		&proxy.Config{
			FUSEDir:     t.TempDir(),
			FUSETempDir: t.TempDir(),
			Token:       "mytoken",
		},
		nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

}

// createStaleUnixSocket leaves a Unix socket file at addr with nothing
// listening on it, as a Proxy process that was killed (e.g., SIGKILL or OOM)
// before closing its listener would.
func createStaleUnixSocket(t *testing.T, addr string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(addr), 0777); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClientInitializationWithStaleUnixSocket(t *testing.T) {
	tcs := []struct {
		desc     string
		inst     string
		sockPath func(dir string) string
	}{
		{
			desc:     "MySQL",
			inst:     mysql,
			sockPath: func(dir string) string { return filepath.Join(dir, mysql) },
		},
		{
			desc:     "Postgres",
			inst:     pg,
			sockPath: func(dir string) string { return filepath.Join(dir, pg, ".s.PGSQL.5432") },
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			testDir, cleanup := createTempDir(t)
			defer cleanup()
			addr := tc.sockPath(testDir)
			createStaleUnixSocket(t, addr)

			c, err := proxy.NewClient(context.Background(), &fakeDialer{}, testLogger, &proxy.Config{
				UnixSocket: testDir,
				Instances:  []proxy.InstanceConnConfig{{Name: tc.inst}},
			}, nil)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}
			defer c.Close()

			conn, err := net.Dial("unix", addr)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}
			conn.Close()
		})
	}
}

func TestClientInitializationWithUnixSocketInUse(t *testing.T) {
	testDir, cleanup := createTempDir(t)
	defer cleanup()
	addr := filepath.Join(testDir, mysql)
	ln, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	c, err := proxy.NewClient(context.Background(), &fakeDialer{}, testLogger, &proxy.Config{
		UnixSocket: testDir,
		Instances:  []proxy.InstanceConnConfig{{Name: mysql}},
	}, nil)
	if err == nil {
		c.Close()
		t.Fatal("want error when another process is listening on the socket, got nil")
	}

	// The socket of the other process must be left in place.
	conn, err := net.Dial("unix", addr)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}
	conn.Close()
}

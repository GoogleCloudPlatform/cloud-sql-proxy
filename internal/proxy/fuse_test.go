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

//go:build !windows && !darwin
// +build !windows,!darwin

package proxy_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
	"github.com/hanwen/go-fuse/v2/fs"
)

func randTmpDir(t interface {
	Fatalf(format string, args ...interface{})
}) string {
	name, err := os.MkdirTemp("", "*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	return name
}

// newTestClient is a convenience function for testing that creates a
// proxy.Client and starts it. The returned cleanup function is also a
// convenience. Callers may choose to ignore it and manually close the client.
func newTestClient(t *testing.T, d cloudsql.Dialer, fuseDir, fuseTempDir string) (*proxy.Client, func()) {
	conf := &proxy.Config{FUSEDir: fuseDir, FUSETempDir: fuseTempDir}
	c, err := proxy.NewClient(context.Background(), d, testLogger, conf)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}

	ready := make(chan struct{})
	go c.Serve(context.Background(), func() { close(ready) })
	select {
	case <-ready:
	case <-time.Tick(5 * time.Second):
		t.Fatal("failed to Serve")
	}
	return c, func() {
		if cErr := c.Close(); cErr != nil {
			t.Logf("failed to close client: %v", cErr)
		}
	}
}

func TestFUSEREADME(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuse tests in short mode.")
	}
	dir := randTmpDir(t)
	d := &fakeDialer{}
	_, cleanup := newTestClient(t, d, dir, randTmpDir(t))

	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat: %v", err)
	}
	if !fi.IsDir() {
		t.Fatalf("fuse mount mode: want = dir, got = %v", fi.Mode())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("os.ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("dir entries: want = 1, got = %v", len(entries))
	}
	e := entries[0]
	if want, got := "README", e.Name(); want != got {
		t.Fatalf("want = %v, got = %v", want, got)
	}

	data, err := os.ReadFile(filepath.Join(dir, "README"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatalf("expected README data, got no data (dir = %v)", dir)
	}

	cleanup() // close the client

	// verify that the FUSE server is no longer mounted
	_, err = os.ReadFile(filepath.Join(dir, "README"))
	if err == nil {
		t.Fatal("expected os.Readfile to fail, but it succeeded")
	}
}

func tryDialUnix(t *testing.T, addr string) net.Conn {
	var (
		conn    net.Conn
		dialErr error
	)
	for i := 0; i < 10; i++ {
		conn, dialErr = net.Dial("unix", addr)
		if conn != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if dialErr != nil {
		t.Fatalf("net.Dial(): %v", dialErr)
	}
	return conn
}

func TestFUSEDialInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuse tests in short mode.")
	}
	fuseDir := randTmpDir(t)
	fuseTempDir := randTmpDir(t)
	tcs := []struct {
		desc         string
		wantInstance string
		socketPath   string
		fuseTempDir  string
	}{
		{
			desc:         "mysql connections create a Unix socket",
			wantInstance: "proj:region:mysql",
			socketPath:   filepath.Join(fuseDir, "proj:region:mysql"),
			fuseTempDir:  fuseTempDir,
		},
		{
			desc:         "postgres connections create a directory with a special file",
			wantInstance: "proj:region:pg",
			socketPath:   filepath.Join(fuseDir, "proj:region:pg", ".s.PGSQL.5432"),
			fuseTempDir:  fuseTempDir,
		},
		{
			desc:         "connecting creates intermediate temp directories",
			wantInstance: "proj:region:mysql",
			socketPath:   filepath.Join(fuseDir, "proj:region:mysql"),
			fuseTempDir:  filepath.Join(fuseTempDir, "doesntexist"),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			d := &fakeDialer{}
			_, cleanup := newTestClient(t, d, fuseDir, tc.fuseTempDir)
			defer cleanup()

			conn := tryDialUnix(t, tc.socketPath)
			defer conn.Close()

			var got []string
			for i := 0; i < 10; i++ {
				got = d.dialedInstances()
				if len(got) == 1 {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if len(got) != 1 {
				t.Fatalf("dialed instances len: want = 1, got = %v", got)
			}
			if want, inst := tc.wantInstance, got[0]; want != inst {
				t.Fatalf("instance: want = %v, got = %v", want, inst)
			}

		})
	}
}

func TestFUSEReadDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuse tests in short mode.")
	}
	fuseDir := randTmpDir(t)
	_, cleanup := newTestClient(t, &fakeDialer{}, fuseDir, randTmpDir(t))
	defer cleanup()

	// Initiate a connection so the FUSE server will list it in the dir entries.
	conn := tryDialUnix(t, filepath.Join(fuseDir, "proj:reg:mysql"))
	defer conn.Close()

	entries, err := os.ReadDir(fuseDir)
	if err != nil {
		t.Fatalf("os.ReadDir(): %v", err)
	}
	// len should be README plus the proj:reg:mysql socket
	if got, want := len(entries), 2; got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if names[0] != "README" || names[1] != "proj:reg:mysql" {
		t.Fatalf("want = %v, got = %v", []string{"README", "proj:reg:mysql"}, names)
	}
}

func TestFUSEErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuse tests in short mode.")
	}
	ctx := context.Background()
	d := &fakeDialer{}
	c, _ := newTestClient(t, d, randTmpDir(t), randTmpDir(t))

	// Simulate FUSE file access by invoking Lookup directly to control
	// how the socket cache is populated.
	_, err := c.Lookup(ctx, "proj:reg:mysql", nil)
	if err != fs.OK {
		t.Fatalf("proxy.Client.Lookup(): %v", err)
	}

	// Close the client to close all open sockets.
	if err := c.Close(); err != nil {
		t.Fatalf("c.Close(): %v", err)
	}

	// Simulate another FUSE file access to directly populated the socket cache.
	_, err = c.Lookup(ctx, "proj:reg:mysql", nil)
	if err != fs.OK {
		t.Fatalf("proxy.Client.Lookup(): %v", err)
	}

	// Verify the dialer was called twice, to prove the previous cache entry was
	// removed when the socket was closed.
	var attempts int
	wantAttempts := 2
	for i := 0; i < 10; i++ {
		attempts = d.engineVersionAttempts()
		if attempts == wantAttempts {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("engine version attempts: want = %v, got = %v", wantAttempts, attempts)
}

func TestFUSEWithBadInstanceName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuse tests in short mode.")
	}
	fuseDir := randTmpDir(t)
	d := &fakeDialer{}
	_, cleanup := newTestClient(t, d, fuseDir, randTmpDir(t))
	defer cleanup()

	_, dialErr := net.Dial("unix", filepath.Join(fuseDir, "notvalid"))
	if dialErr == nil {
		t.Fatalf("net.Dial() should fail")
	}

	if got := d.engineVersionAttempts(); got > 0 {
		t.Fatalf("engine version calls: want = 0, got = %v", got)
	}
}

func TestFUSECheckConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuse tests in short mode.")
	}
	fuseDir := randTmpDir(t)
	d := &fakeDialer{}
	c, cleanup := newTestClient(t, d, fuseDir, randTmpDir(t))
	defer cleanup()

	// first establish a connection to "register" it with the proxy
	conn := tryDialUnix(t, filepath.Join(fuseDir, "proj:reg:mysql"))
	defer conn.Close()

	n, err := c.CheckConnections(context.Background())
	if err != nil {
		t.Fatalf("c.CheckConnections(): %v", err)
	}
	if want, got := 1, n; want != got {
		t.Fatalf("CheckConnections number of connections: want = %v, got = %v", want, got)
	}

	// verify the dialer was invoked twice, once for connect, once for check
	// connection
	var attempts int
	wantAttempts := 2
	for i := 0; i < 10; i++ {
		attempts = d.dialAttempts()
		if attempts == wantAttempts {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("dial attempts: want = %v, got = %v", wantAttempts, attempts)
}

func TestFUSEClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuse tests in short mode.")
	}
	fuseDir := randTmpDir(t)
	d := &fakeDialer{}
	c, _ := newTestClient(t, d, fuseDir, randTmpDir(t))

	// first establish a connection to "register" it with the proxy
	conn := tryDialUnix(t, filepath.Join(fuseDir, "proj:reg:mysql"))
	defer conn.Close()

	// Close the proxy which should close all listeners
	if err := c.Close(); err != nil {
		t.Fatalf("c.Close(): %v", err)
	}

	_, err := net.Dial("unix", filepath.Join(fuseDir, "proj:reg:mysql"))
	if err == nil {
		t.Fatal("net.Dial() should fail")
	}
}

func TestFUSEWithBadDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuse tests in short mode.")
	}
	conf := &proxy.Config{FUSEDir: "/not/a/dir", FUSETempDir: randTmpDir(t)}
	_, err := proxy.NewClient(context.Background(), &fakeDialer{}, testLogger, conf)
	if err == nil {
		t.Fatal("proxy client should fail with bad dir")
	}
}

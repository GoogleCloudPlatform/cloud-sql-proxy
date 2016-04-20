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

// +build !windows

package fuse

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"bazil.org/fuse"
)

var (
	dir    = filepath.Join(os.TempDir(), "cloudsql")
	tmpdir = filepath.Join(os.TempDir(), "cloudsql-tmp")
)

func TestFuseClose(t *testing.T) {
	src, fuse, err := NewConnSrc(dir, tmpdir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := fuse.Close(); err != nil {
		t.Fatal(err)
	}
	if got, ok := <-src; ok {
		t.Fatalf("got new connection %#v, expected closed source", got)
	}
}

// TestBadDir verifies that the fuse module does not create directories, only simple files.
func TestBadDir(t *testing.T) {
	_, fuse, err := NewConnSrc(dir, tmpdir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer fuse.Close()

	_, err = os.Stat(filepath.Join(dir, "dir1", "dir2"))
	if err == nil {
		t.Fatal("able to find a directory inside the mount point, expected only regular files")
	}
	if err := err.(*os.PathError); err.Err != syscall.ENOTDIR {
		t.Fatalf("got %#v, want ENOTDIR (%v)", err.Err, syscall.ENOTDIR)
	}
}

func TestReadme(t *testing.T) {
	_, fuse, err := NewConnSrc(dir, tmpdir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer fuse.Close()

	data, err := ioutil.ReadFile(filepath.Join(dir, "README"))
	if err != nil {
		t.Fatal(err)
	}
	// We just care that the file exists. Print out the contents for
	// informational purposes.
	t.Log(string(data))
}

func TestSingleInstance(t *testing.T) {
	src, fuse, err := NewConnSrc(dir, tmpdir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer fuse.Close()

	const want = "test:instance"
	path := filepath.Join(dir, want)

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if fi.Mode()&os.ModeType != os.ModeSocket {
		t.Fatalf("%q had mode %v (%X), expected a socket file", path, fi.Mode(), uint32(fi.Mode()))
	}

	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	got, ok := <-src
	if !ok {
		t.Fatal("connection source was closed, expected a connection")
	} else if got.Instance != want {
		t.Fatalf("got %q, want %q", got.Instance, want)
	} else if got.Conn == nil {
		t.Fatal("got nil connection, wanted a connection")
	}

	const sent = "test string"
	go func() {
		if _, err := c.Write([]byte(sent)); err != nil {
			t.Error(err)
		}
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	}()

	gotData := new(bytes.Buffer)
	if _, err := io.Copy(gotData, got.Conn); err != nil {
		t.Fatal(err)
	} else if gotData.String() != sent {
		t.Fatalf("got %q, want %v", gotData.String(), sent)
	}
}

func BenchmarkNewConnection(b *testing.B) {
	src, fuse, err := NewConnSrc(dir, tmpdir, nil)
	if err != nil {
		b.Fatal(err)
	}

	const want = "X"
	incomingCount := 0
	var incoming sync.Mutex // Is unlocked when the following goroutine exits.
	go func() {
		incoming.Lock()
		defer incoming.Unlock()

		for c := range src {
			c.Conn.Write([]byte(want))
			c.Conn.Close()
			incomingCount++
		}
	}()

	const instance = "test:instance"
	path := filepath.Join(dir, instance)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, err := net.Dial("unix", path)
		if err != nil {
			b.Errorf("couldn't dial: %v", err)
		}

		data, err := ioutil.ReadAll(c)
		if err != nil {
			b.Errorf("got read error: %v", err)
		} else if got := string(data); got != want {
			b.Errorf("read %q, want %q", string(data), want)
		}
	}
	if err := fuse.Close(); err != nil {
		b.Fatal(err)
	}

	// Wait for the 'incoming' goroutine to finish.
	incoming.Lock()
	if incomingCount != b.N {
		b.Fatalf("got %d connections, want %d", incomingCount, b.N)
	}
}

func TestMain(m *testing.M) {
	// Ensure this directory exists.
	os.MkdirAll(dir, 0777)

	// Unmount before the tests start, else they won't work correctly.
	if err := fuse.Unmount(dir); err != nil {
		log.Printf("couldn't unmount fuse directory %q: %v", dir, err)
	}

	ret := m.Run()
	// Make sure to unmount at the end, so that we don't leave the system in an
	// inconsistent state in case something weird happened.
	if err := fuse.Unmount(dir); err != nil {
		log.Printf("couldn't unmount fuse directory %q: %v", dir, err)
	}

	os.Exit(ret)
}

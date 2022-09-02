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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/log"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
)

func randTmpDir(t interface {
	Fatalf(format string, args ...interface{})
}) string {
	name, err := ioutil.TempDir("", "*")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	return name
}

// tryFunc executes the provided function up to maxCount times, sleeping 100ms
// between attempts.
func tryFunc(f func() error, maxCount int) error {
	var errCount int
	for {
		err := f()
		if err == nil {
			return nil
		}
		errCount++
		if errCount == maxCount {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestREADME(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fuse tests in short mode.")
	}
	ctx := context.Background()

	dir := randTmpDir(t)
	conf := &proxy.Config{
		FUSEDir:     dir,
		FUSETempDir: randTmpDir(t),
	}
	logger := log.NewStdLogger(os.Stdout, os.Stdout)
	d := &fakeDialer{}
	c, err := proxy.NewClient(ctx, d, logger, conf)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}

	ready := make(chan struct{})
	go c.Serve(ctx, func() { close(ready) })
	select {
	case <-ready:
	case <-time.After(time.Minute):
		t.Fatal("proxy.Client failed to start serving")
	}

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

	data, err := ioutil.ReadFile(filepath.Join(dir, "README"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatalf("expected README data, got no data (dir = %v)", dir)
	}

	if cErr := c.Close(); cErr != nil {
		t.Fatalf("c.Close(): %v", cErr)
	}

	// verify that c.Close unmounts the FUSE server
	_, err = ioutil.ReadFile(filepath.Join(dir, "README"))
	if err == nil {
		t.Fatal("expected ioutil.Readfile to fail, but it succeeded")
	}
}

// Copyright 2021 Google LLC All Rights Reserved.
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

// +build !skip_healthcheck

package tests

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

const (
	startupPath   = "/startup"
	readinessPath = "/readiness"
	testPort      = "8090"
)

func requireConnNames(t *testing.T) {
	switch "" {
	case *mysqlConnName:
		t.Fatal("'mysql_conn_name' not set")
	case *postgresConnName:
		t.Fatal("'postgres_conn_name' not set")
	case *sqlserverConnName:
		t.Fatal("'sqlserver_conn_name' not set")
	}
}

// waitForStart blocks until the currently running proxy completes startup.
func waitForStart(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			resp, err := http.Get("http://localhost:" + testPort + startupPath)
			if err == nil && resp.StatusCode == http.StatusOK {
				return
			}
		}
	}
}

// singleInstanceDial verifies that when a proxy client serves the given instance, the readiness
// endpoint serves http.StatusOK.
func singleInstanceDial(t *testing.T, binPath string, connName string, port int) {
	var args []string
	args = append(args, fmt.Sprintf("-instances=%s=tcp:%d", connName, port), "-use_http_health_check")

	cmd := exec.Command(binPath, args...)
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start proxy: %s", err)
	}
	defer cmd.Process.Kill()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()
	waitForStart(ctx)

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want %v, got %v", http.StatusOK, resp.StatusCode)
	}
}

// Test to verify that when a proxy client serves one instance that can successfully be dialed,
// the readiness endpoint serves http.StatusOK.
func TestSingleInstanceDial(t *testing.T) {
	requireConnNames(t)

	binPath, err := compileProxy()
	if err != nil {
		t.Fatalf("Failed to compile proxy: %s", err)
	}
	defer os.RemoveAll(binPath)

	singleInstanceDial(t, binPath, *mysqlConnName, mysqlPort)
	singleInstanceDial(t, binPath, *postgresConnName, postgresPort)
	singleInstanceDial(t, binPath, *sqlserverConnName, sqlserverPort)
}

// Test to verify that when a proxy client serves multiple instances that can all be successfully dialed,
// the readiness endpoint serves http.StatusOK.
func TestMultiInstanceDial(t *testing.T) {
	requireConnNames(t)

	binPath, err := compileProxy()
	if err != nil {
		t.Fatalf("Failed to compile proxy: %s", err)
	}
	defer os.RemoveAll(binPath)

	var args []string
	args = append(args, fmt.Sprintf("-instances=%s=tcp:%d,%s=tcp:%d,%s=tcp:%d", *mysqlConnName, mysqlPort, *postgresConnName, postgresPort, *sqlserverConnName, sqlserverPort))
	args = append(args, "-use_http_health_check")

	cmd := exec.Command(binPath, args...)
	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start proxy: %s", err)
	}
	defer cmd.Process.Kill()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()
	waitForStart(ctx)

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want %v, got %v", http.StatusOK, resp.StatusCode)
	}
}

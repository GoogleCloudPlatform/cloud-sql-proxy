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

package tests

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
)

const (
	startupPath   = "/startup"
	readinessPath = "/readiness"
	testPort      = "8090"
)

var (
	connName = flag.String("conn_name", os.Getenv("CONNECTION_NAME"), "Cloud SQL MYSQL instance connection name, in the form of 'project:region:instance'.")
	port     = flag.String("port", os.Getenv("PORT"), "TCP port for the proxy to listen on.")

	connName2 = flag.String("conn_name_2", os.Getenv("CONNECTION_NAME_2"), "A second Cloud SQL MYSQL instance connection name.")
	port2     = flag.String("port_2", os.Getenv("PORT_2"), "A second TCP port.")
)

// waitForStart blocks until the currently running proxy completes startup.
func waitForStart() {
	for {
		resp, err := http.Get("http://localhost:" + testPort + startupPath)
		if err == nil && resp.StatusCode == http.StatusOK {
			return
		}
	}
}

// Test to verify that when a proxy client serves one instance that can successfully be dialed,
// the readiness endpoint serves http.StatusOK.
func TestSingleInstanceDial(t *testing.T) {
	switch "" {
	case *connName:
		t.Fatal("'conn_name' not set")
	case *port:
		t.Fatal("'port' not set")
	}

	binPath, err := compileProxy()
	if err != nil {
		t.Fatalf("Failed to compile proxy: %s", err)
	}
	defer os.RemoveAll(binPath)

	var args []string
	args = append(args, fmt.Sprintf("-instances=%s=tcp:%s", *connName, *port), "-use_http_health_check")

	cmd := exec.Command(binPath, args...)
	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start proxy: %s", err)
	}
	defer cmd.Process.Kill()

	waitForStart()

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%v returned status code %v instead of %v", readinessPath, resp.StatusCode, http.StatusOK)
	}
}

// Test to verify that when a proxy client serves multiple instances that can all be successfully dialed,
// the readiness endpoint serves http.StatusOK.
func TestMultiInstanceDial(t *testing.T) {
	switch "" {
	case *connName:
		t.Fatal("'conn_name' not set")
	case *port:
		t.Fatal("'port' not set")
	case *connName2:
		t.Fatal("'conn_name_2' not set")
	case *port2:
		t.Fatal("'port_2' not set")
	}

	if connName == connName2 {
		t.Fatal("'conn_name' and 'conn_name_2' are the same")
	}
	if port == port2 {
		t.Fatal("'port' and 'port_2' are the same")
	}

	binPath, err := compileProxy()
	if err != nil {
		t.Fatalf("Failed to compile proxy: %s", err)
	}
	defer os.RemoveAll(binPath)

	var args []string
	args = append(args, fmt.Sprintf("-instances=%s=tcp:%s,%s=tcp:%s", *connName, *port, *connName2, *port2), "-use_http_health_check")

	cmd := exec.Command(binPath, args...)
	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start proxy: %s", err)
	}
	defer cmd.Process.Kill()

	waitForStart()

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%v returned status code %v instead of %v", readinessPath, resp.StatusCode, http.StatusOK)
	}
}

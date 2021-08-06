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

// waitForStart blocks until the currently running proxy completes startup.
func waitForStart(ctx context.Context) {
	for {
		resp, err := http.Get("http://localhost:" + testPort + startupPath)
		if err == nil && resp.StatusCode == http.StatusOK {
			return
		}
	}
}

// Test to verify that when all instances can be dialed successfully, the readiness endpoint 
// writes http.StatusOK.
func TestDialPass(t *testing.T) {
    tests := map[string]struct {
        insts []string
		ports []int
	}{
		"Single instance": {insts: []string{*mysqlConnName}, ports: []int{mysqlPort}},
		"Multiple instances": {insts: []string{*mysqlConnName, *postgresConnName}, ports: []int{mysqlPort, postgresPort}},
	}

	binPath, err := compileProxy()
	if err != nil {
		t.Fatalf("Failed to compile proxy: %s", err)
	}
	defer os.RemoveAll(binPath)

	for name, test := range tests {
		for _, inst := range test.insts {
			if inst == "" {
				t.Fatalf("%v: connection name not set", name)
			}
		}

		func() {
			arg := "-instances="
			for i, inst := range test.insts {
				if i > 0 {
					arg += ","
				}
				arg += fmt.Sprintf("%s=tcp:%d", inst, test.ports[i])
			}

			cmd := exec.Command(binPath, arg, "-use_http_health_check")
			err = cmd.Start()
			if err != nil {
				t.Fatalf("%v: Failed to start proxy: %s", name, err)
			}
			defer cmd.Process.Kill()

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10 * time.Second))
			defer cancel()
			waitForStart(ctx)

			resp, err := http.Get("http://localhost:" + testPort + readinessPath)
			if err != nil {
				t.Fatalf("%v: HTTP GET failed: %v", name, err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%v: Got status code %v instead of %v", name, resp.StatusCode, http.StatusOK)
			}
		}()
	}
}

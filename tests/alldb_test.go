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

// alldb_test.go contains end to end tests that require all environment variables to be defined.
package tests

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// requireAllVars skips the given test if at least one environment variable is undefined.
func requireAllVars(t *testing.T) {
	var allVars []string
	allVars = append(allVars, *mysqlConnName, *mysqlUser, *mysqlPass, *mysqlDb)
	allVars = append(allVars, *postgresConnName, *postgresUser, *postgresPass, *postgresDb)
	allVars = append(allVars, *sqlserverConnName, *sqlserverUser, *sqlserverPass, *sqlserverDb)

	for _, envVar := range allVars {
		if envVar == "" {
			t.Skip("skipping test, all environment variable must be defined")
		}
	}
}

// Test to verify that when a proxy client serves multiple instances that can all be successfully dialed,
// the health check readiness endpoint serves http.StatusOK.
func TestMultiInstanceDial(t *testing.T) {
	// Skipping flaky Github Action test
	// https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1228
	t.Skip("Unblocking WIF Builds!")
	if testing.Short() {
		t.Skip("skipping Health Check integration tests")
	}
	requireAllVars(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Start the proxy.
	args := []string{
		// This test doesn't care what the instance port is, so use "0" which
		// means, let the runtime pick a random port.
		fmt.Sprintf("-instances=%s=tcp:0,%s=tcp:0,%s=tcp:0",
			*mysqlConnName, *postgresConnName, *sqlserverConnName),
		"-use_http_health_check",
	}
	p, err := StartProxy(ctx, args...)
	if err != nil {
		t.Fatalf("unable to start proxy: %v", err)
	}
	defer p.Close()
	output, err := p.WaitForServe(ctx)
	if err != nil {
		t.Fatalf("unable to verify proxy was serving: %s \n %s", err, output)
	}

	resp, err := http.Get("http://localhost:" + testPort + readinessPath)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want %v, got %v", http.StatusOK, resp.StatusCode)
	}
}

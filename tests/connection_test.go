// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tests

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httputil"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const connTestTimeout = time.Minute

// removeAuthEnvVar retrieves an OAuth2 token and a path to a service account key
// and then unsets GOOGLE_APPLICATION_CREDENTIALS. It returns a cleanup function
// that restores the original setup.
func removeAuthEnvVar(t *testing.T) (*oauth2.Token, string, func()) {
	ts, err := google.DefaultTokenSource(context.Background(),
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		t.Errorf("failed to resolve token source: %v", err)
	}
	tok, err := ts.Token()
	if err != nil {
		t.Errorf("failed to get token: %v", err)
	}
	path, ok := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	if !ok {
		t.Fatalf("GOOGLE_APPLICATION_CREDENTIALS was not set in the environment")
	}
	if err := os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS"); err != nil {
		t.Fatalf("failed to unset GOOGLE_APPLICATION_CREDENTIALS")
	}
	return tok, path, func() {
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	}
}

func keyfile(t *testing.T) string {
	path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if path == "" {
		t.Fatal("GOOGLE_APPLICATION_CREDENTIALS not set")
	}
	creds, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("io.ReadAll(): %v", err)
	}
	return string(creds)
}

// proxyConnTest is a test helper to verify the proxy works with a basic connectivity test.
func proxyConnTest(t *testing.T, args []string, driver, dsn string) {
	ctx, cancel := context.WithTimeout(context.Background(), connTestTimeout)
	defer cancel()
	// Start the proxy
	p, err := StartProxy(ctx, args...)
	if err != nil {
		t.Fatalf("unable to start proxy: %v", err)
	}
	defer p.Close()
	output, err := p.WaitForServe(ctx)
	if err != nil {
		t.Fatalf("unable to verify proxy was serving: %s \n %s", err, output)
	}

	// Connect to the instance
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("unable to connect to db: %s", err)
	}
	defer db.Close()
	_, err = db.Exec("SELECT 1;")
	if err != nil {
		t.Fatalf("unable to exec on db: %s", err)
	}
}

// testHealthCheck verifies that when a proxy client serves the given instance,
// the readiness endpoint serves http.StatusOK.
func testHealthCheck(t *testing.T, connName string) {
	ctx, cancel := context.WithTimeout(context.Background(), connTestTimeout)
	defer cancel()

	args := []string{connName, "--health-check"}
	// Start the proxy.
	p, err := StartProxy(ctx, args...)
	if err != nil {
		t.Fatalf("unable to start proxy: %v", err)
	}
	defer p.Close()
	_, err = p.WaitForServe(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var (
		gErr error
		resp *http.Response
	)
	for i := 0; i < 10; i++ {
		resp, gErr = http.Get("http://localhost:9090/readiness")
		if gErr != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		return // The response is OK, the test passes.
	}
	if gErr != nil {
		t.Fatalf("HTTP GET failed: %v", gErr)
	}
	respBody, dErr := httputil.DumpResponse(resp, true)
	if dErr != nil {
		t.Fatalf("failed to dump HTTP response: %v", dErr)
	}
	t.Fatalf("HTTP GET failed: response =\n%v", string(respBody))
}

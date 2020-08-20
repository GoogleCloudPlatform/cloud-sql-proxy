// Copyright 2020 Google LLC
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

// connection_test.go provides some helpers for basic connectivity tests to Cloud SQL instances.
package tests

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
)

// proxyConnTest is a test helper to verify the proxy works with a basic connectivity test.
func proxyConnTest(t *testing.T, connName, driver, dsn string, port int, dir string) {
	ctx := context.Background()

	var args []string
	if dir != "" { // unix port
		args = append(args, fmt.Sprintf("-dir=%s", dir), fmt.Sprintf("-instances=%s", connName))
	} else { // tcp socket
		args = append(args, fmt.Sprintf("-instances=%s=tcp:%d", connName, port))
	}

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

func proxyConnLimitTest(t *testing.T, connName, driver, dsn string, port int) {
	ctx := context.Background()

	maxConn, totConn := 5, 10

	// Start the proxy
	p, err := StartProxy(ctx, fmt.Sprintf("-instances=%s=tcp:%d", connName, port), fmt.Sprintf("-max_connections=%d", maxConn))
	if err != nil {
		t.Fatalf("unable to start proxy: %v", err)
	}
	defer p.Close()
	output, err := p.WaitForServe(ctx)
	if err != nil {
		t.Fatalf("unable to verify proxy was serving: %s \n %s", err, output)
	}

	// Create connection pool
	var stmt string
	switch driver {
	case "mysql":
		stmt = "SELECT sleep(2);"
	case "postgres":
		stmt = "SELECT pg_sleep(2);"
	case "sqlserver":
		stmt = "WAITFOR DELAY '00:00:02'"
	default:
		t.Fatalf("unsupported driver: no sleep query found")
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("unable to connect to db: %s", err)
	}
	db.SetMaxIdleConns(0)
	defer db.Close()

	// Connect with up to totConn and count errors
	var wg sync.WaitGroup
	c := make(chan error, totConn)
	for i := 0; i < totConn; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err = db.ExecContext(ctx, stmt)
			if err != nil {
				c <- err
			}
		}()
	}
	wg.Wait()
	close(c)

	var errs []error
	for e := range c {
		errs = append(errs, e)
	}
	want, got := totConn-maxConn, len(errs)
	if want != got {
		t.Errorf("wrong errCt - want: %d, got %d", want, got)
		for _, e := range errs {
			t.Errorf("%s\n", e)
		}
		t.Fail()
	}
}

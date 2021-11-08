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

// postgres_test runs various tests against a Postgres flavored Cloud SQL instance.
package tests

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/v2/proxy/dialers/postgres"
	_ "github.com/lib/pq"
)

var (
	postgresConnName = flag.String("postgres_conn_name", os.Getenv("POSTGRES_CONNECTION_NAME"), "Cloud SQL Postgres instance connection name, in the form of 'project:region:instance'.")
	postgresUser     = flag.String("postgres_user", os.Getenv("POSTGRES_USER"), "Name of database user.")
	postgresPass     = flag.String("postgres_pass", os.Getenv("POSTGRES_PASS"), "Password for the database user; be careful when entering a password on the command line (it may go into your terminal's history).")
	postgresDb       = flag.String("postgres_db", os.Getenv("POSTGRES_DB"), "Name of the database to connect to.")

	postgresIAMUser = flag.String("postgres_user_iam", os.Getenv("POSTGRES_USER_IAM"), "Name of database user configured with IAM DB Authentication.")

	postgresPort = 5432
)

func requirePostgresVars(t *testing.T) {
	switch "" {
	case *postgresConnName:
		t.Fatal("'postgres_conn_name' not set")
	case *postgresUser:
		t.Fatal("'postgres_user' not set")
	case *postgresPass:
		t.Fatal("'postgres_pass' not set")
	case *postgresDb:
		t.Fatal("'postgres_db' not set")
	}
}

func TestPostgresTcp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)

	dsn := fmt.Sprintf("user=%s password=%s database=%s sslmode=disable", *postgresUser, *postgresPass, *postgresDb)
	proxyConnTest(t, *postgresConnName, "postgres", dsn, postgresPort, "")
}

func TestPostgresSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	if runtime.GOOS == "windows" {
		t.Skip("Skipped Unix socket test on Windows")
	}
	requirePostgresVars(t)

	dir, err := ioutil.TempDir("", "csql-proxy")
	if err != nil {
		log.Fatalf("unable to create tmp dir: %s", err)
	}
	defer os.RemoveAll(dir)

	dsn := fmt.Sprintf("user=%s password=%s database=%s host=%s", *postgresUser, *postgresPass, *postgresDb, path.Join(dir, *postgresConnName))
	proxyConnTest(t, *postgresConnName, "postgres", dsn, 0, dir)
}

func TestPostgresConnLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)

	dsn := fmt.Sprintf("user=%s password=%s database=%s sslmode=disable", *postgresUser, *postgresPass, *postgresDb)
	proxyConnLimitTest(t, *postgresConnName, "postgres", dsn, postgresPort)
}

func TestPostgresIAMDBAuthn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)
	if *postgresIAMUser == "" {
		t.Fatal("'postgres_user_iam' not set")
	}

	ctx := context.Background()

	// Start the proxy
	p, err := StartProxy(ctx, fmt.Sprintf("-instances=%s=tcp:%d", *postgresConnName, 5432), "-enable_iam_login")
	if err != nil {
		t.Fatalf("unable to start proxy: %v", err)
	}
	defer p.Close()
	output, err := p.WaitForServe(ctx)
	if err != nil {
		t.Fatalf("unable to verify proxy was serving: %s \n %s", err, output)
	}

	dsn := fmt.Sprintf("user=%s database=%s sslmode=disable", *postgresIAMUser, *postgresDb)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("unable to connect to db: %s", err)
	}
	defer db.Close()
	_, err = db.Exec("SELECT 1;")
	if err != nil {

		t.Fatalf("unable to exec on db: %s", err)
	}
}

func TestPostgresHook(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable", *postgresConnName, *postgresUser, *postgresPass, *postgresDb)
	db, err := sql.Open("cloudsqlpostgres", dsn)
	if err != nil {
		t.Fatalf("connect failed: %s", err)
	}
	defer db.Close()
	var now time.Time
	err = db.QueryRowContext(ctx, "SELECT NOW()").Scan(&now)
	if err != nil {
		t.Fatalf("query failed: %s", err)
	}
}

// Test to verify that when a proxy client serves one postgres instance that can be
// dialed successfully, the health check readiness endpoint serves http.StatusOK.
func TestPostgresDial(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	switch "" {
	case *postgresConnName:
		t.Fatal("'postgres_conn_name' not set")
	}

	singleInstanceDial(t, *postgresConnName, postgresPort)
}

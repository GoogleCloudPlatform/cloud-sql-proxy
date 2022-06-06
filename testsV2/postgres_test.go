// Copyright 2021 Google LLC

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     https://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// postgres_test runs various tests against a Postgres flavored Cloud SQL instance.
package tests

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/testutil"
	_ "github.com/jackc/pgx/v4/stdlib"
)

var (
	postgresConnName = flag.String("postgres_conn_name", os.Getenv("POSTGRES_CONNECTION_NAME"), "Cloud SQL Postgres instance connection name, in the form of 'project:region:instance'.")
	postgresUser     = flag.String("postgres_user", os.Getenv("POSTGRES_USER"), "Name of database user.")
	postgresPass     = flag.String("postgres_pass", os.Getenv("POSTGRES_PASS"), "Password for the database user; be careful when entering a password on the command line (it may go into your terminal's history).")
	postgresDB       = flag.String("postgres_db", os.Getenv("POSTGRES_DB"), "Name of the database to connect to.")

	postgresIAMUser = flag.String("postgres_user_iam", os.Getenv("POSTGRES_USER_IAM"), "Name of database user configured with IAM DB Authentication.")
)

func requirePostgresVars(t *testing.T) {
	switch "" {
	case *postgresConnName:
		t.Fatal("'postgres_conn_name' not set")
	case *postgresUser:
		t.Fatal("'postgres_user' not set")
	case *postgresPass:
		t.Fatal("'postgres_pass' not set")
	case *postgresDB:
		t.Fatal("'postgres_db' not set")
	}
}

func TestPostgresTCP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)

	dsn := fmt.Sprintf("host=localhost user=%s password=%s database=%s sslmode=disable",
		*postgresUser, *postgresPass, *postgresDB)
	proxyConnTest(t, []string{*postgresConnName}, "pgx", dsn)
}

func TestPostgresUnix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)
	tmpDir, cleanup := createTempDir(t)
	defer cleanup()

	dsn := fmt.Sprintf("host=%s user=%s password=%s database=%s sslmode=disable",
		// re-use utility function to determine the Unix address in a
		// Windows-friendly way.
		proxy.UnixAddress(tmpDir, *postgresConnName),
		*postgresUser, *postgresPass, *postgresDB)

	proxyConnTest(t,
		[]string{"--unix-socket", tmpDir, *postgresConnName}, "pgx", dsn)
}

func createTempDir(t *testing.T) (string, func()) {
	testDir, err := ioutil.TempDir("", "*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return testDir, func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("failed to cleanup temp dir: %v", err)
		}
	}
}

func TestPostgresAuthWithToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)
	tok, _, cleanup := removeAuthEnvVar(t)
	defer cleanup()

	dsn := fmt.Sprintf("host=localhost user=%s password=%s database=%s sslmode=disable",
		*postgresUser, *postgresPass, *postgresDB)
	proxyConnTest(t,
		[]string{"--token", tok.AccessToken, *postgresConnName},
		"pgx", dsn)
}

func TestPostgresAuthWithCredentialsFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)
	_, path, cleanup := removeAuthEnvVar(t)
	defer cleanup()

	dsn := fmt.Sprintf("host=localhost user=%s password=%s database=%s sslmode=disable",
		*postgresUser, *postgresPass, *postgresDB)
	proxyConnTest(t,
		[]string{"--credentials-file", path, *postgresConnName},
		"pgx", dsn)
}

func TestAuthWithGcloudAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)

	cleanup := testutil.ConfigureGcloud(t)
	defer cleanup()

	dsn := fmt.Sprintf("host=localhost user=%s password=%s database=%s sslmode=disable",
		*postgresUser, *postgresPass, *postgresDB)
	proxyConnTest(t,
		[]string{"--gcloud-auth", *postgresConnName},
		"pgx", dsn)
}

func TestPostgresIAMDBAuthn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)
	if *postgresIAMUser == "" {
		t.Fatal("'postgres_user_iam' not set")
	}

	dsn := fmt.Sprintf("host=localhost user=%s database=%s sslmode=disable",
		*postgresIAMUser, *postgresDB)
	// using the global flag
	proxyConnTest(t,
		[]string{"--auto-iam-authn", *postgresConnName},
		"pgx", dsn)
	// using the instance-level query param
	proxyConnTest(t,
		[]string{fmt.Sprintf("%s?auto-iam-authn=true", *postgresConnName)},
		"pgx", dsn)
}

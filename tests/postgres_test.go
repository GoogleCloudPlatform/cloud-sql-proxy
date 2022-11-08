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
	"os"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
	_ "github.com/jackc/pgx/v4/stdlib"
)

var (
	postgresConnName = flag.String("postgres_conn_name", os.Getenv("POSTGRES_CONNECTION_NAME"), "Cloud SQL Postgres instance connection name, in the form of 'project:region:instance'.")
	postgresUser     = flag.String("postgres_user", os.Getenv("POSTGRES_USER"), "Name of database user.")
	postgresPass     = flag.String("postgres_pass", os.Getenv("POSTGRES_PASS"), "Password for the database user; be careful when entering a password on the command line (it may go into your terminal's history).")
	postgresDB       = flag.String("postgres_db", os.Getenv("POSTGRES_DB"), "Name of the database to connect to.")
	postgresIAMUser  = flag.String("postgres_user_iam", os.Getenv("POSTGRES_USER_IAM"), "Name of database user configured with IAM DB Authentication.")
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

func postgresDSN() string {
	return fmt.Sprintf("host=localhost user=%s password=%s database=%s sslmode=disable",
		*postgresUser, *postgresPass, *postgresDB)
}

func TestPostgresTCP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)

	proxyConnTest(t, []string{*postgresConnName}, "pgx", postgresDSN())
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
	testDir, err := os.MkdirTemp("", "*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return testDir, func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Logf("failed to cleanup temp dir: %v", err)
		}
	}
}

func TestPostgresImpersonation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)

	proxyConnTest(t, []string{
		"--impersonate-service-account", *impersonatedUser,
		*postgresConnName},
		"pgx", postgresDSN())
}

func TestPostgresAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)

	creds := keyfile(t)
	tok, path, cleanup := removeAuthEnvVar(t)
	defer cleanup()

	tcs := []struct {
		desc string
		args []string
	}{
		{
			desc: "with token",
			args: []string{"--token", tok.AccessToken, *postgresConnName},
		},
		{
			desc: "with token and impersonation",
			args: []string{
				"--token", tok.AccessToken,
				"--impersonate-service-account", *impersonatedUser,
				*postgresConnName},
		},
		{
			desc: "with credentials file",
			args: []string{"--credentials-file", path, *postgresConnName},
		},
		{
			desc: "with credentials file and impersonation",
			args: []string{
				"--credentials-file", path,
				"--impersonate-service-account", *impersonatedUser,
				*postgresConnName},
		},
		{
			desc: "with credentials JSON",
			args: []string{"--json-credentials", string(creds), *postgresConnName},
		},
		{
			desc: "with credentials JSON and impersonation",
			args: []string{
				"--json-credentials", string(creds),
				"--impersonate-service-account", *impersonatedUser,
				*postgresConnName},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			proxyConnTest(t, tc.args, "pgx", postgresDSN())
		})
	}
}

func TestPostgresGcloudAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)

	tcs := []struct {
		desc string
		args []string
	}{
		{
			desc: "gcloud user authentication",
			args: []string{"--gcloud-auth", *postgresConnName},
		},
		{
			desc: "gcloud user authentication with impersonation",
			args: []string{
				"--gcloud-auth",
				"--impersonate-service-account", *impersonatedUser,
				*postgresConnName},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			proxyConnTest(t, tc.args, "pgx", postgresDSN())
		})
	}

}

func TestPostgresIAMDBAuthn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)
	if *postgresIAMUser == "" {
		t.Fatal("'postgres_user_iam' not set")
	}

	defaultDSN := fmt.Sprintf("host=localhost user=%s database=%s sslmode=disable",
		*postgresIAMUser, *postgresDB)
	impersonatedIAMUser := strings.ReplaceAll(*impersonatedUser, ".gserviceaccount.com", "")

	tcs := []struct {
		desc string
		dsn  string
		args []string
	}{
		{
			desc: "using default flag",
			args: []string{"--auto-iam-authn", *postgresConnName},
			dsn:  defaultDSN,
		},
		{
			desc: "using query param",
			args: []string{fmt.Sprintf("%s?auto-iam-authn=true", *postgresConnName)},
			dsn:  defaultDSN,
		},
		{
			desc: "using impersonation",
			args: []string{
				"--auto-iam-authn",
				"--impersonate-service-account", *impersonatedUser,
				*postgresConnName},
			dsn: fmt.Sprintf("host=localhost user=%s database=%s sslmode=disable",
				impersonatedIAMUser, *postgresDB),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			proxyConnTest(t, tc.args, "pgx", tc.dsn)
		})
	}
}

func TestPostgresHealthCheck(t *testing.T) {
	testHealthCheck(t, *postgresConnName)
}

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
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/gcloud"
	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/v2/proxy/dialers/postgres"
	_ "github.com/lib/pq"
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

	dsn := fmt.Sprintf("user=%s password=%s database=%s sslmode=disable", *postgresUser, *postgresPass, *postgresDB)
	proxyConnTest(t, []string{*postgresConnName}, "postgres", dsn)
}

func TestPostgresAuthWithToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)
	tok, _, cleanup := removeAuthEnvVar(t)
	defer cleanup()

	dsn := fmt.Sprintf("user=%s password=%s database=%s sslmode=disable",
		*postgresUser, *postgresPass, *postgresDB)
	proxyConnTest(t,
		[]string{"--token", tok.AccessToken, *postgresConnName},
		"postgres", dsn)
}

func TestPostgresAuthWithCredentialsFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)
	_, path, cleanup := removeAuthEnvVar(t)
	defer cleanup()

	dsn := fmt.Sprintf("user=%s password=%s database=%s sslmode=disable",
		*postgresUser, *postgresPass, *postgresDB)
	proxyConnTest(t,
		[]string{"--credentials-file", path, *postgresConnName},
		"postgres", dsn)
}

func TestAuthWithGcloudAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	requirePostgresVars(t)

	// The following configures gcloud using only GOOGLE_APPLICATION_CREDENTIALS.
	configureGcloud := func(t *testing.T) func() {
		dir, err := ioutil.TempDir("", "cloudsdk*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		os.Setenv("CLOUDSDK_CONFIG", dir)

		gcloudCmd, err := gcloud.Cmd()
		if err != nil {
			t.Fatal(err)
		}

		_, path, cleanup := removeAuthEnvVar(t)

		cmd := exec.Command(gcloudCmd, "auth", "activate-service-account",
			"--key-file", path)
		buf := &bytes.Buffer{}
		cmd.Stdout = buf

		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to active service account. err = %v, message = %v",
				err, buf.String())
		}
		return func() {
			os.Unsetenv("CLOUDSDK_CONFIG")
			cleanup()
		}

	}
	cleanup := configureGcloud(t)
	defer cleanup()

	dsn := fmt.Sprintf("user=%s password=%s database=%s sslmode=disable",
		*postgresUser, *postgresPass, *postgresDB)
	proxyConnTest(t,
		[]string{"--gcloud-auth", *postgresConnName},
		"postgres", dsn)
}

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
	"testing"

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

	dsn := fmt.Sprintf("user=%s password=%s database=%s sslmode=disable port=5000", *postgresUser, *postgresPass, *postgresDb)
	proxyConnTest(t, *postgresConnName, "postgres", dsn, postgresPort, "")
}

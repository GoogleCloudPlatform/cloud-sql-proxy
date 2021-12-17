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

// sqlserver_test runs various tests against a SqlServer flavored Cloud SQL instance.
package tests

import (
	"flag"
	"fmt"
	"os"
	"testing"

	_ "github.com/denisenkom/go-mssqldb"
)

var (
	sqlserverConnName = flag.String("sqlserver_conn_name", os.Getenv("SQLSERVER_CONNECTION_NAME"), "Cloud SQL SqlServer instance connection name, in the form of 'project:region:instance'.")
	sqlserverUser     = flag.String("sqlserver_user", os.Getenv("SQLSERVER_USER"), "Name of database user.")
	sqlserverPass     = flag.String("sqlserver_pass", os.Getenv("SQLSERVER_PASS"), "Password for the database user; be careful when entering a password on the command line (it may go into your terminal's history).")
	sqlserverDb       = flag.String("sqlserver_db", os.Getenv("SQLSERVER_DB"), "Name of the database to connect to.")

	sqlserverPort = 1433
)

func requireSqlserverVars(t *testing.T) {
	switch "" {
	case *sqlserverConnName:
		t.Fatal("'sqlserver_conn_name' not set")
	case *sqlserverUser:
		t.Fatal("'sqlserver_user' not set")
	case *sqlserverPass:
		t.Fatal("'sqlserver_pass' not set")
	case *sqlserverDb:
		t.Fatal("'sqlserver_db' not set")
	}
}

func TestSqlServerTcp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SQL Server integration tests")
	}
	requireSqlserverVars(t)

	dsn := fmt.Sprintf("sqlserver://%s:%s@127.0.0.1:5000?database=%s", *sqlserverUser, *sqlserverPass, *sqlserverDb)
	proxyConnTest(t, *sqlserverConnName, "sqlserver", dsn, sqlserverPort, "")
}

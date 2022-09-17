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

// mysql_test runs various tests against a MySQL flavored Cloud SQL instance.
package tests

import (
	"flag"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
	mysql "github.com/go-sql-driver/mysql"
)

var (
	mysqlConnName = flag.String("mysql_conn_name", os.Getenv("MYSQL_CONNECTION_NAME"), "Cloud SQL MYSQL instance connection name, in the form of 'project:region:instance'.")
	mysqlUser     = flag.String("mysql_user", os.Getenv("MYSQL_USER"), "Name of database user.")
	mysqlPass     = flag.String("mysql_pass", os.Getenv("MYSQL_PASS"), "Password for the database user; be careful when entering a password on the command line (it may go into your terminal's history).")
	mysqlDB       = flag.String("mysql_db", os.Getenv("MYSQL_DB"), "Name of the database to connect to.")
)

func requireMySQLVars(t *testing.T) {
	switch "" {
	case *mysqlConnName:
		t.Fatal("'mysql_conn_name' not set")
	case *mysqlUser:
		t.Fatal("'mysql_user' not set")
	case *mysqlPass:
		t.Fatal("'mysql_pass' not set")
	case *mysqlDB:
		t.Fatal("'mysql_db' not set")
	}
}

func TestMySQLTCP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MySQL integration tests")
	}
	requireMySQLVars(t)
	cfg := mysql.Config{
		User:                 *mysqlUser,
		Passwd:               *mysqlPass,
		DBName:               *mysqlDB,
		AllowNativePasswords: true,
		Addr:                 "127.0.0.1:3306",
		Net:                  "tcp",
	}
	proxyConnTest(t, []string{*mysqlConnName}, "mysql", cfg.FormatDSN())
}

func TestMySQLUnix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MySQL integration tests")
	}
	requireMySQLVars(t)
	tmpDir, cleanup := createTempDir(t)
	defer cleanup()

	cfg := mysql.Config{
		User:                 *mysqlUser,
		Passwd:               *mysqlPass,
		DBName:               *mysqlDB,
		AllowNativePasswords: true,
		// re-use utility function to determine the Unix address in a
		// Windows-friendly way.
		Addr: proxy.UnixAddress(tmpDir, *mysqlConnName),
		Net:  "unix",
	}
	proxyConnTest(t,
		[]string{"--unix-socket", tmpDir, *mysqlConnName}, "mysql", cfg.FormatDSN())
}

func TestMySQLAuthWithToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MySQL integration tests")
	}
	requireMySQLVars(t)
	tok, _, cleanup := removeAuthEnvVar(t)
	defer cleanup()

	cfg := mysql.Config{
		User:                 *mysqlUser,
		Passwd:               *mysqlPass,
		DBName:               *mysqlDB,
		AllowNativePasswords: true,
		Addr:                 "127.0.0.1:3306",
		Net:                  "tcp",
	}
	proxyConnTest(t,
		[]string{"--token", tok.AccessToken, *mysqlConnName},
		"mysql", cfg.FormatDSN())
}

func TestMySQLAuthWithCredentialsFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MySQL integration tests")
	}
	requireMySQLVars(t)
	_, path, cleanup := removeAuthEnvVar(t)
	defer cleanup()

	cfg := mysql.Config{
		User:                 *mysqlUser,
		Passwd:               *mysqlPass,
		DBName:               *mysqlDB,
		AllowNativePasswords: true,
		Addr:                 "127.0.0.1:3306",
		Net:                  "tcp",
	}
	proxyConnTest(t,
		[]string{"--credentials-file", path, *mysqlConnName},
		"mysql", cfg.FormatDSN())
}

func TestMySQLAuthWithCredentialsJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MySQL integration tests")
	}
	requireMySQLVars(t)
	creds := keyfile(t)
	_, _, cleanup := removeAuthEnvVar(t)
	defer cleanup()

	cfg := mysql.Config{
		User:                 *mysqlUser,
		Passwd:               *mysqlPass,
		DBName:               *mysqlDB,
		AllowNativePasswords: true,
		Addr:                 "127.0.0.1:3306",
		Net:                  "tcp",
	}
	proxyConnTest(t,
		[]string{"--credentials-json", creds, *mysqlConnName},
		"mysql", cfg.FormatDSN())
}

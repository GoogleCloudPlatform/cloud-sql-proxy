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

	mysql "github.com/go-sql-driver/mysql"
)

var (
	mysqlConnName = flag.String("mysql_conn_name", os.Getenv("MYSQL_CONNECTION_NAME"), "Cloud SQL MYSQL instance connection name, in the form of 'project:region:instance'.")
	mysqlUser     = flag.String("mysql_user", os.Getenv("MYSQL_USER"), "Name of database user.")
	mysqlPass     = flag.String("mysql_pass", os.Getenv("MYSQL_PASS"), "Password for the database user; be careful when entering a password on the command line (it may go into your terminal's history).")
	mysqlDb       = flag.String("mysql_db", os.Getenv("MYSQL_DB"), "Name of the database to connect to.")

	mysqlPort = 3306
)

func requireMysqlVars(t *testing.T) {
	switch "" {
	case *mysqlConnName:
		t.Fatal("'mysql_conn_name' not set")
	case *mysqlUser:
		t.Fatal("'mysql_user' not set")
	case *mysqlPass:
		t.Fatal("'mysql_pass' not set")
	case *mysqlDb:
		t.Fatal("'mysql_db' not set")
	}
}

func TestMysqlTcp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MySQL integration tests")
	}
	requireMysqlVars(t)
	cfg := mysql.Config{
		User:                 *mysqlUser,
		Passwd:               *mysqlPass,
		DBName:               *mysqlDb,
		AllowNativePasswords: true,
		Addr:                 "127.0.0.1:5000",
		Net:                  "tcp",
	}
	proxyConnTest(t, *mysqlConnName, "mysql", cfg.FormatDSN(), mysqlPort, "")
}

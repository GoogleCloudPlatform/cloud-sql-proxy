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

	_ "github.com/microsoft/go-mssqldb"
)

var (
	sqlserverConnName = flag.String("sqlserver_conn_name", os.Getenv("SQLSERVER_CONNECTION_NAME"), "Cloud SQL SqlServer instance connection name, in the form of 'project:region:instance'.")
	sqlserverUser     = flag.String("sqlserver_user", os.Getenv("SQLSERVER_USER"), "Name of database user.")
	sqlserverPass     = flag.String("sqlserver_pass", os.Getenv("SQLSERVER_PASS"), "Password for the database user; be careful when entering a password on the command line (it may go into your terminal's history).")
	sqlserverDB       = flag.String("sqlserver_db", os.Getenv("SQLSERVER_DB"), "Name of the database to connect to.")
)

func requireSQLServerVars(t *testing.T) {
	switch "" {
	case *sqlserverConnName:
		t.Fatal("'sqlserver_conn_name' not set")
	case *sqlserverUser:
		t.Fatal("'sqlserver_user' not set")
	case *sqlserverPass:
		t.Fatal("'sqlserver_pass' not set")
	case *sqlserverDB:
		t.Fatal("'sqlserver_db' not set")
	}
}

func sqlserverDSN() string {
	return fmt.Sprintf("sqlserver://%s:%s@127.0.0.1?database=%s",
		*sqlserverUser, *sqlserverPass, *sqlserverDB)
}

func TestSQLServerTCP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SQL Server integration tests")
	}
	requireSQLServerVars(t)

	proxyConnTest(t, []string{*sqlserverConnName}, "sqlserver", sqlserverDSN())
}

func TestSQLServerImpersonation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SQL Server integration tests")
	}
	requireSQLServerVars(t)

	proxyConnTest(t, []string{
		"--impersonate-service-account", *impersonatedUser,
		*sqlserverConnName},
		"sqlserver", sqlserverDSN())
}

func TestSQLServerAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SQL Server integration tests")
	}
	requireSQLServerVars(t)

	creds := keyfile(t)
	tok, path, cleanup := removeAuthEnvVar(t)
	defer cleanup()

	tcs := []struct {
		desc string
		args []string
	}{
		{
			desc: "with token",
			args: []string{"--token", tok.AccessToken, *sqlserverConnName},
		},
		{
			desc: "with token and impersonation",
			args: []string{
				"--token", tok.AccessToken,
				"--impersonate-service-account", *impersonatedUser,
				*sqlserverConnName},
		},
		{
			desc: "with credentials file",
			args: []string{"--credentials-file", path, *sqlserverConnName},
		},
		{
			desc: "with credentials file and impersonation",
			args: []string{
				"--credentials-file", path,
				"--impersonate-service-account", *impersonatedUser,
				*sqlserverConnName},
		},
		{
			desc: "with credentials JSON",
			args: []string{"--json-credentials", string(creds), *sqlserverConnName},
		},
		{
			desc: "with credentials JSON and impersonation",
			args: []string{
				"--json-credentials", string(creds),
				"--impersonate-service-account", *impersonatedUser,
				*sqlserverConnName},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			proxyConnTest(t, tc.args, "sqlserver", sqlserverDSN())
		})
	}
}

func TestSQLServerGcloudAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SQL Server integration tests")
	}
	requireSQLServerVars(t)

	tcs := []struct {
		desc string
		args []string
	}{
		{
			desc: "gcloud user authentication",
			args: []string{"--gcloud-auth", *sqlserverConnName},
		},
		{
			desc: "gcloud user authentication with impersonation",
			args: []string{
				"--gcloud-auth",
				"--impersonate-service-account", *impersonatedUser,
				*sqlserverConnName},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			proxyConnTest(t, tc.args, "sqlserver", sqlserverDSN())
		})
	}
}

func TestSQLServerHealthCheck(t *testing.T) {
	testHealthCheck(t, *sqlserverConnName)
}

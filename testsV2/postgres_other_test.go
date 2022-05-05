// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !windows
// +build !windows

package tests

import (
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
	_ "github.com/jackc/pgx/v4/stdlib"
)

// TestPostgresUnix tests that the Proxy serves a Unix socket that a Postgres
// driver can connect to. Even though Windows supports supports Unix sockets now
// and Postgres itself supports Unix sockets for Windows, both Go libraries used
// for Postgres do not and will instead always assume a TCP host.
// From the Postgres docs:
//
// "On Unix, an absolute path name begins with a slash. On Windows, paths
// starting with drive letters are also recognized."
//
// See https://www.postgresql.org/docs/current/libpq-connect.html under "34.1.2.
// Parameter Key Words" for "host".
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

// Copyright 2023 Google LLC
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

//go:build !windows && !darwin

package tests

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
)

func TestPostgresFUSEConnect(t *testing.T) {
	if v := os.Getenv("IP_TYPE"); v == "private" || v == "psc" {
		t.Skipf("skipping test because IP_TYPE is set to %v", v)
	}	
	if testing.Short() {
		t.Skip("skipping Postgres integration tests")
	}
	tmpDir, cleanup := createTempDir(t)
	defer cleanup()

	host := proxy.UnixAddress(tmpDir, *postgresConnName)
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s database=%s sslmode=disable",
		host, *postgresUser, *postgresPass, *postgresDB,
	)
	testFUSE(t, tmpDir, host, dsn)
}

func testFUSE(t *testing.T, tmpDir, host string, dsn string) {
	tmpDir2, cleanup2 := createTempDir(t)
	defer cleanup2()

	waitForFUSE := func() error {
		var err error
		for i := 0; i < 10; i++ {
			_, err = os.Stat(host)
			if err == nil {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
		return fmt.Errorf("failed to find FUSE mounted Unix socket: %v", err)
	}

	tcs := []struct {
		desc   string
		dbUser string
		args   []string
	}{
		{
			desc: "using default fuse",
			args: []string{fmt.Sprintf("--fuse=%s", tmpDir), fmt.Sprintf("--fuse-tmp-dir=%s", tmpDir2)},
		},
		{
			desc: "using fuse with auto-iam-authn",
			args: []string{fmt.Sprintf("--fuse=%s", tmpDir), "--auto-iam-authn"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			proxyConnTestWithReady(t, tc.args, "pgx", dsn, waitForFUSE)
			// given the kernel some time to unmount the fuse
			time.Sleep(100 * time.Millisecond)
		})
	}

}

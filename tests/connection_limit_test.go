// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Example invocation:
//   go test -v -run TestConnectionLimit -args -project=my-project \
//     -connection_name=my-project:the-region:sql-name
package tests

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"
)

const (
	maxConnections = 5
	sleepDuration  = 30
	clTestTimeout  = 3 * time.Minute
)

// TestConnectionLimit provisions a new GCE VM and verifies that the proxy
// works on it.  It uses application default credentials.
func TestConnectionLimit(t *testing.T) {
	if *project == "" {
		t.Skipf("Test skipped - 'GCP_PROJECT' env var not set.")
	}
	if *connectionName == "" {
		t.Skipf("Test skipped - 'INSTANCE_CONNECTION_NAME' env var not set.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), clTestTimeout)
	defer cancel()

	ssh, err := setupGCEProxy(ctx, t, []string{"-max_connections", fmt.Sprintf("%d", maxConnections)})
	if err != nil {
		t.Fatal(err)
	}

	cmd := fmt.Sprintf(`mysql -u %s -p%s -S cloudsql/%s -e "SELECT 1; SELECT SLEEP(%d);"`, *dbUser, *dbPass, *connectionName, sleepDuration)
	t.Logf("Connecting using: %s", cmd)

	// Use less than the sshd MaxStartups configuration (defaults to 10)
	var wg sync.WaitGroup
	for i := 0; i < maxConnections; i++ {
		go func() {
			wg.Add(1)
			defer wg.Done()

			log.Print("Starting blocking mysql command")
			var sout, serr bytes.Buffer
			if err := sshRun(ssh, cmd, nil, &sout, &serr); err != nil {
				t.Errorf("Error running mysql: %v\n\nstandard out:\n%s\nstandard err:\n%s", err, &sout, &serr)
			}
			t.Logf("Blocking command output %s", &sout)
		}()
	}

	time.Sleep(time.Second * 5)
	var sout, serr bytes.Buffer
	log.Print("Test connection refusal")
	cmd = fmt.Sprintf(`mysql -u %s -p%s -S cloudsql/%s -e "SELECT 1;"`, *dbUser, *dbPass, *connectionName)
	if err = sshRun(ssh, cmd, nil, &sout, &serr); err == nil {
		t.Fatalf("Mysql connection should have been refused:\n\nstandard out:\n%s\nstandard err:\n%s", &sout, &serr)
	}
	log.Print("Test command output: ", &serr)

	// Wait for all goroutines to exit, else the test panics.
	wg.Wait()
}

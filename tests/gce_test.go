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
//   go test -v -run TestGCE -args -project=my-project \
//     -connection_name=my-project:the-region:sql-name
package tests

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"
)

const gceTestTimeout = 3 * time.Minute

// TestGCE provisions a new GCE VM and verifies that the proxy works on it.
func TestGCE(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), gceTestTimeout)
	defer cancel()

	ssh, err := setupGCEProxy(ctx, t, nil)
	if err != nil {
		t.Fatal(err)
	}

	cmd := fmt.Sprintf(`mysql -u %s -p%s -S cloudsql/%s -e "select 1\\G"`, *dbUser, *dbPass, *connectionName)
	t.Logf("Connecting using: %s", cmd)

	var sout, serr bytes.Buffer
	if err = sshRun(ssh, cmd, nil, &sout, &serr); err != nil {
		t.Fatalf("Error running mysql: %v\n\nstandard out:\n%s\nstandard err:\n%s", err, &sout, &serr)
	}
	t.Log(&sout)
}

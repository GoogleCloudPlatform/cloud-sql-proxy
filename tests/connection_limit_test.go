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

// gce_test is an integration test meant to verify the Cloud SQL Proxy works as
// expected on a Google Compute Engine VM. It provisions a GCE VM, loads a
// newly-compiled proxy client onto that VM, and then does some connectivity tests.
//
// If the VM specified by -vm_name doesn't exist already a new VM is created.
// If a VM does already exist, its 'sshKeys' metadata value is set to a newly
// generated key.
//
// Required flags:
//    -db_name, -project
//
// Example invocation:
//     go test -v -run TestConnectionLimit -args -project=my-project -db_name=my-project:the-region:sql-name
package tests

import (
	"bytes"
	"fmt"
	"testing"
)

// TestConnectionLimit provisions a new GCE VM and verifies that the proxy works on it.
// It uses application default credentials.
func TestConnectionLimit(t *testing.T) {
	err, ssh := setupGCEProxy(t, nil)

	// TODO: implement actual test
	cmd := fmt.Sprintf(`mysql -uroot -S cloudsql/%s -e "select 1\\G"`, *databaseName)
	var sout, serr bytes.Buffer
	if err = sshRun(ssh, cmd, nil, &sout, &serr); err != nil {
		t.Fatalf("Error running mysql: %v\n\nstandard out:\n%s\nstandard err:\n%s", err, &sout, &serr)
	}
	t.Log(&sout)
}

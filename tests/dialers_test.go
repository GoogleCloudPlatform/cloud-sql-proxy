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

// dialers_test verifies that the mysql dialers are functioning properly. It
// expects a Cloud SQL Instance to already exist.
//
// Example invocations:
//   Using default credentials
//     go test -v -run TestDialer -args -connection_name=my-project:the-region:instance-name
//   Using a service account credentials json file
//     go test -v -run TestDialer -args -connection_name=my-project:the-region:instance-name -credential_file /path/to/credentials.json
//   Using an access token
//     go test -v -run TestDialer -args -connection_name=my-project:the-region:instance-name -token "an access token"
package tests

import (
	"database/sql"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"golang.org/x/net/context"
)

const dialersTestTimeout = time.Minute

// TestDialer verifies that the mysql dialer works as expected. It assumes that
// the -connection_name flag has been set to an existing instance.
func TestDialer(t *testing.T) {
	if *project == "" {
		t.Skipf("Test skipped - 'GCP_PROJECT' env var not set.")
	}
	if *connectionName == "" {
		t.Skipf("Test skipped - 'INSTANCE_CONNECTION_NAME' env var not set.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialersTestTimeout)
	defer cancel()

	client, err := clientFromCredentials(ctx)
	if err != nil {
		t.Fatal(err)
	}
	proxy.Init(client, nil, nil)

	var db *sql.DB
	if *dbPass == "" {
		db, err = mysql.Dial(*connectionName, *dbUser)
	} else {
		db, err = mysql.DialPassword(*connectionName, *dbUser, *dbPass)
	}
	if err != nil {
		t.Fatal(err)
	}

	// The mysql.Dial already did a Ping, so we know the connection is valid if
	// there was no error returned.
	db.Close()
}

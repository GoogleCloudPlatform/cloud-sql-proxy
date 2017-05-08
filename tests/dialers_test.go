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

// dialers_test verifies that the mysql dialers are functioning properly.
//
// It expects a Cloud SQL Instance to already exist.
//
// Required flags:
//    -db_name
//
// Example invocations:
//   Using default credentials
//     go test -v dialers_test.go -args -db_name=my-project:the-region:instance-name
//   Using a service account credentials json file
//     go test -v dialers_test.go -args -db_name=my-project:the-region:instance-name -credential_file /path/to/credentials.json
//   Using an access token
//     go test -v dialers_test.go -args -db_name=my-project:the-region:instance-name -token "an access token"
package tests

import (
	"database/sql"
	"flag"
	"testing"

	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	goauth "golang.org/x/oauth2/google"
)

var (
	databaseName = flag.String("db_name", "", "Fully-qualified Cloud SQL Instance (in the form of 'project:region:instance-name')")
	dbUser       = flag.String("db_user", "root", "Name of user to use during test")
	dbPassword   = flag.String("db_pass", "", "Password for user; be careful when entering a password on the command line (it may go into your terminal's history). Also note that using a password along with the Cloud SQL Proxy is not necessary as long as you set the hostname of the user appropriately (see https://cloud.google.com/sql/docs/sql-proxy#user)")
	tokenFile    = flag.String("credential_file", "", `If provided, this json file will be used to retrieve Service Account credentials. You may set the GOOGLE_APPLICATION_CREDENTIALS environment variable for the same effect.`)
	token        = flag.String("token", "", "When set, the proxy uses this Bearer token for authorization.")
)

// TestDialer verifies that the mysql dialer works as expected. It assumes that
// the -db_name flag has been set to an existing instance.
func TestDialer(t *testing.T) {
	if *databaseName == "" {
		t.Fatal("Must set -db_name")
	}

	//if token/tokenFile are not set, the github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql library
	//will automatically attempt to find default authentication via proxy.InitDefault */
	if *token != "" || *tokenFile != "" {
		client, err := clientFromCredentials()
		if err != nil {
			t.Fatal(err)
		}

		proxy.Init(client, nil, nil)
	}

	var db *sql.DB
	var err error
	if *dbPassword == "" {
		db, err = mysql.Dial(*databaseName, *dbUser)
	} else {
		db, err = mysql.DialPassword(*databaseName, *dbUser, *dbPassword)
	}
	if err != nil {
		t.Fatal(err)
	}

	// The mysql.Dial already did a Ping, so we know the connection is valid if
	// there was no error returned.
	db.Close()
}

func clientFromCredentials() (*http.Client, error) {

	const SQLScope = "https://www.googleapis.com/auth/sqlservice.admin"

	ctx := context.Background()

	var client *http.Client

	if f := *tokenFile; f != "" {

		all, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("invalid json file %q: %v", f, err)
		}

		cfg, err := goauth.JWTConfigFromJSON(all, SQLScope)
		if err != nil {
			return nil, fmt.Errorf("invalid json file %q: %v", f, err)
		}

		client = cfg.Client(ctx)

		if err != nil {
			return nil, fmt.Errorf("invalid goauth client %v", err)
		}
	} else if tok := *token; tok != "" {

		src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
		client = oauth2.NewClient(ctx, src)
	}

	return client, nil

}

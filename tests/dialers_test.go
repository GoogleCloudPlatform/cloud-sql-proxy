// dialers_test verifies that the mysql dialers are functioning properly.
//
// It expects a Cloud SQL Instance to already exist.
//
// Required flags:
//    -db_name
//
// Example invocation:
//     go test -v dialers_test.go -args -db_name=my-project:the-region:sql-name
package tests

import (
	"flag"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql"
)

var (
	databaseName = flag.String("db_name", "", "Fully-qualified Cloud SQL Instance (in the form of 'project:region:instance-name')")
	dbUser       = flag.String("db_user", "root", "Name of user to use during test")
)

// TestDialer verifies that the mysql dialer works as expected. It assumes that
// the -db_name flag has been set to an existing instance.
func TestDialer(t *testing.T) {
	if *databaseName == "" {
		t.Fatal("Must set -db_name")
	}
	db, err := mysql.Dial(*databaseName, *dbUser)
	if err != nil {
		t.Fatal(err)
	}
	// The mysql.Dial already did a Ping, so we know the connection is valid.
	db.Close()
}

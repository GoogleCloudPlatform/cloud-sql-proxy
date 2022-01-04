// Copyright 2022 Google LLC
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

package tests

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/certs"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1"
)

func TestClientHandlesSSLReset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping dialer integration tests")
	}
	newClient := func(c *http.Client) *proxy.Client {
		return &proxy.Client{
			Port: 3307,
			Certs: certs.NewCertSourceOpts(c, certs.RemoteOpts{
				UserAgent:      "cloud_sql_proxy/test_build",
				IPAddrTypeOpts: []string{"PUBLIC", "PRIVATE"},
			}),
			Conns: proxy.NewConnSet(),
		}
	}
	connectToDB := func(c *proxy.Client) (*sql.DB, error) {
		var (
			dbUser = *postgresUser
			dbPwd  = *postgresPass
			dbName = *postgresDb
		)
		dsn := fmt.Sprintf("user=%s password=%s database=%s", dbUser, dbPwd, dbName)
		config, err := pgx.ParseConfig(dsn)
		if err != nil {
			return nil, err
		}
		config.DialFunc = func(ctx context.Context, network, instance string) (net.Conn, error) {
			return c.DialContext(ctx, *postgresConnName)
		}
		dbURI := stdlib.RegisterConnConfig(config)
		return sql.Open("pgx", dbURI)
	}
	resetSSL := func(c *http.Client) error {
		svc, err := sqladmin.NewService(context.Background(), option.WithHTTPClient(c))
		if err != nil {
			return err
		}
		project, _, instance, _, _ := proxy.ParseInstanceConnectionName(*postgresConnName)
		t.Log("Resetting SSL config.")
		op, err := svc.Instances.ResetSslConfig(project, instance).Do()
		if err != nil {
			return err
		}
		for {
			t.Log("Waiting for operation to complete.")
			op, err = svc.Operations.Get(project, op.Name).Do()
			if err != nil {
				return err
			}
			if op.Status == "DONE" {
				t.Log("reset SSL config operation complete")
				break
			}
			time.Sleep(time.Second)
		}
		return nil
	}

	// SETUP: create HTTP client and proxy client, then connect to database
	src, err := google.DefaultTokenSource(context.Background(), proxy.SQLScope)
	if err != nil {
		t.Fatal(err)
	}
	client := oauth2.NewClient(context.Background(), src)
	proxyClient := newClient(client)

	db, err := connectToDB(proxyClient)
	if err != nil {
		t.Fatalf("failed to connect to DB: %v", err)
	}

	// Begin database transaction
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	resetSSL(client)

	// Re-dial twice, once to invalidate config, once to establish connection
	var attempts int
	for {
		t.Log("Re-dialing instance")
		_, err = proxyClient.DialContext(context.Background(), *postgresConnName)
		if err != nil {
			t.Logf("Dial error: %v", err)
		}
		if err == nil {
			break
		}
		attempts++
		if attempts > 1 {
			t.Fatalf("could not dial: %v", err)
		}
		time.Sleep(time.Second)
	}

	for i := 0; i < 5; i++ {
		row, err := tx.Query("SELECT 1")
		if err != nil {
			t.Logf("Query after Reset SSL failed as expected after %v retries (error was %v)", i, err)
			break
		}
		row.Close()
		time.Sleep(time.Second)
	}

	if err = db.Ping(); err != nil {
		t.Fatalf("could not re-stablish a DB connection: %v", err)
	}
}

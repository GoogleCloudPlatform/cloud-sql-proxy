package tests

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
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
	src, err := google.DefaultTokenSource(context.Background(), proxy.SQLScope)
	if err != nil {
		t.Fatal(err)
	}
	client := oauth2.NewClient(context.Background(), src)
	c := &proxy.Client{
		Port: 3307,
		Certs: certs.NewCertSourceOpts(client, certs.RemoteOpts{
			UserAgent:      "cloud_sql_proxy/test_build",
			IPAddrTypeOpts: []string{"PUBLIC", "PRIVATE"},
		}),
		Conns: proxy.NewConnSet(),
	}

	var (
		dbUser = *postgresUser
		dbPwd  = *postgresPass
		dbName = *postgresDb
	)
	dsn := fmt.Sprintf("user=%s password=%s database=%s", dbUser, dbPwd, dbName)
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.DialFunc = func(ctx context.Context, network, instance string) (net.Conn, error) {
		return c.DialContext(ctx, *postgresConnName)
	}
	dbURI := stdlib.RegisterConnConfig(config)
	db, err := sql.Open("pgx", dbURI)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	// reset SSL
	svc, err := sqladmin.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}

	project, _, instance, _, _ := proxy.ParseInstanceConnectionName(*postgresConnName)

	t.Log("Reseting SSL config.")
	op, err := svc.Instances.ResetSslConfig(project, instance).Do()
	if err != nil {
		t.Fatal(err)
	}
	for {
		t.Log("Waiting for operation to complete.")
		op, err = svc.Operations.Get(project, op.Name).Do()
		if err != nil {
			t.Fatal(err)
		}
		if op.Status == "DONE" {
			t.Log("reset SSL config operation complete")
			break
		}
		time.Sleep(time.Second)
	}

	// Re-dial twice, once to invalidate config, once to establish connection
	var attempts int
	for {
		t.Log("Re-dialing instance")
		_, err = c.DialContext(context.Background(), os.Getenv("POSTGRES_CONNECTION_NAME"))
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

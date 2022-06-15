module github.com/GoogleCloudPlatform/cloudsql-proxy/v2

go 1.16

require (
	cloud.google.com/go/cloudsqlconn v0.3.2-0.20220603184906-d7e0037d4c9e
	cloud.google.com/go/compute v1.6.1
	contrib.go.opencensus.io/exporter/prometheus v0.4.1
	contrib.go.opencensus.io/exporter/stackdriver v0.13.13
	github.com/GoogleCloudPlatform/cloudsql-proxy v1.29.0
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/denisenkom/go-mssqldb v0.12.2
	github.com/go-sql-driver/mysql v1.6.0
	github.com/google/go-cmp v0.5.8
	github.com/hanwen/go-fuse/v2 v2.1.0
	github.com/jackc/pgx/v4 v4.16.1
	github.com/lib/pq v1.10.6
	github.com/spf13/cobra v1.2.1
	go.opencensus.io v0.23.0
	go.uber.org/zap v1.21.0
	golang.org/x/crypto v0.0.0-20220507011949-2cf3adece122 // indirect
	golang.org/x/net v0.0.0-20220607020251-c690dde0001d
	golang.org/x/oauth2 v0.0.0-20220608161450-d0670ef3b1eb
	golang.org/x/sys v0.0.0-20220610221304-9f5ed59c137d
	golang.org/x/time v0.0.0-20220609170525-579cf78fd858
	google.golang.org/api v0.84.0
)

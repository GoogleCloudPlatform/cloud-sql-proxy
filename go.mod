module github.com/GoogleCloudPlatform/cloudsql-proxy/v2

go 1.16

require (
	cloud.google.com/go/cloudsqlconn v0.4.1-0.20220701163030-bda891776d5d
	cloud.google.com/go/compute v1.7.0
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
	golang.org/x/net v0.0.0-20220624214902-1bab6f366d9e
	golang.org/x/oauth2 v0.0.0-20220722155238-128564f6959c
	golang.org/x/sys v0.0.0-20220624220833-87e55d714810
	golang.org/x/time v0.0.0-20220609170525-579cf78fd858
	google.golang.org/api v0.89.0
)

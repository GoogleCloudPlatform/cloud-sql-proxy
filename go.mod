module github.com/GoogleCloudPlatform/cloudsql-proxy

go 1.16

require (
	cloud.google.com/go/compute v1.9.0
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/denisenkom/go-mssqldb v0.12.2
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/googleapis/gax-go/v2 v2.5.1 // indirect
	github.com/hanwen/go-fuse/v2 v2.1.0
	github.com/jackc/pgx/v4 v4.17.0
	github.com/lib/pq v1.10.7
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.23.0
	golang.org/x/net v0.0.0-20220907135653-1e95f45603a7
	golang.org/x/oauth2 v0.0.0-20220822191816-0ebed06d0094
	golang.org/x/sys v0.0.0-20220907062415-87db552b00fd
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9
	google.golang.org/api v0.95.0
	google.golang.org/genproto v0.0.0-20220902135211-223410557253 // indirect
	google.golang.org/grpc v1.49.0 // indirect
)

replace go.uber.org/atomic v1.10.0 => go.uber.org/atomic v1.7.0

replace go.uber.org/multierr v1.8.0 => go.uber.org/multierr v1.6.0

replace go.uber.org/zap v1.23.0 => go.uber.org/zap v1.22.0

replace golang.org/x/net v0.0.0-20220907135653-1e95f45603a7 => golang.org/x/net v0.0.0-20220624214902-1bab6f366d9e

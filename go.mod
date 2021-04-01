module github.com/GoogleCloudPlatform/cloudsql-proxy

go 1.13

require (
	bazil.org/fuse v0.0.0-20180421153158-65cc252bf669
	cloud.google.com/go v0.80.0
	github.com/denisenkom/go-mssqldb v0.9.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/lib/pq v1.10.0
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20210331212208-0fccb6fa2b5c
	golang.org/x/oauth2 v0.0.0-20210323180902-22b0adad7558
	golang.org/x/sys v0.0.0-20210331175145-43e1dd70ce54
	google.golang.org/api v0.43.0
)

replace bazil.org/fuse => bazil.org/fuse v0.0.0-20180421153158-65cc252bf669 // pin to latest version that supports macOS. see https://github.com/bazil/fuse/issues/224

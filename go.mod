module github.com/GoogleCloudPlatform/cloudsql-proxy

go 1.13

require (
	bazil.org/fuse v0.0.0-20180421153158-65cc252bf669
	cloud.google.com/go v0.94.1
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/denisenkom/go-mssqldb v0.9.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/lib/pq v1.10.3
	go.uber.org/zap v1.19.0
	golang.org/x/net v0.0.0-20210903162142-ad29c8ab022f
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
	golang.org/x/sys v0.0.0-20210906170528-6f6e22806c34
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	google.golang.org/api v0.56.0
)

replace bazil.org/fuse => bazil.org/fuse v0.0.0-20180421153158-65cc252bf669 // pin to latest version that supports macOS. see https://github.com/bazil/fuse/issues/224

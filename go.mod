module github.com/GoogleCloudPlatform/cloudsql-proxy

go 1.13

require (
	bazil.org/fuse v0.0.0-20180421153158-65cc252bf669
	cloud.google.com/go v0.81.0
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/denisenkom/go-mssqldb v0.9.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/lib/pq v1.10.1
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20210510120150-4163338589ed
	golang.org/x/oauth2 v0.0.0-20210427180440-81ed05c6b58c
	golang.org/x/sys v0.0.0-20210507161434-a76c4d0a0096
	google.golang.org/api v0.46.0
)

replace bazil.org/fuse => bazil.org/fuse v0.0.0-20180421153158-65cc252bf669 // pin to latest version that supports macOS. see https://github.com/bazil/fuse/issues/224

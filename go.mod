module github.com/GoogleCloudPlatform/cloudsql-proxy

go 1.13

require (
	bazil.org/fuse v0.0.0-20180421153158-65cc252bf669
	cloud.google.com/go v0.71.0
	github.com/denisenkom/go-mssqldb v0.0.0-20201105234430-3df9532b717d
	github.com/go-sql-driver/mysql v1.5.0
	github.com/lib/pq v1.8.0
	golang.org/x/net v0.0.0-20201031054903-ff519b6c9102
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	google.golang.org/api v0.34.0
)

replace bazil.org/fuse => bazil.org/fuse v0.0.0-20180421153158-65cc252bf669 // pin to latest version that supports macOS. see https://github.com/bazil/fuse/issues/224

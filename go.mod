module github.com/GoogleCloudPlatform/cloudsql-proxy

go 1.13

require (
	bazil.org/fuse v0.0.0-20180421153158-65cc252bf669
	cloud.google.com/go v0.64.0
	github.com/denisenkom/go-mssqldb v0.0.0-20200620013148-b91950f658ec
	github.com/go-sql-driver/mysql v1.5.0
	github.com/lib/pq v1.3.0
	golang.org/x/net v0.0.0-20200822124328-c89045814202
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api v0.30.0
)

replace bazil.org/fuse => bazil.org/fuse v0.0.0-20180421153158-65cc252bf669 // pin to latest version that supports macOS. see https://github.com/bazil/fuse/issues/224

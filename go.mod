module github.com/GoogleCloudPlatform/cloudsql-proxy

go 1.13

require (
	bazil.org/fuse v0.0.0-20180421153158-65cc252bf669
	cloud.google.com/go v0.56.0
	github.com/go-sql-driver/mysql v1.5.0
	github.com/golang/protobuf v1.4.0 // indirect
	github.com/lib/pq v1.3.0
	golang.org/x/crypto v0.0.0-20200420201142-3c4aac89819a
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20200420163511-1957bb5e6d1f // indirect
	google.golang.org/api v0.21.0
	google.golang.org/genproto v0.0.0-20200420144010-e5e8543f8aeb // indirect
	google.golang.org/grpc v1.28.1 // indirect
)

replace bazil.org/fuse => bazil.org/fuse v0.0.0-20180421153158-65cc252bf669 // pin to latest version that supports macOS. see https://github.com/bazil/fuse/issues/224

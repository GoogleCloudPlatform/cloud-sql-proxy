# Cloud SQL proxy dialer for Go

You can also use the Cloud SQL proxy directly from a Go program.

These packages are primarily used as implementation for the Cloud SQL proxy command,
and may be changed in backwards incompatible ways in the future.

## To use inside a Go program:
If your program is written in [Go](https://golang.org) you can use the Cloud SQL Proxy as a library,
avoiding the need to start the Proxy as a companion process.

### MySQL
If you're using the the MySQL [go-sql-driver](https://github.com/go-sql-driver/mysql)
you can use helper functions found in the [`proxy/dialers/mysql` package](https://godoc.org/github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql). See [example usage](https://github.com/GoogleCloudPlatform/cloudsql-proxy/blob/master/tests/dialers_test.go).

### Postgres
If you're using the the Postgres [lib/pq](https://github.com/lib/pq), you can use the `cloudsqlpostgres` driver from [here](https://github.com/GoogleCloudPlatform/cloudsql-proxy/tree/master/proxy/dialers/postgres). See [example usage](https://github.com/GoogleCloudPlatform/cloudsql-proxy/blob/master/proxy/dialers/postgres/hook_test.go).

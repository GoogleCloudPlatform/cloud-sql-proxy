# Cloud SQL proxy dialer for Go

## Note

This package is deprecated. Instead use the [Cloud SQL Go
Connector][go-connector] which has support for MySQL, Postgres, and SQL Server,
in addition to better configurability, support for metrics and tracing, and a
number of additional features. See the [migration guide][migration-guide] for
how to switch.

## Overview

You can also use the Cloud SQL proxy directly from a Go program.

These packages are primarily used as implementation for the Cloud SQL proxy
command, and may be changed in backwards incompatible ways in the future.

## Usage

If your program is written in [Go](https://golang.org) you can use the Cloud SQL
Proxy as a library, avoiding the need to start the Proxy as a companion process.

Alternatively, there are Cloud SQL Connectors for [Java][] and [Python][].


### MySQL

If you're using the MySQL [go-sql-driver][go-mysql] you can use helper
functions found in the [`proxy/dialers/mysql`][mysql-godoc]

See [example usage](dialers/mysql/hook_test.go).

### Postgres

If you're using the Postgres [lib/pq](https://github.com/lib/pq), you can
use the `cloudsqlpostgres` driver from [here](proxy/dialers/postgres).

See [example usage](dialers/postgres/hook_test.go).

[Java]: https://github.com/GoogleCloudPlatform/cloud-sql-jdbc-socket-factory
[Python]: https://github.com/GoogleCloudPlatform/cloud-sql-python-connector
[go-mysql]: https://github.com/go-sql-driver/mysql
[mysql-godoc]: https://pkg.go.dev/github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql
[go-connector]: https://pkg.go.dev/cloud.google.com/go/cloudsqlconn
[migration-guide]: https://github.com/GoogleCloudPlatform/cloud-sql-go-connector/blob/main/migration-guide.md

# Cloud SQL Auth proxy

[![CI][ci-badge]][ci-build]

The Cloud SQL Auth proxy is a binary that provides IAM-based authorization and
TLS 1.3 encryption for client connections to a Cloud SQL instance.

In addition, the proxy has support for:

* [Auto IAM Authentication][iam-auth] (Postgres only)
* [Cloud Monitoring][] and [Cloud Trace][]
* [Prometheus][]
* [HTTP Healthchecks][health-check-example] for use with Kubernetes

For users migrating from v1, see the [Migration Guide](migration-guide).

[Cloud Monitoring]: https://cloud.google.com/monitoring
[Cloud Trace]: https://cloud.google.com/trace
[Prometheus]: https://prometheus.io/

## Installation

For 64-bit Linux, run:

```shell
wget "https://storage.googleapis.com/cloud-sql-connectors/v2.0.0-preview.1/cloudsql-proxy.linux.amd64" -O cloudsql-proxy
chmod +x cloudsql-proxy
```

Releases for additional OS's and architectures and be found on the [releases page][releases].

### Install from Source

To install from source, ensure you have the latest version of [Go installed](https://go.dev/doc/install).

Then, simply run:

```shell
go install github.com/GoogleCloudPlatform/cloudsql-proxy@latest
```

The `cloudsql-proxy` will be placed in `$GOPATH/bin` or `$HOME/go/bin`.

## Usage

The following examples all reference an `INSTANCE_CONNECTION_NAME`, which takes
the form: `myproject:myregion:myinstance`.

To find your Cloud SQL instance's `INSTANCE_CONNECTION_NAME`, run:

```shell
gcloud sql instances describe <INSTANCE_NAME> --format='value(connectionName)'
```

### Basic Usage

To start the proxy, use:

```shell
# starts the proxy listening on localhost with the default database engine port
# For example:
#   MySQL      localhost:3306
#   Postgres   localhost:5432
#   SQL Server localhost:1433
./cloudsql-proxy <INSTANCE_CONNECTION_NAME>
```

The proxy supports multiple instances:

``` shell
./cloudsql-proxy <INSTANCE_CONNECTION_NAME_1> <INSTANCE_CONNECTION_NAME_2>
```

To override the port, use the `--port` flag:

```shell
# Starts a listener on localhost:6000
./cloudsql-proxy --port 6000 <INSTANCE_CONNECTION_NAME>
```

To overide the choice of `localhost`, use the `--addr` flag:

```shell
# Starts a listener on all interfaces at port 5432
./cloudsql-proxy --addr 0.0.0.0 <INSTANCE_CONNECTION_NAME>
```

By default, the proxy attempts to connect to an instance's public IP. To enable
private IP, use:

``` shell
# Starts a listener connected to the private IP of the Cloud SQL instance.
# Note: there must be a network path present for this to work.
./cloudsql-proxy --private-ip <INSTANCE_CONNECTION_NAME>
```

The proxy also supports Unix sockets. To start the proxy with Unix sockets, run:

``` shell
# Uses the directory "/mycooldir" to create a Unix socket
# For example, the following directory would be created:
#   /mycooldir/myproject:myregion:myinstance
./cloudsql-proxy --unix-socket /mycooldir <INSTANCE_CONNECTION_NAME>
```

To see a full list of flags, use:

``` shell
./cloudsql-proxy -h
```

### Advanced Usage

The proxy supports overriding settings on a per-instance basis using a
query-param style syntax. When using the query-param syntax, instance connection
names will need to be wrapped in quotes to avoid any unintended interaction with
the shell.

For example, to configure ports on a per instance basis, use:

``` shell
# Starts a listener on localhost:5000 for the instance called "postgres"
# and starts a listener on localhost:3306 for the instance called "mysql"
./cloudsql-proxy 'myproject:my-region:postgres?port=5000' \
    myproject:my-region:mysql
```

To bind one listener to all interfaces for only a single instance, use:

``` shell
# Starts a listener on 0.0.0.0 for "postgres" at port 5432
# and a listener on localhost:3306 for "mysql"
./cloudsql-proxy 'myproject:my-region:postgres?addr=0.0.0.0' \
    myproject:my-region:mysql
```

Many flags are also supported as a query-params. For a list of what flags are
supported, use:

``` shell
./cloudsql-proxy -h
```

## Credentials

The Cloud SQL proxy uses a Cloud IAM principal to authorize connections against 
a Cloud SQL instance. The proxy sources the credentials using 
[Application Default Credentials](https://cloud.google.com/docs/authentication/production).

Note: Any IAM principal connecting to a Cloud SQL database will need one of the
following IAM roles:

- Cloud SQL Client (preferred)
- Cloud SQL Editor
- Cloud SQL Admin

Or one may manually assign the following IAM permissions:

- `cloudsql.instances.connect`
- `cloudsql.instances.get`

See [Roles and Permissions in Cloud SQL][roles-and-permissions] for details.

When the proxy authenticates under the Compute Engine VM's default service
account, the VM must have at least the `sqlservice.admin` API scope (i.e.,
"https://www.googleapis.com/auth/sqlservice.admin") and the associated project
must have the SQL Admin API enabled. The default service account must also have
at least writer or editor privileges to any projects of target SQL instances.

The proxy also supports three flags related to credentials:

- `--token` to use an OAuth2 token
- `--credentials-file` to use a service account key file
- `--gcloud-auth` to use the Gcloud user's credentials (local development only)

## Container Images

There are containerized versions of the proxy available from the following
Google Cloud Container Registry repositories:

- `gcr.io/cloud-sql-connectors/auth-proxy`
- `us.gcr.io/cloud-sql-connectors/auth-proxy`
- `eu.gcr.io/cloud-sql-connectors/auth-proxy`
- `asia.gcr.io/cloud-sql-connectors/auth-proxy`

Each image is tagged with the associated proxy version. The following tags are
currently supported:

- `$VERSION` (default)
- `$VERSION-alpine`
- `$VERSION-buster`

We recommend pinning to a specific version tag and using automation with a CI pipeline
to update regularly.

The default container image uses [distroless][]. If you need a shell
or related tools, use the Alpine or Buster images listed above.

[distroless]: https://github.com/GoogleContainerTools/distroless

## Running as a Kubernetes Sidecar

See the [example here][sidecar-example] as well as [Connecting from Google
Kubernetes Engine][connect-to-k8s].

## Running behind a Socks5 proxy

The Cloud SQL Auth proxy includes support for sending requests through a SOCKS5
proxy. If a SOCKS5 proxy is running on `localhost:8000`, the command to start
the Cloud SQL Auth Proxy would look like:

```
ALL_PROXY=socks5://localhost:8000 \
HTTPS_PROXY=socks5://localhost:8000 \
    cloudsql-proxy <INSTANCE_CONNECTION_NAME>
```

The `ALL_PROXY` environment variable specifies the proxy for all TCP
traffic to and from a Cloud SQL instance. The `ALL_PROXY` environment variable
supports `socks5` and `socks5h` protocols. To route DNS lookups through a proxy,
use the `socks5h` protocol.

The `HTTPS_PROXY` (or `HTTP_PROXY`) specifies the proxy for all HTTP(S) traffic
to the SQL Admin API. Specifying `HTTPS_PROXY` or `HTTP_PROXY` is only necessary
when you want to proxy this traffic. Otherwise, it is optional. See
[`http.ProxyFromEnvironment`](https://pkg.go.dev/net/http@go1.17.3#ProxyFromEnvironment)
for possible values.

## Reference Documentation

- [Cloud SQL][cloud-sql]
- [Cloud SQL Auth proxy Documentation][proxy-page]
- [Cloud SQL Auth proxy Quickstarts][quickstarts]
- [Cloud SQL Code Samples][code-samples]
- [Cloud SQL Auth proxy Package Documentation][pkg-docs]

## Support policy

### Major version lifecycle

This project uses [semantic versioning](https://semver.org/), and uses the
following lifecycle regarding support for a major version:

**Active** - Active versions get all new features and security fixes (that
wouldn’t otherwise introduce a breaking change). New major versions are
guaranteed to be "active" for a minimum of 1 year.
**Deprecated** - Deprecated versions continue to receive security and critical
bug fixes, but do not receive new features. Deprecated versions will be publicly
supported for 1 year.
**Unsupported** - Any major version that has been deprecated for >=1 year is
considered publicly unsupported.

### Supported Go Versions

We test and support at least the latest 3 Go versions. Changes in supported Go
versions will be considered a minor change, and will be noted in the release notes.

### Release cadence

The Cloud SQL Auth proxy aims for a minimum monthly release cadence. If no new
features or fixes have been added, a new PATCH version with the latest
dependencies is released.

## Contributing

Contributions are welcome. Please, see the [CONTRIBUTING][contributing] document
for details.

Please note that this project is released with a Contributor Code of Conduct.
By participating in this project you agree to abide by its terms. See
[Contributor Code of Conduct][code-of-conduct] for more information.

[about-proxy]: https://cloud.google.com/sql/docs/mysql/sql-proxy
[ci-badge]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/actions/workflows/tests.yaml/badge.svg?event=push
[ci-build]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/actions/workflows/tests.yaml?query=event%3Apush+branch%3Amain
[cloud-sql]: https://cloud.google.com/sql
[code-samples]: https://cloud.google.com/sql/docs/mysql/samples
[code-of-conduct]: CONTRIBUTING.md#contributor-code-of-conduct
[connect-to-k8s]: https://cloud.google.com/sql/docs/mysql/connect-kubernetes-engine
[connection-overview]: https://cloud.google.com/sql/docs/mysql/connect-overview
[contributing]: CONTRIBUTING.md
[health-check-example]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/tree/main/examples/k8s-health-check#cloud-sql-proxy-health-checks
[iam-auth]: https://cloud.google.com/sql/docs/postgres/authentication
[pkg-badge]: https://pkg.go.dev/badge/github.com/GoogleCloudPlatform/cloudsql-proxy.svg
[pkg-docs]: https://pkg.go.dev/github.com/GoogleCloudPlatform/cloudsql-proxy
[private-ip]: https://cloud.google.com/sql/docs/mysql/private-ip#requirements_for_private_ip
[proxy-page]: https://cloud.google.com/sql/docs/mysql/sql-proxy
[quickstarts]: https://cloud.google.com/sql/docs/mysql/quickstarts
[releases]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/releases
[roles-and-permissions]: https://cloud.google.com/sql/docs/mysql/roles-and-permissions
[service-account]: https://cloud.google.com/iam/docs/service-accounts
[sidecar-example]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/tree/master/examples/k8s-sidecar#run-the-cloud-sql-proxy-as-a-sidecar

# Cloud SQL Auth proxy

[![CI][ci-badge]][ci-build]

The Cloud SQL Auth proxy is a utility for ensuring secure connections to your
Cloud SQL instances. It provides IAM authorization, allowing you to control who
can connect to your instance through IAM permissions, and TLS 1.3 encryption,
without having to manage certificates.

See the [Connecting Overview][connection-overview] page for more information on
connecting to a Cloud SQL instance, or the [About the proxy][about-proxy] page
for details on how the Cloud SQL proxy works.

The Cloud SQL Auth proxy has support for:

- [Automatic IAM Authentication][iam-auth] (Postgres only)
- Metrics ([Cloud Monitoring][], [Cloud Trace][], and [Prometheus][])
- [HTTP Healthchecks][health-check-example]
- Service account impersonation
- Separate Dialer functionality released as the [Cloud SQL Go Connector][go connector]
- Configuration with environment variables
- Fully POSIX-compliant flags

If you're using Go, Java, or Python, consider using the corresponding Cloud SQL
connector which does everything the proxy does, but in process:

- [Cloud SQL Go connector][go connector]
- [Cloud SQL Java connector][java connector]
- [Cloud SQL Python connector][python connector]

For users migrating from v1, see the [Migration Guide](migration-guide.md).
The [v1 README][v1 readme] is still available.

NOTE: The proxy does not configure the network between the VM it's running on
and the Cloud SQL instance. You MUST ensure the proxy can reach your Cloud SQL
instance, either by deploying it in a VPC that has access to your Private IP
instance, or by configuring Public IP.

[cloud monitoring]: https://cloud.google.com/monitoring
[cloud trace]: https://cloud.google.com/trace
[prometheus]: https://prometheus.io/
[go connector]: https://github.com/GoogleCloudPlatform/cloud-sql-go-connector
[java connector]: https://github.com/GoogleCloudPlatform/cloud-sql-jdbc-socket-factory
[python connector]: https://github.com/GoogleCloudPlatform/cloud-sql-python-connector
[v1 readme]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/blob/5f5b09b62eb6dfcaa58ce399d0131c1544bf813f/README.md

## Installation

Check for the latest version on the [releases page][releases] and use the
following instructions for your OS and CPU architecture.

<details open>
<summary>Linux amd64</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.0.0-preview.3"

wget "$URL/cloud-sql-proxy.linux.amd64" -O cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Linux 386</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.0.0-preview.3"

wget "$URL/cloud-sql-proxy.linux.386" -O cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Linux arm64</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.0.0-preview.3"

wget "$URL/cloud-sql-proxy.linux.arm64" -O cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Linux arm</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.0.0-preview.3"

wget "$URL/cloud-sql-proxy.linux.arm" -O cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Mac (Intel)</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.0.0-preview.3"

wget "$URL/cloud-sql-proxy.darwin.amd64" -O cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Mac (Apple Silicon)</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.0.0-preview.3"

wget "$URL/cloud-sql-proxy.darwin.arm64" -O cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Windows x64</summary>

```sh
# see Releases for other versions
wget https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.0.0-preview.3/cloud-sql-proxy.x64.exe -O cloud-sql-proxy.exe
```

</details>

<details>
<summary>Windows x86</summary>

```sh
# see Releases for other versions
wget https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.0.0-preview.3/cloud-sql-proxy.x86.exe -O cloud-sql-proxy.exe
```

</details>

### Install from Source

To install from source, ensure you have the latest version of [Go installed](https://go.dev/doc/install).

Then, simply run:

```shell
go install github.com/GoogleCloudPlatform/cloud-sql-proxy/v2@latest
```

The `cloud-sql-proxy` will be placed in `$GOPATH/bin` or `$HOME/go/bin`.

## Usage

The following examples all reference an `INSTANCE_CONNECTION_NAME`, which takes
the form: `myproject:myregion:myinstance`.

To find your Cloud SQL instance's `INSTANCE_CONNECTION_NAME`, visit the detail
page of your Cloud SQL instance in the console, or use `gcloud` with:

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
./cloud-sql-proxy <INSTANCE_CONNECTION_NAME>
```

The proxy will automatically detect the default database engine's port and start
a corresponding listener. Production deployments should use the --port flag to
reduce startup time.

The proxy supports multiple instances:

```shell
./cloud-sql-proxy <INSTANCE_CONNECTION_NAME_1> <INSTANCE_CONNECTION_NAME_2>
```

### Configuring Port

To override the port, use the `--port` flag:

```shell
# Starts a listener on localhost:6000
./cloud-sql-proxy --port 6000 <INSTANCE_CONNECTION_NAME>
```

When specifying multiple instances, the port will increment from the flag value:

```shell
# Starts a listener on localhost:6000 for INSTANCE_CONNECTION_1
# and localhost:6001 for INSTANCE_CONNECTION_NAME_2.
./cloud-sql-proxy --port 6000 <INSTANCE_CONNECTION_NAME_1> <INSTANCE_CONNECTION_NAME_2>
```

To configure ports on a per instance basis, use the `port` query param:

```shell
# Starts a listener on localhost:5000 for the instance called "postgres"
# and starts a listener on localhost:6000 for the instance called "mysql"
./cloud-sql-proxy \
    'myproject:my-region:postgres?port=5000' \
    'myproject:my-region:mysql?port=6000'
```

### Configuring Listening Address

To overide the choice of `localhost`, use the `--address` flag:

```shell
# Starts a listener on all interfaces at port 5432
./cloud-sql-proxy --address 0.0.0.0 <INSTANCE_CONNECTION_NAME>
```

To override address on a per-instance basis, use the `address` query param:

```shell
# Starts a listener on 0.0.0.0 for "postgres" at port 5432
# and a listener on 10.0.0.1:3306 for "mysql"
./cloud-sql-proxy \
    'myproject:my-region:postgres?address=0.0.0.0' \
    'myproject:my-region:mysql?address=10.0.0.1"
```

### Configuring Private IP

By default, the proxy attempts to connect to an instance's public IP. To enable
private IP, use:

```shell
# Starts a listener connected to the private IP of the Cloud SQL instance.
# Note: there must be a network path present for this to work.
./cloud-sql-proxy --private-ip <INSTANCE_CONNECTION_NAME>
```

NOTE: The proxy does not configure the network. You MUST ensure the proxy can
reach your Cloud SQL instance, either by deploying it in a VPC that has access
to your Private IP instance, or by configuring Public IP.

### Configuring Unix domain sockets

The proxy also supports [Unix domain sockets](https://en.wikipedia.org/wiki/Unix_domain_socket).
To start the proxy with Unix sockets, run:

```shell
# Uses the directory "/mycooldir" to create a Unix socket
# For example, the following directory would be created:
#   /mycooldir/myproject:myregion:myinstance
./cloud-sql-proxy --unix-socket /mycooldir <INSTANCE_CONNECTION_NAME>
```

To configure a Unix domain socket on a per-instance basis, use the `unix-socket`
query param:

```shell
# Starts a TCP listener on localhost:5432 for "postgres"
# and creates a Unix domain socket for "mysql":
#     /cloudsql/myproject:my-region:mysql
./cloud-sql-proxy \
    myproject:my-region:postgres \
    'myproject:my-region:mysql?unix-socket=/cloudsql'
```

NOTE: The proxy supports Unix domain sockets on recent versions of Windows, but
replaces colons with periods:

```shell
# Starts a Unix domain socket at the path:
#    C:\cloudsql\myproject.my-region.mysql
./cloud-sql-proxy --unix-socket C:\cloudsql myproject:my-region:mysql
```

### Additional flags

To see a full list of flags, use:

```shell
./cloud-sql-proxy --help
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

- `gcr.io/cloud-sql-connectors/cloud-sql-proxy`
- `us.gcr.io/cloud-sql-connectors/cloud-sql-proxy`
- `eu.gcr.io/cloud-sql-connectors/cloud-sql-proxy`
- `asia.gcr.io/cloud-sql-connectors/cloud-sql-proxy`

Each image is tagged with the associated proxy version. The following tags are
currently supported:

- `$VERSION` (default)
- `$VERSION-alpine`
- `$VERSION-buster`
- `$VERSION-bullseye`

The `$VERSION` is the proxy version without the leading "v" (e.g.,
`2.0.0-preview.3`).

For example, to pull a particular version, use a command like:

``` shell
# $VERSION is 2.0.0-preview.3
docker pull gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.0.0-preview.3
```

We recommend pinning to a specific version tag and using automation with a CI pipeline
to update regularly.

The default container image uses [distroless][] with a non-root user. If you
need a shell or related tools, use the Alpine or Buster images listed above.

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
    cloud-sql-proxy <INSTANCE_CONNECTION_NAME>
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

## Support for Metrics and Tracing

The proxy supports [Cloud Monitoring][], [Cloud Trace][], and [Prometheus][].

Supported metrics include:

- `/cloudsqlconn/dial_latency`: The distribution of dialer latencies (ms)
- `/cloudsqlconn/open_connections`: The current number of open Cloud SQL
  connections
- `/cloudsqlconn/dial_failure_count`: The number of failed dial attempts
- `/cloudsqlconn/refresh_success_count`: The number of successful certificate
  refresh operations
- `/cloudsqlconn/refresh_failure_count`: The number of failed refresh
  operations.

Supported traces include:

- `cloud.google.com/go/cloudsqlconn.Dial`: The dial operation including
  refreshing an ephemeral certificate and connecting the instance
- `cloud.google.com/go/cloudsqlconn/internal.InstanceInfo`: The call to retrieve
  instance metadata (e.g., database engine type, IP address, etc)
- `cloud.google.com/go/cloudsqlconn/internal.Connect`: The connection attempt
  using the ephemeral certificate
- SQL Admin API client operations

To enable Cloud Monitoring and Cloud Trace, use the `--telemetry-project` flag
with the project where you want to view metrics and traces. To configure the
metrics prefix used by Cloud Monitoring, use the `--telemetry-prefix` flag. When
enabling telementry, both Cloud Monitoring and Cloud Trace are enabled. To
disable Cloud Monitoring, use `--disable-metrics`. To disable Cloud Trace, use
`--disable-traces`.

To enable Prometheus, use the `--prometheus` flag. This will start an HTTP
server on localhost with a `/metrics` endpoint. The Prometheus namespace may
optionally be set with `--prometheus-namespace`.

## Localhost Admin Server

The Proxy includes support for an admin server on localhost. By default, the
admin server is not enabled. To enable the server, pass the `--debug` flag.
This will start the server on localhost at port 9091. To change the port, use
the `--admin-port` flag.

The admin server includes Go's pprof tool and is available at `/debug/pprof/`.

See the [documentation on pprof][pprof] for details on how to use the
profiler.

[pprof]: https://pkg.go.dev/net/http/pprof.

## Frequently Asked Questions

### Why would I use the proxy?

The proxy is a convenient way to control access to your database using IAM
permissions while ensuring a secure connection to your Cloud SQL instance. When
using the proxy, you do not have to manage database client certificates,
configured Authorized Networks, or ensure clients connect securely. The proxy
handles all of this for you.

### How should I use the proxy?

The proxy is a gateway to your Cloud SQL instance. Clients connect to the proxy
over an unencrypted connection and are authorized using the environment's IAM
principal. The proxy then encrypts the connection to your Cloud SQL instance.

Because client connections are not encrypted and authorized using the
environment's IAM principal, we recommend running the proxy on the same VM or
Kubernetes pod as your application and using the proxy's default behavior of
allowing connections from only the local network interface. This is the most
secure configuration: unencrypted traffic does not leave the VM, and only
connections from applications on the VM are allowed.

Here are some common examples of how to run the proxy in different environments:

- [Connect to Cloud SQL for MySQL from your local computer][local-quickstart]
- [Connect to Cloud SQL for MySQL from Google Kubernetes Engine][gke-quickstart]

[local-quickstart]: https://cloud.google.com/sql/docs/mysql/connect-instance-local-computer
[gke-quickstart]: https://cloud.google.com/sql/docs/mysql/connect-instance-kubernetes

### Why can't the proxy connect to my private IP instance?

The proxy does not configure the network between the VM it's running on and the
Cloud SQL instance. You MUST ensure the proxy can reach your Cloud SQL
instance, either by deploying it in a VPC that has access to your Private IP
instance, or by configuring Public IP.

### Is there a library version of the proxy that I can use?

Yes. Cloud SQL supports three language connectors:

- [Cloud SQL Go Connector][go connector]
- [Cloud SQL Java Connector](https://github.com/GoogleCloudPlatform/cloud-sql-jdbc-socket-factory)
- [Cloud SQL Python Connector](https://github.com/GoogleCloudPlatform/cloud-sql-python-connector)

The connectors for Go, Java, and Python offer the best experience when you are
writing an application in those languages. Use the proxy when your application
uses another language.

### Should I use the proxy for large deployments?

We recommend deploying the proxy on the host machines that are running the
application. However, large deployments may exceed the request quota for the SQL
Admin API . If your proxy reports request quota errors, we recommend deploying
the proxy with a connection pooler like [pgbouncer][] or [ProxySQL][]. For
details, see [Running the Cloud SQL Proxy as a Service][service-example].

### Can I share the proxy across mulitple applications?

Instead of using a single proxy across multiple applications, we recommend using
one proxy instance for every application process. The proxy uses the context's
IAM principal and so have a 1-to-1 mapping between application and IAM principal
is best. If multiple applications use the same proxy instance, then it becomes
unclear from an IAM perspective which principal is doing what.\*\*\*\*

[pgbouncer]: https://www.pgbouncer.org/
[proxysql]: https://www.proxysql.com/

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

### Release cadence

The Cloud SQL Auth proxy aims for a minimum monthly release cadence. If no new
features or fixes have been added, a new PATCH version with the latest
dependencies is released.

We support releases for 1 year from the release date.

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
[iam-auth]: https://cloud.google.com/sql/docs/postgres/authentication#automatic
[pkg-badge]: https://pkg.go.dev/badge/github.com/GoogleCloudPlatform/cloudsql-proxy.svg
[pkg-docs]: https://pkg.go.dev/github.com/GoogleCloudPlatform/cloudsql-proxy
[private-ip]: https://cloud.google.com/sql/docs/mysql/private-ip#requirements_for_private_ip
[proxy-page]: https://cloud.google.com/sql/docs/mysql/sql-proxy
[quickstarts]: https://cloud.google.com/sql/docs/mysql/quickstarts
[releases]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/releases
[roles-and-permissions]: https://cloud.google.com/sql/docs/mysql/roles-and-permissions
[service-account]: https://cloud.google.com/iam/docs/service-accounts
[sidecar-example]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/tree/master/examples/k8s-sidecar#run-the-cloud-sql-proxy-as-a-sidecar
[service-example]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/tree/main/examples/k8s-service

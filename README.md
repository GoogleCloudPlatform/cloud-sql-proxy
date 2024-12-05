# Cloud SQL Auth Proxy

[![CI][ci-badge]][ci-build]

The Cloud SQL Auth Proxy is a utility for ensuring secure connections to your
Cloud SQL instances. It provides IAM authorization, allowing you to control who
can connect to your instance through IAM permissions, and TLS 1.3 encryption,
without having to manage certificates.

See the [Connecting Overview][connection-overview] page for more information on
connecting to a Cloud SQL instance, or the [About the Proxy][about-proxy] page
for details on how the Cloud SQL Proxy works.

The Cloud SQL Auth Proxy has support for:

- [Automatic IAM Authentication][iam-auth] (Postgres and MySQL only)
- Metrics ([Cloud Monitoring][], [Cloud Trace][], and [Prometheus][])
- [HTTP Healthchecks][health-check-example]
- Service account impersonation
- Separate Dialer functionality released as the [Cloud SQL Go Connector][go connector]
- Configuration with [environment variables](#config-environment-variables)
- Fully POSIX-compliant flags

If you're using Go, Java, Python, or Node.js, consider using the corresponding Cloud SQL
connector which does everything the Proxy does, but in process:

- [Cloud SQL Go connector][go connector]
- [Cloud SQL Java connector][java connector]
- [Cloud SQL Python connector][python connector]
- [Cloud SQL Node.js connector][node connector]

For users migrating from v1, see the [Migration Guide](migration-guide.md).
The [v1 README][v1 readme] is still available.

> [!IMPORTANT]
> 
> The Proxy does not configure the network between the VM it's running on
> and the Cloud SQL instance. You MUST ensure the Proxy can reach your Cloud SQL
> instance, either by deploying it in a VPC that has access to your Private IP
> instance, or by configuring Public IP.

[cloud monitoring]: https://cloud.google.com/monitoring
[cloud trace]: https://cloud.google.com/trace
[prometheus]: https://prometheus.io/
[go connector]: https://github.com/GoogleCloudPlatform/cloud-sql-go-connector
[java connector]: https://github.com/GoogleCloudPlatform/cloud-sql-jdbc-socket-factory
[python connector]: https://github.com/GoogleCloudPlatform/cloud-sql-python-connector
[node connector]: https://github.com/GoogleCloudPlatform/cloud-sql-nodejs-connector
[v1 readme]: https://github.com/GoogleCloudPlatform/cloudsql-proxy/blob/5f5b09b62eb6dfcaa58ce399d0131c1544bf813f/README.md

## Installation

Check for the latest version on the [releases page][releases] and use the
following instructions for your OS and CPU architecture.

<!-- {x-release-please-start-version} -->
<details open>
<summary>Linux amd64</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1"

curl "$URL/cloud-sql-proxy.linux.amd64" -o cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Linux 386</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1"

curl "$URL/cloud-sql-proxy.linux.386" -o cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Linux arm64</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1"

curl "$URL/cloud-sql-proxy.linux.arm64" -o cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Linux arm</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1"

curl "$URL/cloud-sql-proxy.linux.arm" -o cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Mac (Intel)</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1"

curl "$URL/cloud-sql-proxy.darwin.amd64" -o cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Mac (Apple Silicon)</summary>

```sh
# see Releases for other versions
URL="https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1"

curl "$URL/cloud-sql-proxy.darwin.arm64" -o cloud-sql-proxy

chmod +x cloud-sql-proxy
```

</details>

<details>
<summary>Windows x64</summary>

```sh
# see Releases for other versions
curl https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1/cloud-sql-proxy.x64.exe -o cloud-sql-proxy.exe
```

</details>

<details>
<summary>Windows x86</summary>

```sh
# see Releases for other versions
curl https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1/cloud-sql-proxy.x86.exe -o cloud-sql-proxy.exe
```

</details>
<!-- {x-release-please-end} -->

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

### Credentials

The Cloud SQL Proxy uses a Cloud IAM principal to authorize connections against
a Cloud SQL instance. The Proxy sources the credentials using
[Application Default Credentials](https://cloud.google.com/docs/authentication/production).

> [!NOTE]
>
> Any IAM principal connecting to a Cloud SQL database will need one of the
> following IAM roles:
>
> - Cloud SQL Client (preferred)
> - Cloud SQL Editor
> - Cloud SQL Admin
>
> Or one may manually assign the following IAM permissions:
> 
> - `cloudsql.instances.connect`
> - `cloudsql.instances.get`
>
> See [Roles and Permissions in Cloud SQL][roles-and-permissions] for details.

When the Proxy authenticates under the Compute Engine VM's default service
account, the VM must have at least the `sqlservice.admin` API scope (i.e.,
"https://www.googleapis.com/auth/sqlservice.admin") and the associated project
must have the SQL Admin API enabled. The default service account must also have
at least writer or editor privileges to any projects of target SQL instances.

The Proxy also supports three flags related to credentials:

- `--token` to use an OAuth2 token
- `--credentials-file` to use a service account key file
- `--gcloud-auth` to use the Gcloud user's credentials (local development only)

### Basic Usage

To start the Proxy, use:

```shell
# starts the Proxy listening on localhost with the default database engine port
# For example:
#   MySQL      localhost:3306
#   Postgres   localhost:5432
#   SQL Server localhost:1433
./cloud-sql-proxy <INSTANCE_CONNECTION_NAME>
```

The Proxy will automatically detect the default database engine's port and start
a corresponding listener. Production deployments should use the --port flag to
reduce startup time.

The Proxy supports multiple instances:

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

To override the choice of `localhost`, use the `--address` flag:

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

By default, the Proxy attempts to connect to an instance's public IP. To enable
private IP, use:

```shell
# Starts a listener connected to the private IP of the Cloud SQL instance.
# Note: there must be a network path present for this to work.
./cloud-sql-proxy --private-ip <INSTANCE_CONNECTION_NAME>
```

> [!IMPORTANT]
> 
> The Proxy does not configure the network. You MUST ensure the Proxy can
> reach your Cloud SQL instance, either by deploying it in a VPC that has access
> to your Private IP instance, or by configuring Public IP.

### Configuring Unix domain sockets

The Proxy also supports [Unix domain sockets](https://en.wikipedia.org/wiki/Unix_domain_socket).
To start the Proxy with Unix sockets, run:

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

> [!NOTE]
>
> The Proxy supports Unix domain sockets on recent versions of Windows, but
> replaces colons with periods:
> 
> ```shell
> # Starts a Unix domain socket at the path:
> #    C:\cloudsql\myproject.my-region.mysql
> ./cloud-sql-proxy --unix-socket C:\cloudsql myproject:my-region:mysql
> ```

### Testing Connectivity

The Proxy includes support for a connection test on startup. This test helps
ensure the Proxy can reach the associated instance and is a quick debugging
tool. The test will attempt to connect to the specified instance(s) and fail
if the instance is unreachable. If the test fails, the Proxy will exit with
a non-zero exit code.

```shell
./cloud-sql-proxy --run-connection-test <INSTANCE_CONNECTION_NAME>
```

### Config file

The Proxy supports a configuration file. Supported file types are TOML, JSON,
and YAML. Load the file with the `--config-file` flag:

```shell
./cloud-sql-proxy --config-file /path/to/config.[toml|json|yaml]
```

The configuration file format supports all flags. The key names should match
the flag names. For example:

``` toml
# use instance-connection-name-0, instance-connection-name-1, etc.
# for multiple instances
instance-connection-name = "proj:region:inst"
auto-iam-authn = true
debug = true
debug-logs = true
```

Run `./cloud-sql-proxy --help` for more details. See the full documentation
in [docs/cmd](docs/cmd).

### Config environment variables

The proxy supports configuration through environment variables. 
Each environment variable uses "CSQL_PROXY" as a prefix and is 
the uppercase version of the flag using underscores as word delimiters. 

For example, the `--auto-iam-authn` flag may be set with the environment variable 
`CSQL_PROXY_AUTO_IAM_AUTHN`. 

An invocation of the Proxy using environment variables would look like the following: 

```shell
CSQL_PROXY_AUTO_IAM_AUTHN=true \ 
    ./cloud-sql-proxy <INSTANCE_CONNECTION_NAME>
```

Run `./cloud-sql-proxy --help` for more details.

### Configuring a Lazy Refresh

The `--lazy-refresh` flag configures the Proxy to retrieve connection info
lazily and as-needed. Otherwise, no background refresh cycle runs. This setting
is useful in environments where the CPU may be throttled outside of a request
context, e.g., Cloud Run, Cloud Functions, etc.

### Additional flags

To see a full list of flags, use:

```shell
./cloud-sql-proxy --help
```


## Container Images

There are containerized versions of the Proxy available from the following
[Artifact Registry](https://cloud.google.com/artifact-registry) repositories:

- `gcr.io/cloud-sql-connectors/cloud-sql-proxy`
- `us.gcr.io/cloud-sql-connectors/cloud-sql-proxy`
- `eu.gcr.io/cloud-sql-connectors/cloud-sql-proxy`
- `asia.gcr.io/cloud-sql-connectors/cloud-sql-proxy`

> [!NOTE]
>
> The above container images were migrated from Google Container Registry (deprecated)
> to Artifact Registry which is why they begin with the old naming pattern (`gcr.io`)

Each image is tagged with the associated Proxy version. The following tags are
currently supported:

- `$VERSION` (default)
- `$VERSION-alpine`
- `$VERSION-bullseye`
- `$VERSION-bookworm`

<!-- {x-release-please-start-version} -->
The `$VERSION` is the Proxy version without the leading "v" (e.g.,
`2.14.1`).

For example, to pull a particular version, use a command like:

``` shell
# $VERSION is 2.14.1
docker pull gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.14.1
```

<!-- {x-release-please-end} -->
We recommend pinning to a specific version tag and using automation with a CI pipeline
to update regularly.

The default container image uses [distroless][] with a non-root user. If you
need a shell or related tools, use the Alpine or Debian-based container images
(bullseye or bookworm) listed above.

[distroless]: https://github.com/GoogleContainerTools/distroless

### Working with Docker and the Proxy

The containers have the proxy as an `ENTRYPOINT` so, to use the proxy from a
container, all you need to do is specify options using the command, and expose
the proxy's internal port to the host. For example, you can use:

```shell
docker run --publish <host-port>:<proxy-port> \
    gcr.io/cloud-sql-connectors/cloud-sql-proxy:latest \
    --address "0.0.0.0" --port <proxy-port> <instance-connection-name>
```

You'll need the `--address "0.0.0.0"` so that the proxy doesn't only listen for
connections originating from *within* the container.

You will need to authenticate using one of the methods outlined in the
[credentials](#credentials) section. If using a credentials file you must mount
the file and ensure that the non-root user that runs the proxy has *read access*
to the file. These alternatives might help:

1. Change the group of your local file and add read permissions to the group
with `chgrp 65532 key.json && chmod g+r key.json`.
1. If you can't control your file's group, you can directly change the public
permissions of your file by doing `chmod o+r key.json`.

> [!WARNING]
> 
> This can be insecure because it allows any user in the host system to read
> the credential file which they can use to authenticate to services in GCP.

For example, a full command using a JSON credentials file might look like

```shell
docker run \
    --publish <host-port>:<proxy-port> \
    --mount type=bind,source="$(pwd)"/sa.json,target=/config/sa.json \
    gcr.io/cloud-sql-connectors/cloud-sql-proxy:latest \
    --address 0.0.0.0 \
    --port <proxy-port> \
    --credentials-file /config/sa.json <instance-connection-name>
```

## Running as a Kubernetes Sidecar

See the [example here][sidecar-example] as well as [Connecting from Google
Kubernetes Engine][connect-to-k8s].

## Running behind a Socks5 proxy

The Cloud SQL Auth Proxy includes support for sending requests through a SOCKS5
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

The Proxy supports [Cloud Monitoring][], [Cloud Trace][], and [Prometheus][].

Supported metrics include:

- `cloudsqlconn/dial_latency`: The distribution of dialer latencies (ms)
- `cloudsqlconn/open_connections`: The current number of open Cloud SQL
  connections
- `cloudsqlconn/dial_failure_count`: The number of failed dial attempts
- `cloudsqlconn/refresh_success_count`: The number of successful certificate
  refresh operations
- `cloudsqlconn/refresh_failure_count`: The number of failed refresh
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
enabling telemetry, both Cloud Monitoring and Cloud Trace are enabled. To
disable Cloud Monitoring, use `--disable-metrics`. To disable Cloud Trace, use
`--disable-traces`.

To enable Prometheus, use the `--prometheus` flag. This will start an HTTP
server on localhost with a `/metrics` endpoint. The Prometheus namespace may
optionally be set with `--prometheus-namespace`.

## Debug logging

To enable debug logging to report on internal certificate refresh operations,
use the `--debug-logs` flag. Typical use of the Proxy should not require debug
logs, but if you are surprised by the Proxy's behavior, debug logging should
provide insight into internal operations and can help when reporting issues.

## Localhost Admin Server

The Proxy includes support for an admin server on localhost. By default, the
the admin server is not enabled. To enable the server, pass the --debug or
--quitquitquit flag. This will start the server on localhost at port 9091.
To change the port, use the --admin-port flag.

When --debug is set, the admin server enables Go's profiler available at
/debug/pprof/.

See the [documentation on pprof][pprof] for details on how to use the
profiler.

When --quitquitquit is set, the admin server adds an endpoint at
/quitquitquit. The admin server exits gracefully when it receives a GET or POST
request at /quitquitquit.

[pprof]: https://pkg.go.dev/net/http/pprof.

## Frequently Asked Questions

### Why would I use the Proxy?

The Proxy is a convenient way to control access to your database using IAM
permissions while ensuring a secure connection to your Cloud SQL instance. When
using the Proxy, you do not have to manage database client certificates,
configured Authorized Networks, or ensure clients connect securely. The Proxy
handles all of this for you.

### How should I use the Proxy?

The Proxy is a gateway to your Cloud SQL instance. Clients connect to the Proxy
over an unencrypted connection and are authorized using the environment's IAM
principal. The Proxy then encrypts the connection to your Cloud SQL instance.

Because client connections are not encrypted and authorized using the
environment's IAM principal, we recommend running the Proxy on the same VM or
Kubernetes pod as your application and using the Proxy's default behavior of
allowing connections from only the local network interface. This is the most
secure configuration: unencrypted traffic does not leave the VM, and only
connections from applications on the VM are allowed.

Here are some common examples of how to run the Proxy in different environments:

- [Connect to Cloud SQL for MySQL from your local computer][local-quickstart]
- [Connect to Cloud SQL for MySQL from Google Kubernetes Engine][gke-quickstart]

[local-quickstart]: https://cloud.google.com/sql/docs/mysql/connect-instance-local-computer
[gke-quickstart]: https://cloud.google.com/sql/docs/mysql/connect-instance-kubernetes

### Why can't the Proxy connect to my private IP instance?

The Proxy does not configure the network between the VM it's running on and the
Cloud SQL instance. You MUST ensure the Proxy can reach your Cloud SQL
instance, either by deploying it in a VPC that has access to your Private IP
instance, or by configuring Public IP.

### Should I use the Proxy for large deployments?

We recommend deploying the Proxy on the host machines that are running the
application. However, large deployments may exceed the request quota for the SQL
Admin API . If your Proxy reports request quota errors, we recommend deploying
the Proxy with a connection pooler like [pgbouncer][] or [ProxySQL][]. For
details, see [Running the Cloud SQL Proxy as a Service][service-example].

### Can I share the Proxy across multiple applications?

Instead of using a single Proxy across multiple applications, we recommend using
one Proxy instance for every application process. The Proxy uses the context's
IAM principal and so have a 1-to-1 mapping between application and IAM principal
is best. If multiple applications use the same Proxy instance, then it becomes
unclear from an IAM perspective which principal is doing what.

### How do I verify the shasum of a downloaded Proxy binary?

After downloading a binary from the releases page, copy the sha256sum value
that corresponds with the binary you chose.

Then run this command (make sure to add the asterisk before the file name):

``` shell
echo '<RELEASE_PAGE_SHA_HERE> *<NAME_OF_FILE_HERE>' | shasum -c
```

For example, after downloading the v2.1.0 release of the Linux AMD64 Proxy, you
would run:

``` shell
$ echo "547b24faf0dfe5e3d16bbc9f751dfa6b34dfd5e83f618f43a2988283de5208f2 *cloud-sql-proxy" | shasum -c
cloud-sql-proxy: OK
```

If you see `OK`, the binary is a verified match.


[pgbouncer]: https://www.pgbouncer.org/
[proxysql]: https://www.proxysql.com/

## Reference Documentation

- [Cloud SQL][cloud-sql]
- [Cloud SQL Auth Proxy Documentation][proxy-page]
- [Cloud SQL Auth Proxy Quickstarts][quickstarts]
- [Cloud SQL Code Samples][code-samples]
- [Cloud SQL Auth Proxy Package Documentation][pkg-docs]

## Support policy

### Major version lifecycle

This project uses [semantic versioning](https://semver.org/), and uses the
following lifecycle regarding support for a major version:

- **Active** - Active versions get all new features and security fixes (that
wouldn’t otherwise introduce a breaking change). New major versions are
guaranteed to be "active" for a minimum of 1 year.

- **Maintenance** - Maintenance versions continue to receive security and critical
bug fixes, but do not receive new features.

### Release cadence

The Cloud SQL Auth Proxy aims for a minimum monthly release cadence. If no new
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
[code-of-conduct]: CODE_OF_CONDUCT.md
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

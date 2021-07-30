# Cloud SQL Auth proxy

![CI][ci-badge]
[![Go Reference][pkg-badge]][pkg-docs]

The [Cloud SQL Auth proxy][proxy-page] is a binary that provides IAM-based
authorization and encryption when connecting to a Cloud SQL instance.

See the [Connecting Overview][connection-overview] page for more information on
connecting to a Cloud SQL instance, or the [About the proxy][about-proxy] page
for details on how the Cloud SQL proxy works.

Note: The Proxy *cannot* provide a network path to a Cloud SQL instance if one
is not already present (e.g., the proxy cannot access a VPC if it does not
already have access to it).

## Installation

For 64-bit Linux, run:

```
VERSION=v1.21.0 # see Releases for other versions
wget "https://storage.googleapis.com/cloudsql-proxy/$VERSION/cloud_sql_proxy.linux.amd64" -O cloud_sql_proxy
chmod +x cloud_sql_proxy
```

Releases for additional OS's and architectures and be found on the [releases
page][releases].

For alternative distributions, see below under [third party](#third-party).

### Container Images

There are containerized versions of the proxy available from the following
Google Cloud Container Registry repositories:

* `gcr.io/cloudsql-docker/gce-proxy`
* `us.gcr.io/cloudsql-docker/gce-proxy`
* `eu.gcr.io/cloudsql-docker/gce-proxy`
* `asia.gcr.io/cloudsql-docker/gce-proxy`

Each image is tagged with the associated proxy version. The following tags are
currently supported:

* `$VERSION` - default image (recommended)
* `$VERSION-alpine` - uses [`alpine:3`](https://hub.docker.com/_/alpine)
  as a base image (only supported from v1.17 up)
* `$VERSION-buster` - uses [`debian:buster`](https://hub.docker.com/_/debian)
  as a base image (only supported from v1.17 up)

We recommend using the latest version of the proxy and updating the version
regularly. However, we also recommend pinning to a specific tag and avoid the
latest tag. Note: the tagged version is only that of the proxy. Changes in base
images may break specific setups, even on non-major version increments. As such,
it's a best practice to test changes before deployment, and use automated
rollbacks to revert potential failures.

### Install from Source

To install from source, ensure you have the latest version of [Go
installed](https://golang.org/doc/install).

Then, simply run:

```
go get github.com/GoogleCloudPlatform/cloudsql-proxy/cmd/cloud_sql_proxy
```

The `cloud_sql_proxy` will be placed in `$GOPATH/bin` after `go get` completes.

## Usage

All the following invocations assume valid credentials are present in the
environment. The following examples all reference an `INSTANCE_CONNECTION_NAME`,
which takes the form: `myproject:myregion:myinstance`. To find the
`INSTANCE_CONNECTION_NAME`, run `gcloud sql instances describe <INSTANCE_NAME>`
where `INSTANCE_NAME` is the name of the database instance.

### TCP socket example

``` bash
# Starts the proxy listening on 127.0.0.1:5432
cloud_sql_proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:5432
```

``` bash
# Starts the proxy listening on port 5432 on *all* interfaces
cloud_sql_proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:0.0.0.0:5432
```

### Unix socket example

``` bash
# The proxy will mount a Unix domain socket at /cloudsql/<INSTANCE_CONNECTION_NAME>
# Note: The directory specified by `-dir` must exist and the socket file path
# (i.e., dir plus INSTANCE_CONNECTION_NAME) must be under your platform's
# limit (typically 108 characters on many Unix systems, but varies by platform).
cloud_sql_proxy -dir=/cloudsql -instances=<INSTANCE_CONNECTION_NAME>
```

### Private IP example

```
cloud_sql_proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:5432 -ip_address_types=PRIVATE
```

In order to connect using Private IP, you must have access through your
project's VPC. For more details, see [Private IP Requirements][private-ip].

## Credentials

The Cloud SQL proxy uses a Cloud IAM account to authorize connections against a
Cloud SQL instance. The proxy sources the credentials for these accounts in the
following order:

1. The `-credential_file` flag
2. The `-token` flag
3. The service account key at the path stored in the
   `GOOGLE_APPLICATION_CREDENTIALS` environment variable.
4. The gcloud user credentials (set from `gcloud auth login`)
5. The [Application Default Credentials](https://cloud.google.com/docs/authentication/production)

Note: Any account connecting to a Cloud SQL database will need one of the
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


## CLI Flags

The Cloud SQL Auth proxy takes a few arguments to configure what instances to connect
to and connection behavior. For a full list of flags supported by the proxy,
use `cloud_sql_proxy -help`.

### Authentication Flags

#### `-credential_file`

Specifies the path to a JSON [service account][service-account] key the proxy
uses to authorize or authenticate connections.

#### `-token`

When set, the proxy uses this Bearer token for authorization.

#### `-enable_iam_login`

Enables the proxy to use Cloud SQL IAM database authentication. This will cause
the proxy to use IAM account credentials for database user authentication. For
details, see [Overview of Cloud SQL IAM database authentication][iam-auth].
NOTE: This feature only works with Postgres database instances.

### Connection Flags

#### `-instances="project1:region:instance1,project3:region:instance1"`

A comma-separated list of instances to open inside `-dir`. Also supports
exposing a TCP port and renaming the default Unix Domain Sockets; see examples
below.  Same list can be provided via INSTANCES environment variable, in case
when both are provided - proxy will use command line flag.

**Example**

Using TCP sockets:

```
./cloud_sql_proxy -instances=my-project:us-central1:sql-inst=tcp:3306 &
mysql -u root -h 127.0.0.1
```

Using Unix sockets:

```
./cloud_sql_proxy -dir=/cloudsql -instances=my-project:us-central1:sql-inst &
mysql -u root -S /cloudsql/my-project:us-central1:sql-inst
```

To specify a custom Unix socket name:

```
./cloud_sql_proxy -dir=/cloudsql \
    -instances=my-project:us-central1:sql-inst=unix:custom_socket_name &
mysql -u root -S /cloudsql/custom_socket_name
```

To specify a custom location for a Unix socket (overrides `-dir`):

```
./cloud_sql_proxy -dir=/cloudsql \
    -instances=my-project:us-central1:sql-inst=unix:/my/custom/sql-socket &
mysql -u root -S /my/custom/sql-socket
```

#### `-fuse`

Requires access to `/dev/fuse` as well as the `fusermount` binary. An optional
`-fuse_tmp` flag can specify where to place temporary files. The directory
indicated by `-dir` is mounted.

**Example**

Using `-fuse`, you do not need to specify instance names ahead of time:

```
./cloud_sql_proxy -dir=/cloudsql -fuse &
mysql -u root -S /cloudsql/my-project:us-central1:sql-inst
```

#### `-instances_metadata=metadata_key`

Usable on [GCE](https://cloud.google.com/compute/docs/quickstart) only. The
given [GCE metadata](https://cloud.google.com/compute/docs/metadata) key will be
polled for a list of instances to open in `-dir`. The metadata key is relative
from `computeMetadata/v1/`. The format for the value is the same as the
'instances' flag. A hanging-poll strategy is used, meaning that changes to the
metadata value will be reflected in the `-dir` even while the proxy is running.
When an instance is removed from the list the corresponding socket will be
removed from `-dir` as well (unless it was also specified in `-instances`), but
any existing connections to this instance will NOT be terminated.

**Example**

```
./cloud_sql_proxy -dir=/cloudsql \
    -instances_metadata instance/attributes/<custom-metadata-key> &
mysql -u root -S /cloudsql/my-project:us-central1:sql-inst
```

Note: `-instances` and `-instances_metadata` may be used at the same time but
are not compatible with the `-fuse` flag.

#### `-max_connections`

If provided, the maximum number of connections to establish before refusing new
connections. Defaults to 0 (no limit).

### Additional Flags

#### `-ip_address_types=PUBLIC,PRIVATE`

A comma-delimited list of preferred IP types for connecting to an instance. For
example, setting this to PRIVATE will force the proxy to connect to instances
using an instance's associated private IP. Defaults to `PUBLIC,PRIVATE`

#### `-term_timeout=30s`

How long to wait for connections to close before shutting down the proxy.
Defaults to 0.

#### `-skip_failed_instance_config`

Setting this flag will prevent the proxy from terminating if any errors occur
during instance configuration. Please note that this means some instances may
fail to be set up correctly while others may work if the proxy restarts.

#### `-log_debug_stdout=true`

This is to log non-error output to standard out instead of standard error. For
example, if you don't want connection related messages to log as errors, set
this flag to true.  Defaults to false.

#### `-structured_logs`

Writes all logging output as JSON with the following keys: level, ts, caller,
msg. For example, the startup message looks like:

```
{"level":"info","ts":1616014011.8132386,"caller":"cloud_sql_proxy/cloud_sql_proxy.go:510","msg":"Using
gcloud's active project: [my-project-id]"}

```

#### `-use_http_health_check`

Enables HTTP health checks for the proxy, including startup, liveness, and readiness probing.
Requires that you configure the Kubernetes container with HTTP probes ([sample](https://github.com/GoogleCloudPlatform/cloudsql-proxy/tree/main/examples/k8s-health-check/proxy_with_http_health_check.yaml)).

#### `-health_check_port=8090`

Specifies the port that the health check server listens and serves on. Defaults to 8090.

## Running as a Kubernetes Sidecar

See the [example here][sidecar-example] as well as [Connecting from Google
Kubernetes Engine][connect-to-k8s].

## Reference Documentation

- [Cloud SQL][cloud-sql]
- [Cloud SQL Auth proxy Documentation][proxy-page]
- [Cloud SQL Auth proxy Quickstarts][quickstarts]
- [Cloud SQL Code Samples][code-samples]
- [Cloud SQL Auth proxy Package Documentation][pkg-docs]

## Contributing

Contributions are welcome. Please, see the [CONTRIBUTING][contributing] document
for details.

Please note that this project is released with a Contributor Code of Conduct.
By participating in this project you agree to abide by its terms.  See
[Contributor Code of Conduct][code-of-conduct] for more information.

## Third Party

__WARNING__: _These distributions are not officially supported by Google._

### Homebrew

There is Homebrew formula for Cloud SQL Auth proxy [here](https://github.com/tclass/homebrew-cloud_sql_proxy).

### Kubernetes Cluster Service using Helm

Follow these [instructions](https://github.com/rimusz/charts/tree/master/stable/gcloud-sqlproxy).

This chart creates a Deployment and a Service, but we recommend deploying the
proxy as a sidecar container in your pods.

### .Net Proxy Wrapper (Nuget Package)

Install via Nuget, follow these
[instructions](https://github.com/expert1-pty-ltd/cloudsql-proxy#install-via-nuget).


[about-proxy]: https://cloud.google.com/sql/docs/mysql/sql-proxy
[ci-badge]: https://storage.googleapis.com/cloud-devrel-public/cloud-sql-connectors/proxy/go1.16_linux.svg
[cloud-sql]: https://cloud.google.com/sql
[code-samples]: https://cloud.google.com/sql/docs/mysql/samples
[code-of-conduct]: CONTRIBUTING.md#contributor-code-of-conduct
[connect-to-k8s]: https://cloud.google.com/sql/docs/mysql/connect-kubernetes-engine
[connection-overview]: https://cloud.google.com/sql/docs/mysql/connect-overview
[contributing]: CONTRIBUTING.md
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
[source-install]: docs/install-from-source.md

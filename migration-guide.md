# Migrating from v1 to v2

The Cloud SQL Auth Proxy v2 CLI interface maintains a close match to the v1
interface. Migrating to v2 will require minimal changes. Below are a number
of examples of v1 vs v2 invocations covering the most common uses. See
[Flag Changes](#flag-changes) for details.

All the examples below use `<INSTANCE_CONNECTION_NAME>` as a placeholder for
your instance connection name, e.g., `my-cool-project:us-central1:my-db`.

## Container Image Name Change

As part of releasing a v2, we have updated the image name to be more descriptive.
Compare:

```
# v1
gcr.io/cloudsql-docker/gce-proxy
```

vs

```
# v2
gcr.io/cloud-sql-connectors/cloud-sql-proxy
```

To update to the v2 container, make sure to update the image name.

## Behavior Differences

In v1, when a client connected, the Proxy would first try to use a public IP
and then attempt to use a private IP. In v2, the Proxy now defaults to public
IP without trying private IP. If you want to use private IP, you must pass
either the `--private-ip` flag or the query parameter. See the README for details.

In some cases, the v1 behavior may be preferable. Use the `--auto-ip` flag to
mimic v1 behavior. We generally recommend using deterministic IP address selection,
but recognize in some legacy environments `--auto-ip` may be necessary.

## Executable Name Change

Note that the name of the executable has changed, using hyphens rather than underscores:

```shell
# v1
./cloud_sql_proxy
```

vs

```shell
# v2
./cloud-sql-proxy
```

## Sample Invocations

### Listen on TCP socket

```shell
# v1
./cloud_sql_proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:5432

# v2
# Using automatic database port selection (MySQL 3306, Postgres 5432, SQL Server 1433)
./cloud-sql-proxy <INSTANCE_CONNECTION_NAME>
```

### Listen on Unix Socket

```shell
# v1
./cloud_sql_proxy -dir /cloudsql -instances=<INSTANCE_CONNECTION_NAME>

# v2
./cloud-sql-proxy --unix-socket /cloudsql <INSTANCE_CONNECTION_NAME>
```

### Listen on multiple TCP sockets with incrementing ports

```shell
# v1
./cloud_sql_proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:5000,<INSTANCE_CONNECTION_NAME2>=tcp:5001

# v2
# starts listener on port 5000, increments for additional listeners
./cloud-sql-proxy --port 5000 <INSTANCE_CONNECTION_NAME> <INSTANCE_CONNECTION_NAME2>
```

### Listen on multiple TCP sockets with non-sequential ports

```shell
# v1
./cloud_sql_proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:6000,<INSTANCE_CONNECTION_NAME2>=tcp:7000

# v2
./cloud-sql-proxy '<INSTANCE_CONNECTION_NAME>?port=6000' '<INSTANCE_CONNECTION_NAME2>?port=7000'
```

### Listen on all interfaces

```shell
# v1
./cloud_sql_proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:0.0.0.0:6000

# v2
./cloud-sql-proxy --address 0.0.0.0 --port 6000 <INSTANCE_CONNECTION_NAME>
```

## Environment variable changes

In v1 it was possible to do this:

``` shell
export INSTANCES="<INSTANCE_CONNECTION_NAME_1>=tcp:3306,<INSTANCE_CONNECTION_NAME_2>=tcp:5432"

./cloud_sql_proxy
```

In v2, we've significantly expanded the support for environment variables.
All flags can be set with an environment variable including instance connection names.

For example, in v2 this is possible:

``` shell
export CSQL_PROXY_INSTANCE_CONNECTION_NAME_0="<INSTANCE_CONNECTION_NAME_1>?port=3306"
export CSQL_PROXY_INSTANCE_CONNECTION_NAME_1="<INSTANCE_CONNECTION_NAME_2>?port=5432"

export CSQL_PROXY_AUTO_IAM_AUTHN=true

./cloud-sql-proxy
```

See the [help message][] for more details.

[help message]: https://github.com/GoogleCloudPlatform/cloud-sql-proxy/blob/10bec27e4d44c14fe9e68f25fef6c373324e8bab/cmd/root.go#L240-L264

## Flag Changes

The following table lists in alphabetical order v1 flags and their v2 version.

- 🗓️: Planned
- ❌: Not supported in V2
- 🤔: Unplanned, but has open feature request

| v1                          | v2                          | Notes                                                                                |
| --------------------------- | --------------------------- | ------------------------------------------------------------------------------------ |
| check_region                | ❌                          |                                                                                      |
| credential_file             | credentials-file            |                                                                                      |
| dir                         | unix-socket                 |                                                                                      |
| enable_iam_login            | auto-iam-authn              |                                                                                      |
| fd_rlimit                   | 🤔                          | [Feature Request](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1258) |
| fuse                        | fuse                        |                                                                                      |
| fuse_tmp                    | fuse-temp-dir               |                                                                                      |
| health_check_port           | http-port                   |  Use --http-address=0.0.0.0 when using a health check in Kubernetes                  |
| host                        | sqladmin-api-endpoint       |                                                                                      |
| instances_metadata          | 🤔                          | [Feature Request](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1259) |
| ip_address_types            | private-ip                  | Defaults to public. To connect to a private IP, you must add the --private-ip flag   |
| log_debug_stdout            | ❌                          | v2 logs to stdout, errors to stderr by default                                       |
| max_connections             | max-connections             |                                                                                      |
| projects                    | ❌                          | v2 prefers explicit connection configuration to avoid user error                     |
| quiet                       | quiet                       | quiet disables all logging except errors                                             |
| quota_project               | quota-project               |                                                                                      |
| refresh_config_throttle     | ❌                          |                                                                                      |
| skip_failed_instance_config | skip-failed-instance-config |                                                                                      |
| structured_logs             | structured-logs             |                                                                                      |
| term_timeout                | max-sigterm-delay           |                                                                                      |
| token                       | token                       |                                                                                      |
| use_http_health_check       | health-check                |                                                                                      |
| verbose                     | ❌                          |                                                                                      |
| version                     | version                     |                                                                                      |

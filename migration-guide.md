# Migrating from v1 to v2

The Cloud SQL Auth proxy v2 CLI interface maintains a close match to the v1
interface. Migrating to v2 will require minimal changes. Below are a number
of examples of v1 vs v2 invocations covering the most common uses. See
[Flag Changes](#flag-changes) for details.

All the examples below use `<INSTANCE_CONNECTION_NAME>` as a placeholder for
your instance connection name, e.g., `my-cool-project:us-central1:my-db`.

## Sample Invocations

### Listen on TCP socket

```shell
# v1
./cloud-sql-proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:5432

# v2
# Using automatic database port selection (MySQL 3306, Postgres 5432, SQL Server 1433)
./cloud-sql-proxy <INSTANCE_CONNECTION_NAME>
```

### Listen on Unix Socket

```shell
# v1
./cloud-sql-proxy -dir /cloudsql -instances=<INSTANCE_CONNECTION_NAME>

# v2
./cloud-sql-proxy --unix-socket /cloudsql <INSTANCE_CONNECTION_NAME>
```

### Listen on multiple TCP sockets with incrementing ports

```shell
# v1
./cloud-sql-proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:5000,<INSTANCE_CONNECTION_NAME2>=tcp:5001

# v2
# starts listener on port 5000, increments for additional listeners
./cloud-sql-proxy --port 5000 <INSTANCE_CONNECTION_NAME> <INSTANCE_CONNECTION_NAME2>
```

### Listen on multiple TCP sockets with non-sequential ports

```shell
# v1
./cloud-sql-proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:6000,<INSTANCE_CONNECTION_NAME2>=tcp:7000

# v2
./cloud-sql-proxy '<INSTANCE_CONNECTION_NAME>?port=6000' '<INSTANCE_CONNECTION_NAME2>?port=7000'
```

### Listen on all interfaces

```shell
# v1
./cloud-sql-proxy -instances=<INSTANCE_CONNECTION_NAME>=tcp:0.0.0.0:6000

# v2
./cloud-sql-proxy --address 0.0.0.0 --port 6000 <INSTANCE_CONNECTION_NAME>
```

## Flag Changes

The following table lists in alphabetical order v1 flags and their v2 version.

- 🗓️: Planned
- ❌: Not supported in V2
- 🤔: Unplanned, but has open feature request

| v1                          | v2                    | Notes                                                                                |
| --------------------------- | --------------------- | ------------------------------------------------------------------------------------ |
| check_region                | ❌                    |                                                                                      |
| credential_file             | credentials-file      |                                                                                      |
| dir                         | unix-socket           |                                                                                      |
| enable_iam_login            | auto-iam-authn        |                                                                                      |
| fd_rlimit                   | 🤔                    | [Feature Request](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1258) |
| fuse                        | fuse                  |                                                                                      |
| fuse_tmp                    | fuse-temp-dir         |                                                                                      |
| health_check_port           | http-port             |  Use --http-address=0.0.0.0 when using a health check in Kubernetes                  |
| host                        | sqladmin-api-endpoint |                                                                                      |
| instances_metadata          | 🤔                    | [Feature Request](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1259) |
| ip_address_types            | private-ip            | Defaults to public                                                                   |
| log_debug_stdout            | ❌                    | v2 logs to stdout, errors to stderr by default                                       |
| max_connections             | max-connections       |                                                                                      |
| projects                    | ❌                    | v2 prefers explicit connection configuration to avoid user error                     |
| quiet                       | quiet                 | quiet disables all logging except errors                                             |
| quota_project               | quota-project         |                                                                                      |
| refresh_config_throttle     | ❌                    |                                                                                      |
| skip_failed_instance_config | ❌                    | This flag was only necessary with Unix sockets. Use TCP sockets to avoid failed startup. |
| structured_logs             | structured-logs       |                                                                                      |
| term_timeout                | max-sigterm-delay     |                                                                                      |
| token                       | token                 |                                                                                      |
| use_http_health_check       | health-check          |                                                                                      |
| verbose                     | ❌                    |                                                                                      |
| version                     | version               |                                                                                      |

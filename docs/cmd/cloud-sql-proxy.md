## cloud-sql-proxy

cloud-sql-proxy authorizes and encrypts connections to Cloud SQL.

### Synopsis


Overview

  The Cloud SQL Auth Proxy is a utility for ensuring secure connections to
  your Cloud SQL instances. It provides IAM authorization, allowing you to
  control who can connect to your instance through IAM permissions, and TLS
  1.3 encryption, without having to manage certificates.

  NOTE: The Proxy does not configure the network. You MUST ensure the Proxy
  can reach your Cloud SQL instance, either by deploying it in a VPC that has
  access to your Private IP instance, or by configuring Public IP.

  For every provided instance connection name, the Proxy creates:

  - a socket that mimics a database running locally, and
  - an encrypted connection using TLS 1.3 back to your Cloud SQL instance.

  The Proxy uses an ephemeral certificate to establish a secure connection to
  your Cloud SQL instance. The Proxy will refresh those certificates on an
  hourly basis. Existing client connections are unaffected by the refresh
  cycle.

Starting the Proxy

  To start the Proxy, you will need your instance connection name, which may
  be found in the Cloud SQL instance overview page or by using gcloud with the
  following command:

      gcloud sql instances describe INSTANCE --format='value(connectionName)'

  For example, if your instance connection name is
  "my-project:us-central1:my-db-server", starting the Proxy will be:

      ./cloud-sql-proxy my-project:us-central1:my-db-server

  By default, the Proxy will determine the database engine and start a
  listener on localhost using the default database engine's port, i.e., MySQL
  is 3306, Postgres is 5432, SQL Server is 1433. If multiple instances are
  specified which all use the same database engine, the first will be started
  on the default database port and subsequent instances will be incremented
  from there (e.g., 3306, 3307, 3308, etc). To disable this behavior (and
  reduce startup time), use the --port flag. All subsequent listeners will
  increment from the provided value.

  All socket listeners use the localhost network interface. To override this
  behavior, use the --address flag.

Instance Level Configuration

  The Proxy supports overriding configuration on an instance-level with an
  optional query string syntax using the corresponding full flag name. The
  query string takes the form of a URL query string and should be appended to
  the INSTANCE_CONNECTION_NAME, e.g.,

      'my-project:us-central1:my-db-server?key1=value1&key2=value2'

  When using the optional query string syntax, quotes must wrap the instance
  connection name and query string to prevent conflicts with the shell. For
  example, to override the address and port for one instance but otherwise use
  the default behavior, use:

      ./cloud-sql-proxy \
  	    my-project:us-central1:my-db-server \
  	    'my-project:us-central1:my-other-server?address=0.0.0.0&port=7000'

  When necessary, you may specify the full path to a Unix socket. Set the
  unix-socket-path query parameter to the absolute path of the Unix socket for
  the database instance. The parent directory of the unix-socket-path must
  exist when the Proxy starts or else socket creation will fail. For Postgres
  instances, the Proxy will ensure that the last path element is
  '.s.PGSQL.5432' appending it if necessary. For example,

      ./cloud-sql-proxy \
        'my-project:us-central1:my-db-server?unix-socket-path=/path/to/socket'

Health checks

  When enabling the --health-check flag, the Proxy will start an HTTP server
  on localhost with three endpoints:

  - /startup: Returns 200 status when the Proxy has finished starting up.
  Otherwise returns 503 status.

  - /readiness: Returns 200 status when the Proxy has started, has available
  connections if max connections have been set with the --max-connections
  flag, and when the Proxy can connect to all registered instances. Otherwise,
  returns a 503 status. Optionally supports a min-ready query param (e.g.,
  /readiness?min-ready=3) where the Proxy will return a 200 status if the
  Proxy can connect successfully to at least min-ready number of instances. If
  min-ready exceeds the number of registered instances, returns a 400.

  - /liveness: Always returns 200 status. If this endpoint is not responding,
  the Proxy is in a bad state and should be restarted.

  To configure the address, use --http-address. To configure the port, use
  --http-port.

Service Account Impersonation

  The Proxy supports service account impersonation with the
  --impersonate-service-account flag and matches gclouds flag. When enabled,
  all API requests are made impersonating the supplied service account. The
  IAM principal must have the iam.serviceAccounts.getAccessToken permission or
  the role roles/iam.serviceAccounts.serviceAccountTokenCreator.

  For example:

      ./cloud-sql-proxy \
          --impersonate-service-account=impersonated@my-project.iam.gserviceaccount.com
          my-project:us-central1:my-db-server

  In addition, the flag supports an impersonation delegation chain where the
  value is a comma-separated list of service accounts. The first service
  account in the list is the impersonation target. Each subsequent service
  account is a delegate to the previous service account. When delegation is
  used, each delegate must have the permissions named above on the service
  account it is delegating to.

  For example:

      ./cloud-sql-proxy \
          --impersonate-service-account=SERVICE_ACCOUNT_1,SERVICE_ACCOUNT_2,SERVICE_ACCOUNT_3
          my-project:us-central1:my-db-server

  In this example, the environment's IAM principal impersonates
  SERVICE_ACCOUNT_3 which impersonates SERVICE_ACCOUNT_2 which then
  impersonates the target SERVICE_ACCOUNT_1.

Configuration using environment variables

  Instead of using CLI flags, the Proxy may be configured using environment
  variables. Each environment variable uses "CSQL_PROXY" as a prefix and is
  the uppercase version of the flag using underscores as word delimiters. For
  example, the --auto-iam-authn flag may be set with the environment variable
  CSQL_PROXY_AUTO_IAM_AUTHN. An invocation of the Proxy using environment
  variables would look like the following:

      CSQL_PROXY_AUTO_IAM_AUTHN=true \
          ./cloud-sql-proxy my-project:us-central1:my-db-server

  In addition to CLI flags, instance connection names may also be specified
  with environment variables. If invoking the Proxy with only one instance
  connection name, use CSQL_PROXY_INSTANCE_CONNECTION_NAME. For example:

      CSQL_PROXY_INSTANCE_CONNECTION_NAME=my-project:us-central1:my-db-server \
          ./cloud-sql-proxy

  If multiple instance connection names are used, add the index of the
  instance connection name as a suffix. For example:

      CSQL_PROXY_INSTANCE_CONNECTION_NAME_0=my-project:us-central1:my-db-server \
      CSQL_PROXY_INSTANCE_CONNECTION_NAME_1=my-other-project:us-central1:my-other-server \
          ./cloud-sql-proxy

Configuration using a configuration file

  Instead of using CLI flags, the Proxy may be configured using a configuration
  file. The configuration file is a TOML, YAML or JSON file with the same keys
  as the environment variables. The configuration file is specified with the
  --config-file flag. An invocation of the Proxy using a configuration file
  would look like the following:

      ./cloud-sql-proxy --config-file=config.toml

  The configuration file may look like the following:

      instance-connection-name = "my-project:us-central1:my-server-instance"
      auto-iam-authn = true

  If multiple instance connection names are used, add the index of the
  instance connection name as a suffix. For example:

      instance-connection-name-0 = "my-project:us-central1:my-db-server"
      instance-connection-name-1 = "my-other-project:us-central1:my-other-server"

  The configuration file may also contain the same keys as the environment
  variables and flags. For example:

      auto-iam-authn = true
      debug = true
      max-connections = 5

Localhost Admin Server

  The Proxy includes support for an admin server on localhost. By default,
  the admin server is not enabled. To enable the server, pass the --debug or
  --quitquitquit flag. This will start the server on localhost at port 9091.
  To change the port, use the --admin-port flag.

  When --debug is set, the admin server enables Go's profiler available at
  /debug/pprof/.

  See the documentation on pprof for details on how to use the
  profiler at https://pkg.go.dev/net/http/pprof.

  When --quitquitquit is set, the admin server adds an endpoint at
  /quitquitquit. The admin server exits gracefully when it receives a GET or POST
  request at /quitquitquit.

Debug logging

  On occasion, it can help to enable debug logging which will report on
  internal certificate refresh operations. To enable debug logging, use:

      ./cloud-sql-proxy <INSTANCE_CONNECTION_NAME> --debug-logs

Waiting for Startup

  See the wait subcommand's help for details.

(*) indicates a flag that may be used as a query parameter

Third Party Licenses

  To view all licenses for third party dependencies used within this
  distribution please see:

  https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1/third_party/licenses.tar.gz 


```
cloud-sql-proxy INSTANCE_CONNECTION_NAME... [flags]
```

### Options

```
  -a, --address string                       (*) Address to bind Cloud SQL instance listeners. (default "127.0.0.1")
      --admin-port string                    Port for localhost-only admin server (default "9091")
  -i, --auto-iam-authn                       (*) Enables Automatic IAM Authentication for all instances
      --auto-ip                              Supports legacy behavior of v1 and will try to connect to first IP
                                             address returned by the SQL Admin API. In most cases, this flag should not be used.
                                             Prefer default of public IP or use --private-ip instead.
      --config-file string                   Path to a TOML file containing configuration options.
  -c, --credentials-file string              Use service account key file as a source of IAM credentials.
      --debug                                Enable pprof on the localhost admin server
      --debug-logs                           Enable debug logging
      --disable-metrics                      Disable Cloud Monitoring integration (used with --telemetry-project)
      --disable-traces                       Disable Cloud Trace integration (used with --telemetry-project)
      --exit-zero-on-sigterm                 Exit with 0 exit code when Sigterm received (default is 143)
      --fuse string                          Mount a directory at the path using FUSE to access Cloud SQL instances.
      --fuse-tmp-dir string                  Temp dir for Unix sockets created with FUSE (default "/tmp/csql-tmp")
  -g, --gcloud-auth                          Use gclouds user credentials as a source of IAM credentials.
                                             NOTE: this flag is a legacy feature and generally should not be used.
                                             Instead prefer Application Default Credentials
                                             (enabled with: gcloud auth application-default login) which
                                             the Proxy will then pick-up automatically.
      --health-check                         Enables health check endpoints /startup, /liveness, and /readiness on localhost.
  -h, --help                                 Display help information for cloud-sql-proxy
      --http-address string                  Address for Prometheus and health check server (default "localhost")
      --http-port string                     Port for Prometheus and health check server (default "9090")
      --impersonate-service-account string   Comma separated list of service accounts to impersonate. Last value
                                             is the target account.
  -j, --json-credentials string              Use service account key JSON as a source of IAM credentials.
      --lazy-refresh                         Configure a lazy refresh where connection info is retrieved only if
                                             the cached copy has expired. Use this setting in environments where the
                                             CPU may be throttled and a background refresh cannot run reliably
                                             (e.g., Cloud Run)
      --login-token string                   Use bearer token as a database password (used with token and auto-iam-authn only)
      --max-connections uint                 Limit the number of connections. Default is no limit.
      --max-sigterm-delay duration           Maximum number of seconds to wait for connections to close after receiving a TERM signal.
      --min-sigterm-delay duration           The number of seconds to accept new connections after receiving a TERM signal.
  -p, --port int                             (*) Initial port for listeners. Subsequent listeners increment from this value.
      --private-ip                           (*) Connect to the private ip address for all instances
      --prometheus                           Enable Prometheus HTTP endpoint /metrics on localhost
      --prometheus-namespace string          Use the provided Prometheus namespace for metrics
      --psc                                  (*) Connect to the PSC endpoint for all instances
      --quiet                                Log error messages only
      --quitquitquit                         Enable quitquitquit endpoint on the localhost admin server
      --quota-project string                 Specifies the project to use for Cloud SQL Admin API quota tracking.
                                             The IAM principal must have the "serviceusage.services.use" permission
                                             for the given project. See https://cloud.google.com/service-usage/docs/overview and
                                             https://cloud.google.com/storage/docs/requester-pays
      --run-connection-test                  Runs a connection test
                                             against all specified instances. If an instance is unreachable, the Proxy exits with a failure
                                             status code.
      --sqladmin-api-endpoint string         API endpoint for all Cloud SQL Admin API requests. (default: https://sqladmin.googleapis.com)
  -l, --structured-logs                      Enable structured logging with LogEntry format
      --telemetry-prefix string              Prefix for Cloud Monitoring metrics.
      --telemetry-project string             Enable Cloud Monitoring and Cloud Trace with the provided project ID.
      --telemetry-sample-rate int            Set the Cloud Trace sample rate. A smaller number means more traces. (default 10000)
  -t, --token string                         Use bearer token as a source of IAM credentials.
      --universe-domain string               Universe Domain for TPC environments. (default: googleapis.com)
  -u, --unix-socket string                   (*) Enables Unix sockets for all listeners with the provided directory.
      --user-agent string                    Space separated list of additional user agents, e.g. cloud-sql-proxy-operator/0.0.1
  -v, --version                              Print the cloud-sql-proxy version
```

### SEE ALSO

* [cloud-sql-proxy completion](cloud-sql-proxy_completion.md)	 - Generate the autocompletion script for the specified shell
* [cloud-sql-proxy wait](cloud-sql-proxy_wait.md)	 - Wait for another Proxy process to start


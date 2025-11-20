## cloud-sql-proxy shutdown

Signal a running Proxy process to shut down

### Synopsis


Shutting Down the Proxy

  The shutdown command signals a running Proxy process to gracefully shut
  down. This is useful for scripting and for Kubernetes environments.

  The shutdown command requires that the Proxy be started in another process
  with the admin server enabled. For example:

  ./cloud-sql-proxy <INSTANCE_CONNECTION_NAME> --quitquitquit

  Invoke the shutdown command like this:

  # signals another Proxy process to shut down
  ./cloud-sql-proxy shutdown

Configuration

  If the running Proxy is configured with a non-default admin port, the
  shutdown command must also be told to use the same custom value:

  ./cloud-sql-proxy shutdown --admin-port 9192


```
cloud-sql-proxy shutdown [flags]
```

### Options

```
      --admin-port string   port for the admin server (default "9091")
  -h, --help                help for shutdown
```

### Options inherited from parent commands

```
      --http-address string   Address for Prometheus and health check server (default "localhost")
      --http-port string      Port for Prometheus and health check server (default "9090")
```

### SEE ALSO

* [cloud-sql-proxy](cloud-sql-proxy.md)	 - cloud-sql-proxy authorizes and encrypts connections to Cloud SQL.


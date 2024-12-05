## cloud-sql-proxy wait

Wait for another Proxy process to start

### Synopsis


Waiting for Proxy Startup

  Sometimes it is necessary to wait for the Proxy to start.

  To help ensure the Proxy is up and ready, the Proxy includes a wait
  subcommand with an optional --max flag to set the maximum time to wait.
  The wait command uses a separate Proxy's startup endpoint to determine
  if the other Proxy process is ready.

  Invoke the wait command, like this:

  # waits for another Proxy process' startup endpoint to respond
  ./cloud-sql-proxy wait

Configuration

  By default, the Proxy will wait up to the maximum time for the startup
  endpoint to respond. The wait command requires that the Proxy be started in
  another process with the HTTP health check enabled. If an alternate health
  check port or address is used, as in:

  ./cloud-sql-proxy <INSTANCE_CONNECTION_NAME> \
    --http-address 0.0.0.0 \
    --http-port 9191

  Then the wait command must also be told to use the same custom values:

  ./cloud-sql-proxy wait \
    --http-address 0.0.0.0 \
    --http-port 9191

  By default the wait command will wait 30 seconds. To alter this value,
  use:

  ./cloud-sql-proxy wait --max 10s


```
cloud-sql-proxy wait [flags]
```

### Options

```
  -h, --help           help for wait
  -m, --max duration   maximum amount of time to wait for startup (default 30s)
```

### Options inherited from parent commands

```
      --http-address string   Address for Prometheus and health check server (default "localhost")
      --http-port string      Port for Prometheus and health check server (default "9090")
```

### SEE ALSO

* [cloud-sql-proxy](cloud-sql-proxy.md)	 - cloud-sql-proxy authorizes and encrypts connections to Cloud SQL.


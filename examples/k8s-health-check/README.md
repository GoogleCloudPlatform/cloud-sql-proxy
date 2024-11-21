# Cloud SQL Auth Proxy health checks

Kubernetes supports [three types of health checks][k8s-docs]:

1. Startup probes determine whether a container is done starting up. As soon as
   this probe succeeds, Kubernetes switches over to using liveness and readiness
   probing.
2. Liveness probes determine whether a container is healthy. When this probe is
   unsuccessful, the container is restarted.
3. Readiness probes determine whether a container can serve new traffic. When
   this probe fails, Kubernetes will wait to send requests to the container.

[k8s-docs]: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/

When enabling the `--health-check` flag, the proxy will start an HTTP server on
localhost with three endpoints:

- `/startup`: Returns 200 status when the proxy has finished starting up.
Otherwise returns 503 status.

- `/liveness`: Always returns 200 status. If this endpoint is not responding,
the proxy is in a bad state and should be restarted.

- `/readiness`: Returns 200 status when the proxy has started, has available
  connections if max connections have been set with the `--max-connections`
  flag, and when the proxy can connect to all registered instances. Otherwise,
  returns a 503 status. Optionally supports a min-ready query param (e.g.,
  `/readiness?min-ready=3`) where the proxy will return a 200 status if the
  proxy can connect successfully to at least min-ready number of instances. If
  min-ready exceeds the number of registered instances, returns a 400.


To configure the address, use `--http-address`. To configure the port, use
`--http-port`.

## Running Cloud SQL Auth Proxy with health checks in Kubernetes
1. Configure your Cloud SQL Auth Proxy container to include health check probes.
    > [proxy_with_http_health_check.yaml](proxy_with_http_health_check.yaml#L77-L111)
```yaml
# Recommended configurations for health check probes.
# Probe parameters can be adjusted to best fit the requirements of your application.
# For details, see https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
startupProbe:
   # We recommend adding a startup probe to the proxy sidecar
   # container. This will ensure that service traffic will be routed to
   # the pod only after the proxy has successfully started.
   httpGet:
      path: /startup
      port: 9090
   periodSeconds: 1
   timeoutSeconds: 5
   failureThreshold: 20
livenessProbe:
   # We recommend adding a liveness probe to the proxy sidecar container.
   httpGet:
      path: /liveness
      port: 9801
   # Number of seconds after the container has started before the first probe is scheduled. Defaults to 0.
   # Not necessary when the startup probe is in use.
   initialDelaySeconds: 0
   # Frequency of the probe.
   periodSeconds: 60
   # Number of seconds after which the probe times out.
   timeoutSeconds: 30
   # Number of times the probe is allowed to fail before the transition
   # from healthy to failure state.
   #
   # If periodSeconds = 60, 5 tries will result in five minutes of
   # checks. The proxy starts to refresh a certificate five minutes
   # before its expiration. If those five minutes lapse without a
   # successful refresh, the liveness probe will fail and the pod will be
   # restarted.
   failureThreshold: 5
# We do not recommend adding a readiness probe under most circumstances
```

2. Enable the health checks by setting `--http-address` and `--http-port` (optional) to your
   proxy container configuration under `command: `.
    > [proxy_with_http_health_check.yaml](proxy_with_http_health_check.yaml#L53-L76)

```yaml
args:
# Replace <INSTANCE_CONNECTION_NAME> with the instance connection
# name in the format: "project_name:region:instance_name"
- <INSTANCE_CONNECTION_NAME>

env:
# It can be easier to manage the k8s configuration file when you
# use environment variables instead of CLI flags. This is the
# recommended configuration. This configuration is enabled by default
# when the cloud-sql-proxy-operator configures a proxy image

# Replace <DB_PORT> with the port that the proxy should open
# to listen for database connections from the application
- name: CSQL_PROXY_PORT
  value: <DB_PORT>

# Enable HTTP healthchecks on port 9801. This enables /liveness,
# /readiness and /startup health check endpoints. Allow connections
# listen for connections on any interface (0.0.0.0) so that the
# k8s management components can reach these endpoints.
- name: CSQL_PROXY_HEALTH_CHECK
  value: "true"
- name: CSQL_PROXY_HTTP_PORT
  value: "9801"
- name: CSQL_PROXY_HTTP_ADDRESS
  value: 0.0.0.0

# Configure the proxy to exit gracefully when sent a k8s configuration
# file.
- name: CSQL_PROXY_EXIT_ZERO_ON_SIGTERM
  value: "true"

```

### Readiness Health Check Configuration

For most common usage, adding a readiness healthcheck to the proxy sidecar 
container is unnecessary. An improperly configured readiness check can degrade 
the application's availability.

The proxy readiness probe fails when (1) the proxy used all its available
concurrent connections to a database, (2) the network connection
to the database is interrupted, (3) the database server is unavailable due
to a maintenance operation. These are transient states that usually resolve
within a few seconds.

Most applications are resilient to transient database connection failures, and
do not need to be restarted. We recommend adding a readiness check to the
application container instead of the proxy container. The application can be
programmed to report whether it is ready to receive requests, and the healthcheck
can be tuned to restart the pod when the application is permanently stuck. 

You should use the proxy container's readiness probe when these circumstances
should cause k8s to terminate the entire pod:

- The proxy can't connect to the database instances.
- The max number of connections are in use.

When you do use the proxy pod's readiness probe, be sure to set the 
`failureThreshold` and `periodSeconds` to avoid restarting the pod on frequent
transient failures.

### Readiness Health Check Examples

The DBA team performs database fail-overs drills without notice. A
batch job should fail if it cannot connect the database for 3 minutes. 
Set the readiness check so that the pod will be terminated after 3 minutes
of consecutive readiness check failures. (6 failed readiness checks taken every 30
seconds, 6 x 30sec = 3 minutes.)

```yaml
readinessProbe:
  httpGet:
    path: /readiness
    port: 9801
  initialDelaySeconds: 30
  # 30 sec period x 6 failures = 3 min until the pod is terminated
  periodSeconds: 30
  failureThreshold: 6
  timeoutSeconds: 10
  successThreshold: 1
```

A web application has a database connection pool leak and the 
engineering team can't find the root cause. To keep the system running, 
the application should be automatically restarted if it consumes 50 connections 
for more than 1 minute.

<!-- {x-release-please-start-version} -->
```yaml
    containers:
    - name: my-application
      image: gcr.io/my-container/my-application:1.1
    - name: cloud-sql-proxy
      image: gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.14.1
      args:
        # Set the --max-connections flag to 50
        - "--max-connections"
        - "50"
        - "--port=<DB_PORT>"
        - "<INSTANCE_CONNECTION_NAME>"
# ...
    readinessProbe:
        httpGet:
            path: /readiness
            port: 9801
        initialDelaySeconds: 10
        # 5 sec period x 12 failures = 60 sec until the pod is terminated
        periodSeconds: 5
        failureThreshold: 12 
        timeoutSeconds: 5
        successThreshold: 1
```
<!-- {x-release-please-end} -->

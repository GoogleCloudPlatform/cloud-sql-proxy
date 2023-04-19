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

For most common usage, we do not recommend adding a readiness probe to the 
proxy sidecar container because it may cause unnecessary interruption to the
application's availability.

This readiness probe will fail when (1) the proxy used all its available
concurrent connections to a database or (2) the network connection
to the database is interrupted. These are transient states
that usually resolve within a few seconds. If the application is resilliant to
transient database connection failures, then it should recover without requiring
k8s to restart the pod. 

However, if the readiness check fails, k8s will kill the pod: both application
container and the proxy container. If your application would otherwise be able
to recover from a transient failure, this is an unnecessary interruption which
degrades the availability of the application.

We recommend adding a readiness check to the application container instead of
the proxy container.

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
      port: 9090
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

2. Add `--http-address` and `--http-port` (optional) to your
   proxy container configuration under `command: `.
    > [proxy_with_http_health_check.yaml](proxy_with_http_health_check.yaml#L53-L76)

```yaml
args:
  # If connecting from a VPC-native GKE cluster, you can use the
  # following flag to have the proxy connect over private IP
  # - "--private-ip"

  # Enable HTTP health checks
  - "--health-check"

  # Listen on all addresses so the kubelet can reach the endpoints
  - "--http-address=0.0.0.0"

  # Set the port where the HTTP server listens
  # - "--http-port 9090"

  # Enable structured logging with LogEntry format:
  - "--structured-logs"

  # This flag specifies where the service account key can be found
  # Remove this argument if you are using workload identity
  - "--credentials-file=/secrets/service_account.json"

  # Replace DB_PORT with the port the proxy should listen on
  - "--port=<DB_PORT>"
  - "<INSTANCE_CONNECTION_NAME>"
```

# Cloud SQL proxy health checks

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

- `/readiness`: Returns 200 status when the proxy has started, has available
connections if max connections have been set with the `--max-connections`
flag, and when the proxy can connect to all registered instances. Otherwise,
returns a 503 status. Optionally supports a min-ready query param (e.g.,
`/readiness?min-ready=3`) where the proxy will return a 200 status if the
proxy can connect successfully to at least min-ready number of instances. If
min-ready exceeds the number of registered instances, returns a 400.

- `/liveness`: Always returns 200 status. If this endpoint is not responding,
the proxy is in a bad state and should be restarted.

To configure the address, use `--http-address`. To configure the port, use
`--http-port`.

## Running Cloud SQL proxy with health checks in Kubernetes
1. Configure your Cloud SQL proxy container to include health check probes.
    > [proxy_with_http_health_check.yaml](proxy_with_http_health_check.yaml#L77-L111)
```yaml
# Recommended configurations for health check probes.
# Probe parameters can be adjusted to best fit the requirements of your application.
# For details, see https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
livenessProbe:
  httpGet:
    path: /liveness
    port: 8090
  # Number of seconds after the container has started before the first probe is scheduled. Defaults to 0.
  # Not necessary when the startup probe is in use.
  initialDelaySeconds: 0
  # Frequency of the probe. Defaults to 10.
  periodSeconds: 10
  # Number of seconds after which the probe times out. Defaults to 1.
  timeoutSeconds: 5
  # Number of times the probe is allowed to fail before the transition from healthy to failure state.
  # Defaults to 3.
  failureThreshold: 1
readinessProbe:
  httpGet:
    path: /readiness
    port: 8090
  initialDelaySeconds: 0
  periodSeconds: 10
  timeoutSeconds: 5
  # Number of times the probe must report success to transition from failure to healthy state.
  # Defaults to 1 for readiness probe.
  successThreshold: 1
  failureThreshold: 1
startupProbe:
  httpGet:
    path: /startup
    port: 8090
  periodSeconds: 1
  timeoutSeconds: 5
  failureThreshold: 20
```

2. Add `-use_http_health_check` and `-health-check-port` (optional) to your
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
  - "--http-address 0.0.0.0"

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

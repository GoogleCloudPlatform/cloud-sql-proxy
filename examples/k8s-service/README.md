# Running the Cloud SQL Proxy as a Service

This example demonstrates how to run the Cloud SQL Auth Proxy with PgBouncer on
Kubernetes as a service. It assumes you have already successfully completed all
the steps in [Using the Cloud SQL Auth Proxy on Kubernetes][sidecar].

In this example, you will deploy [PgBouncer][] with the Cloud SQL Auth Proxy as
a sidecar, in addition to configuring encryption between the application and
PgBouncer.

## A Word of Warning

Running PgBouncer with the Cloud SQL Auth Proxy may pose a significant
operational burden and should be undertaken with caution given the attendant
complexity.

In general, we recommend [running the proxy as a sidecar][sidecar] to your
application because it is simple, there is less overhead, it is secure out of
the box, and there is less latency involved.

However, the service pattern is useful when you are at very large scale, when
you clearly need a database connection pooler, and when you are running into SQL
Admin API quota problems.

## Initial Setup

Before we deploy PgBouncer with the Cloud SQL Auth Proxy, there are three
initial steps to take.

### Generate Certificates for PgBouncer

First, you will need to generate certificates to encrypt the connection between
the application and PgBouncer. We recommend using [CFSSL][] to handle
certificate generation. Note: this example uses self-signed certificates. In
some cases, using a certificate signed by a public certificate authority may be
preferred.  Alternatively, Kubernetes includes [an API for issuing
certificates][k8s-tls]. See the documentation on
[certificates][certificate-docs] for more details.

The certificate signing request is encoded as JSON in
[`ca_csr.json`](ca_csr.json) for the certificate authority and in
[`server_csr.json`](server_csr.json) for the "server," here PgBouncer.

First, we initialize our certificate authority.

``` shell
# This step produces ca-key.pem (the CA private key)
# and ca.pem (the CA certificate).
cfssl genkey -initca ca_csr.json | cfssljson -bare ca
```

Next, we generate a public and private key for the server. These will be what
we will use to encrypt traffic from the application to PgBouncer.

``` shell
# This step produces server-key.pem (the server private key)
# and server.pem (the server certicate).
cfssl gencert -ca cert -ca-key key server_csr.json | cfssljson -bare server
```

### Save the certificates as secrets

Second, with all the necessary certificates generated, we will save them as
secrets:

``` shell
# First the CA cert
kubectl create secret tls <YOUR-CA-SECRET> --key="ca-key.pem" --cert="ca.pem"

# Next the server cert
kubectl create secret tls <YOUR-SERVER-CERT-SECRET> --key="server-key.pem" \
  --cert="server.pem"
```

### Containerize PgBouncer

Third, we will containerize PgBouncer. Some users may prefer to containerize
PgBouncer themselves. For this example, we will make use of an open source
container, [edoburu/pgbouncer][edoburu]. One nice benefit of `edoburu/pgbouncer`
is that it will generate all the PgBouncer configuration based on environment
variables passed to the container.

## Deploy PgBouncer as a Service

With PgBouncer containerized, we will now create a deployment with PgBouncer and
the proxy as a sidecar.

First, we mount our CA certificate and server certificate and private key,
renaming the certificate secrets to `cert.pem` and server private key to
`key.pem`:

> [`pgbouncer_deployment.yaml`](pgbouncer_deployment.yaml#L15-L29)

``` yaml
volumes:
- name: cacert
  secret:
    secretName: <YOUR-CA-SECRET>
    items:
    - key: tls.crt
      path: cert.pem
- name: servercert
  secret:
    secretName: <YOUR-SERVER-CERT-SECRET>
    items:
    - key: tls.crt
      path: cert.pem
    - key: tls.key
      path: key.pem
```

Next, we specify volume mounts in our PgBouncer container where the secrets will
be stored:

> [`pgbouncer_deployment.yaml`](pgbouncer_deployment.yaml#L31-L41)

``` yaml
- name: pgbouncer
  image: <PG-BOUNCER-CONTAINER>
  ports:
  - containerPort: 5432
  volumeMounts:
  - name: cacert
    mountPath: "/etc/ca"
    readOnly: true
  - name: servercert
    mountPath: "/etc/server"
    readOnly: true
```

Then we configure PgBouncer through environment variables. Note: we use 5431 for
`DB_PORT` to leave 5432 available.

> [`pgbouncer_deployment.yaml`](pgbouncer_deployment.yaml#L42-L69)

``` yaml
env:
- name: DB_HOST
  value: "127.0.0.1"
- name: DB_USER
  valueFrom:
    secretKeyRef:
      name: <YOUR-DB-SECRET>
      key: username
- name: DB_PASSWORD
  valueFrom:
    secretKeyRef:
      name: <YOUR-DB-SECRET>
      key: password
- name: DB_NAME
  valueFrom:
    secretKeyRef:
      name: <YOUR-DB-SECRET>
      key: database
- name: DB_PORT
  value: "5431"
- name: CLIENT_TLS_SSLMODE
  value: "require"
- name: CLIENT_TLS_CA_FILE
  value: "/etc/ca/cert.pem"
- name: CLIENT_TLS_KEY_FILE
  value: "/etc/server/key.pem"
- name: CLIENT_TLS_CERT_FILE
  value: "/etc/server/cert.pem"
```

For the PgBouncer deployment, we add the proxy as a sidecar, starting it on port
5431:

> [`pgbouncer_deployment.yaml`](pgbouncer_deployment.yaml#L70-L76)

``` yaml
- name: cloud-sql-proxy
  image: gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.0.0.preview.0  # make sure the use the latest version
  args:
    # Replace DB_PORT with the port the proxy should listen on
    - "--port=<DB_PORT>"
    - "<INSTANCE_CONNECTION_NAME>"
  securityContext:
    runAsNonRoot: true
```

Next, we create a PgBouncer service, listening on port 5342:

> [`pgbouncer_service.yaml`](pgbouncer_service.yaml#L1-L11)

``` yaml
apiVersion: v1
kind: Service
metadata:
  name: <YOUR-SERVICE-NAME>
spec:
  selector:
    app: <YOUR-APPLICATION-NAME>
  ports:
  - protocol: TCP
    port: 5432
    targetPort: 5432
```

With the PgBouncer service and deployment done, we are ready to point our
application at it.

## Configure your application

First, we configure a volume for the CA certificate, mapping the file name to
`cert.pem`.

> [`deployment.yaml`](deployment.yaml#L1-L11)

``` yaml
volumes:
- name: cacert
  secret:
    secretName: <YOUR-CA-CERT>
    items:
    - key: tls.crt
      path: cert.pem
```

Next, we mount the volume within the application container:

> [`deployment.yaml`](deployment.yaml#L28-L31)

``` yaml
volumeMounts:
- name: cacert
  mountPath: "/etc/ca"
  readOnly: true
```

Then, we configure environment variables for connecting to the database, this
time including a `CA_CERT`:

> [`deployment.yaml`](deployment.yaml#L32-L53)

``` yaml
env:
- name: DB_HOST
  value: "<YOUR-SERVICE-NAME>.default.svc.cluster.local" # using the "default" namespace
- name: DB_USER
  valueFrom:
    secretKeyRef:
      name: <YOUR-DB-SECRET>
      key: username
- name: DB_PASS
  valueFrom:
    secretKeyRef:
      name: <YOUR-DB-SECRET>
      key: password
- name: DB_NAME
  valueFrom:
    secretKeyRef:
      name: <YOUR-DB-SECRET>
      key: database
- name: DB_PORT
  value: "5432"
- name: CA_CERT
  value: "/etc/ca/cert.pem"
```

Note: now the `DB_HOST` value uses an internal DNS record pointing at the
PgBouncer service.

Finally, when configuring a database connection string, the application must
provide the additional properties:

1. `sslmode` must be set to at least `verify-ca`
1. `sslrootcert` must set to the environment variable `CA_CERT`


[certificate-docs]: https://kubernetes.io/docs/tasks/administer-cluster/certificates/
[CFSSL]:            https://github.com/cloudflare/cfssl
[edoburu]:          https://hub.docker.com/r/edoburu/pgbouncer
[sidecar]:          ../k8s-sidecar/README.md
[k8s-tls]:          https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/
[PgBouncer]:        https://www.pgbouncer.org


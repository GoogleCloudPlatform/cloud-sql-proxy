# Cloud SQL Auth Proxy Sidecar

In the following example, we will deploy the Cloud SQL Proxy as a sidecar to an
existing application which connects to a Cloud SQL instance.

## Before you begin

1. If you haven't already, [create a project](https://cloud.google.com/resource-manager/docs/creating-managing-projects#creating_a_project).

1. [Enable the APIs](https://console.cloud.google.com/flows/enableapi?apiid=run.googleapis.com,sqladmin.googleapis.com,run.googleapis.com) that will be used during this tutorial:

  * Cloud SQL Admin
  * Cloud Build
  * Cloud Run
  
1. Create a Cloud SQL Postgres Instance by following these
[instructions](https://cloud.google.com/sql/docs/postgres/create-instance).
Note the connection string and default password that you create.

1. Create a database for your application by following these
[instructions](https://cloud.google.com/sql/docs/postgres/create-manage-databases).
Note the database name.

1. Optionally, create a database user for your instance following these
[instructions](https://cloud.google.com/sql/docs/postgres/create-manage-users).
Otherwise, use the username "postgres".

## Deploying the Application

The application you will be deploying should connect to the Cloud SQL Proxy using
TCP mode (for example, using the address "127.0.0.1:5432"). Follow the examples
on the [Connect Auth Proxy documentation](https://cloud.google.com/sql/docs/postgres/connect-auth-proxy#expandable-1)
page to correctly configure your application.

The connection pool is configured in the following sample:

```ruby
require 'sinatra'
require 'sequel'

set :bind, '0.0.0.0'
set :port, 8080

# Configure a connection pool that connects to the proxy via TCP
def connect_tcp
    Sequel.connect(
        adapter: 'postgres',
        host: ENV["INSTANCE_HOST"],
        port: ENV["DB_PORT"],
        database: ENV["DB_NAME"],
        user: ENV["DB_USER"],
        password: ENV["DB_PASS"],
        pool_timeout: 5,
        max_connections: 5,
    )
end

DB = connect_tcp()
```

Next, build the container image for the main application and deploy it:

```bash
gcloud builds submit --tag gcr.io/<YOUR_PROJECT_ID>/run-cloudsql
```

Finally, update the `multicontainer.yaml` file with the correct values for your
deployment for `YOUR_PROJECT_ID`, `DB_USER`, `DB_PASS`, `DB_NAME`, and `INSTANCE_CONNECTION_NAME`
listing the Cloud SQL container image as a sidecar:

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  annotations:
     run.googleapis.com/launch-stage: ALPHA
  name: multicontainer-service
spec:
  template:
    metadata:
      annotations:
        run.googleapis.com/execution-environment: gen1 #or gen2

    spec:
      containers:
      - name: my-app
        image: gcr.io/<YOUR_PROJECT_ID>/run-cloudsql
        ports:
          - containerPort: 8080
        env:
          - name: DB_USER
            value: <DB_USER>
          - name: DB_PASS
            value: <DB_PASS>
          - name: DB_NAME
            value: <DB_NAME>
          - name: INSTANCE_HOST
            value: "127.0.0.1"
          - name: DB_PORT
            value: "5432"
      initContainers:
      - name: cloud-sql-proxy
        restartPolicy: Always
        image: gcr.io/cloud-sql-connectors/cloud-sql-proxy:latest
        args:
             # Ensure the port number on the --port argument matches the value of
             # the DB_PORT env var on the my-app container.
             - "--port=5432"
             - "<INSTANCE_CONNECTION_NAME>"

```

You can optionally use Secret Manager to store the database password. See
[this documentation](https://cloud.google.com/run/docs/deploying#yaml) for more details.

Before deploying, you will need to make sure that the service account associated
with the Cloud Run deployment has the Cloud SQL Client role.
See [this documentation](https://cloud.google.com/sql/docs/postgres/roles-and-permissions)
for more details. The default service account will already have these permissions.

Finally, you can deploy the service using:

```bash
gcloud run services replace multicontainer.yaml
```

Once the service is deployed, the console should print out a URL. You can test
the service by sending a curl request with your gcloud identity token in the headers:

```bash
curl -H \
"Authorization: Bearer $(gcloud auth print-identity-token)" \
<SERVICE_URL>
```

# Cloud SQL Auth Proxy Sidecar
In the following example, we will deploy the Cloud SQL Proxy as a sidecar to an existing application which connects to a Cloud SQL instance. Before starting, make sure you have a working Cloud SQL instance. Make note of the Instance Connection Name, and the database name, username, and password needed for authentication.

 The application you will be deploying should connect to the Cloud SQL Proxy using TCP mode (for example, using the address "127.0.0.1:3306"). Follow the examples on the [Connect Auth Proxy documentation](https://cloud.google.com/sql/docs/mysql/connect-auth-proxy#expandable-1) page to correctly configure your application. 

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
gcloud builds submit --tag gcr.io/<YOUR_PROJECT_ID>/run-alloydb
```

Finally, create a revision YAML file (multicontainers.yaml), using the `example.yaml` file as a referece for the deployment, listing the AlloyDB container image as a sidecar:

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
            value: "3306"
      - name: cloud-sql-proxy
        image: gcr.io/cloud-sql-connectors/cloud-sql-proxy:latest
        args:
             # If connecting from within a VPC network, you can use the
             # following flag to have the proxy connect over private IP
             # - "--private-ip"

             # Replace DB_PORT with the port the proxy should listen on
             - "--port=5432"
             - "<INSTANCE_CONNECTION_NAME>"

```

Before deploying, you will need to make sure that the service account associated with the Cloud Run Deployment has the Cloud SQL Client role. See [this documentation](https://cloud.google.com/sql/docs/mysql/roles-and-permissions) for more details.

Finally, you can deploy the service using:

```bash
gcloud run services replace multicontainers.yaml
```
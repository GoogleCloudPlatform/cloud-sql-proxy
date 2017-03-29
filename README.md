
## Cloud SQL Proxy
The Cloud SQL Proxy allows a user with the appropriate permissions to connect
to a Second Generation Cloud SQL database without having to deal with IP whitelisting or SSL
certificates manually. It works by opening unix/tcp sockets on the local machine
and proxying connections to the associated Cloud SQL instances when the sockets
are used.

To build from source, ensure you have [go installed](https://golang.org/doc/install)
and have set [GOPATH](https://github.com/golang/go/wiki/GOPATH). Then, simply do a go get:

    go get github.com/GoogleCloudPlatform/cloudsql-proxy/cmd/cloud_sql_proxy

The cloud_sql_proxy will be placed in $GOPATH/bin after go get completes.

cloud_sql_proxy takes a few arguments to configure:

* `-fuse`: requires access to `/dev/fuse` as well as the `fusermount` binary. An
  optional `-fuse_tmp` flag can specify where to place temporary files. The
  directory indicated by `-dir` is mounted.
* `-instances="project1:region:instance1,project3:region:instance1"`: A comma-separated list
  of instances to open inside `-dir`. Also supports exposing a tcp port instead of using Unix Domain Sockets; see examples below.
* `-instances_metadata=metadata_key`: Usable on [GCE](https://cloud.google.com/compute/docs/quickstart) only. The given [GCE metadata](https://cloud.google.com/compute/docs/metadata) key will be
  polled for a list of instances to open in `-dir`. The format for the value is the same as the 'instances' flag. A hanging-poll strategy is used, meaning that changes to
  the metadata value will be reflected in the `-dir` even while the proxy is
  running. When an instance is removed from the list the corresponding socket
  will be removed from `-dir` as well (unless it was also specified in
  `-instances`), but any existing connections to this instance will NOT be
  terminated.

Note: `-instances` and `-instances_metadata` may be used at the same time but
are not compatible with the `-fuse` flag.

By default, the proxy will authenticate under the default service account of the
Compute Engine VM it is running on. Therefore, the VM must have at least the
sqlservice.admin API scope ("https://www.googleapis.com/auth/sqlservice.admin")
and the associated project must have the SQL Admin API
enabled.  The default service account must also have at least WRITER/EDITOR
priviledges to any projects of target SQL instances.

Specifying the `-credential_file` flag allows use of the proxy outside of
Google's cloud. Simply [create a new service
account](https://console.developers.google.com/project/_/apiui/credential/serviceaccount),
download the associated JSON file, and set `-credential_file` to the path of the
JSON file. You may also set the GOOGLE_APPLICATION_CREDENTIALS environment variable instead of passing this flag.

## Example invocations:

    ./cloud_sql_proxy -dir=/cloudsql -instances=my-project:us-central1:sql-inst &
    mysql -u root -S /cloudsql/my-project:us-central1:sql-inst

    # For -fuse you do not need to specify instance names ahead of time:
    ./cloud_sql_proxy -dir=/cloudsql -fuse &
    mysql -u root -S /cloudsql/my-project:us-central1:sql-inst

    # For programs which do not support using Unix Domain Sockets, specify tcp:
    ./cloud_sql_proxy -dir=/cloudsql -instances=my-project:us-central1:sql-inst=tcp:3306 &
    mysql -u root -h 127.0.0.1

    # Caution: This should be executed in a closed network. If executing on a system without a correctly configured firewall this could potentially allow anything on the internet to access the database.
    # For accessing from another host in a network, specify host:
    ./cloud_sql_proxy -dir=/cloudsql -instances=my-project:us-central1:sql-inst=tcp:0.0.0.0:3306
    # From another host:
    mysql -u root -h [proxy-machine-ip]

## To use inside a Go program:
If your program is written in [Go](https://golang.org) you can use the Cloud SQL Proxy as a library,
avoiding the need to start the Proxy as a companion process.

### MySQL
If you're using the the MySQL [go-sql-driver](https://github.com/go-sql-driver/mysql)
you can use helper functions found in the [`proxy/dialers/mysql` package](https://godoc.org/github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql). See [example usage](https://github.com/GoogleCloudPlatform/cloudsql-proxy/blob/master/tests/dialers_test.go).

### Postgres
If you're using the the Postgres [lib/pq](https://github.com/lib/pq), you can use the `cloudsqlpostgres` driver from [here](https://github.com/GoogleCloudPlatform/cloudsql-proxy/tree/master/proxy/dialers/postgres). See [example usage](https://github.com/GoogleCloudPlatform/cloudsql-proxy/blob/master/proxy/dialers/postgres/hook_test.go).

I'm open to adding more drivers, feel free to file an issue.

## To use from Kubernetes:

[Kubernetes](http://kubernetes.io) does not support the metadata server that is used by default for credentials,
so we have to manually pass the credentials to the proxy as a Kubernetes
[Secret](http://kubernetes.io/v1.1/docs/user-guide/secrets.html). At a high level, we have to create a Secret,
add it as a Volume in a Pod and mount that Volume into the proxy container. Here are some detailed steps:

* Create a Service Account and download the JSON credential file, following [these steps](https://cloud.google.com/docs/authentication#developer_workflow).
* Create a local Kubernetes Secret named `sqlcreds` from this file by base64 encoding the Service Account file, and creating a secret.yaml file with that content:
```
apiVersion: v1
kind: Secret
metadata:
  name: sqlcreds
type: Opaque
data:
  file.json: "BASE64 encoded Service Account credential file."
```

* Create this Secret using `kubectl create`.
```
$ kubectl create -f secret.yaml
```

* Add the `sqlcreds` Secret in your Pod by creating a volume like this:
```
  - name: secret-volume
    # This GCE PD must already exist.
    secret:
      secretName: sqlcreds
```

* You'll also need to create a `hostPath` volume allowing the SQL proxy to read SSL certificates:
```
- name: ssl-certs
  hostPath:
    path: /etc/ssl/certs
```

* Create an emptydir volume named `cloudsql` for the SQL proxy to place it's socket:
```
  - name: cloudsql
    emptyDir:
```

* Add the SQL proxy container to your pod, and mount the `sqlcreds` and 'ssl-certs' volumes, making sure to pass the correct instance and project.
```
  - image: gcr.io/cloudsql-docker/gce-proxy:1.06
    volumeMounts:
    - name: cloudsql
      mountPath: /cloudsql
    - name: secret-volume
      mountPath: /secret/
    - name: ssl-certs
      mountPath: /etc/ssl/certs
    command: ["/cloud_sql_proxy", "-dir=/cloudsql", "-credential_file=/secret/file.json", "-instances=MYPROJECT:MYREGION:MYINSTANCE"]
```
Note that we pass the path to the secret file in the command line arguments to the proxy.
We also pass the project and Cloud SQL instance name we want to connect to using the "--instances" flag.

* To use the proxy from your application container, mount the shared cloudsql volume:
```
volumeMounts:
    - name: cloudsql
      mountPath: /cloudsql
```
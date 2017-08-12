Cloud SQL Proxy in a Kubernetes cluster
=======================================

The goal of this guide is to help you set-up and use Google Cloud SQL in
a Kubernetes cluster (GKE or not), through the Cloud SQL Proxy.

To make this as easy as possible, we will use the prepared docker image
so we can minimize the number of steps. No compilation needed!

Pre-requisites:
---------------

In order to set-up the Cloud SQL you will need,

- One or more Google Cloud SQL Databases. Refer to [the
  documentation](https://cloud.google.com/sql/docs/) to create them.

- We will assume the name of the database instances are as follow:
  `project:database1`, `project:database2`, etc.

- You need a service-account token with "Project Editor" privilegies,
  and we will assume the file is in `$HOME/credentials.json`. Refer to
  [the documentation](https://cloud.google.com/docs/authentication#developer_workflow)
  to get the json credential file.

- Your `$HOME/.kube/config` points to your cluster and the namespace you
  want to use.

Overview
--------

The recommended way to use the Cloud SQL Proxy in a Kubernetes cluster
is to use a TCP connection, as this allows the pod to be located on any
node. We will use [Kubernetes DNS
service](http://kubernetes.io/docs/admin/dns/) to connect to the proxy
seamlessly.

Setting-up the credentials
--------------------------

We need to create a secret to store the credentials that the Cloud Proxy
needs to connect to the project database instances:

```
kubectl create secret generic service-account-token --from-file=credentials.json=$HOME/credentials.json
```

Creating the Cloud SQL Proxy deployment
---------------------------------------

We need to create a deployment that will keep the Cloud SQL Proxy
container image alive.

Here is an example deployment file, `sqlproxy-deployment.yaml`:

```
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: cloudsqlproxy
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: cloudsqlproxy
    spec:
      containers:
       # Make sure to specify image tag in production
       # Check out the newest version in release page
       # https://github.com/GoogleCloudPlatform/cloudsql-proxy/releases
      - image: b.gcr.io/cloudsql-docker/gce-proxy:latest
       # 'Always' if imageTag is 'latest', else set to 'IfNotPresent'
        imagePullPolicy: Always
        name: cloudsqlproxy
        command:
        - /cloud_sql_proxy
        - -dir=/cloudsql
        - -instances=project:database1=tcp:0.0.0.0:3306,project:database2=tcp:0.0.0.0:3307
        - -credential_file=/credentials/credentials.json
        ports:
        - name: port-database1
          containerPort: 3306
	- name: port-database2
	  containerPort: 3307
        volumeMounts:
        - mountPath: /cloudsql
          name: cloudsql
        - mountPath: /credentials
          name: service-account-token
      volumes:
      - name: cloudsql
        emptyDir:
      - name: service-account-token
        secret:
          secretName: service-account-token
```

And then, create the deployment:

```
kubectl apply -f sqlproxy-deployment.yaml
```

This deployment will create pods that listen for connections on port
`3306` for `project:database1`, and `3307` for `project:database2`.

You can also change the number of replicas to increase availability.

Services to find the proxy
--------------------------

We can create services to find the pods. We have decided to use one
service per database to be able to select the database by name rather
than by port.

Create the services configuration, `sqlproxy-services.yaml`:

```
apiVersion: v1
kind: Service
metadata:
  name: sqlproxy-service-database1
spec:
  ports:
  - port: 3306
    targetPort: port-database1
  selector:
    app: cloudsqlproxy
---
apiVersion: v1
kind: Service
metadata:
  name: sqlproxy-service-database2
spec:
  ports:
  - port: 3306
    targetPort: port-database2
  selector:
    app: cloudsqlproxy
```

This will create two different services, `sqlproxy-service-database1`
and `sqlproxy-service-database2`.

Apply the configuration to create them:

```
kubectl apply -f sqlproxy-services.yaml
```

You can now connect using the same port `3306` to each database:

```
mysql --host=sqlproxy-service-database1 --port=3306 ...
mysql --host=sqlproxy-service-database2 --port=3306 ...
```

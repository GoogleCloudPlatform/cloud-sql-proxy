# Using the Cloud SQL proxy on Kubernetes

The Cloud SQL proxy is the recommended way to connect to Cloud SQL, even when
using private IP. This is because the proxy provides strong encryption and
authentication using IAM, which help keep your database secure.

## Configure your application with Secrets

In Kubernetes, [Secrets][ksa-secret] are a secure way to pass configuration
details to your application. Each Secret object can contain multiple key/value
pairs that can be pass to your application in multiple ways. When connecting to
a database, you can create a Secret with details such as your database name,
user, and password which can be injected into your application as env vars.

1. Create a secret with information needed to access your database:
    ```shell
    kubectl create secret generic <YOUR-DB-SECRET> \
        --from-literal=username=<YOUR-DATABASE-USER> \
        --from-literal=password=<YOUR-DATABASE-PASSWORD> \
        --from-literal=database=<YOUR-DATABASE-NAME>
    ```
2. Next, configure your application's container to mount the secrets as env
   vars:
    > [proxy_with_workload_identity.yaml](proxy_with_workload_identity.yaml#L21-L36)
    ```yaml
       env:
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
    ```
3. Finally, configure your application to use these values. In the example
above, the values will be in the env vars `DB_USER`, `DB_PASS`, and `DB_NAME`.

[ksa-secret]: https://kubernetes.io/docs/concepts/configuration/secret/

## Setting up a service account

The first step to running the Cloud SQL proxy in Kubernetes is creating a
service account to represent your application. It is recommended that you create
a service account unique to each application, instead of using the same service
account everywhere. This model is more secure since it allows your to limit
permissions on a per-application basis.

The service account for your application needs to meet the following criteria:

1. Belong to a project with the [Cloud SQL Admin API][admin-api] enabled
1. [Has been granted][grant-sa] the
   [`Cloud SQL Client` IAM role (or equivalent)][csql-roles]
   for the project containing the instance you want to connect to
1. If connecting using private IP, you must use a
   [VPC-native GKE cluster][vpc-gke], in the same VPC as your Cloud SQL instance

[admin-api]: https://console.cloud.google.com/flows/enableapi?apiid=sqladmin&redirect=https://console.cloud.google.com
[grant-sa]: https://cloud.google.com/iam/docs/granting-roles-to-service-accounts
[csql-roles]: https://cloud.google.com/iam/docs/understanding-roles#cloud-sql-roles
[vpc-gke]: https://cloud.google.com/kubernetes-engine/docs/how-to/alias-ips

## Providing the service account to the proxy

Next, you need to configure Kubernetes to provide the service account to the
Cloud SQL Auth proxy. There are two recommended ways to do this.

### Workload Identity

If you are using [Google Kubernetes Engine][gke],  the preferred method is to
use GKE's [Workload Identity][workload-id] feature. This method allows you to
bind a [Kubernetes Service Account (KSA)][ksa] to a Google Service Account
(GSA). The GSA will then be accessible to applications using the matching KSA.

1. [Enable Workload Identity for your cluster][enable-wi]
1. [Enable Workload Identity for your node pool][enable-wi-node-pool]
1. Create a KSA for your application `kubectl apply -f service-account.yaml`:

    > [service-account.yaml](service_account.yaml#L2-L5)
    ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: <YOUR-KSA-NAME> # TODO(developer): replace these values
    ```
1. Enable the IAM binding between your `<YOUR-GSA-NAME>` and `<YOUR-KSA-NAME>`:
    ```sh
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member "serviceAccount:<YOUR-GCP-PROJECT>.svc.id.goog[<YOUR-K8S-NAMESPACE>/<YOUR-KSA-NAME>]" \
      <YOUR-GSA-NAME>@<YOUR-GCP-PROJECT>.iam.gserviceaccount.com
    ```
1. Add an annotation to `<YOUR-KSA-NAME>` to complete the binding:
    ```sh
    kubectl annotate serviceaccount \
       <YOUR-KSA-NAME> \
       iam.gke.io/gcp-service-account=<YOUR-GSA-NAME>@<YOUR-GCP-PROJECT>.iam.gserviceaccount.com
    ```
1. Finally, make sure to specify the service account for the k8s pod spec:
    > [proxy_with_workload_identity.yaml](proxy_with_workload_identity.yaml#L2-L15)
    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: <YOUR-DEPLOYMENT-NAME>
    spec:
      selector:
        matchLabels:
          app: <YOUR-APPLICATION-NAME>
      template:
        metadata:
          labels:
            app: <YOUR-APPLICATION-NAME>
        spec:
          serviceAccountName: <YOUR-KSA-NAME>
    ```

[gke]: https://cloud.google.com/kubernetes-engine
[workload-id]: https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity
[ksa]: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
[enable-wi]: https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#enable_on_existing_cluster
[enable-wi-node-pool]: https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#option_2_node_pool_modification

### Service account key file

Alternatively, if your can't use Workload Identity, the recommended pattern is
to mount a service account key file into the Cloud SQL proxy pod and use the
`-credential_file` flag.

1. Create a credential file for your service account key:
    ```sh
    gcloud iam service-accounts keys create ~/key.json \
      --iam-account <YOUR-SA-NAME>@project-id.iam.gserviceaccount.com
    ```
1. Turn your service account key into a k8s [Secret][k8s-secret]:
    ```shell
    kubectl create secret generic <YOUR-SA-SECRET> \
    --from-file=service_account.json=~/key.json
    ```
3. Mount the secret as a volume under the`spec:` for your k8s object:
    > [proxy_with_sa_key.yaml](proxy_with_sa_key.yaml#L74-L77)
    ```yaml
    volumes:
    - name: <YOUR-SA-SECRET-VOLUME>
      secret:
        secretName: <YOUR-SA-SECRET>
    ```

4. Follow the instructions in the next section to access the volume from the
   proxy's pod.

[k8s-secret]: https://kubernetes.io/docs/concepts/configuration/secret/

## Run the Cloud SQL proxy as a sidecar

We recommend running the proxy in a "sidecar" pattern (as an additional
container sharing a pod with your application). We recommend this over running
as a separate service for several reasons:

* Prevents your SQL traffic from being exposed locally - the proxy provides
  encryption on outgoing connections, but you should limit exposure for
  incoming connections
* Prevents a single point of failure - each application's access to
  your database is independent from the others, making it more resilient.
* Limits access to the proxy, allowing you to use IAM permissions per
  application rather than exposing the database to the entire cluster
* Allows you to scope resource requests more accurately - because the
  proxy consumes resources linearly to usage, this pattern allows you to more
  accurately scope and request resources to match your applications as it
  scales

1. Add the Cloud SQL proxy to the pod configuration under `containers`:
    > [proxy_with_workload-identity.yaml](proxy_with_workload_identity.yaml#L39-L69)
    ```yaml
    - name: cloud-sql-proxy
      # It is recommended to use the latest version of the Cloud SQL proxy
      # Make sure to update on a regular schedule!
      image: gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.0.0.preview.0  # make sure the use the latest version
      args:
        # If connecting from a VPC-native GKE cluster, you can use the
        # following flag to have the proxy connect over private IP
        # - "--private-ip"

        # Replace DB_PORT with the port the proxy should listen on
        - "--port=<DB_PORT>"
        - "<INSTANCE_CONNECTION_NAME>"
      securityContext:
        # The default Cloud SQL proxy image runs as the
        # "nonroot" user and group (uid: 65532) by default.
        runAsNonRoot: true
      # Resource configuration depends on an application's requirements. You
      # should adjust the following values based on what your application
      # needs. For details, see https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
      resources:
        requests:
          # The proxy's memory use scales linearly with the number of active
          # connections. Fewer open connections will use less memory. Adjust
          # this value based on your application's requirements.
          memory: "2Gi"
          # The proxy's CPU use scales linearly with the amount of IO between
          # the database and the application. Adjust this value based on your
          # application's requirements.
          cpu:    "1"
    ```
   If you are using a service account key, specify your secret volume and add
   the `-credential_file` flag to the command:

   > [proxy_with_sa_key.yaml](proxy_with_sa_key.yaml#L49-L58)
    ```yaml
      # This flag specifies where the service account key can be found
      - "-credential_file=/secrets/service_account.json"
    securityContext:
      # The default Cloud SQL proxy image runs as the
      # "nonroot" user and group (uid: 65532) by default.
      runAsNonRoot: true
    volumeMounts:
    - name: <YOUR-SA-SECRET-VOLUME>
      mountPath: /secrets/
      readOnly: true
    ```

1. Finally, configure your application to connect via `127.0.0.1` on whichever
   `<DB_PORT>` you specified in the command section.


## Connecting without the Cloud SQL proxy

While not as secure, it is possible to connect from a VPC-native GKE cluster to
a Cloud SQL instance on the same VPC using private IP without the proxy.

1. Create a secret with your instance's private IP address:
    ```shell
    kubectl create secret generic <YOUR-PRIVATE-IP-SECRET> \
        --from-literal=db_host=<YOUR-PRIVATE-IP-ADDRESS>
    ```

2. Next make sure you add the secret to your application's container:
   > [no_proxy_private_ip.yaml](no_proxy_private_ip.yaml#L34-L38)
   ```yaml
   - name: DB_HOST
     valueFrom:
       secretKeyRef:
         name: <YOUR-PRIVATE-IP-SECRET>
         key: db_host
   ```

3. Finally, configure your application to connect using the IP address from the
   `DB_HOST` env var. You will need to use the correct port for your db-engine
   (MySQL: `3306`, Postgres: `5432`, SQLServer: `1433`).

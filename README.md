
The Cloud SQL Proxy allows a user with the appropriate permissions to connect
to a Cloud SQL database without having to deal with IP whitelisting or SSL
certificates manually. It works by opening unix/tcp sockets on the local machine
and proxying connections to the associated Cloud SQL instances when the sockets
are used.

cloud_sql_proxy takes a few arguments to configure:

* `-fuse`: requires access to `/dev/fuse` as well as the `fusermount` binary. An
  optional `-fuse_tmp` flag can specify where to place temporary files. The
  directory indicated by `-dir` is mounted.
* `-instances="project1:instance1,project3:instance1"`: A comma-separated list
  of instances to open inside `-dir`.
* `-instances_metadata=metadata_key`: The given GCE metadata key will be
  polled for a list of instances to open in `-dir`. The format for the value is a
  comma-separated list. A hanging-poll strategy is used, meaning that changes to
  the metadata value will be reflected in the `-dir` even while the proxy is
  running. When an instance is removed from the list the corresponding socket
  will be removed from `-dir` as well (unless it was also specified in
  `-instances`), but any existing connections to this instance will NOT be
  terminated.

Note: `-instances` and `-instances_metadata` may be used at the same time but
are not compatible with the `-fuse` flag since using FUSE requires `-dir` to
be empty when it is mounted.

By default, the proxy will authenticate under the default service account of the
Compute Engine VM it is running on. Therefore, the VM must have at least the
sql-admin API scope and the associated project must have the SQL Admin API
enabled.  The default service account must also have at least WRITER/EDITOR
priviledges to any projects of target SQL instances.

Specifying the `-credential_file` flag allows use of the proxy outside of
Google's cloud. Simply [create a new service
account](https://console.developers.google.com/project/_/apiui/credential/serviceaccount),
download the associated JSON file, and set `-credential_file` to the path of the
JSON file.

Examples:
  ./cloud_sql_proxy -dir=/cloudsql -instances=my-project:us-central:sql-inst &
  mysql -u user [-p] -S /cloudsql/my-project:us-central:sql-inst

  # For -fuse you do not need to specify instance names ahead of time:
  ./cloud_sql_proxy -dir=/cloudsql -fuse &
  mysql -u user [-p] -S /cloudsql/my-project:us-central:sql-inst


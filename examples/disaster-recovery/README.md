# Coordinate disaster recovery with Secret Manager

## Background

This document assumes you are already using the following strategy for
detecting and triggering failovers:
1. Using an independent service to detect when the primary is down
2. Trigger a promotion of an existing read replica to become a primary
3. Update a Secret Manager secret with the name of the current primary

## Restart Auth Proxy when secret changes

This option uses a wrapper script around the Cloud SQL Auth Proxy to detect
when the secret has changed, and restart the Proxy with the new value. This
could be done in many languages, but here’s an example using bash:

> [failover.sh](examples/disaster-recovery/failover.sh)
```sh
#! /bin/bash

SECRET_ID="my-secret-id" # TODO(developer): replace this value
REFRESH_INTERVAL=5
PORT=5432

# Get the latest version of the secret and start the Proxy
INSTANCE=$(gcloud secrets versions access "latest" --secret="$SECRET_ID")
cloud-sql-proxy --port "$PORT" "$INSTANCE" &
PID=$!

# Every 5s, get the latest version of the secret. If it's changed, restart the
# Proxy with the new value.
while true; do
    sleep $REFRESH_INTERVAL
    NEW=$(gcloud secrets versions access "latest" --secret="$SECRET_ID")
    if [ "$INSTANCE" != "$NEW" ]; then
        INSTANCE=$NEW
        kill $PID
        wait $PID
        cloud-sql-proxy --port "$PORT" "$INSTANCE" &
        PID=$!
    fi
done
```

## Benefits of this approach

Using this approach will help assist with failovers without needing to
reconfigure your application. Instead, by changing the Proxy the application
will always connect to 127.0.0.1 and won’t need to restart to apply
configuration changes. Additionally, it will prevent split brain syndrome by
ensuring that your application can only connect to the current “primary”.

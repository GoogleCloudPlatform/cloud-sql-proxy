# Copyright 2022 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# [START cloud_sql_proxy_secret_manager_failover]
#!/bin/bash

SECRET_ID="my-secret-id" # TODO(developer): replace this value
REFRESH_INTERVAL=5
PORT=5432                # TODO(developer): change this port as needed

# Get the latest version of the secret and start the proxy
INSTANCE=$(gcloud secrets versions access "latest" --secret="$SECRET_ID")
cloud_sql_proxy -instances="$INSTANCE"=tcp:"$PORT" &
PID=$!

# Every 5s, get the latest version of the secret. If it's changed, restart the
# proxy with the new value.
while true; do
    sleep $REFRESH_INTERVAL
    NEW=$(gcloud secrets versions access "latest" --secret="$SECRET_ID")
    if [ "$INSTANCE" != "$NEW" ]; then
        INSTANCE=$NEW
        kill $PID
        wait $PID
        cloud_sql_proxy -instances="$INSTANCE"=tcp:"$PORT" &
        PID=$!
    fi
done
# [END cloud_sql_proxy_secret_manager_failover]


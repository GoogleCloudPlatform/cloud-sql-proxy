# Copyright 2023 Google LLC
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
#! /bin/bash

SECRET_ID="my-secret-id" # TODO(developer): replace this value
PORT=5432

# Get the latest version of the secret and start the proxy
INSTANCE=$(gcloud secrets versions access "latest" --secret="$SECRET_ID")
cloud-sql-proxy --port "$PORT" "$INSTANCE" &
PID=$!

# Every 5s, get the latest version of the secret. If it's changed, restart the
# proxy with the new value.
while true; do
    sleep 5
    NEW=$(gcloud secrets versions access "latest" --secret="$SECRET_ID")
    if [ "$INSTANCE" != "$NEW" ]; then
        INSTANCE=$NEW
        kill $PID
        wait $PID
        cloud-sql-proxy --port "$PORT" "$INSTANCE" &
        PID=$!
    fi
done
# [END cloud_sql_proxy_secret_manager_failover]


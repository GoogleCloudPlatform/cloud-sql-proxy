#!/bin/bash -e

# This script downloads the latest build of the Cloud SQL Proxy and uses gcloud
# to suggest a configuration based on the current project.

echo "Downloading the Cloud SQL Proxy..."
wget -q https://dl.google.com/cloudsql/cloud_sql_proxy.linux.amd64
mv cloud_sql_proxy.linux.amd64 cloud_sql_proxy
chmod +x cloud_sql_proxy
mkdir -p cloudsql
PROJECT=$(gcloud config list 2>/dev/null | grep project | cut -d\  -f3)
INSTANCES=$(gcloud sql instances list | awk "{ print \$3,\"$PROJECT:\" \$1 }" | grep "^db-" | cut -d\  -f2 | tr '\n' ',')
if [[ "$INSTANCES" == "" ]]; then
        echo "No Cloud SQL Second Generation instances found:"
        gcloud sql instances list
        exit 1
fi
echo
echo "Run the following command to start the proxy:"
echo "    ./cloud_sql_proxy -dir cloudsql -instances $INSTANCES >proxy_log 2>&1 &"
echo
echo "Then connect to any of these instances:"
for A in ${INSTANCES//,/ }; do
        echo "  $A"
done
anInstance=$(echo $INSTANCES | cut -d, -f1)
echo
echo "For example:"
echo "    mysql -u root -S cloudsql/$anInstance"


#! /bin/bash
# Copyright 2020 Google LLC
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

# This script distributes the artifacts for the Cloud SQL proxy to their different channels.

set -e # exit immediatly if any step fails

PROJ_ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. >/dev/null 2>&1 && pwd )"
cd $PROJ_ROOT

# get the current version
export VERSION=$(cat cmd/version.txt)
if [ -z "$VERSION" ]; then
  echo "error: No version.txt found in $PROJ_ROOT"
  exit 1
fi


read -p "This will release new Cloud SQL proxy artifacts for \"$VERSION\", even if they already exist. Are you sure (y/Y)? " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]
then
    exit 1
fi

# Build and push the container images
gcloud builds submit --async --config .build/default.yaml --substitutions _VERSION=$VERSION
gcloud builds submit --async --config .build/alpine.yaml --substitutions _VERSION=$VERSION
gcloud builds submit --async --config .build/buster.yaml --substitutions _VERSION=$VERSION
gcloud builds submit --async --config .build/bullseye.yaml --substitutions _VERSION=$VERSION

# Build the binarys and upload to GCS
gcloud builds submit --config .build/gcs_upload.yaml --substitutions _VERSION=$VERSION
# cleam up any artifacts.json left by previous builds
gsutil rm -f gs://cloud-sql-connectors/cloud-sql-proxy/v$VERSION/*.json 2> /dev/null || true

# Generate sha256 hashes for authentication
echo -e "Add the following table to the release notes on GitHub: \n\n"
echo "| filename | sha256 hash |"
echo "|----------|-------------|"
for f in $(gsutil ls "gs://cloud-sql-connectors/cloud-sql-proxy/v$VERSION/cloud-sql-proxy*"); do
    file=$(basename $f)
    sha=$(gsutil cat $f | sha256sum --binary | head -c 64)
    echo "| [$file](https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v$VERSION/$file) | $sha |"
done

tag_latest() {
    local new_version=$1
    for registry in "gcr.io" "us.gcr.io" "eu.gcr.io" "asia.gcr.io"
    do
        local base_image="$registry/cloud-sql-connectors/cloud-sql-proxy"
        echo "Tagging $new_version as latest in $registry"
        gcloud container images add-tag --quiet "$base_image:$new_version" "$base_image:latest"
    done
}

tag_latest $VERSION

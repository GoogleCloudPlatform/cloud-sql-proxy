#!/bin/bash
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


# This script finds all container images with the provided version tag and adds
# the "latest" tag to them.
#
# For example:
# 1. Add a "latest" tag to the v1.23.1 images
#
#     ./tag_latest 1.23.1
#
# 2. Print out the gcloud commands without running them:
#
#     ./tag_latest 1.23.1 -dry-run
#

if [ "$1" = "" ]
then
    echo "Usage: $0 <version without the v prefix to tag as latest> [-dry-run]"
    exit 1
fi

dry_run=false
if [ "$2" = "-dry-run" ]
then
    dry_run=true
fi

tag_latest() {
    local new_version=$1
    for registry in "gcr.io" "us.gcr.io" "eu.gcr.io" "asia.gcr.io"
    do
        local base_image="$registry/cloud-sql-connectors/cloud-sql-proxy"
        if [ "$dry_run" != true ]
        then
            gcloud container images add-tag --quiet "$base_image:$new_version" "$base_image:latest"
        else
            echo [DRY RUN] gcloud container images add-tag "$base_image:$new_version" "$base_image:latest"
        fi
    done
}

tag_latest "$1"

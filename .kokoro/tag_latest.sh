#!/bin/bash

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
        local base_image="$registry/cloudsql-docker/gce-proxy"
        if [ "$dry_run" != true ]
        then
            gcloud container images add-tag --quiet "$base_image:$new_version" "$base_image:latest"
        else
            echo [DRY RUN] gcloud container images add-tag "$base_image:$new_version" "$base_image:latest"
        fi
    done
}

tag_latest "$1"

#!/usr/bin/env bash

# Copyright 2025 Google LLC.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http=//www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Set SCRIPT_DIR to the current directory of this file.
SCRIPT_DIR=$(cd -P "$(dirname "$0")" >/dev/null 2>&1 && pwd)
SCRIPT_FILE="${SCRIPT_DIR}/$(basename "$0")"

##
## Local Development
##
## These functions should be used to run the local development process
##

## clean - Cleans the build output
function clean() {
  if [[ -d '.tools' ]] ; then
    rm -rf .tools
  fi
}

## build - Builds the project without running tests.
function build() {
   go build -o ./cloud-sql-proxy main.go
}

## test - Runs local unit tests.
function test() {
  go test -v -race -cover -short ./...
}

## e2e - Runs end-to-end integration tests.
function e2e() {
  if [[ ! -f .envrc ]] ; then
    write_e2e_env .envrc
  fi
  source .envrc
  e2e_ci
}

# e2e_ci - Run end-to-end integration tests in the CI system.
#   This assumes that the secrets in the env vars are already set.
function e2e_ci() {
  go test -race -v ./... | tee test_results.txt
}

function get_golang_tool() {
  name="$1"
  github_repo="$2"
  package="$3"

  # Download goimports tool
  version=$(curl -s "https://api.github.com/repos/$github_repo/tags" | jq -r '.[].name' | head -n 1)
  mkdir -p "$SCRIPT_DIR/.tools"
  cmd="$SCRIPT_DIR/.tools/$name"
  versioned_cmd="$SCRIPT_DIR/.tools/$name-$version"
  if [[ ! -f "$versioned_cmd" ]] ; then
    GOBIN="$SCRIPT_DIR/.tools" go install "$package@$version"
    mv "$cmd" "$versioned_cmd"
    if [[ -f "$cmd" ]] ; then
      unlink "$cmd"
    fi
    ln -s "$versioned_cmd" "$cmd"
  fi
}

## fix - Fixes code format.
function fix() {
  # run code formatting
  get_golang_tool 'goimports' 'golang/tools' 'golang.org/x/tools/cmd/goimports'
  ".tools/goimports" -w .
  go mod tidy
  go fmt ./...

  # Generate CMD docs
  go run ./cmd/gendocs/gen_cloud-sql-proxy_docs.go
}

## lint - runs the linters
function lint() {
  # run lint checks
  get_golang_tool 'golangci-lint' 'golangci/golangci-lint' 'github.com/golangci/golangci-lint/v2/cmd/golangci-lint'
  ".tools/golangci-lint" run --timeout 3m

  # Check the commit includes a go.mod that is fully
  # up to date.
  fix
  if [[ -d "$SCRIPT_DIR/.git" ]] ; then
    git diff --exit-code
  fi
}

# lint_ci - runs lint in the CI build job, exiting with an error code if lint fails.
function lint_ci() {
  lint # run lint
  git diff --exit-code # fail if any files changed
}

## deps - updates project dependencies to latest
function deps() {
  go get -u ./...
  go get -t -u ./...

  # Update the image label in the dockerfiles
  for n in Dockerfile Dockerfile.* ; do
    dockerfile_from_deps "$n"
  done
}

# find
function dockerfile_from_deps() {
  # FROM gcr.io/distroless/static:nonroot@sha256:627d6c5a23ad24e6bdff827f16c7b60e0289029b0c79e9f7ccd54ae3279fb45f
  # curl -X GET https://gcr.io/v2/distroless/static/manifests/nonroot
  file=$1

  # Get the last FROM statement from the dockerfile
  # those ar
  fromLine=$(grep "FROM" $1 | tail -n1)
  imageUrl="${fromLine#FROM *}"

  # If the image URL does not contain a hash, then don't do anything.
  if [[ $imageUrl != *@* ]] ; then
    echo "Image does not contain a digest, ignoring"
    return
  fi

  oldDigest="${imageUrl#*@}" #after the '@'
  imageWithoutHash="${imageUrl%%@sha256*}" #before the '@sha256'
  imageName="${imageWithoutHash%%:*}" #before the ':'

  imageLabel="${imageWithoutHash#*:}" #after the ':'
  # If none found, use "latest" as the label
  if [[ "$imageLabel" == "$imageName" ]] ; then
    imageLabel=latest
  fi

  imageRepo="${imageName%%/*}" #first part of the image name path, may be a repo hostname
  if [[ "$imageRepo" == *.* ]]; then
    imageName="${imageName#*/}" # trim repo name host from imageName
    manifestUrl="https://${imageRepo}/v2/${imageName}/manifests/${imageLabel}"
    digest=$(curl -X GET "$manifestUrl" | \
      jq -r '.manifests[] | select(.platform.architecture=="amd64" and .platform.os=="linux") | .digest')

  else
    # registry-1.docker.io requires a token
    docker_io_token=$(curl -s "https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/alpine:pull" | jq -r .token)
    manifestUrl="https://registry-1.docker.io/v2/${imageName}/manifests/${imageLabel}"
    digest=$(curl -s -H "Authorization: Bearer $docker_io_token" \
         -H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
         https://registry-1.docker.io/v2/library/alpine/manifests/3 | \
          jq -r '.manifests[] | select(.platform.architecture=="amd64" and .platform.os=="linux") | .digest')
  fi

  if [[ "$oldDigest" == "$digest" ]] ; then
    echo "No update to image to $file"
  else
    echo "Updating docker image to $file to $digest"
    set -x
    sed -i.bak -e "s/$oldDigest/$digest/g" "$file"
  fi
  if [[ -f "$file.bak" ]] ; then
    rm "$file.bak"
  fi

}

# write_e2e_env - Loads secrets from the gcloud project and writes
#     them to target/e2e.env to run e2e tests.
function write_e2e_env(){
  # All secrets used by the e2e tests in the form <env_name>=<secret_name>
  secret_vars=(
    MYSQL_CONNECTION_NAME=MYSQL_CONNECTION_NAME
    MYSQL_USER=MYSQL_USER
    MYSQL_PASS=MYSQL_PASS
    MYSQL_DB=MYSQL_DB
    MYSQL_MCP_CONNECTION_NAME=MYSQL_MCP_CONNECTION_NAME
    MYSQL_MCP_PASS=MYSQL_MCP_PASS
    POSTGRES_CONNECTION_NAME=POSTGRES_CONNECTION_NAME
    POSTGRES_USER=POSTGRES_USER
    POSTGRES_USER_IAM=POSTGRES_USER_IAM
    POSTGRES_PASS=POSTGRES_PASS
    POSTGRES_DB=POSTGRES_DB
    POSTGRES_CAS_CONNECTION_NAME=POSTGRES_CAS_CONNECTION_NAME
    POSTGRES_CAS_PASS=POSTGRES_CAS_PASS
    POSTGRES_CUSTOMER_CAS_CONNECTION_NAME=POSTGRES_CUSTOMER_CAS_CONNECTION_NAME
    POSTGRES_CUSTOMER_CAS_PASS=POSTGRES_CUSTOMER_CAS_PASS
    POSTGRES_CUSTOMER_CAS_DOMAIN_NAME=POSTGRES_CUSTOMER_CAS_DOMAIN_NAME
    POSTGRES_MCP_CONNECTION_NAME=POSTGRES_MCP_CONNECTION_NAME
    POSTGRES_MCP_PASS=POSTGRES_MCP_PASS
    SQLSERVER_CONNECTION_NAME=SQLSERVER_CONNECTION_NAME
    SQLSERVER_USER=SQLSERVER_USER
    SQLSERVER_PASS=SQLSERVER_PASS
    SQLSERVER_DB=SQLSERVER_DB
    IMPERSONATED_USER=IMPERSONATED_USER
  )

  if [[ -z "$TEST_PROJECT" ]] ; then
    echo "Set TEST_PROJECT environment variable to the project containing"
    echo "the e2e test suite secrets."
    exit 1
  fi

  local_user=$(gcloud auth list --format 'value(account)' | tr -d '\n')

  echo "Getting test secrets from $TEST_PROJECT into $1"
  {
  for env_name in "${secret_vars[@]}" ; do
    env_var_name="${env_name%%=*}"
    secret_name="${env_name##*=}"
    set -x
    val=$(gcloud secrets versions access latest --project "$TEST_PROJECT" --secret="$secret_name")
    echo "export $env_var_name='$val'"
  done

  # Set IAM User env vars to the local gcloud user
  echo "export MYSQL_IAM_USER='${local_user%%@*}'"
  echo "export POSTGRES_USER_IAM='$local_user'"
  } > "$1"

}

## build_image - Builds and pushes the proxy container image using local source.
## Usage: ./build.sh build_image [image-url]
function build_image() {
  local image_url="${1:-}"
  local push_arg=""

  if [[ -n "$image_url" ]]; then
    push_arg="--push"
    echo "Preparing to build and push proxy image: $image_url"
  else
    echo "Preparing to build proxy image (no push)..."
    push_arg="--load"
    image_url="cloud-sql-proxy:local"
  fi

  function cleanup_build() {
      rm -f cloud-sql-proxy Dockerfile.local
  }
  trap cleanup_build EXIT

  echo "Building binary locally..."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cmd.metadataString=container" -o cloud-sql-proxy

  echo "Creating temporary Dockerfile..."
  cat > Dockerfile.local <<EOF
FROM gcr.io/distroless/static:nonroot
COPY cloud-sql-proxy /cloud-sql-proxy
USER 65532
ENTRYPOINT ["/cloud-sql-proxy"]
EOF

  echo "Building Docker image..."
  docker buildx build \
    --platform "linux/amd64" \
    -f Dockerfile.local \
    -t "$image_url" \
    $push_arg \
    .
  
  echo "Done."
}

## help - prints the help details
##
function help() {
   # This will print the comments beginning with ## above each function
   # in this file.

   echo "build.sh <command> <arguments>"
   echo
   echo "Commands to assist with local development and CI builds."
   echo
   echo "Commands:"
   echo
   grep -e '^##' "$SCRIPT_FILE" | sed -e 's/##/ /'
}

set -euo pipefail

# Check CLI Arguments
if [[ "$#" -lt 1 ]] ; then
  help
  exit 1
fi

cd "$SCRIPT_DIR"

"$@"


#!/bin/bash
#
# build.sh wraps `go build` to make compiling the Cloud SQL Proxy for
# distribution more streamlined. When doing normal development on the proxy,
# `go build .` (or even `go run .`) is sufficient for iterating on the code.
# This script simply allows a convenient way to cross compile and build a docker
# container.
#
# With no arguments, this script will build a binary marked with "development"
#
# Specifying 'release' as the first argument to this script will cross compile
# for all supported operating systems and architectures. This requires a version
# identifier to be supplied as the second argument.
#
# Specifying 'docker' as the first argument to this script will build a
# container, tagging it with the second argument. The tag will also be used as
# the version identifier.

files=$(git status -s)
if [[ $? != 0 ]]; then
  echo >&2 "Error running git status"
  exit 2
fi

# build builds a new binary. Expected variables:
#   VERSION: string to print out when --version is passed to the final binary
#   OS: operation system to target (windows, darwin, linux, etc)
#   ARCH: architecture to target (amd64, 386, etc)
#   OUT: location to place binary
build() {
  echo "   Compile -> $OUT"
  CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build \
    -ldflags "-X 'main.versionString=$VERSION'" -a -installsuffix cgo -o $OUT \
    github.com/GoogleCloudPlatform/cloudsql-proxy/cmd/cloud_sql_proxy
}

# git_version echos out version information related to the git repo and date.
git_version() {
  edits=""
  if [[ "$files" != "" ]]; then
    edits=" (local edits)"
  fi
  echo "sha $(git rev-parse HEAD)$edits built $(date)"
}

set -e

case $1 in
"release")
  if [[ "$files" != "" ]]; then
    echo >&2 "Can't build a release version with local edits; files:"
    echo >&2 "$files"
    exit 1
  fi
  if [[ "$2" == "" ]]; then
    echo >&2 "Must provide a version number to use as the second parameter"
    exit 1
  fi
  VERSION="version $2; $(git_version)"
  echo "Cross-compiling $VERSION..."

  for OS in windows darwin linux; do
    for ARCH in amd64 386; do
      OUT="cloud_sql_proxy.$OS.$ARCH"
      build
    done
  done
  ;;
"docker")
  if [[ "$2" == "" ]]; then
    echo >&2 "Must provide a version number to use as the second parameter"
    exit 1
  fi
  VERSION="version $2; $(git_version)"
  OS="linux"
  ARCH="amd64"
  OUT=cloud_sql_proxy.docker
  echo "Compiling $VERSION for docker..."
  build

  cat >Dockerfile <<EOF
FROM scratch

COPY cloud_sql_proxy.docker /cloud_sql_proxy
EOF
  echo "Building docker container (tag: $2)..."
  docker build -t "$2" .

  # Cleanup
  rm Dockerfile cloud_sql_proxy.docker
  ;;
*)
  VERSION="development; $(git_version)"
  echo "Compiling $VERSION..."
  OUT=cloud_sql_proxy
  build
esac

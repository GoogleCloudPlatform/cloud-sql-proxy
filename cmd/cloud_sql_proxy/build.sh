#!/bin/bash

files=$(git status -s)
if [[ $? != 0 ]]; then
  echo >&2 "Error running git status"
  exit 2
fi

RELEASE=0
if [[ "$1" == "release" ]]; then
  if [[ "$files" != "" ]]; then
    echo >&2 "Can't build a release version with local edits; files:"
    echo >&2 "$files"
    exit 1
  fi
  if [[ "$2" == "" ]]; then
    echo >&2 "Must provide a version number to use as a second parameter"
    exit 1
  fi
  VERSION="version $2"
  RELEASE=1
else
  VERSION="development"
fi

VERSION+="; sha $(git rev-parse HEAD) built $(date)"
echo "Compiling $VERSION..."

if [[ $RELEASE == 0 ]]; then
  CGO_ENABLED=0 go build -ldflags "-X 'main.versionString=$VERSION'" -a -installsuffix cgo -o cloud_sql_proxy .
else
  for OS in windows darwin linux; do
    for ARCH in amd64 386; do
      OUT="cloud_sql_proxy.$OS.$ARCH"
      echo "   Compile $OUT"
      CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -ldflags "-X 'main.versionString=$VERSION'" -a -installsuffix cgo -o $OUT .
    done
  done
fi

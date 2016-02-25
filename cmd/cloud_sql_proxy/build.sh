#!/bin/bash

files=$(git status -s)
if [[ $? != 0 ]]; then
  echo >&2 "Error running git status"
  exit 2
fi


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
else
  VERSION="development"
fi

VERSION+="; sha $(git rev-parse HEAD) built $(date)"

echo "Compiling $VERSION"
CGO_ENABLED=0 GOOS=linux go build -x -ldflags "-X 'main.versionString=$VERSION'" -a -installsuffix cgo -o cloud_sql_proxy .

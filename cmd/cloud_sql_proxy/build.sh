#!/bin/bash

VERSION=1.01

CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.versionString=$VERSION" -a -installsuffix cgo -o cloud_sql_proxy .

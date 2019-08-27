#!/bin/bash
# Copyright 2019 Google Inc.
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

# `-e` enables the script to automatically fail when a command fails
set -e

# Ensure both ipv4 and ipv6 functionality is tested.
#
# Tests will fail if the environment doesn't support both nets and it means we
# need to find a test env that supports both.
export EXPECT_IPV4_AND_IPV6=true

# Re-organize files into proper Go format
export GOPATH=$PWD/gopath
target=$GOPATH/src/github.com/GoogleCloudPlatform
mkdir -p $target
mv github/cloud-sql-proxy $target
cd $target/cloud-sql-proxy

echo "******************** Getting dependencies... ********************"
# Get the dependencies
go get -t -v ./...
echo -e "\n"
echo "******************** Dependencies complete.  ********************"

echo -e "\n\n******************** Starting tests. ********************"

echo "************ [gofmt] running... ***********"
diff -u <(echo -n) <(gofmt -d .)
echo "************ [gofmt] complete.  ***********"

echo -e "\n******************** Tests complete. ********************\n"

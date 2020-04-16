#!/bin/bash
# Copyright 2020 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDIcd TIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# `-e` enables the script to automatically fail when a command fails
set -e

# Move into project directory
cd github/cloud-sql-proxy

# Download and verify dependencies are valid
echo -e "******************** Verifing dependencies... ********************\n"
go get -t -v ./...
echo -e "******************** Dependencies verified.  ********************\n"

# Load in secrets
if [ -n "$KOKORO_GFILE_DIR" ]; then
  source "${KOKORO_GFILE_DIR}/TEST_SECRETS.sh"
fi


echo -e "******************** Running tests... ********************\n"
go test -v ./...
echo -e "******************** Tests complete.  ********************\n"
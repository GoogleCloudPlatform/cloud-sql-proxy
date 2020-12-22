#! /bin/bash
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

export GO111MODULE=on

# kokoro setup
if [ -n "$KOKORO_GFILE_DIR" ]; then
  # move into project directory
  cd github/cloud-sql-proxy
  # install fuse project
  brew update > /dev/null
  brew install --cask --quiet osxfuse
  # source secrets
  source "${KOKORO_GFILE_DIR}/TEST_SECRETS.sh"
  export GOOGLE_APPLICATION_CREDENTIALS="${KOKORO_GFILE_DIR}/testing-service-account.json"
fi

# On macOS, the default $TMPDIR is too long for suitable use due to the unix socket length limits
export TMPDIR="/tmp"
echo -e "******************** Running tests... ********************\n"
go test -v ./...
echo -e "******************** Tests complete.  ********************\n"
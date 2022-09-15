// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy_test

import (
	"strings"
	"testing"
)

var (
	pg         = strings.ReplaceAll("proj:region:pg", ":", ".")
	pg2        = strings.ReplaceAll("proj:region:pg2", ":", ".")
	mysql      = strings.ReplaceAll("proj:region:mysql", ":", ".")
	mysql2     = strings.ReplaceAll("proj:region:mysql2", ":", ".")
	sqlserver  = strings.ReplaceAll("proj:region:sqlserver", ":", ".")
	sqlserver2 = strings.ReplaceAll("proj:region:sqlserver2", ":", ".")
)

func verifySocketPermissions(t *testing.T, addr string) {
	// On Linux and Darwin, we check that the socket named by addr exists with
	// os.Stat. That operation is not supported on Windows.
	// See https://github.com/microsoft/Windows-Containers/issues/97#issuecomment-887713195
}

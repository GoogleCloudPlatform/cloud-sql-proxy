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

import "strings"

var (
	pg         = strings.Replace("proj:region:pg", ":", ".")
	pg2        = strings.Replace("proj:region:pg2", ":", ".")
	mysql      = strings.Replace("proj:region:mysql", ":", ".")
	mysql2     = strings.Replace("proj:region:mysql2", ":", ".")
	sqlserver  = strings.Replace("proj:region:sqlserver", ":", ".")
	sqlserver2 = strings.Replace("proj:region:sqlserver2", ":", ".")
)

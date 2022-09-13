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

//go:build !windows
// +build !windows

package proxy_test

import (
	"os"
	"testing"
)

var (
	pg         = "proj:region:pg"
	pg2        = "proj:region:pg2"
	mysql      = "proj:region:mysql"
	mysql2     = "proj:region:mysql2"
	sqlserver  = "proj:region:sqlserver"
	sqlserver2 = "proj:region:sqlserver2"
)

func verifySocketPermissions(t *testing.T, addr string) {
	fi, err := os.Stat(addr)
	if err != nil {
		t.Fatalf("os.Stat(%v): %v", addr, err)
	}
	if fm := fi.Mode(); fm != 0777|os.ModeSocket {
		t.Fatalf("file mode: want = %v, got = %v", 0777|os.ModeSocket, fm)
	}
}

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

package proxy

import "path/filepath"

// UnixAddress is defined as a function to distinguish between Unix-based
// implementations where the dir and inst are simply joined, and Windows-based
// implementations where the inst must be further altered.
func UnixAddress(dir, inst string) string {
	return filepath.Join(dir, inst)
}

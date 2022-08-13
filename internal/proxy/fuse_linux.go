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

package proxy

import (
	"errors"
	"os/exec"
)

// SupportsFUSE checks if the fusermount binary is present in the PATH or a well
// known location.
func SupportsFUSE() error {
	// This code follows the same strategy found in hanwen/go-fuse.
	// See https://github.com/hanwen/go-fuse/blob/0f728ba15b38579efefc3dc47821882ca18ffea7/fuse/mount_linux.go#L184-L198.
	if _, err := exec.LookPath("fusermount"); err != nil {
		if _, err := exec.LookPath("/bin/fusermount"); err != nil {
			return errors.New("fusermount binary not found in PATH or /bin")
		}
	}
	return nil
}

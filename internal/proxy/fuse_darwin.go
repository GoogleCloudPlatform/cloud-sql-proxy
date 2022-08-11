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
	"os"
)

const (
	macfusePath = "/Library/Filesystems/macfuse.fs/Contents/Resources/mount_macfuse"
	osxfusePath = "/Library/Filesystems/osxfuse.fs/Contents/Resources/mount_osxfuse"
)

// SupportsFUSE checks if macfuse or osxfuse are installed on the host by
// looking for both in their known installation location.
func SupportsFUSE() error {
	// This code follows the same strategy as hanwen/go-fuse.
	// See https://github.com/hanwen/go-fuse/blob/0f728ba15b38579efefc3dc47821882ca18ffea7/fuse/mount_darwin.go#L121-L124.

	// check for macfuse first (newer version of osxfuse)
	if _, err := os.Stat(macfusePath); err != nil {
		// if that fails, check for osxfuse next
		if _, err := os.Stat(osxfusePath); err != nil {
			return errors.New("failed to find osxfuse or macfuse: verify FUSE installation and try again (see https://osxfuse.github.io).")
		}
	}
	return nil
}

// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/v2/fs"
)

// readme represents a static read-only text file.
type readme struct {
	fs.Inode
}

const readmeText = `
When programs attempt to open files in this directory, a remote connection to
the Cloud SQL instance of the same name will be established.

That is, running:

	mysql -u root -S "/path/to/this/directory/project:region:instance-2"
	-or-
	psql "host=/path/to/this/directory/project:region:instance-2 dbname=mydb user=myuser"

will open a new connection to the specified instance, given you have the correct
permissions.

Listing the contents of this directory will show all instances with active
connections.
`

// Getattr implements fs.NodeGetattrer and indicates that this file is a regular
// file.
func (*readme) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	*out = fuse.AttrOut{Attr: fuse.Attr{
		Mode: 0444 | syscall.S_IFREG,
		Size: uint64(len(readmeText)),
	}}
	return fs.OK
}

// Read implements fs.NodeReader and supports incremental reads.
func (*readme) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	end := int(off) + len(dest)
	if end > len(readmeText) {
		end = len(readmeText)
	}
	return fuse.ReadResultData([]byte(readmeText[off:end])), fs.OK
}

// Open implements fs.NodeOpener and supports opening the README as a read-only
// file.
func (*readme) Open(ctx context.Context, mode uint32) (fs.FileHandle, uint32, syscall.Errno) {
	df := nodefs.NewDataFile([]byte(readmeText))
	rf := nodefs.NewReadOnlyFile(df)
	return rf, 0, fs.OK
}

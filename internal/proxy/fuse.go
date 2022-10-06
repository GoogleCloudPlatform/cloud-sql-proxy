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

//go:build !windows && !openbsd && !freebsd
// +build !windows,!openbsd,!freebsd

package proxy

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fuse/nodefs"
)

// symlink implements a symbolic link, returning the underlying path when
// Readlink is called.
type symlink struct {
	fs.Inode
	path string
}

// Readlink implements fs.NodeReadlinker and returns the symlink's path.
func (s *symlink) Readlink(_ context.Context) ([]byte, syscall.Errno) {
	return []byte(s.path), fs.OK
}

// readme represents a static read-only text file.
type readme struct {
	fs.Inode
}

const readmeText = `
When applications attempt to open files in this directory, a remote connection
to the Cloud SQL instance of the same name will be established.

For example, when you run one of the following commands, the proxy will initiate
a connection to the corresponding Cloud SQL instance, given you have the correct
IAM permissions.

	mysql -u root -S "/somedir/project:region:instance"

    # or

	psql "host=/somedir/project:region:instance dbname=mydb user=myuser"

For MySQL, the proxy will create a socket with the instance connection name
(e.g., project:region:instance) in this directory. For Postgres, the proxy will
create a directory with the instance connection name, and create a socket inside
that directory with the special Postgres name: .s.PGSQL.5432.

Listing the contents of this directory will show all instances with active
connections.
`

// Getattr implements fs.NodeGetattrer and indicates that this file is a regular
// file.
func (*readme) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	*out = fuse.AttrOut{Attr: fuse.Attr{
		Mode: 0444 | syscall.S_IFREG,
		Size: uint64(len(readmeText)),
	}}
	return fs.OK
}

// Read implements fs.NodeReader and supports incremental reads.
func (*readme) Read(_ context.Context, _ fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	end := int(off) + len(dest)
	if end > len(readmeText) {
		end = len(readmeText)
	}
	return fuse.ReadResultData([]byte(readmeText[off:end])), fs.OK
}

// Open implements fs.NodeOpener and supports opening the README as a read-only
// file.
func (*readme) Open(_ context.Context, _ uint32) (fs.FileHandle, uint32, syscall.Errno) {
	df := nodefs.NewDataFile([]byte(readmeText))
	rf := nodefs.NewReadOnlyFile(df)
	return rf, 0, fs.OK
}

// Copyright 2015 Google Inc. All Rights Reserved.
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

//go:build !windows && !openbsd
// +build !windows,!openbsd

// Package fuse provides a connection source wherein the user does not need to
// specify which instance they are connecting to before they start the
// executable. Instead, simply attempting to access a file in the provided
// directory will transparently create a proxied connection to an instance
// which has that name.
//
// Specifically, given that NewConnSrc was called with the mounting directory
// as /cloudsql:
//
// 1) Execute `mysql -S /cloudsql/speckle:instance`
// 2) The 'mysql' executable looks up the file "speckle:instance" inside "/cloudsql"
// 3) This lookup is intercepted by the code in this package. A local unix socket
//    located in a temporary directory is opened for listening and the lookup for
//    "speckle:instance" returns to mysql saying that it is a symbolic link
//    pointing to this new local socket.
// 4) mysql dials the local unix socket, creating a new connection to the
//    specified instance.
package fuse

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/logging"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/proxy/proxy"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fuse/nodefs"
	"golang.org/x/net/context"
)

// NewConnSrc returns a source of new connections based on Lookups in the
// provided mount directory. If there isn't a directory located at tmpdir one
// is created. The second return parameter can be used to shutdown and release
// any resources. As a result of this shutdown, or during any other fatal
// error, the returned chan will be closed.
//
// The connset parameter is optional.
func NewConnSrc(mountdir, tmpdir string, client *proxy.Client, connset *proxy.ConnSet) (<-chan proxy.Conn, io.Closer, error) {
	if err := os.MkdirAll(tmpdir, 0777); err != nil {
		return nil, nil, err
	}
	if connset == nil {
		// Make a dummy one.
		connset = proxy.NewConnSet()
	}
	conns := make(chan proxy.Conn, 1)
	root := &fsRoot{
		tmpDir:  tmpdir,
		linkDir: mountdir,
		dst:     conns,
		links:   make(map[string]*symlink),
		connset: connset,
		client:  client,
	}

	srv, err := fs.Mount(mountdir, root, &fs.Options{
		MountOptions: fuse.MountOptions{AllowOther: true},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("FUSE mount failed: %q: %v", mountdir, err)
	}

	closer := fuseCloser(func() error {
		err := srv.Unmount() // Best effort unmount
		if err != nil {
			logging.Errorf("Unmount failed: %v", err)
		}
		return root.Close()
	})
	return conns, closer, nil
}

type fuseCloser func() error

func (fc fuseCloser) Close() error {
	return fc()
}

// symlink implements a symbolic link, returning the underlying path when
// Readlink is called.
type symlink struct {
	fs.Inode
	path string
}

var _ fs.NodeReadlinker = &symlink{}

func (s *symlink) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	return []byte(s.path), fs.OK
}

// fsRoot provides the in-memory file system that supports lazy connections to
// Cloud SQL instances.
type fsRoot struct {
	fs.Inode

	// tmpDir defines a temporary directory where all the sockets are placed
	// faciliating connections to Cloud SQL instances.
	tmpDir string
	// linkDir is the directory that holds symbolic links to the tmp dir for
	// each Cloud SQL instance connection. After shutdown, this directory is
	// cleaned out.
	linkDir string

	client  *proxy.Client
	connset *proxy.ConnSet

	// sockLock protects fields in this struct related to sockets; specifically
	// 'links' and 'closers'.
	sockLock sync.Mutex
	links    map[string]*symlink
	// closers includes a reference to all open Unix socket listeners. When
	// fs.Close is called, all of these listeners are also closed.
	closers []io.Closer

	sync.RWMutex
	dst chan<- proxy.Conn
}

var _ interface {
	fs.InodeEmbedder
	fs.NodeGetattrer
	fs.NodeLookuper
	fs.NodeReaddirer
} = &fsRoot{}

func (r *fsRoot) newConn(instance string, c net.Conn) {
	r.RLock()
	// dst will be nil if Close has been called already.
	if ch := r.dst; ch != nil {
		ch <- proxy.Conn{Instance: instance, Conn: c}
	} else {
		logging.Errorf("Ignored new conn request to %q: system has been closed", instance)
	}
	r.RUnlock()
}

// Close shuts down the fsRoot filesystem and closes all open Unix socket
// listeners.
func (r *fsRoot) Close() error {
	r.Lock()
	if r.dst != nil {
		// Since newConn only sends on dst while holding a reader lock, holding the
		// writer lock is sufficient to ensure there are no pending sends on the
		// channel when it is closed.
		close(r.dst)
		// Setting it to nil prevents further sends.
		r.dst = nil
	}
	r.Unlock()

	var errs bytes.Buffer
	r.sockLock.Lock()
	for _, c := range r.closers {
		if err := c.Close(); err != nil {
			fmt.Fprintln(&errs, err)
		}
	}
	r.sockLock.Unlock()

	if errs.Len() == 0 {
		return nil
	}
	logging.Errorf("Close %q: %v", r.linkDir, errs.String())
	return errors.New(errs.String())
}

// Getattr implements fs.NodeGetattrer and represents fsRoot as a directory.
func (r *fsRoot) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	*out = fuse.AttrOut{Attr: fuse.Attr{
		Mode: 0555 | fuse.S_IFDIR,
	}}
	return fs.OK
}

// Lookup implements fs.NodeLookuper and handles all requests, either for the
// README, or for a new connection to a Cloud SQL instance. When receiving a
// request for a Cloud SQL instance, Lookup will return a symlink to a Unix
// socket that provides connectivity to a remote instance.
func (r *fsRoot) Lookup(ctx context.Context, instance string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if instance == "README" {
		return r.NewInode(ctx, &readme{}, fs.StableAttr{}), fs.OK
	}
	r.sockLock.Lock()
	defer r.sockLock.Unlock()

	if _, _, _, _, err := proxy.ParseInstanceConnectionName(instance); err != nil {
		return nil, syscall.ENOENT
	}

	if ret, ok := r.links[instance]; ok {
		return ret.EmbeddedInode(), fs.OK
	}

	// path is the location of the Unix socket
	path := filepath.Join(r.tmpDir, instance)
	os.RemoveAll(path) // Best effort; the following will fail if this does.
	// linkpath is the location the symlink points to
	linkpath := path

	// Add a ".s.PGSQL.5432" suffix to path for Postgres instances
	if r.client != nil {
		version, err := r.client.InstanceVersionContext(ctx, instance)
		if err != nil {
			logging.Errorf("Failed to get Instance version for %s: %v", instance, err)
			return nil, syscall.ENOENT
		}
		if strings.HasPrefix(strings.ToLower(version), "postgres") {
			if err := os.MkdirAll(path, 0755); err != nil {
				logging.Errorf("Failed to create path %s: %v", path, err)
				return nil, syscall.EIO
			}
			path = filepath.Join(linkpath, ".s.PGSQL.5432")
		}
	}
	// TODO: check path length -- if it exceeds the max supported socket length,
	// return an error that helps the user understand what went wrong.
	// Otherwise, we get a "bind: invalid argument" error.

	sock, err := net.Listen("unix", path)
	if err != nil {
		logging.Errorf("couldn't listen at %q: %v", path, err)
		return nil, syscall.EEXIST
	}
	if err := os.Chmod(path, 0777|os.ModeSocket); err != nil {
		logging.Errorf("couldn't update permissions for socket file %q: %v; other users may be unable to connect", path, err)
	}

	go r.listenerLifecycle(sock, instance, path)

	ret := &symlink{path: linkpath}
	inode := r.NewInode(ctx, ret, fs.StableAttr{Mode: 0777 | fuse.S_IFLNK})
	r.links[instance] = ret
	// TODO(chowski): memory leak when listeners exit on their own via removeListener.
	r.closers = append(r.closers, sock)

	return inode, fs.OK
}

// removeListener marks that a Listener for an instance has exited and is no
// longer serving new connections.
func (r *fsRoot) removeListener(instance, path string) {
	r.sockLock.Lock()
	defer r.sockLock.Unlock()
	v, ok := r.links[instance]
	if ok && v.path == path {
		delete(r.links, instance)
	} else {
		logging.Errorf("Removing a listener for %q at %q which was already replaced", instance, path)
	}
}

// listenerLifecycle calls l.Accept in a loop, and for each new connection
// r.newConn is called. After the Listener returns an error it is removed.
func (r *fsRoot) listenerLifecycle(l net.Listener, instance, path string) {
	for {
		start := time.Now()
		c, err := l.Accept()
		if err != nil {
			logging.Errorf("error in Accept for %q: %v", instance, err)
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				d := 10*time.Millisecond - time.Since(start)
				if d > 0 {
					time.Sleep(d)
				}
				continue
			}
			break
		}
		r.newConn(instance, c)
	}
	r.removeListener(instance, path)
	l.Close()
	if err := os.Remove(path); err != nil {
		logging.Errorf("couldn't remove %q: %v", path, err)
	}
}

// Readdir implements fs.NodeReaddirer and returns a list of files for each
// instance to which the proxy is actively connected. In addition, the list
// includes a README.
func (r *fsRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	activeConns := r.connset.IDs()
	entries := []fuse.DirEntry{
		{Name: "README", Mode: 0555 | fuse.S_IFREG},
	}
	for _, conn := range activeConns {
		entries = append(entries, fuse.DirEntry{
			Name: conn,
			Mode: 0777 | syscall.S_IFSOCK,
		})
	}
	ds := fs.NewListDirStream(entries)
	return ds, fs.OK
}

// readme represents a static read-only text file.
type readme struct {
	fs.Inode
}

var _ interface {
	fs.InodeEmbedder
	fs.NodeGetattrer
	fs.NodeReader
	fs.NodeOpener
} = &readme{}

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

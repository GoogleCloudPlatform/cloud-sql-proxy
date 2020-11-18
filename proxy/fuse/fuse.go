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
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"golang.org/x/net/context"
)

// Supported returns true if the current system supports FUSE.
// TODO: for OSX, check to see if OSX FUSE is installed.
func Supported() bool {
	return true
}

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

	logging.Verbosef("Mounting %v...", mountdir)
	c, err := fuse.Mount(mountdir, fuse.AllowOther())
	if err != nil {
		// a common cause of failed mounts is that a previous instance did not shutdown cleanly, leaving an abandoned mount
		logging.Errorf("WARNING: Mount failed - attempting to unmount dir to resolve...", mountdir)
		if err = fuse.Unmount(mountdir); err != nil {
			logging.Errorf("Unmount failed: %v", err)
		}
		if c, err = fuse.Mount(mountdir, fuse.AllowOther()); err != nil {
			return nil, nil, fmt.Errorf("cannot mount %q: %v", mountdir, err)
		}
	}
	logging.Infof("Mounted %v", mountdir)

	if connset == nil {
		// Make a dummy one.
		connset = proxy.NewConnSet()
	}
	conns := make(chan proxy.Conn, 1)
	root := &fsRoot{
		tmpDir:  tmpdir,
		linkDir: mountdir,
		dst:     conns,
		links:   make(map[string]symlink),
		closers: []io.Closer{c},
		connset: connset,
		client:  client,
	}

	server := fs.New(c, &fs.Config{
		Debug: func(msg interface{}) {
			if false {
				logging.Verbosef("%s", msg)
			}
		},
	})

	go func() {
		if err := server.Serve(root); err != nil {
			logging.Errorf("serve %q exited due to error: %v", mountdir, err)
		}
		// The server exited but we don't know whether this is because of a
		// graceful reason (via root.Close) or via an external force unmounting.
		// Closing the root will ensure the 'dst' chan is closed correctly to
		// signify that no new connections are possible.
		if err := root.Close(); err != nil {
			logging.Errorf("root.Close() error: %v", err)
		}
		logging.Infof("FUSE exited")
	}()

	return conns, root, nil
}

// symlink implements a symbolic link, returning the underlying string when
// Readlink is called.
type symlink string

var _ interface {
	fs.Node
	fs.NodeReadlinker
} = symlink("")

func (s symlink) Readlink(context.Context, *fuse.ReadlinkRequest) (string, error) {
	return string(s), nil
}

// Attr helps implement fs.Node.
func (symlink) Attr(ctx context.Context, a *fuse.Attr) error {
	*a = fuse.Attr{
		Mode: 0777 | os.ModeSymlink,
	}
	return nil
}

type fsRoot struct {
	tmpDir, linkDir string

	client  *proxy.Client
	connset *proxy.ConnSet

	// sockLock protects fields in this struct related to sockets; specifically
	// 'links' and 'closers'.
	sockLock sync.Mutex
	links    map[string]symlink
	// closers holds a slice of things to close when fsRoot.Close is called.
	closers []io.Closer

	sync.RWMutex
	dst chan<- proxy.Conn
}

// Ensure that fsRoot implements the following interfaces.
var _ interface {
	fs.FS
	fs.Node
	fs.NodeRequestLookuper
	fs.HandleReadDirAller
} = &fsRoot{}

func (r *fsRoot) newConn(instance string, c net.Conn) {
	r.RLock()
	// dst will be nil if Close has been called already.
	if ch := r.dst; ch != nil {
		ch <- proxy.Conn{instance, c}
	} else {
		logging.Errorf("Ignored new conn request to %q: system has been closed", instance)
	}
	r.RUnlock()
}

func (r *fsRoot) Forget() {
	logging.Verbosef("Forget called on %q", r.linkDir)
}

func (r *fsRoot) Destroy() {
	logging.Verbosef("Destroy called on %q", r.linkDir)
}

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

	logging.Infof("unmount %q", r.linkDir)
	if err := fuse.Unmount(r.linkDir); err != nil {
		return err
	}

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

// Root returns the fsRoot itself as the root directory.
func (r *fsRoot) Root() (fs.Node, error) {
	return r, nil
}

// Attr helps implement fs.Node
func (r *fsRoot) Attr(ctx context.Context, a *fuse.Attr) error {
	*a = fuse.Attr{
		Mode: 0555 | os.ModeDir,
	}
	return nil
}

// Lookup helps implement fs.NodeRequestLookuper. If the requested file isn't
// the README, it returns a node which is a symbolic link to a socket which
// provides connectivity to a remote instance.  The instance which is connected
// to is determined by req.Name.
func (r *fsRoot) Lookup(_ context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	if req.Name == "README" {
		return readme{}, nil
	}
	instance := req.Name
	r.sockLock.Lock()
	defer r.sockLock.Unlock()

	if ret, ok := r.links[instance]; ok {
		return ret, nil
	}

	path := filepath.Join(r.tmpDir, instance)
	os.RemoveAll(path) // Best effort; the following will fail if this does.
	linkpath := path

	// MySQL expects a Unix domain socket at the symlinked path whereas Postgres expects
	// a socket named ".s.PGSQL.5432" in the directory given by the database path.
	// Look up instance database version to determine the correct socket path.
	// Client is nil in unit tests.
	if r.client != nil {
		version, err := r.client.InstanceVersion(instance)
		if err != nil {
			return nil, fuse.ENOENT
		}
		if strings.HasPrefix(strings.ToLower(version), "postgres") {
			if err := os.MkdirAll(path, 0755); err != nil {
				return nil, fuse.EIO
			}
			path = filepath.Join(linkpath, ".s.PGSQL.5432")
		}
	}

	sock, err := net.Listen("unix", path)
	if err != nil {
		logging.Errorf("couldn't listen at %q: %v", path, err)
		return nil, fuse.EEXIST
	}
	if err := os.Chmod(path, 0777|os.ModeSocket); err != nil {
		logging.Errorf("couldn't update permissions for socket file %q: %v; other users may be unable to connect", path, err)
	}

	go r.listenerLifecycle(sock, instance, path)

	ret := symlink(linkpath)
	r.links[instance] = ret
	// TODO(chowski): memory leak when listeners exit on their own via removeListener.
	r.closers = append(r.closers, sock)

	return ret, nil
}

// removeListener marks that a Listener for an instance has exited and is no
// longer serving new connections.
func (r *fsRoot) removeListener(instance, path string) {
	r.sockLock.Lock()
	v, ok := r.links[instance]
	if ok && string(v) == path {
		delete(r.links, instance)
	} else {
		logging.Errorf("Removing a listener for %q at %q which was already replaced", instance, path)
	}
	r.sockLock.Unlock()
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

// ReadDirAll returns a list of files contained in the root directory.
// It contains a README file which explains how to use the directory.
// In addition, there will be a file for each instance to which the
// proxy is actively connected.
func (r *fsRoot) ReadDirAll(context.Context) ([]fuse.Dirent, error) {
	ret := []fuse.Dirent{
		{Name: "README", Type: fuse.DT_File},
	}

	for _, v := range r.connset.IDs() {
		ret = append(ret, fuse.Dirent{Name: v, Type: fuse.DT_Socket})
	}

	return ret, nil
}

// readme implements the REAME file found in the mounted folder. It is a
// static read-only text file.
type readme struct{}

var _ interface {
	fs.Node
	fs.HandleReadAller
} = readme{}

const readmeText = `
When programs attempt to open files in this directory, a remote connection to
the Cloud SQL instance of the same name will be established.

That is, running :

	mysql -u root -S "/path/to/this/directory/project:instance-2"

will open a new connection to project:instance-2, given you have the correct
permissions.

Listing the contents of this directory will show all instances with active
connections.
`

// Attr helps implement fs.Node.
func (readme) Attr(ctx context.Context, a *fuse.Attr) error {
	*a = fuse.Attr{
		Mode: 0444,
		Size: uint64(len(readmeText)),
	}
	return nil
}

// ReadAll helps implement fs.HandleReadAller.
func (readme) ReadAll(context.Context) ([]byte, error) {
	return []byte(readmeText), nil
}

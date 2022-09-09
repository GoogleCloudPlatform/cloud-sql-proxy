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

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// UnixAddress is defined as a function to distinguish between Linux-based
// implementations where the dir and inst and simply joins, and Windows-based
// implementations where the inst must be further altered.
func UnixAddress(dir, inst string) string {
	return filepath.Join(dir, inst)
}

type socketSymlink struct {
	socket  *socketMount
	symlink *symlink
}

func configureFUSE(c *Client, conf *Config) (*Client, error) {
	if _, err := os.Stat(conf.FUSEDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(conf.FUSETempDir, 0777); err != nil {
		return nil, err
	}
	c.fuseDir = conf.FUSEDir
	c.fuseTempDir = conf.FUSETempDir
	c.fuseSockets = map[string]socketSymlink{}
	return c, nil
}

// socketMount is a tcp/unix socket that listens for a Cloud SQL instance.
type socketMount struct {
	inst     string
	listener net.Listener
	dialOpts []cloudsqlconn.DialOption
}

// Client proxies connections from a local client to the remote server side
// proxy for multiple Cloud SQL instances.
type Client struct {
	// connCount tracks the number of all open connections from the Client to
	// all Cloud SQL instances.
	connCount uint64

	// maxConns is the maximum number of allowed connections tracked by
	// connCount. If not set, there is no limit.
	maxConns uint64

	dialer cloudsql.Dialer

	// mnts is a list of all mounted sockets for this client
	mnts []*socketMount

	// waitOnClose is the maximum duration to wait for open connections to close
	// when shutting down.
	waitOnClose time.Duration

	logger cloudsql.Logger

	// fuseDir specifies the directory where a FUSE server is mounted. The value
	// is empty if FUSE is not enabled. The directory holds symlinks to Unix
	// domain sockets in the fuseTmpDir.
	fuseDir     string
	fuseTempDir string
	// fuseMu protects access to fuseSockets.
	fuseMu sync.Mutex
	// fuseSockets is a map of instance connection name to socketMount and
	// symlink.
	fuseSockets  map[string]socketSymlink
	fuseServerMu sync.Mutex
	fuseServer   *fuse.Server
	fuseWg       sync.WaitGroup

	// Inode adds support for FUSE operations.
	fs.Inode
}

// Readdir returns a list of all active Unix sockets in addition to the README.
func (c *Client) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries := []fuse.DirEntry{
		{Name: "README", Mode: 0555 | fuse.S_IFREG},
	}
	var active []string
	c.fuseMu.Lock()
	for k := range c.fuseSockets {
		active = append(active, k)
	}
	c.fuseMu.Unlock()

	for _, a := range active {
		entries = append(entries, fuse.DirEntry{
			Name: a,
			Mode: 0777 | syscall.S_IFSOCK,
		})
	}
	return fs.NewListDirStream(entries), fs.OK
}

// Lookup implements the fs.NodeLookuper interface and returns an index node
// (inode) for a symlink that points to a Unix domain socket. The Unix domain
// socket is connected to the requested Cloud SQL instance. Lookup returns a
// symlink (instead of the socket itself) so that multiple callers all use the
// same Unix socket.
func (c *Client) Lookup(ctx context.Context, instance string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if instance == "README" {
		return c.NewInode(ctx, &readme{}, fs.StableAttr{}), fs.OK
	}

	if _, err := parseConnName(instance); err != nil {
		return nil, syscall.ENOENT
	}

	c.fuseMu.Lock()
	defer c.fuseMu.Unlock()
	if l, ok := c.fuseSockets[instance]; ok {
		return l.symlink.EmbeddedInode(), fs.OK
	}

	version, err := c.dialer.EngineVersion(ctx, instance)
	if err != nil {
		c.logger.Errorf("could not resolve version for %q: %v", instance, err)
		return nil, syscall.ENOENT
	}

	s, err := newSocketMount(
		ctx, &Config{UnixSocket: c.fuseTempDir},
		nil, InstanceConnConfig{Name: instance}, version,
	)
	if err != nil {
		c.logger.Errorf("could not create socket for %q: %v", instance, err)
		return nil, syscall.ENOENT
	}

	c.fuseWg.Add(1)
	go func() {
		defer c.fuseWg.Done()
		sErr := c.serveSocketMount(ctx, s)
		if sErr != nil {
			c.fuseMu.Lock()
			delete(c.fuseSockets, instance)
			c.fuseMu.Unlock()
		}
	}()

	// Return a symlink that points to the actual Unix socket within the
	// temporary directory. For Postgres, return a symlink that points to the
	// directory which holds the ".s.PGSQL.5432" Unix socket.
	sl := &symlink{path: filepath.Join(c.fuseTempDir, instance)}
	c.fuseSockets[instance] = socketSymlink{
		socket:  s,
		symlink: sl,
	}
	return c.NewInode(ctx, sl, fs.StableAttr{
		Mode: 0777 | fuse.S_IFLNK},
	), fs.OK
}

func (c *Client) serveFuse(ctx context.Context, notify func()) error {
	srv, err := fs.Mount(c.fuseDir, c, &fs.Options{
		MountOptions: fuse.MountOptions{AllowOther: true},
	})
	if err != nil {
		return fmt.Errorf("FUSE mount failed: %q: %v", c.fuseDir, err)
	}
	c.fuseServerMu.Lock()
	c.fuseServer = srv
	c.fuseServerMu.Unlock()
	notify()
	<-ctx.Done()
	return ctx.Err()
}

func (c *Client) fuseMounts() []*socketMount {
	var mnts []*socketMount
	c.fuseMu.Lock()
	for _, m := range c.fuseSockets {
		mnts = append(mnts, m.socket)
	}
	c.fuseMu.Unlock()
	return mnts
}

// TODO add a stub to windows
func (c *Client) unmountFUSEMounts(mErr MultiErr) MultiErr {
	c.fuseServerMu.Lock()
	if err := c.fuseServer.Unmount(); err != nil {
		mErr = append(mErr, err)
	}
	c.fuseServerMu.Unlock()
	return mErr
}

func (c *Client) waitForFUSEMounts() { c.fuseWg.Wait() }

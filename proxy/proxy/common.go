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

// Package proxy implements client and server code for proxying an unsecure connection over SSL.
package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"log"
)

type dbgConn struct {
	net.Conn
}

func (d dbgConn) Write(b []byte) (int, error) {
	x, y := d.Conn.Write(b)
	log.Printf("write(%q) => (%v, %v)", b, x, y)
	return x, y
}

func (d dbgConn) Read(b []byte) (int, error) {
	x, y := d.Conn.Read(b)
	log.Printf("read: (%v, %v) => %q", x, y, b[:x])
	return x, y
}

func (d dbgConn) Close() error {
	err := d.Conn.Close()
	log.Printf("close: %v", err)
	return err
}

func copyThenClose(dbg string, dst io.WriteCloser, src io.ReadCloser) {
	if _, err := io.Copy(dst, src); err != nil {
		log.Printf("%s: %v", dbg, err)
	}
	src.Close()
	dst.Close()
}

// NewConnSet initializes a new ConnSet and returns it.
func NewConnSet() *ConnSet {
	return &ConnSet{m: make(map[string][]net.Conn)}
}

// A ConnSet tracks net.Conns associated with a provided ID.
type ConnSet struct {
	sync.RWMutex
	m map[string][]net.Conn
}

// String returns a debug string for the ConnSet.
func (c *ConnSet) String() string {
	var b bytes.Buffer

	c.RLock()
	for id, conns := range c.m {
		fmt.Fprintf(&b, "ID %s:", id)
		for i, c := range conns {
			fmt.Fprintf(&b, "\n\t%d: %v", i, c)
		}
	}
	c.RUnlock()

	return b.String()
}

// Add saves the provided conn and associates it with the given string
// identifier.
func (c *ConnSet) Add(id string, conn net.Conn) {
	c.Lock()
	c.m[id] = append(c.m[id], conn)
	log.Printf("ConnSet.Add(%v, %v)", id, conn)
	c.Unlock()
}

// IDs returns a slice of all identifiers which still have active connections.
func (c *ConnSet) IDs() []string {
	ret := make([]string, 0, len(c.m))

	c.RLock()
	for k := range c.m {
		ret = append(ret, k)
	}
	c.RUnlock()

	return ret
}

// Conns returns all active connections associated with the provided ids.
func (c *ConnSet) Conns(ids ...string) []net.Conn {
	var ret []net.Conn

	c.RLock()
	for _, id := range ids {
		ret = append(ret, c.m[id]...)
	}
	c.RUnlock()

	return ret
}

// Remove undoes an Add operation to have the set forget about a conn. Do not
// Remove an id/conn pair more than it has been Added.
func (c *ConnSet) Remove(id string, conn net.Conn) error {
	c.Lock()
	defer c.Unlock()

	pos := -1
	conns := c.m[id]
	for i, cc := range conns {
		if cc == conn {
			pos = i
			break
		}
	}

	log.Printf("ConnSet.Remove(%v, %v); pos=%d", id, conn, pos)
	if pos == -1 {
		return fmt.Errorf("couldn't find connection %v for id %s", conn, id)
	}

	if len(conns) == 1 {
		delete(c.m, id)
	} else {
		c.m[id] = append(conns[:pos], conns[pos+1:]...)
	}

	return nil
}

// Close closes every net.Conn contained in the set.
func (c *ConnSet) Close() error {
	var errs bytes.Buffer

	c.Lock()
	for id, conns := range c.m {
		for _, c := range conns {
			if err := c.Close(); err != nil {
				fmt.Fprintf(&errs, "%s close error: %v\n", id, err)
			}
		}
	}
	c.Unlock()

	if errs.Len() == 0 {
		return nil
	}

	return errors.New(errs.String())
}

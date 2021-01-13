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

// This file contains tests for common.go

package proxy

import (
	"context"
	"io"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	c1 = newDummyConn(nil, nil, nil)
	c2 = newDummyConn(nil, nil, nil)
	c3 = newDummyConn(nil, nil, nil)
)

// dummyConn is a fake network loopback connection between a io.ReadCloser and io.WriteCloser pair.
// These pairs are typically created using a pair of io.Pipe()'s with their ends bridged into two dummyConn's.
type dummyConn struct {
	IdleTrackingConn
	sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	in           io.ReadCloser
	out          io.WriteCloser
	lastActivity time.Time
	closed       bool
}

func newDummyConn(parent context.Context, in io.ReadCloser, out io.WriteCloser) *dummyConn {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &dummyConn{
		ctx:    ctx,
		cancel: cancel,
		in:     in,
		out:    out,
	}
}

func (c *dummyConn) Read(b []byte) (n int, err error) {
	complete := make(chan bool)
	go func() {
		if c.in == nil {
			n = 0
			err = io.EOF
		} else {
			n, err = c.in.Read(b)
		}
		complete <- true
	}()

	c.Lock()
	c.lastActivity = time.Now()
	c.Unlock()

	select {
	case <-complete:
		return n, err
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	}
}

func (c *dummyConn) Write(b []byte) (n int, err error) {
	complete := make(chan bool)
	go func() {
		if c.out == nil {
			n = 0
			err = io.EOF
		} else {
			n, err = c.out.Write(b)
		}
		complete <- true
	}()

	c.Lock()
	c.lastActivity = time.Now()
	c.Unlock()

	select {
	case <-complete:
		return n, err
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	}
}

func (c *dummyConn) idleDuration() time.Duration {
	c.Lock()
	defer c.Unlock()
	return time.Since(c.lastActivity)
}

func (c *dummyConn) Close() error {
	c.cancel()
	if c.in != nil {
		if err := c.in.Close(); err != nil {
			return err
		}
	}
	if c.out != nil {
		if err := c.out.Close(); err != nil {
			return err
		}
	}
	c.closed = true
	return nil
}

func (c *dummyConn) LocalAddr() net.Addr {
	return &dummyAddr{}
}

type dummyAddr struct{}

func (a *dummyAddr) Network() string {
	return "dummy"
}

func (a *dummyAddr) String() string {
	return "fake.address"
}

func TestConnSetAdd(t *testing.T) {
	s := NewConnSet()

	s.Add("a", c1)
	aSlice := []string{"a"}
	if !reflect.DeepEqual(s.IDs(), aSlice) {
		t.Fatalf("got %v, want %v", s.IDs(), aSlice)
	}

	s.Add("a", c2)
	if !reflect.DeepEqual(s.IDs(), aSlice) {
		t.Fatalf("got %v, want %v", s.IDs(), aSlice)
	}

	s.Add("b", c3)
	ids := s.IDs()
	if len(ids) != 2 {
		t.Fatalf("got %d ids, wanted 2", len(ids))
	}
	ok := ids[0] == "a" && ids[1] == "b" ||
		ids[1] == "a" && ids[0] == "b"

	if !ok {
		t.Fatalf(`got %v, want only "a" and "b"`, ids)
	}
}

func TestConnSetRemove(t *testing.T) {
	s := NewConnSet()

	s.Add("a", c1)
	s.Add("a", c2)
	s.Add("b", c3)

	s.Remove("b", c3)
	if got := s.Conns("b"); got != nil {
		t.Fatalf("got %v, want nil", got)
	}

	aSlice := []string{"a"}
	if !reflect.DeepEqual(s.IDs(), aSlice) {
		t.Fatalf("got %v, want %v", s.IDs(), aSlice)
	}

	s.Remove("a", c1)
	if !reflect.DeepEqual(s.IDs(), aSlice) {
		t.Fatalf("got %v, want %v", s.IDs(), aSlice)
	}

	s.Remove("a", c2)
	if len(s.IDs()) != 0 {
		t.Fatalf("got %v, want empty set", s.IDs())
	}
}

func TestConns(t *testing.T) {
	s := NewConnSet()

	s.Add("a", c1)
	s.Add("a", c2)
	s.Add("b", c3)

	got := s.Conns("b")
	if !reflect.DeepEqual(got, []IdleTrackingConn{c3}) {
		t.Fatalf("got %v, wanted only %v", got, c3)
	}

	looking := map[net.Conn]bool{
		c1: true,
		c2: true,
		c3: true,
	}

	for _, v := range s.Conns("a", "b") {
		if _, ok := looking[v]; !ok {
			t.Errorf("got unexpected conn %v", v)
		}
		delete(looking, v)
	}
	if len(looking) != 0 {
		t.Fatalf("didn't find %v in list of Conns", looking)
	}
}

func TestConnSetCloseIdle(t *testing.T) {
	s := NewConnSet()

	a := newDummyConn(nil, nil, nil)
	b := newDummyConn(nil, nil, nil)

	// Connection 'a' is idle, and 'b' is active.
	a.lastActivity = time.Now().Add(-10 * time.Second)
	b.lastActivity = time.Now()

	s.Add("c", a)
	s.Add("c", b)

	if err := s.CloseIdle(5 * time.Second); err != nil {
		t.Errorf("failed to close idle connections: %v", err)
	}

	if connA, ok := a.IdleTrackingConn.(*dummyConn); ok {
		if !connA.closed {
			t.Errorf("connection a should be marked as idle")

		}
	}

	if connB, ok := b.IdleTrackingConn.(*dummyConn); ok {
		if connB.closed {
			t.Errorf("connection b was incorrectly marked as idle")
		}
	}
}

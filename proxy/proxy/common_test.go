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
	"net"
	"reflect"
	"testing"
)

var c1, c2, c3 = &dummyConn{}, &dummyConn{}, &dummyConn{}

type dummyConn struct{ net.Conn }

func (c dummyConn) Close() error {
	return nil
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
	if !reflect.DeepEqual(got, []net.Conn{c3}) {
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

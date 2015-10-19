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

package main

// This file contains code for supporting local sockets for the Cloud SQL Proxy.

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"proxy/proxy"

	"log"
)

// WatchInstances handles the lifecycle of local sockets used for proxying
// local connections.  Values received from the updates channel are
// interpretted as a comma-separated list of instances.  The set of sockets in
// 'dir' is the union of 'instances' and the most recent list from 'updates'.
func WatchInstances(dir string, instances []string, updates <-chan string) (<-chan proxy.Conn, error) {
	ch := make(chan proxy.Conn, 1)

	// Instances specified statically (e.g. as flags to the binary) will always
	// be available. They are ignored if also returned by the GCE metadata since
	// the socket will already be open.
	staticInstances := make(map[string]net.Listener, len(instances))
	for _, v := range instances {
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		l, err := listenInstance(ch, dir, v)
		if err != nil {
			return nil, err
		}
		staticInstances[v] = l
	}

	if updates != nil {
		go watchInstancesLoop(ch, dir, updates, staticInstances)
	}
	return ch, nil
}

func watchInstancesLoop(dst chan<- proxy.Conn, dir string, updates <-chan string, static map[string]net.Listener) {
	dynamicInstances := make(map[string]net.Listener)
	for instances := range updates {
		stillOpen := make(map[string]net.Listener)

		list := strings.Split(instances, ",")
		for _, instance := range list {
			if len(instance) == 0 {
				continue
			}
			// If the instance is specified in the static list don't do anything:
			// it's already open and should stay open forever.
			if _, ok := static[instance]; ok {
				continue
			}

			if l, ok := dynamicInstances[instance]; ok {
				delete(dynamicInstances, instance)
				stillOpen[instance] = l
				continue
			}

			l, err := listenInstance(dst, dir, instance)
			if err != nil {
				log.Printf("Couldn't open socket for %q: %v", instance, err)
				continue
			}
			stillOpen[instance] = l
		}

		// Any instance in dynamicInstances was not in the most recent metadata
		// update. Clean up those instances' sockets by closing them; note that
		// this does not affect any existing connections instance.
		for instance, listener := range dynamicInstances {
			log.Printf("Closing socket for instance %v", instance)
			listener.Close()
		}

		dynamicInstances = stillOpen
	}

	for _, v := range static {
		if err := v.Close(); err != nil {
			log.Printf("Error closing %q: %v", v.Addr(), err)
		}
	}
	for _, v := range dynamicInstances {
		if err := v.Close(); err != nil {
			log.Printf("Error closing %q: %v", v.Addr(), err)
		}
	}
}

func remove(path string) {
	err := os.Remove(path)
	log.Printf("Remove(%q) error: %v", path, err)
}

// listenInstance starts listening on a new unix socket in dir to connect to the
// specified instance. New connections to this socket are sent to dst.
func listenInstance(dst chan<- proxy.Conn, dir, instance string) (net.Listener, error) {
	log.Printf("listenInstance: %q", instance)

	var path string
	var l net.Listener
	if eq := strings.Index(instance, "="); eq != -1 {
		spl := strings.SplitN(instance[eq+1:], ":", 2)
		if len(spl) == 1 {
			return nil, fmt.Errorf("invalid format in %q; expected 'project:instance=tcp:port'", instance)
		}

		instance = instance[:eq]

		var err error
		if l, err = net.Listen(spl[0], ":"+spl[1]); err != nil {
			return nil, err
		}
		path = "localhost:" + spl[1]
	} else {
		path = filepath.Join(dir, instance)
		remove(path)
		var err error
		if l, err = net.Listen("unix", path); err != nil {
			return nil, err
		}
		if err := os.Chmod(path, 0777|os.ModeSocket); err != nil {
			log.Printf("couldn't update permissions for socket file %q: %v; other users may not be unable to connect", path, err)
		}
	}

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				log.Printf("Error in accept for %q on %v: %v", instance, path, err)
				l.Close()
				return
			}
			log.Printf("Got a connection for %q", instance)
			dst <- proxy.Conn{instance, c}
		}
	}()

	log.Printf("Open socket for %q at %q", instance, path)
	return l, nil
}

// Check verifies that the dir parameter is set and that either 'fuse' is true or
// at least one of instances/instancesSrc is set, but not both.
func Check(dir string, fuse bool, instances []string, instancesSrc string) error {
	switch {
	case dir == "":
		return errors.New("must set -dir")
	case !fuse:
		if len(instances) == 0 && instancesSrc == "" {
			return errors.New("must specify -fuse, -instances, or -instances_metadata")
		}
	case len(instances) != 0 || instancesSrc != "":
		return errors.New("-fuse is not compatible with -instances or -instances_metadata")
	}
	return nil
}

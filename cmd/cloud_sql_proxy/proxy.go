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
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/fuse"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/util"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// WatchInstances handles the lifecycle of local sockets used for proxying
// local connections.  Values received from the updates channel are
// interpretted as a comma-separated list of instances.  The set of sockets in
// 'dir' is the union of 'instances' and the most recent list from 'updates'.
func WatchInstances(dir string, cfgs []instanceConfig, updates <-chan string, cl *http.Client) (<-chan proxy.Conn, error) {
	ch := make(chan proxy.Conn, 1)

	// Instances specified statically (e.g. as flags to the binary) will always
	// be available. They are ignored if also returned by the GCE metadata since
	// the socket will already be open.
	staticInstances := make(map[string]net.Listener, len(cfgs))
	for _, v := range cfgs {
		l, err := listenInstance(ch, v)
		if err != nil {
			return nil, err
		}
		staticInstances[v.Instance] = l
	}

	if updates != nil {
		go watchInstancesLoop(dir, ch, updates, staticInstances, cl)
	}
	return ch, nil
}

func watchInstancesLoop(dir string, dst chan<- proxy.Conn, updates <-chan string, static map[string]net.Listener, cl *http.Client) {
	dynamicInstances := make(map[string]net.Listener)
	for instances := range updates {
		list, err := parseInstanceConfigs(dir, strings.Split(instances, ","), cl)
		if err != nil {
			logging.Errorf("%v", err)
		}

		stillOpen := make(map[string]net.Listener)
		for _, cfg := range list {
			instance := cfg.Instance

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

			l, err := listenInstance(dst, cfg)
			if err != nil {
				logging.Errorf("Couldn't open socket for %q: %v", instance, err)
				continue
			}
			stillOpen[instance] = l
		}

		// Any instance in dynamicInstances was not in the most recent metadata
		// update. Clean up those instances' sockets by closing them; note that
		// this does not affect any existing connections instance.
		for instance, listener := range dynamicInstances {
			logging.Infof("Closing socket for instance %v", instance)
			listener.Close()
		}

		dynamicInstances = stillOpen
	}

	for _, v := range static {
		if err := v.Close(); err != nil {
			logging.Errorf("Error closing %q: %v", v.Addr(), err)
		}
	}
	for _, v := range dynamicInstances {
		if err := v.Close(); err != nil {
			logging.Errorf("Error closing %q: %v", v.Addr(), err)
		}
	}
}

func remove(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logging.Infof("Remove(%q) error: %v", path, err)
	}
}

// listenInstance starts listening on a new unix socket in dir to connect to the
// specified instance. New connections to this socket are sent to dst.
func listenInstance(dst chan<- proxy.Conn, cfg instanceConfig) (net.Listener, error) {
	unix := cfg.Network == "unix"
	if unix {
		remove(cfg.Address)
	}
	l, err := net.Listen(cfg.Network, cfg.Address)
	if err != nil {
		return nil, err
	}
	if unix {
		if err := os.Chmod(cfg.Address, 0777|os.ModeSocket); err != nil {
			logging.Errorf("couldn't update permissions for socket file %q: %v; other users may not be unable to connect", cfg.Address, err)
		}
	}

	go func() {
		for {
			start := time.Now()
			c, err := l.Accept()
			if err != nil {
				logging.Errorf("Error in accept for %q on %v: %v", cfg, cfg.Address, err)
				if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
					d := 10*time.Millisecond - time.Since(start)
					if d > 0 {
						time.Sleep(d)
					}
					continue
				}
				l.Close()
				return
			}
			logging.Verbosef("New connection for %q", cfg.Instance)

			switch clientConn := c.(type) {
			case *net.TCPConn:
				clientConn.SetKeepAlive(true)
				clientConn.SetKeepAlivePeriod(1 * time.Minute)

			}
			dst <- proxy.Conn{cfg.Instance, c}
		}
	}()

	logging.Infof("Listening on %s for %s", cfg.Address, cfg.Instance)
	return l, nil
}

type instanceConfig struct {
	Instance         string
	Network, Address string
}

// loopbackForNet maps a network (e.g. tcp6) to the loopback address for that
// network. It is updated during the initialization of validNets to include a
// valid loopback address for "tcp".
var loopbackForNet = map[string]string{
	"tcp4": "127.0.0.1",
	"tcp6": "[::1]",
}

// validNets tracks the networks that are valid for this platform and machine.
var validNets = func() map[string]bool {
	m := map[string]bool{
		"unix": runtime.GOOS != "windows",
	}

	anyTCP := false
	for _, n := range []string{"tcp4", "tcp6"} {
		addr, ok := loopbackForNet[n]
		if !ok {
			// This is effectively a compile-time error.
			panic(fmt.Sprintf("no loopback address found for %v", n))
		}
		// Open any port to see if the net is valid.
		x, err := net.Listen(n, addr+":0")
		if err != nil {
			// Error is too verbose to be useful.
			continue
		}
		x.Close()
		m[n] = true

		if !anyTCP {
			anyTCP = true
			// Set the loopback value for generic tcp if it hasn't already been
			// set. (If both tcp4/tcp6 are supported the first one in the list
			// (tcp4's 127.0.0.1) is used.
			loopbackForNet["tcp"] = addr
		}
	}
	if anyTCP {
		m["tcp"] = true
	}
	return m
}()

func parseInstanceConfig(dir, instance string, cl *http.Client) (instanceConfig, error) {
	var ret instanceConfig
	eq := strings.Index(instance, "=")
	if eq != -1 {
		spl := strings.SplitN(instance[eq+1:], ":", 3)
		ret.Instance = instance[:eq]

		switch len(spl) {
		default:
			return ret, fmt.Errorf("invalid %q: expected 'project:instance=tcp:port'", instance)
		case 2:
			// No "host" part of the address. Be safe and assume that they want a
			// loopback address.
			ret.Network = spl[0]
			addr, ok := loopbackForNet[spl[0]]
			if !ok {
				return ret, fmt.Errorf("invalid %q: unrecognized network %v", instance, spl[0])
			}
			ret.Address = fmt.Sprintf("%s:%s", addr, spl[1])
		case 3:
			// User provided a host and port; use that.
			ret.Network = spl[0]
			ret.Address = fmt.Sprintf("%s:%s", spl[1], spl[2])
		}
	} else {
		sql, err := sqladmin.New(cl)
		if err != nil {
			return instanceConfig{}, err
		}
		sql.BasePath = *host
		ret.Instance = instance
		// Default to unix socket.
		ret.Network = "unix"

		proj, _, name := util.SplitName(instance)
		if proj == "" || name == "" {
			return instanceConfig{}, fmt.Errorf("invalid instance name: must be in the form `project:region:instance-name`; invalid name was %q", instance)
		}
		// We allow people to omit the region due to historical reasons. It'll
		// fail later in the code if this isn't allowed, so just assume it's
		// allowed until we actually need the region in this API call.
		in, err := sql.Instances.Get(proj, name).Do()
		if err != nil {
			return instanceConfig{}, err
		}
		if strings.HasPrefix(strings.ToLower(in.DatabaseVersion), "postgres") {
			path := filepath.Join(dir, instance)
			if err := os.MkdirAll(path, 0755); err != nil {
				return instanceConfig{}, err
			}
			ret.Address = filepath.Join(path, ".s.PGSQL.5432")
		} else {
			ret.Address = filepath.Join(dir, instance)
		}
	}

	if !validNets[ret.Network] {
		return ret, fmt.Errorf("invalid %q: unsupported network: %v", instance, ret.Network)
	}
	return ret, nil
}

// parseInstanceConfigs calls parseInstanceConfig for each instance in the
// provided slice, collecting errors along the way. There may be valid
// instanceConfigs returned even if there's an error.
func parseInstanceConfigs(dir string, instances []string, cl *http.Client) ([]instanceConfig, error) {
	errs := new(bytes.Buffer)
	var cfg []instanceConfig
	for _, v := range instances {
		if v == "" {
			continue
		}
		if c, err := parseInstanceConfig(dir, v, cl); err != nil {
			fmt.Fprintf(errs, "\n\t%v", err)
		} else {
			cfg = append(cfg, c)
		}
	}

	var err error
	if errs.Len() > 0 {
		err = fmt.Errorf("errors parsing config:%s", errs)
	}
	return cfg, err
}

// CreateInstanceConfigs verifies that the parameters passed to it are valid
// for the proxy for the platform and system and then returns a slice of valid
// instanceConfig.
func CreateInstanceConfigs(dir string, useFuse bool, instances []string, instancesSrc string, cl *http.Client) ([]instanceConfig, error) {
	if useFuse && !fuse.Supported() {
		return nil, errors.New("FUSE not supported on this system")
	}

	cfgs, err := parseInstanceConfigs(dir, instances, cl)
	if err != nil {
		logging.Errorf("%v", err)
	}

	if dir == "" {
		// Reasons to set '-dir':
		//    - Using -fuse
		//    - Using the metadata to get a list of instances
		//    - Having an instance that uses a 'unix' network
		if useFuse {
			return nil, errors.New("must set -dir because -fuse was set")
		} else if instancesSrc != "" {
			return nil, errors.New("must set -dir because -instances_metadata was set")
		} else {
			for _, v := range cfgs {
				if v.Network == "unix" {
					return nil, fmt.Errorf("must set -dir: using a unix socket for %v", v.Instance)
				}
			}
		}
		// Otherwise it's safe to not set -dir
	}

	if useFuse {
		if len(instances) != 0 || instancesSrc != "" {
			return nil, errors.New("-fuse is not compatible with -projects, -instances, or -instances_metadata")
		}
		return nil, nil
	}
	// FUSE disabled.
	if len(instances) == 0 && instancesSrc == "" {
		// Failure to specifying instance can be caused by following reasons.
		// 1. not enough information is provided by flags
		// 2. failed to invoke gcloud
		var flags string
		if fuse.Supported() {
			flags = "-projects, -fuse, -instances or -instances_metadata"
		} else {
			flags = "-projects, -instances or -instances_metadata"
		}

		errStr := fmt.Sprintf("no instance selected because none of %s is specified", flags)
		return nil, errors.New(errStr)
	}
	return cfgs, nil
}

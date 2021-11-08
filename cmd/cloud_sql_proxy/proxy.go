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

// This file contains code for supporting local sockets for the Cloud SQL Auth proxy.

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

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/logging"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/proxy/fuse"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/proxy/proxy"
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
		// All instances were legal when we started, so we pass false below to ensure we don't skip them
		// later if they became unhealthy for some reason; this would be a serious enough problem.
		list, err := parseInstanceConfigs(dir, strings.Split(instances, ","), cl, false)
		if err != nil {
			logging.Errorf("%v", err)
			// If we do not have a valid list of instances, skip this update
			continue
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
	"tcp6": "::1",
}

// validNets tracks the networks that are valid for this platform and machine.
var validNets = func() map[string]bool {
	m := map[string]bool{
		"unix": runtime.GOOS != "windows",
	}

	anyTCP := false
	for _, n := range []string{"tcp4", "tcp6"} {
		host, ok := loopbackForNet[n]
		if !ok {
			// This is effectively a compile-time error.
			panic(fmt.Sprintf("no loopback address found for %v", n))
		}
		// Open any port to see if the net is valid.
		x, err := net.Listen(n, net.JoinHostPort(host, "0"))
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
			loopbackForNet["tcp"] = host
		}
	}
	if anyTCP {
		m["tcp"] = true
	}
	return m
}()

func parseInstanceConfig(dir, instance string, cl *http.Client) (instanceConfig, error) {
	var ret instanceConfig
	proj, region, name, args, err := proxy.ParseInstanceConnectionName(instance)
	if err != nil {
		return instanceConfig{}, err
	}
	ret.Instance = args[0]
	regionName := fmt.Sprintf("%s~%s", region, name)
	if len(args) == 1 {
		// Default to listening via unix socket in specified directory
		ret.Network = "unix"
		ret.Address = filepath.Join(dir, instance)
	} else {
		// Parse the instance options if present.
		opts := strings.SplitN(args[1], ":", 2)
		if len(opts) != 2 {
			return instanceConfig{}, fmt.Errorf("invalid instance options: must be in the form `unix:/path/to/socket`, `tcp:port`, `tcp:host:port`; invalid option was %q", strings.Join(opts, ":"))
		}
		ret.Network = opts[0]
		var err error
		if ret.Network == "unix" {
			if strings.HasPrefix(opts[1], "/") {
				ret.Address = opts[1] // Root path.
			} else {
				ret.Address = filepath.Join(dir, opts[1])
			}
		} else {
			ret.Address, err = parseTCPOpts(opts[0], opts[1])
		}
		if err != nil {
			return instanceConfig{}, err
		}
	}

	// Use the SQL Admin API to verify compatibility with the instance.
	sql, err := sqladmin.New(cl)
	if err != nil {
		return instanceConfig{}, err
	}
	if *host != "" {
		sql.BasePath = *host
	}
	inst, err := sql.Connect.Get(proj, regionName).Do()
	if err != nil {
		return instanceConfig{}, err
	}
	if inst.BackendType == "FIRST_GEN" {
		logging.Errorf("WARNING: proxy client does not support first generation Cloud SQL instances.")
		return instanceConfig{}, fmt.Errorf("%q is a first generation instance", instance)
	}
	// Postgres instances use a special suffix on the unix socket.
	// See https://www.postgresql.org/docs/11/runtime-config-connection.html
	if ret.Network == "unix" && strings.HasPrefix(strings.ToLower(inst.DatabaseVersion), "postgres") {
		// Verify the directory exists.
		if err := os.MkdirAll(ret.Address, 0755); err != nil {
			return instanceConfig{}, err
		}
		ret.Address = filepath.Join(ret.Address, ".s.PGSQL.5432")
	}

	if !validNets[ret.Network] {
		return ret, fmt.Errorf("invalid %q: unsupported network: %v", instance, ret.Network)
	}
	return ret, nil
}

// parseTCPOpts parses the instance options when specifying tcp port options.
func parseTCPOpts(ntwk, addrOpt string) (string, error) {
	if strings.Contains(addrOpt, ":") {
		return addrOpt, nil // User provided a host and port; use that.
	}
	// No "host" part of the address. Be safe and assume that they want a loopback address.
	addr, ok := loopbackForNet[ntwk]
	if !ok {
		return "", fmt.Errorf("invalid %q:%q: unrecognized network %v", ntwk, addrOpt, ntwk)
	}
	return net.JoinHostPort(addr, addrOpt), nil
}

// parseInstanceConfigs calls parseInstanceConfig for each instance in the
// provided slice, collecting errors along the way. There may be valid
// instanceConfigs returned even if there's an error.
func parseInstanceConfigs(dir string, instances []string, cl *http.Client, skipFailedInstanceConfigs bool) ([]instanceConfig, error) {
	errs := new(bytes.Buffer)
	var cfg []instanceConfig
	for _, v := range instances {
		if v == "" {
			continue
		}
		if c, err := parseInstanceConfig(dir, v, cl); err != nil {
			if skipFailedInstanceConfigs {
				logging.Infof("There was a problem when parsing an instance configuration but ignoring due to the configuration. Error: %v", err)
			} else {
				fmt.Fprintf(errs, "\n\t%v", err)
			}

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
// instanceConfig. It is possible for the instanceConfig to be empty if no valid
// configurations were specified, however `err` will be set.
func CreateInstanceConfigs(dir string, useFuse bool, instances []string, instancesSrc string, cl *http.Client, skipFailedInstanceConfigs bool) ([]instanceConfig, error) {
	if useFuse && !fuse.Supported() {
		return nil, errors.New("FUSE not supported on this system")
	}

	cfgs, err := parseInstanceConfigs(dir, instances, cl, skipFailedInstanceConfigs)
	if err != nil {
		return nil, err
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

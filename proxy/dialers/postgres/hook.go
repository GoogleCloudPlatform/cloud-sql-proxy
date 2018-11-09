// Copyright 2017 Google Inc. All Rights Reserved.
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

// Package postgres adds a 'cloudsqlpostgres' driver to use when you want
// to access a Cloud SQL Database via the go database/sql library.
// It is a wrapper over the driver found at github.com/lib/pq.
// To use this driver, you can look at an example in
// postgres_test package in the hook_test.go file
package postgres

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net"
	"regexp"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"github.com/lib/pq"
)

func init() {
	sql.Register("cloudsqlpostgres", &Driver{})
}

type Driver struct{}

type dialer struct{}

// instanceRegexp is used to parse the addr returned by lib/pq.
// lib/pq returns the format '[project:region:instance]:port'
var instanceRegexp = regexp.MustCompile(`^\[(.+)\]:[0-9]+$`)

func (d dialer) Dial(ntw, addr string) (net.Conn, error) {
	matches := instanceRegexp.FindStringSubmatch(addr)
	if len(matches) != 2 {
		return nil, fmt.Errorf("failed to parse addr: %q. It should conform to the regular expression %q", addr, instanceRegexp)
	}
	instance := matches[1]
	return proxy.Dial(instance)
}

func (d dialer) DialTimeout(ntw, addr string, timeout time.Duration) (net.Conn, error) {
	return nil, fmt.Errorf("timeout is not currently supported for cloudsqlpostgres dialer")
}

func (d *Driver) Open(name string) (driver.Conn, error) {
	return pq.DialOpen(dialer{}, name)
}

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
	sql.Register("cloudsqlpostgres", &drv{})
}

type drv struct{}

type dialer struct{}

func (d dialer) Dial(ntw, addr string) (net.Conn, error) {
	// Drop the port number at the end (:5432) from the address
	addr = regexp.MustCompile(":[0-9]+$").ReplaceAllString(addr, "")
	// lib/pq converts the instance to the format '[project:region:instance]'
	// format, so we need to drop the first and last character before we
	// pass it to thr proxy dialer
	if len(addr) < 2 {
		return nil, fmt.Errorf("Failed to parse instance name: %q", addr)
	}
	addr = addr[1 : len(addr)-1]
	return proxy.Dial(addr)
}

func (d dialer) DialTimeout(ntw, addr string, timeout time.Duration) (net.Conn, error) {
	return nil, fmt.Errorf("Timeout is not currently supported for cloudsqlpostgres dialer")
}

func (d *drv) Open(name string) (driver.Conn, error) {
	return pq.DialOpen(dialer{}, name)
}

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

// Package mysql adds a 'cloudsql' network to use when you want to access a
// Cloud SQL Database via the mysql driver found at
// github.com/go-sql-driver/mysql. It also exposes helper functions for
// dialing.
package mysql

import (
	"database/sql"
	"errors"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"github.com/go-sql-driver/mysql"
)

func init() {
	mysql.RegisterDial("cloudsql", proxy.Dial)
}

// Dial logs into the specified Cloud SQL Instance using the given user and no
// password. To set more options, consider calling DialCfg instead.
//
// The provided instance should be in the form project-name:region:instance-name.
//
// The returned *sql.DB may be valid even if there's also an error returned
// (e.g. if there was a transient connection error).
func Dial(instance, user string) (*sql.DB, error) {
	return DialCfg(&mysql.Config{
		User: user,
		Addr: instance,
		// Set in DialCfg:
		// Net: "cloudsql",
	})
}

// DialPassword is similar to Dial, but allows you to specify a password.
//
// Note that using a password with the proxy is not necessary as long as the
// user's hostname in the mysql.user table is 'cloudsqlproxy~'. For more
// information, see:
//    https://cloud.google.com/sql/docs/sql-proxy#user
func DialPassword(instance, user, password string) (*sql.DB, error) {
	return DialCfg(&mysql.Config{
		User:   user,
		Passwd: password,
		Addr:   instance,
		// Set in DialCfg:
		// Net: "cloudsql",
	})
}

// Cfg returns the effective *mysql.Config to represent connectivity to the
// provided instance via the given user and password. The config can be
// modified and passed to DialCfg to connect. If you don't modify the returned
// config before dialing, consider using Dial or DialPassword.
func Cfg(instance, user, password string) *mysql.Config {
	return &mysql.Config{
		Addr:   instance,
		User:   user,
		Passwd: password,
		Net:    "cloudsql",
	}
}

// DialCfg opens up a SQL connection to a Cloud SQL Instance specified by the
// provided configuration. It is otherwise the same as Dial.
//
// The cfg.Addr should be the instance's connection string, in the format of:
//	      project-name:region:instance-name.
func DialCfg(cfg *mysql.Config) (*sql.DB, error) {
	if cfg.TLSConfig != "" {
		return nil, errors.New("do not specify TLS when using the Proxy")
	}

	// Copy the config so that we can modify it without feeling bad.
	c := *cfg
	c.Net = "cloudsql"
	dsn := c.FormatDSN()

	db, err := sql.Open("mysql", dsn)
	if err == nil {
		err = db.Ping()
	}
	return db, err
}

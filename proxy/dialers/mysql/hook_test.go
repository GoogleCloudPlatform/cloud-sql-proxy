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

package mysql_test

import (
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/proxy/dialers/mysql"
)

// ExampleCfg shows how to use Cloud SQL Auth proxy dialer if you must update some
// settings normally passed in the DSN such as the DBName or timeouts.
func ExampleCfg() {
	cfg := mysql.Cfg("project:region:instance-name", "user", "")
	cfg.DBName = "DB_1"
	cfg.ParseTime = true

	const timeout = 10 * time.Second
	cfg.Timeout = timeout
	cfg.ReadTimeout = timeout
	cfg.WriteTimeout = timeout

	db, err := mysql.DialCfg(cfg)
	if err != nil {
		panic("couldn't dial: " + err.Error())
	}
	// Close db after this method exits since we don't need it for the
	// connection pooling.
	defer db.Close()

	var now time.Time
	fmt.Println(db.QueryRow("SELECT NOW()").Scan(&now))
	fmt.Println(now)
}

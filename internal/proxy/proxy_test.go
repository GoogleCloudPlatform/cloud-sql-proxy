// Copyright 2022 Google LLC
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

package proxy_test

import (
	"context"
	"net"
	"testing"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
	"github.com/spf13/cobra"
)

func TestClientInitialization(t *testing.T) {
	ctx := context.Background()
	pg := newFakeCSQLInstance("proj", "region", "pg", "POSTGRES_14")
	pg2 := newFakeCSQLInstance("proj", "region", "pg2", "POSTGRES_14")
	mysql := newFakeCSQLInstance("proj", "region", "mysql", "MYSQL_8_0")
	mysql2 := newFakeCSQLInstance("proj", "region", "mysql2", "MYSQL_8_0")
	sqlserver := newFakeCSQLInstance("proj", "region", "sqlserver", "SQLSERVER_2019_STANDARD")
	sqlserver2 := newFakeCSQLInstance("proj", "region", "sqlserver2", "SQLSERVER_2019_STANDARD")
	allDB := []fakeCSQLInstance{pg, pg2, mysql, mysql2, sqlserver, sqlserver2}

	tcs := []struct {
		desc      string
		in        *proxy.Config
		wantAddrs []string
	}{
		{
			desc: "multiple instances",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg.String()},
					{Name: mysql.String()},
					{Name: sqlserver.String()},
				},
			},
			wantAddrs: []string{"127.0.0.1:5000", "127.0.0.1:5001", "127.0.0.1:5002"},
		},
		{
			desc: "with instance address",
			in: &proxy.Config{
				Addr: "1.1.1.1", // bad address, binding shouldn't happen here.
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Addr: "0.0.0.0", Name: pg.String()},
				},
			},
			wantAddrs: []string{"0.0.0.0:5000"},
		},
		{
			desc: "IPv6 support",
			in: &proxy.Config{
				Addr: "::1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg.String()},
				},
			},
			wantAddrs: []string{"[::1]:5000"},
		},
		{
			desc: "with instance port",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg.String(), Port: 6000},
				},
			},
			wantAddrs: []string{"127.0.0.1:6000"},
		},
		{
			desc: "with global port and instance port",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []proxy.InstanceConnConfig{
					{Name: pg.String()},
					{Name: mysql.String(), Port: 6000},
					{Name: sqlserver.String()},
				},
			},
			wantAddrs: []string{
				"127.0.0.1:5000",
				"127.0.0.1:6000",
				"127.0.0.1:5001",
			},
		},
		{
			desc: "with automatic port selection",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Instances: []proxy.InstanceConnConfig{
					{Name: pg.String()},
					{Name: mysql.String()},
					{Name: sqlserver.String()},
				},
			},
			wantAddrs: []string{
				"127.0.0.1:5432",
				"127.0.0.1:3306",
				"127.0.0.1:1433",
			},
		},
		{
			desc: "with incrementing automatic port selection",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Instances: []proxy.InstanceConnConfig{
					{Name: pg.String()},
					{Name: pg2.String()},
					{Name: mysql.String()},
					{Name: mysql2.String()},
					{Name: sqlserver.String()},
					{Name: sqlserver2.String()},
				},
			},
			wantAddrs: []string{
				"127.0.0.1:5432",
				"127.0.0.1:5433",
				"127.0.0.1:3306",
				"127.0.0.1:3307",
				"127.0.0.1:1433",
				"127.0.0.1:1434",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			var reqs []*request
			for _, db := range allDB {
				reqs = append(reqs,
					instanceGetSuccess(db, 1),
					createEphemeralSuccess(db, 1),
				)
			}
			cl, url, cleanup := httpClient(reqs...)
			defer cleanup()
			d, err := cloudsqlconn.NewDialer(ctx,
				cloudsqlconn.WithHTTPClient(cl),
				cloudsqlconn.WithAdminAPIEndpoint(url),
			)
			if err != nil {
				t.Fatalf("failed to initialize dialer: %v", err)
			}

			c, err := proxy.NewClient(ctx, d, &cobra.Command{}, tc.in)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}
			defer c.Close()
			for _, addr := range tc.wantAddrs {
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					t.Fatalf("want error = nil, got = %v", err)
				}
				defer conn.Close()
			}
		})
	}
}

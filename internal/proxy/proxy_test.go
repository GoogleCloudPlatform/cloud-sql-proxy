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

package proxy

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/cloudsql"
	"github.com/spf13/cobra"
)

type fakeDialer struct {
	cloudsql.Dialer
}

func (fakeDialer) Close() error {
	return nil
}

func (fakeDialer) EngineVersion(_ context.Context, inst string) (string, error) {
	switch {
	case strings.Contains(inst, "pg"):
		return "POSTGRES_14", nil
	case strings.Contains(inst, "mysql"):
		return "MYSQL_8_0", nil
	case strings.Contains(inst, "sqlserver"):
		return "SQLSERVER_2019_STANDARD", nil
	default:
		return "POSTGRES_14", nil
	}
}

func TestClientInitialization(t *testing.T) {
	ctx := context.Background()
	pg := "proj:region:pg"
	pg2 := "proj:region:pg2"
	mysql := "proj:region:mysql"
	mysql2 := "proj:region:mysql2"
	sqlserver := "proj:region:sqlserver"
	sqlserver2 := "proj:region:sqlserver2"

	tcs := []struct {
		desc      string
		in        *Config
		wantAddrs []string
	}{
		{
			desc: "multiple instances",
			in: &Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []InstanceConnConfig{
					{Name: pg},
					{Name: mysql},
					{Name: sqlserver},
				},
			},
			wantAddrs: []string{"127.0.0.1:5000", "127.0.0.1:5001", "127.0.0.1:5002"},
		},
		{
			desc: "with instance address",
			in: &Config{
				Addr: "1.1.1.1", // bad address, binding shouldn't happen here.
				Port: 5000,
				Instances: []InstanceConnConfig{
					{Addr: "0.0.0.0", Name: pg},
				},
			},
			wantAddrs: []string{"0.0.0.0:5000"},
		},
		{
			desc: "IPv6 support",
			in: &Config{
				Addr: "::1",
				Port: 5000,
				Instances: []InstanceConnConfig{
					{Name: pg},
				},
			},
			wantAddrs: []string{"[::1]:5000"},
		},
		{
			desc: "with instance port",
			in: &Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []InstanceConnConfig{
					{Name: pg, Port: 6000},
				},
			},
			wantAddrs: []string{"127.0.0.1:6000"},
		},
		{
			desc: "with global port and instance port",
			in: &Config{
				Addr: "127.0.0.1",
				Port: 5000,
				Instances: []InstanceConnConfig{
					{Name: pg},
					{Name: mysql, Port: 6000},
					{Name: sqlserver},
				},
			},
			wantAddrs: []string{
				"127.0.0.1:5000",
				"127.0.0.1:6000",
				"127.0.0.1:5001",
			},
		},
		{
			desc: "with incrementing automatic port selection",
			in: &Config{
				Addr: "127.0.0.1",
				Instances: []InstanceConnConfig{
					{Name: pg},
					{Name: pg2},
					{Name: mysql},
					{Name: mysql2},
					{Name: sqlserver},
					{Name: sqlserver2},
				},
				postgres:  10000, // override defaults to avoid port in-use errors
				mysql:     11000,
				sqlserver: 12000,
			},
			wantAddrs: []string{
				"127.0.0.1:10000",
				"127.0.0.1:10001",
				"127.0.0.1:11000",
				"127.0.0.1:11001",
				"127.0.0.1:12000",
				"127.0.0.1:12001",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c, err := NewClient(ctx, fakeDialer{}, &cobra.Command{}, tc.in)
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

func TestConfigDialerOpts(t *testing.T) {
	tcs := []struct {
		desc    string
		config  Config
		wantLen int
	}{
		{
			desc:    "when there are no options",
			config:  Config{},
			wantLen: 0,
		},
		{
			desc:    "when a token is present",
			config:  Config{Token: "my-token"},
			wantLen: 1,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			if got := tc.config.DialerOpts(); tc.wantLen != len(got) {
				t.Errorf("want len = %v, got = %v", tc.wantLen, len(got))
			}
		})
	}
}

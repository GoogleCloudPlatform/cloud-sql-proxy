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

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
)

type mockTripper struct {
}

func (m *mockTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: ioutil.NopCloser(bytes.NewReader([]byte("{}")))}, nil
}

var mockClient = &http.Client{Transport: &mockTripper{}}

func TestCreateInstanceConfigs(t *testing.T) {
	for _, v := range []struct {
		desc string
		//inputs
		dir          string
		useFuse      bool
		instances    []string
		instancesSrc string

		// We don't need to check the []instancesConfig return value, we already
		// have a TestParseInstanceConfig.
		wantErr bool

		skipFailedInstanceConfig bool
	}{
		{
			"setting -fuse and -dir",
			"dir", true, nil, "", false, false,
		}, {
			"setting -fuse",
			"", true, nil, "", true, false,
		}, {
			"setting -fuse, -dir, and -instances",
			"dir", true, []string{"proj:reg:x"}, "", true, false,
		}, {
			"setting -fuse, -dir, and -instances_metadata",
			"dir", true, nil, "md", true, false,
		}, {
			"setting -dir and -instances (unix socket)",
			"dir", false, []string{"proj:reg:x"}, "", false, false,
		}, {
			// tests for the case where invalid configs can still exist, when skipped
			"setting -dir and -instances (unix socket) w/ something invalid",
			"dir", false, []string{"proj:reg:x", "INVALID_PROJECT_STRING"}, "", false, true,
		}, {
			"Seting -instance (unix socket)",
			"", false, []string{"proj:reg:x"}, "", true, false,
		}, {
			"setting -instance (tcp socket)",
			"", false, []string{"proj:reg:x=tcp:1234"}, "", false, false,
		}, {
			"setting -instance (tcp socket) and -instances_metadata",
			"", false, []string{"proj:reg:x=tcp:1234"}, "md", true, false,
		}, {
			"setting -dir, -instance (tcp socket), and -instances_metadata",
			"dir", false, []string{"proj:reg:x=tcp:1234"}, "md", false, false,
		}, {
			"setting -dir, -instance (unix socket), and -instances_metadata",
			"dir", false, []string{"proj:reg:x"}, "md", false, false,
		}, {
			"setting -dir and -instances_metadata",
			"dir", false, nil, "md", false, false,
		}, {
			"setting -instances_metadata",
			"", false, nil, "md", true, false,
		},
	} {
		_, err := CreateInstanceConfigs(v.dir, v.useFuse, v.instances, v.instancesSrc, mockClient, v.skipFailedInstanceConfig)
		if v.wantErr {
			if err == nil {
				t.Errorf("CreateInstanceConfigs passed when %s, wanted error", v.desc)
			}
			continue
		}
		if err != nil {
			t.Errorf("CreateInstanceConfigs gave error when %s: %v", v.desc, err)
		}
	}
}

func TestParseInstanceConfig(t *testing.T) {
	// sentinel values
	var (
		anyLoopbackAddress = "<any loopback address>"
		wantErr            = instanceConfig{"<want error>", "", ""}
	)

	tcs := []struct {
		// inputs
		dir, instance string

		wantCfg instanceConfig
	}{
		{
			"/x", "domain.com:my-proj:my-reg:my-instance",
			instanceConfig{"domain.com:my-proj:my-reg:my-instance", "unix", "/x/domain.com:my-proj:my-reg:my-instance"},
		}, {
			"/x", "my-proj:my-reg:my-instance",
			instanceConfig{"my-proj:my-reg:my-instance", "unix", "/x/my-proj:my-reg:my-instance"},
		}, {
			"/x", "my-proj:my-reg:my-instance=unix:socket_name",
			instanceConfig{"my-proj:my-reg:my-instance", "unix", "/x/socket_name"},
		}, {
			"/x", "my-proj:my-reg:my-instance=unix:/my/custom/sql-socket",
			instanceConfig{"my-proj:my-reg:my-instance", "unix", "/my/custom/sql-socket"},
		}, {
			"/x", "my-proj:my-reg:my-instance=tcp:1234",
			instanceConfig{"my-proj:my-reg:my-instance", "tcp", anyLoopbackAddress},
		}, {
			"/x", "my-proj:my-reg:my-instance=tcp4:1234",
			instanceConfig{"my-proj:my-reg:my-instance", "tcp4", "127.0.0.1:1234"},
		}, {
			"/x", "my-proj:my-reg:my-instance=tcp6:1234",
			instanceConfig{"my-proj:my-reg:my-instance", "tcp6", "[::1]:1234"},
		}, {
			"/x", "my-proj:my-reg:my-instance=tcp:my-host:1111",
			instanceConfig{"my-proj:my-reg:my-instance", "tcp", "my-host:1111"},
		}, {
			"/x", "my-proj:my-reg:my-instance=",
			wantErr,
		}, {
			"/x", "my-proj:my-reg:my-instance=cool network",
			wantErr,
		}, {
			"/x", "my-proj:my-reg:my-instance=cool network:1234",
			wantErr,
		}, {
			"/x", "my-proj:my-reg:my-instance=oh:so:many:colons",
			wantErr,
		},
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("parseInstanceConfig(%q, %q)", tc.dir, tc.instance), func(t *testing.T) {
			if os.Getenv("EXPECT_IPV4_AND_IPV6") != "true" {
				// Skip ipv4 and ipv6 if they are not supported by the machine.
				// (assumption is that validNets isn't buggy)
				if tc.wantCfg.Network == "tcp4" || tc.wantCfg.Network == "tcp6" {
					if !validNets[tc.wantCfg.Network] {
						t.Skipf("%q net not supported, skipping", tc.wantCfg.Network)
					}
				}
			}

			got, err := parseInstanceConfig(tc.dir, tc.instance, mockClient)
			if tc.wantCfg == wantErr {
				if err != nil {
					return // pass. an error was expected and returned.
				}
				t.Fatalf("parseInstanceConfig(%s, %s) = %+v, wanted error", tc.dir, tc.instance, got)
			}
			if err != nil {
				t.Fatalf("parseInstanceConfig(%s, %s) had unexpected error: %v", tc.dir, tc.instance, err)
			}

			if tc.wantCfg.Address == anyLoopbackAddress {
				host, _, err := net.SplitHostPort(got.Address)
				if err != nil {
					t.Fatalf("net.SplitHostPort(%v): %v", got.Address, err)
				}
				ip := net.ParseIP(host)
				if !ip.IsLoopback() {
					t.Fatalf("want loopback, got addr: %v", got.Address)
				}

				// use a placeholder address, so the rest of the config can be compared
				got.Address = "<loopback>"
				tc.wantCfg.Address = got.Address
			}

			if got != tc.wantCfg {
				t.Errorf("parseInstanceConfig(%s, %s) = %+v, want %+v", tc.dir, tc.instance, got, tc.wantCfg)
			}
		})
	}
}

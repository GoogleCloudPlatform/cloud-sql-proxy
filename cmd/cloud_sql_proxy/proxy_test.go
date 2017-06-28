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
	"io/ioutil"
	"net/http"
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
		dir            string
		useFuse        bool
		instances      []string
		instancesSrc   string
		maxConnections int

		// We don't need to check the []instancesConfig return value, we already
		// have a TestParseInstanceConfig.
		wantErr bool
	}{
		{
			"setting -fuse and -dir",
			"dir", true, nil, "", 0, false,
		}, {
			"setting -fuse",
			"", true, nil, "", 0, true,
		}, {
			"setting -fuse, -dir, and -instances",
			"dir", true, []string{"proj:reg:x"}, "", 0, true,
		}, {
			"setting -fuse, -dir, and -instances_metadata",
			"dir", true, nil, "md", 0, true,
		}, {
			"setting -dir and -instances (unix socket)",
			"dir", false, []string{"proj:reg:x"}, "", 0, false,
		}, {
			"setting -instance (unix socket)",
			"", false, []string{"proj:reg:x"}, "", 0, true,
		}, {
			"setting -instance (tcp socket)",
			"", false, []string{"proj:reg:x=tcp:1234"}, "", 0, false,
		}, {
			"setting -instance (tcp socket) and -instances_metadata",
			"", false, []string{"proj:reg:x=tcp:1234"}, "md", 0, true,
		}, {
			"setting -dir, -instance (tcp socket), and -instances_metadata",
			"dir", false, []string{"proj:reg:x=tcp:1234"}, "md", 0, false,
		}, {
			"setting -dir, -instance (unix socket), and -instances_metadata",
			"dir", false, []string{"proj:reg:x"}, "md", 0, false,
		}, {
			"setting -dir and -instances_metadata",
			"dir", false, nil, "md", 0, false,
		}, {
			"setting -instances_metadata",
			"", false, nil, "md", 0, true,
		}, {
			"setting -max_connections",
			"dir", true, nil, "", 10, false,
		},
	} {
		_, err := CreateInstanceConfigs(v.dir, v.useFuse, v.instances, v.instancesSrc, v.maxConnections, mockClient)
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
	for _, v := range []struct {
		// inputs
		dir, instance string

		wantCfg instanceConfig
		wantErr bool
	}{
		{
			"/x", "domain.com:my-proj:my-reg:my-instance",
			instanceConfig{"domain.com:my-proj:my-reg:my-instance", "unix", "/x/domain.com:my-proj:my-reg:my-instance"},
			false,
		}, {
			"/x", "my-proj:my-reg:my-instance",
			instanceConfig{"my-proj:my-reg:my-instance", "unix", "/x/my-proj:my-reg:my-instance"},
			false,
		}, {
			"/x", "my-proj:my-reg:my-instance=tcp:1234",
			instanceConfig{"my-proj:my-reg:my-instance", "tcp", "127.0.0.1:1234"},
			false,
		}, {
			"/x", "my-proj:my-reg:my-instance=tcp:my-host:1111",
			instanceConfig{"my-proj:my-reg:my-instance", "tcp", "my-host:1111"},
			false,
		}, {
			"/x", "my-proj:my-reg:my-instance=",
			instanceConfig{},
			true,
		}, {
			"/x", "my-proj:my-reg:my-instance=cool network",
			instanceConfig{},
			true,
		}, {
			"/x", "my-proj:my-reg:my-instance=cool network:1234",
			instanceConfig{},
			true,
		}, {
			"/x", "my-proj:my-reg:my-instance=oh:so:many:colons",
			instanceConfig{},
			true,
		},
	} {
		got, err := parseInstanceConfig(v.dir, v.instance, mockClient)
		if v.wantErr {
			if err == nil {
				t.Errorf("parseInstanceConfig(%s, %s) = %+v, wanted error", got)
			}
			continue
		} else if err != nil {
			t.Errorf("parseInstanceConfig(%s, %s) had unexpected error: %v", v.dir, v.instance, err)
			continue
		}
		if got != v.wantCfg {
			t.Errorf("parseInstanceConfig(%s, %s) = %+v, want %+v", v.dir, v.instance, got, v.wantCfg)
		}
	}
}

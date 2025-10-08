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
	"testing"
	"unsafe"

	"github.com/google/go-cmp/cmp"
)

func TestClientUsesSyncAtomicAlignment(t *testing.T) {
	// The sync/atomic pkg has a bug that requires the developer to guarantee
	// 64-bit alignment when using 64-bit functions on 32-bit systems.
	c := &Client{} //nolint:staticcheck

	if a := unsafe.Offsetof(c.connCount); a%64 != 0 {
		t.Errorf("Client.connCount is not 64-bit aligned: want 0, got %v", a)
	}
}

func TestParseImpersonationChain(t *testing.T) {
	tcs := []struct {
		desc       string
		in         string
		wantTarget string
		wantChain  []string
	}{
		{
			desc:       "when there is only a target",
			in:         "sv1@developer.gserviceaccount.com",
			wantTarget: "sv1@developer.gserviceaccount.com",
		},
		{
			desc:       "when there are delegates",
			in:         "sv1@developer.gserviceaccount.com,sv2@developer.gserviceaccount.com,sv3@developer.gserviceaccount.com",
			wantTarget: "sv1@developer.gserviceaccount.com",
			wantChain: []string{
				"sv3@developer.gserviceaccount.com",
				"sv2@developer.gserviceaccount.com",
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			gotTarget, gotChain := parseImpersonationChain(tc.in)
			if gotTarget != tc.wantTarget {
				t.Fatalf("target: want = %v, got = %v", tc.wantTarget, gotTarget)
			}
			if !equalSlice(tc.wantChain, gotChain) {
				t.Fatalf("want chain != got chain: %v", cmp.Diff(tc.wantChain, gotChain))
			}
		})
	}
}

func equalSlice[T comparable](x, y []T) bool {
	if len(x) != len(y) {
		return false
	}
	for i := 0; i < len(x); i++ {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func TestDialOptions(t *testing.T) {
	tcs := []struct {
		desc         string
		conf         Config
		instConf     InstanceConnConfig
		wantOptsLen  int
		checkOptions func(t *testing.T, opts []interface{})
	}{
		{
			desc:        "global override-ip is set",
			conf:        Config{OverrideIP: "10.0.0.1"},
			instConf:    InstanceConnConfig{},
			wantOptsLen: 2, // WithOneOffDialFunc + WithPublicIP
		},
		{
			desc:        "instance override-ip is set",
			conf:        Config{},
			instConf:    InstanceConnConfig{OverrideIP: stringPtr("10.0.0.2")},
			wantOptsLen: 2, // WithOneOffDialFunc + WithPublicIP
		},
		{
			desc:        "instance override-ip overrides global",
			conf:        Config{OverrideIP: "10.0.0.1"},
			instConf:    InstanceConnConfig{OverrideIP: stringPtr("10.0.0.2")},
			wantOptsLen: 2, // WithOneOffDialFunc + WithPublicIP
		},
		{
			desc:        "private-ip without override",
			conf:        Config{PrivateIP: true},
			instConf:    InstanceConnConfig{},
			wantOptsLen: 1, // WithPrivateIP only
		},
		{
			desc:        "psc without override",
			conf:        Config{PSC: true},
			instConf:    InstanceConnConfig{},
			wantOptsLen: 1, // WithPSC only
		},
		{
			desc:        "auto-ip without override",
			conf:        Config{AutoIP: true},
			instConf:    InstanceConnConfig{},
			wantOptsLen: 1, // WithAutoIP only
		},
		{
			desc:        "iam-authn is set",
			conf:        Config{},
			instConf:    InstanceConnConfig{IAMAuthN: boolPtr(true)},
			wantOptsLen: 1, // WithDialIAMAuthN only
		},
		{
			desc:        "iam-authn with override-ip",
			conf:        Config{OverrideIP: "10.0.0.1"},
			instConf:    InstanceConnConfig{IAMAuthN: boolPtr(true)},
			wantOptsLen: 3, // WithDialIAMAuthN + WithOneOffDialFunc + WithPublicIP
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			opts := dialOptions(tc.conf, tc.instConf)
			if len(opts) != tc.wantOptsLen {
				t.Errorf("want %d dial options, got %d", tc.wantOptsLen, len(opts))
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

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

package cmd

import (
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
)

func TestNewCommandArguments(t *testing.T) {
	withDefaults := func(c *proxy.Config) *proxy.Config {
		if c.Addr == "" {
			c.Addr = "127.0.0.1"
		}
		if c.Instances == nil {
			c.Instances = []proxy.InstanceConnConfig{{}}
		}
		if i := &c.Instances[0]; i.Name == "" {
			i.Name = "proj:region:inst"
		}
		return c
	}
	tcs := []struct {
		desc string
		args []string
		want *proxy.Config
	}{
		{
			desc: "basic invocation with defaults",
			args: []string{"proj:region:inst"},
			want: withDefaults(&proxy.Config{
				Addr:      "127.0.0.1",
				Instances: []proxy.InstanceConnConfig{{Name: "proj:region:inst"}},
			}),
		},
		{
			desc: "using the address flag",
			args: []string{"--address", "0.0.0.0", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				Addr:      "0.0.0.0",
				Instances: []proxy.InstanceConnConfig{{Name: "proj:region:inst"}},
			}),
		},
		{
			desc: "using the address (short) flag",
			args: []string{"-a", "0.0.0.0", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				Addr:      "0.0.0.0",
				Instances: []proxy.InstanceConnConfig{{Name: "proj:region:inst"}},
			}),
		},
		{
			desc: "using the address query param",
			args: []string{"proj:region:inst?address=0.0.0.0"},
			want: withDefaults(&proxy.Config{
				Addr: "127.0.0.1",
				Instances: []proxy.InstanceConnConfig{{
					Addr: "0.0.0.0",
					Name: "proj:region:inst",
				}},
			}),
		},
		{
			desc: "using the port flag",
			args: []string{"--port", "6000", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				Port: 6000,
			}),
		},
		{
			desc: "using the port (short) flag",
			args: []string{"-p", "6000", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				Port: 6000,
			}),
		},
		{
			desc: "using the port query param",
			args: []string{"proj:region:inst?port=6000"},
			want: withDefaults(&proxy.Config{
				Instances: []proxy.InstanceConnConfig{{
					Port: 6000,
				}},
			}),
		},
		{
			desc: "using the token flag",
			args: []string{"--token", "MYCOOLTOKEN", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				Token: "MYCOOLTOKEN",
			}),
		},
		{
			desc: "using the token (short) flag",
			args: []string{"-t", "MYCOOLTOKEN", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				Token: "MYCOOLTOKEN",
			}),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c := NewCommand()
			// Keep the test output quiet
			c.SilenceUsage = true
			c.SilenceErrors = true
			// Disable execute behavior
			c.RunE = func(*cobra.Command, []string) error {
				return nil
			}
			c.SetArgs(tc.args)

			err := c.Execute()
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}

			if got := c.conf; !cmp.Equal(tc.want, got) {
				t.Fatalf("want = %#v\ngot = %#v\ndiff = %v", tc.want, got, cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestNewCommandWithErrors(t *testing.T) {
	tcs := []struct {
		desc string
		args []string
	}{
		{
			desc: "basic invocation missing instance connection name",
			args: []string{},
		},
		{
			desc: "when the query string is bogus",
			args: []string{"proj:region:inst?ke;y=b;ad"},
		},
		{
			desc: "when the address query param is empty",
			args: []string{"proj:region:inst?address="},
		},
		{
			desc: "using the address flag with a bad IP address",
			args: []string{"--address", "bogus", "proj:region:inst"},
		},
		{
			desc: "when the address query param is not an IP address",
			args: []string{"proj:region:inst?address=世界"},
		},
		{
			desc: "when the address query param contains multiple values",
			args: []string{"proj:region:inst?address=0.0.0.0&address=1.1.1.1&address=2.2.2.2"},
		},
		{
			desc: "when the query string is invalid",
			args: []string{"proj:region:inst?address=1.1.1.1?foo=2.2.2.2"},
		},
		{
			desc: "when the port query param contains multiple values",
			args: []string{"proj:region:inst?port=1&port=2"},
		},
		{
			desc: "when the port query param is not a number",
			args: []string{"proj:region:inst?port=hi"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c := NewCommand()
			// Keep the test output quiet
			c.SilenceUsage = true
			c.SilenceErrors = true
			// Disable execute behavior
			c.RunE = func(*cobra.Command, []string) error {
				return nil
			}
			c.SetArgs(tc.args)

			err := c.Execute()
			if err == nil {
				t.Fatal("want error != nil, got = nil")
			}
		})
	}
}

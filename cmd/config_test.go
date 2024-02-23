// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"testing"
)

func assert[T comparable](t *testing.T, want, got T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNewCommandWithConfigFile(t *testing.T) {
	tcs := []struct {
		desc   string
		args   []string
		setup  func()
		assert func(t *testing.T, command *Command)
	}{
		{
			desc:  "toml config file",
			args:  []string{"--config-file", "testdata/config.toml"},
			setup: func() {},
			assert: func(t *testing.T, command *Command) {
				assert(t, 1, len(command.conf.Instances))
				assert(t, true, command.conf.Debug)
			},
		},
		{
			desc:  "yaml config file",
			args:  []string{"--config-file", "testdata/config.yaml"},
			setup: func() {},
			assert: func(t *testing.T, command *Command) {
				assert(t, 1, len(command.conf.Instances))
				assert(t, true, command.conf.Debug)
			},
		},
		{
			desc:  "json config file",
			args:  []string{"--config-file", "testdata/config.json"},
			setup: func() {},
			assert: func(t *testing.T, command *Command) {
				assert(t, 1, len(command.conf.Instances))
				assert(t, true, command.conf.Debug)
			},
		},
		{
			desc:  "config file with two instances",
			args:  []string{"--config-file", "testdata/two-instances.toml"},
			setup: func() {},
			assert: func(t *testing.T, command *Command) {
				assert(t, 2, len(command.conf.Instances))
			},
		},
		{
			desc: "instance argument overrides env config precedence",
			args: []string{"proj:region:inst"},
			setup: func() {
				t.Setenv("CSQL_PROXY_INSTANCE_CONNECTION_NAME", "p:r:i")
			},
			assert: func(t *testing.T, command *Command) {
				assert(t, "proj:region:inst", command.conf.Instances[0].Name)
			},
		},
		{
			desc: "instance env overrides config file precedence",
			args: []string{"--config-file", "testdata/config.json"},
			setup: func() {
				t.Setenv("CSQL_PROXY_INSTANCE_CONNECTION_NAME", "p:r:i")
			},
			assert: func(t *testing.T, command *Command) {
				assert(t, "p:r:i", command.conf.Instances[0].Name)
			},
		},
		{
			desc: "flag overrides env config precedence",
			args: []string{"proj:region:inst", "--debug"},
			setup: func() {
				t.Setenv("CSQL_PROXY_DEBUG", "false")
			},
			assert: func(t *testing.T, command *Command) {
				assert(t, true, command.conf.Debug)
			},
		},
		{
			desc: "flag overrides config file precedence",
			args: []string{
				"proj:region:inst",
				"--config-file", "testdata/config.toml",
				"--debug",
			},
			setup: func() {},
			assert: func(t *testing.T, command *Command) {
				assert(t, true, command.conf.Debug)
			},
		},
		{
			desc: "env overrides config file precedence",
			args: []string{
				"proj:region:inst",
				"--config-file", "testdata/config.toml",
			},
			setup: func() {
				t.Setenv("CSQL_PROXY_DEBUG", "false")
			},
			assert: func(t *testing.T, command *Command) {
				assert(t, false, command.conf.Debug)
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			tc.setup()

			cmd, err := invokeProxyCommand(tc.args)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}

			tc.assert(t, cmd)
		})
	}
}

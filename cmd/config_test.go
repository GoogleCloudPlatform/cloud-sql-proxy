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

func Assert[T comparable](t *testing.T, want, got T) {
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
		assert func(command *Command)
	}{
		{
			desc: "toml config file",
			args: []string{"--config-file", "testdata/cloud-sql-proxy.config.test-toml.toml"},
			setup: func() {
			},
			assert: func(command *Command) {
				Assert(t, 1, len(command.conf.Instances))
				Assert(t, true, command.conf.Debug)
			},
		},
		{
			desc: "yaml config file",
			args: []string{"--config-file", "testdata/cloud-sql-proxy.config.test-yaml.yaml"},
			setup: func() {
			},
			assert: func(command *Command) {
				Assert(t, 1, len(command.conf.Instances))
				Assert(t, true, command.conf.Debug)
			},
		},
		{
			desc: "json config file",
			args: []string{"--config-file", "testdata/cloud-sql-proxy.config.test-json.json"},
			setup: func() {
			},
			assert: func(command *Command) {
				Assert(t, 1, len(command.conf.Instances))
				Assert(t, true, command.conf.Debug)
			},
		},
		{
			desc: "toml config file with two instances",
			args: []string{"--config-file", "testdata/cloud-sql-proxy-two-instances.config.test-toml.toml"},
			setup: func() {
			},
			assert: func(command *Command) {
				Assert(t, 2, len(command.conf.Instances))
			},
		},
		{
			desc: "yaml config file with two instances",
			args: []string{"--config-file", "testdata/cloud-sql-proxy-two-instances.config.test-yaml.yaml"},
			setup: func() {
			},
			assert: func(command *Command) {
				Assert(t, 2, len(command.conf.Instances))
			},
		},
		{
			desc: "json config file with two instances",
			args: []string{"--config-file", "testdata/cloud-sql-proxy-two-instances.config.test-json.json"},
			setup: func() {
			},
			assert: func(command *Command) {
				Assert(t, 2, len(command.conf.Instances))
			},
		},
		{
			desc: "flag overrides env config precedence",
			args: []string{"proj:region:inst", "--debug"},
			setup: func() {
				t.Setenv("CSQL_PROXY_DEBUG", "false")
			},
			assert: func(command *Command) {
				Assert(t, true, command.conf.Debug)
			},
		},
		{
			desc: "flag overrides config file precedence",
			args: []string{"proj:region:inst", "--config-file", "../testdata/cloud-sql-proxy.config.test-toml.toml", "--debug"},
			setup: func() {
			},
			assert: func(command *Command) {
				Assert(t, true, command.conf.Debug)
			},
		},
		{
			desc: "env overrides config file precedence",
			args: []string{"proj:region:inst", "--config-file", "../testdata/cloud-sql-proxy.config.test-toml.toml"},
			setup: func() {
				t.Setenv("CSQL_PROXY_DEBUG", "false")
			},
			assert: func(command *Command) {
				Assert(t, false, command.conf.Debug)
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

			tc.assert(cmd)
		})
	}
}

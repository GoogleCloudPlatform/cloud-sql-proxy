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
	"errors"
	"fmt"
	"io"
	"runtime"
	"testing"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/log"
	"github.com/spf13/cobra"
)

type testDialer struct {
	cloudsql.Dialer
}

func TestCommandOptions(t *testing.T) {
	logger := log.NewStdLogger(io.Discard, io.Discard)
	dialer := &testDialer{}
	tcs := []struct {
		desc    string
		isValid func(*Command) error
		option  Option
		skip    bool
	}{
		{
			desc: "with logger",
			isValid: func(c *Command) error {
				if c.logger != logger {
					return errors.New("loggers do not match")
				}
				return nil
			},
			option: WithLogger(logger),
		},
		{
			desc: "with dialer",
			isValid: func(c *Command) error {
				if c.dialer != dialer {
					return errors.New("dialers do not match")
				}
				return nil
			},
			option: WithDialer(dialer),
		},
		{
			desc: "with FUSE dir",
			isValid: func(c *Command) error {
				if c.conf.FUSEDir != "somedir" {
					return fmt.Errorf(
						"want = %v, got = %v", "somedir", c.conf.FUSEDir,
					)
				}
				return nil
			},
			option: WithFuseDir("somedir"),
			// FUSE isn't available on GitHub macOS runners
			// and FUSE isn't supported on Windows, so skip this test.
			skip: runtime.GOOS == "darwin" || runtime.GOOS == "windows",
		},
		{
			desc: "with FUSE temp dir",
			isValid: func(c *Command) error {
				if c.conf.FUSETempDir != "somedir" {
					return fmt.Errorf(
						"want = %v, got = %v", "somedir", c.conf.FUSEDir,
					)
				}
				return nil
			},
			option: WithFuseTempDir("somedir"),
			// FUSE isn't available on GitHub macOS runners
			// and FUSE isn't supported on Windows, so skip this test.
			skip: runtime.GOOS == "darwin" || runtime.GOOS == "windows",
		},
		{
			desc: "with max connections",
			isValid: func(c *Command) error {
				if c.conf.MaxConnections != 1 {
					return fmt.Errorf(
						"want = %v, got = %v", 1, c.conf.MaxConnections,
					)
				}
				return nil
			},
			option: WithMaxConnections(1),
		},
		{
			desc: "with user agent",
			isValid: func(c *Command) error {
				if c.conf.OtherUserAgents != "agents-go-here" {
					return fmt.Errorf(
						"want = %v, got = %v",
						"agents-go-here", c.conf.OtherUserAgents,
					)
				}
				return nil
			},
			option: WithUserAgent("agents-go-here"),
		},
		{
			desc: "with auto IP",
			isValid: func(c *Command) error {
				if !c.conf.AutoIP {
					return errors.New("auto IP was false, but should be true")
				}
				return nil
			},
			option: WithAutoIP(),
		},
		{
			desc: "with quiet logging",
			isValid: func(c *Command) error {
				if !c.conf.Quiet {
					return errors.New("quiet was false, but should be true")
				}
				return nil
			},
			option: WithQuietLogging(),
		},
		{
			desc: "with lazy refresh",
			isValid: func(c *Command) error {
				if !c.conf.LazyRefresh {
					return errors.New(
						"LazyRefresh was false, but should be true",
					)
				}
				return nil
			},
			option: WithLazyRefresh(),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.skip {
				t.Skip("skipping unsupported test case")
			}
			got, err := invokeProxyWithOption(nil, tc.option)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.isValid(got); err != nil {
				t.Errorf("option did not initialize command correctly: %v", err)
			}
		})
	}
}

func TestCommandOptionsOverridesCLI(t *testing.T) {
	tcs := []struct {
		desc    string
		isValid func(*Command) error
		option  Option
		args    []string
	}{
		{
			desc: "with duplicate max connections",
			isValid: func(c *Command) error {
				if c.conf.MaxConnections != 10 {
					return errors.New("max connections do not match")
				}
				return nil
			},
			option: WithMaxConnections(10),
			args:   []string{"--max-connections", "20"},
		},
		{
			desc: "with quiet logging",
			isValid: func(c *Command) error {
				if !c.conf.Quiet {
					return errors.New("quiet was false, but should be true")
				}
				return nil
			},
			option: WithQuietLogging(),
			args:   []string{"--quiet", "false"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := invokeProxyWithOption(tc.args, tc.option)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.isValid(got); err != nil {
				t.Errorf("option did not initialize command correctly: %v", err)
			}
		})
	}
}

func invokeProxyWithOption(args []string, o Option) (*Command, error) {
	c := NewCommand(o)
	// Keep the test output quiet
	c.SilenceUsage = true
	c.SilenceErrors = true
	// Disable execute behavior
	c.RunE = func(*cobra.Command, []string) error {
		return nil
	}
	args = append(args, "test-project:us-central1:test-instance")
	c.SetArgs(args)

	err := c.Execute()

	return c, err
}

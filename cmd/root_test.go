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
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
)

func withDefaults(c *proxy.Config) *proxy.Config {
	if c.UserAgent == "" {
		c.UserAgent = defaultUserAgent
	}
	if c.Addr == "" {
		c.Addr = "127.0.0.1"
	}
	if c.FUSEDir == "" {
		if c.Instances == nil {
			c.Instances = []proxy.InstanceConnConfig{{}}
		}
		if i := &c.Instances[0]; i.Name == "" {
			i.Name = "proj:region:inst"
		}
	}
	if c.FUSETempDir == "" {
		c.FUSETempDir = filepath.Join(os.TempDir(), "csql-tmp")
	}
	if c.HTTPAddress == "" {
		c.HTTPAddress = "localhost"
	}
	if c.HTTPPort == "" {
		c.HTTPPort = "9090"
	}
	if c.AdminPort == "" {
		c.AdminPort = "9091"
	}
	if c.TelemetryTracingSampleRate == 0 {
		c.TelemetryTracingSampleRate = 10_000
	}
	return c
}

// pointer returns the address of v and makes it easy to take the address of a
// predeclared identifier. Compare:
//
//	t := true
//	pt := &t
//
// vs
//
//	pt := pointer(true)
func pointer[T any](v T) *T {
	return &v
}

func invokeProxyCommand(args []string) (*Command, error) {
	c := NewCommand()
	// Keep the test output quiet
	c.SilenceUsage = true
	c.SilenceErrors = true
	// Disable execute behavior
	c.RunE = func(*cobra.Command, []string) error {
		return nil
	}
	c.SetArgs(args)

	err := c.Execute()

	return c, err
}

func TestUserAgentWithVersionEnvVar(t *testing.T) {
	os.Setenv("CSQL_PROXY_USER_AGENT", "cloud-sql-proxy-operator/0.0.1")
	defer os.Unsetenv("CSQL_PROXY_USER_AGENT")

	cmd, err := invokeProxyCommand([]string{"proj:region:inst"})
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}

	want := "cloud-sql-proxy-operator/0.0.1"
	got := cmd.conf.UserAgent
	if !strings.Contains(got, want) {
		t.Errorf("expected user agent to contain: %v; got: %v", want, got)
	}
}

func TestUserAgent(t *testing.T) {
	cmd, err := invokeProxyCommand(
		[]string{
			"--user-agent",
			"cloud-sql-proxy-operator/0.0.1",
			"proj:region:inst",
		},
	)
	if err != nil {
		t.Fatalf("want error = nil, got = %v", err)
	}

	want := "cloud-sql-proxy-operator/0.0.1"
	got := cmd.conf.UserAgent
	if !strings.Contains(got, want) {
		t.Errorf("expected userAgent to contain: %v; got: %v", want, got)
	}
}

func TestNewCommandArguments(t *testing.T) {
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
		{
			desc: "using the credentiale file flag",
			args: []string{"--credentials-file", "/path/to/file", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				CredentialsFile: "/path/to/file",
			}),
		},
		{
			desc: "using the (short) credentiale file flag",
			args: []string{"-c", "/path/to/file", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				CredentialsFile: "/path/to/file",
			}),
		},
		{
			desc: "using the JSON credentials",
			args: []string{"--json-credentials", `{"json":"goes-here"}`, "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				CredentialsJSON: `{"json":"goes-here"}`,
			}),
		},
		{
			desc: "using the (short) JSON credentials",
			args: []string{"-j", `{"json":"goes-here"}`, "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				CredentialsJSON: `{"json":"goes-here"}`,
			}),
		},
		{
			desc: "using the gcloud auth flag",
			args: []string{"--gcloud-auth", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				GcloudAuth: true,
			}),
		},
		{
			desc: "using the (short) gcloud auth flag",
			args: []string{"-g", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				GcloudAuth: true,
			}),
		},
		{
			desc: "using the api-endpoint flag without trailing slash",
			args: []string{"--sqladmin-api-endpoint", "https://test.googleapis.com", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				APIEndpointURL: "https://test.googleapis.com/",
			}),
		},
		{
			desc: "using the api-endpoint flag with trailing slash",
			args: []string{"--sqladmin-api-endpoint", "https://test.googleapis.com/", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				APIEndpointURL: "https://test.googleapis.com/",
			}),
		},
		{
			desc: "using the unix socket flag",
			args: []string{"--unix-socket", "/path/to/dir/", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				UnixSocket: "/path/to/dir/",
			}),
		},
		{
			desc: "using the (short) unix socket flag",
			args: []string{"-u", "/path/to/dir/", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				UnixSocket: "/path/to/dir/",
			}),
		},
		{
			desc: "using the unix socket query param",
			args: []string{"proj:region:inst?unix-socket=/path/to/dir/"},
			want: withDefaults(&proxy.Config{
				Instances: []proxy.InstanceConnConfig{{
					UnixSocket: "/path/to/dir/",
				}},
			}),
		},
		{
			desc: "using the iam authn login flag",
			args: []string{"--auto-iam-authn", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				IAMAuthN: true,
			}),
		},
		{
			desc: "using the (short) iam authn login flag",
			args: []string{"-i", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				IAMAuthN: true,
			}),
		},
		{
			desc: "using the iam authn login query param",
			args: []string{"proj:region:inst?auto-iam-authn=true"},
			want: withDefaults(&proxy.Config{
				Instances: []proxy.InstanceConnConfig{{
					IAMAuthN: pointer(true),
				}},
			}),
		},
		{
			desc: "enabling structured logging",
			args: []string{"--structured-logs", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				StructuredLogs: true,
			}),
		},
		{
			desc: "using the max connections flag",
			args: []string{"--max-connections", "1", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				MaxConnections: 1,
			}),
		},
		{
			desc: "using wait after signterm flag",
			args: []string{"--max-sigterm-delay", "10s", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				WaitOnClose: 10 * time.Second,
			}),
		},
		{
			desc: "using the private-ip flag",
			args: []string{"--private-ip", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				PrivateIP: true,
			}),
		},
		{
			desc: "using the private-ip flag query param",
			args: []string{"proj:region:inst?private-ip=true"},
			want: withDefaults(&proxy.Config{
				Instances: []proxy.InstanceConnConfig{{
					PrivateIP: pointer(true),
				}},
			}),
		},
		{
			desc: "using the quota project flag",
			args: []string{"--quota-project", "proj", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				QuotaProject: "proj",
			}),
		},
		{
			desc: "using the impersonate service account flag",
			args: []string{"--impersonate-service-account",
				"sv1@developer.gserviceaccount.com",
				"proj:region:inst"},
			want: withDefaults(&proxy.Config{
				ImpersonationChain: "sv1@developer.gserviceaccount.com",
			}),
		},
		{
			desc: "using the debug flag",
			args: []string{"--debug", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				Debug: true,
			}),
		},
		{
			desc: "using the admin port flag",
			args: []string{"--admin-port", "7777", "proj:region:inst"},
			want: withDefaults(&proxy.Config{
				AdminPort: "7777",
			}),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c, err := invokeProxyCommand(tc.args)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}

			if got := c.conf; !cmp.Equal(tc.want, got) {
				t.Fatalf("want = %#v\ngot = %#v\ndiff = %v", tc.want, got, cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestNewCommandWithEnvironmentConfigPrivateFields(t *testing.T) {
	tcs := []struct {
		desc     string
		envName  string
		envValue string
		isValid  func(cmd *Command) bool
	}{}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			os.Setenv(tc.envName, tc.envValue)
			defer os.Unsetenv(tc.envName)

			c, err := invokeProxyCommand([]string{"proj:region:inst"})
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}

			if !tc.isValid(c) {
				t.Fatal("want valid, got invalid")
			}
		})
	}
}

func TestNewCommandWithEnvironmentConfigInstanceConnectionName(t *testing.T) {
	tcs := []struct {
		desc string
		env  map[string]string
		args []string
		want *proxy.Config
	}{
		{
			desc: "with one instance connection name",
			env: map[string]string{
				"CSQL_PROXY_INSTANCE_CONNECTION_NAME": "proj:reg:inst",
			},
			want: withDefaults(&proxy.Config{Instances: []proxy.InstanceConnConfig{
				{Name: "proj:reg:inst"},
			}}),
		},
		{
			desc: "with multiple instance connection names",
			env: map[string]string{
				"CSQL_PROXY_INSTANCE_CONNECTION_NAME_0": "proj:reg:inst0",
				"CSQL_PROXY_INSTANCE_CONNECTION_NAME_1": "proj:reg:inst1",
			},
			want: withDefaults(&proxy.Config{Instances: []proxy.InstanceConnConfig{
				{Name: "proj:reg:inst0"},
				{Name: "proj:reg:inst1"},
			}}),
		},
		{
			desc: "with query params",

			env: map[string]string{
				"CSQL_PROXY_INSTANCE_CONNECTION_NAME_0": "proj:reg:inst0?auto-iam-authn=true",
			},
			want: withDefaults(&proxy.Config{Instances: []proxy.InstanceConnConfig{
				{Name: "proj:reg:inst0", IAMAuthN: pointer(true)},
			}}),
		},
		{
			desc: "when the index skips a number",
			env: map[string]string{
				"CSQL_PROXY_INSTANCE_CONNECTION_NAME_0": "proj:reg:inst0",
				"CSQL_PROXY_INSTANCE_CONNECTION_NAME_2": "proj:reg:inst1",
			},
			want: withDefaults(&proxy.Config{Instances: []proxy.InstanceConnConfig{
				{Name: "proj:reg:inst0"},
			}}),
		},
		{
			desc: "when there are CLI args provided",
			env: map[string]string{
				"CSQL_PROXY_INSTANCE_CONNECTION_NAME": "proj:reg:inst0",
			},
			args: []string{"myotherproj:myreg:myinst"},
			want: withDefaults(&proxy.Config{Instances: []proxy.InstanceConnConfig{
				{Name: "myotherproj:myreg:myinst"},
			}}),
		},
		{
			desc: "when only an index instance connection name is defined",
			env: map[string]string{
				"CSQL_PROXY_INSTANCE_CONNECTION_NAME_0": "proj:reg:inst0",
			},
			want: withDefaults(&proxy.Config{Instances: []proxy.InstanceConnConfig{
				{Name: "proj:reg:inst0"},
			}}),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			var cleanup []string
			for k, v := range tc.env {
				os.Setenv(k, v)
				cleanup = append(cleanup, k)
			}
			defer func() {
				for _, k := range cleanup {
					os.Unsetenv(k)
				}
			}()

			c, err := invokeProxyCommand(tc.args)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}

			if got := c.conf; !cmp.Equal(tc.want, got) {
				t.Fatalf("want = %#v\ngot = %#v\ndiff = %v", tc.want, got, cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestNewCommandWithEnvironmentConfig(t *testing.T) {
	tcs := []struct {
		desc     string
		envName  string
		envValue string
		want     *proxy.Config
	}{
		{
			desc:     "using the address envvar",
			envName:  "CSQL_PROXY_ADDRESS",
			envValue: "0.0.0.0",
			want: withDefaults(&proxy.Config{
				Addr: "0.0.0.0",
			}),
		},
		{
			desc:     "using the port envvar",
			envName:  "CSQL_PROXY_PORT",
			envValue: "6000",
			want: withDefaults(&proxy.Config{
				Port: 6000,
			}),
		},
		{
			desc:     "using the token envvar",
			envName:  "CSQL_PROXY_TOKEN",
			envValue: "MYCOOLTOKEN",
			want: withDefaults(&proxy.Config{
				Token: "MYCOOLTOKEN",
			}),
		},
		{
			desc:     "using the credentiale file envvar",
			envName:  "CSQL_PROXY_CREDENTIALS_FILE",
			envValue: "/path/to/file",
			want: withDefaults(&proxy.Config{
				CredentialsFile: "/path/to/file",
			}),
		},
		{
			desc:     "using the JSON credentials",
			envName:  "CSQL_PROXY_JSON_CREDENTIALS",
			envValue: `{"json":"goes-here"}`,
			want: withDefaults(&proxy.Config{
				CredentialsJSON: `{"json":"goes-here"}`,
			}),
		},
		{
			desc:     "using the gcloud auth envvar",
			envName:  "CSQL_PROXY_GCLOUD_AUTH",
			envValue: "true",
			want: withDefaults(&proxy.Config{
				GcloudAuth: true,
			}),
		},
		{
			desc:     "using the api-endpoint envvar",
			envName:  "CSQL_PROXY_SQLADMIN_API_ENDPOINT",
			envValue: "https://test.googleapis.com/",
			want: withDefaults(&proxy.Config{
				APIEndpointURL: "https://test.googleapis.com/",
			}),
		},
		{
			desc:     "using the unix socket envvar",
			envName:  "CSQL_PROXY_UNIX_SOCKET",
			envValue: "/path/to/dir/",
			want: withDefaults(&proxy.Config{
				UnixSocket: "/path/to/dir/",
			}),
		},
		{
			desc:     "using the iam authn login envvar",
			envName:  "CSQL_PROXY_AUTO_IAM_AUTHN",
			envValue: "true",
			want: withDefaults(&proxy.Config{
				IAMAuthN: true,
			}),
		},
		{
			desc:     "enabling structured logging",
			envName:  "CSQL_PROXY_STRUCTURED_LOGS",
			envValue: "true",
			want: withDefaults(&proxy.Config{
				StructuredLogs: true,
			}),
		},
		{
			desc:     "using the max connections envvar",
			envName:  "CSQL_PROXY_MAX_CONNECTIONS",
			envValue: "1",
			want: withDefaults(&proxy.Config{
				MaxConnections: 1,
			}),
		},
		{
			desc:     "using wait after signterm envvar",
			envName:  "CSQL_PROXY_MAX_SIGTERM_DELAY",
			envValue: "10s",
			want: withDefaults(&proxy.Config{
				WaitOnClose: 10 * time.Second,
			}),
		},
		{
			desc:     "using the private-ip envvar",
			envName:  "CSQL_PROXY_PRIVATE_IP",
			envValue: "true",
			want: withDefaults(&proxy.Config{
				PrivateIP: true,
			}),
		},
		{
			desc:     "using the quota project envvar",
			envName:  "CSQL_PROXY_QUOTA_PROJECT",
			envValue: "proj",
			want: withDefaults(&proxy.Config{
				QuotaProject: "proj",
			}),
		},
		{
			desc:     "using the imopersonate service accounn envvar",
			envName:  "CSQL_PROXY_IMPERSONATE_SERVICE_ACCOUNT",
			envValue: "sv1@developer.gserviceaccount.com",
			want: withDefaults(&proxy.Config{
				ImpersonationChain: "sv1@developer.gserviceaccount.com",
			}),
		},
		{
			desc:     "using the disable traces envvar",
			envName:  "CSQL_PROXY_DISABLE_TRACES",
			envValue: "true",
			want: withDefaults(&proxy.Config{
				DisableTraces: true,
			}),
		},
		{
			desc:     "using the telemetry sample rate envvar",
			envName:  "CSQL_PROXY_TELEMETRY_SAMPLE_RATE",
			envValue: "500",
			want: withDefaults(&proxy.Config{
				TelemetryTracingSampleRate: 500,
			}),
		},
		{
			desc:     "using the disable metrics envvar",
			envName:  "CSQL_PROXY_DISABLE_METRICS",
			envValue: "true",
			want: withDefaults(&proxy.Config{
				DisableMetrics: true,
			}),
		},
		{
			desc:     "using the telemetry project envvar",
			envName:  "CSQL_PROXY_TELEMETRY_PROJECT",
			envValue: "mycoolproject",
			want: withDefaults(&proxy.Config{
				TelemetryProject: "mycoolproject",
			}),
		},
		{
			desc:     "using the telemetry prefix envvar",
			envName:  "CSQL_PROXY_TELEMETRY_PREFIX",
			envValue: "myprefix",
			want: withDefaults(&proxy.Config{
				TelemetryPrefix: "myprefix",
			}),
		},
		{
			desc:     "using the prometheus envvar",
			envName:  "CSQL_PROXY_PROMETHEUS",
			envValue: "true",
			want: withDefaults(&proxy.Config{
				Prometheus: true,
			}),
		},
		{
			desc:     "using the prometheus namespace envvar",
			envName:  "CSQL_PROXY_PROMETHEUS_NAMESPACE",
			envValue: "myns",
			want: withDefaults(&proxy.Config{
				PrometheusNamespace: "myns",
			}),
		},
		{
			desc:     "using the health check envvar",
			envName:  "CSQL_PROXY_HEALTH_CHECK",
			envValue: "true",
			want: withDefaults(&proxy.Config{
				HealthCheck: true,
			}),
		},
		{
			desc:     "using the http address envvar",
			envName:  "CSQL_PROXY_HTTP_ADDRESS",
			envValue: "0.0.0.0",
			want: withDefaults(&proxy.Config{
				HTTPAddress: "0.0.0.0",
			}),
		},
		{
			desc:     "using the http port envvar",
			envName:  "CSQL_PROXY_HTTP_PORT",
			envValue: "5555",
			want: withDefaults(&proxy.Config{
				HTTPPort: "5555",
			}),
		},
		{
			desc:     "using the debug envvar",
			envName:  "CSQL_PROXY_DEBUG",
			envValue: "true",
			want: withDefaults(&proxy.Config{
				Debug: true,
			}),
		},
		{
			desc:     "using the admin port envvar",
			envName:  "CSQL_PROXY_ADMIN_PORT",
			envValue: "7777",
			want: withDefaults(&proxy.Config{
				AdminPort: "7777",
			}),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			os.Setenv(tc.envName, tc.envValue)
			defer os.Unsetenv(tc.envName)

			c, err := invokeProxyCommand([]string{"proj:region:inst"})
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}

			if got := c.conf; !cmp.Equal(tc.want, got) {
				t.Fatalf("want = %#v\ngot = %#v\ndiff = %v", tc.want, got, cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestAutoIAMAuthNQueryParams(t *testing.T) {
	tcs := []struct {
		desc string
		args []string
		want *bool
	}{
		{
			desc: "when the query string is absent",
			args: []string{"proj:region:inst"},
			want: nil,
		},
		{
			desc: "when the query string is true",
			args: []string{"proj:region:inst?auto-iam-authn=true"},
			want: pointer(true),
		},
		{
			desc: "when the query string is (short) t",
			args: []string{"proj:region:inst?auto-iam-authn=t"},
			want: pointer(true),
		},
		{
			desc: "when the query string is false",
			args: []string{"proj:region:inst?auto-iam-authn=false"},
			want: pointer(false),
		},
		{
			desc: "when the query string is (short) f",
			args: []string{"proj:region:inst?auto-iam-authn=f"},
			want: pointer(false),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c, err := invokeProxyCommand(tc.args)
			if err != nil {
				t.Fatalf("command.Execute: %v", err)
			}
			if tc.want == nil && c.conf.Instances[0].IAMAuthN == nil {
				return
			}
			if got := c.conf.Instances[0].IAMAuthN; *got != *tc.want {
				t.Errorf("args = %v, want = %v, got = %v", tc.args, *tc.want, *got)
			}
		})
	}
}

func TestPrivateIPQueryParams(t *testing.T) {
	tcs := []struct {
		desc string
		args []string
		want *bool
	}{
		{
			desc: "when the query string is absent",
			args: []string{"proj:region:inst"},
			want: nil,
		},
		{
			desc: "when the query string has no value",
			args: []string{"proj:region:inst?private-ip"},
			want: pointer(true),
		},
		{
			desc: "when the query string is true",
			args: []string{"proj:region:inst?private-ip=true"},
			want: pointer(true),
		},
		{
			desc: "when the query string is True",
			args: []string{"proj:region:inst?private-ip=True"},
			want: pointer(true),
		},
		{
			desc: "when the query string is (short) T",
			args: []string{"proj:region:inst?private-ip=T"},
			want: pointer(true),
		},
		{
			desc: "when the query string is (short) t",
			args: []string{"proj:region:inst?private-ip=t"},
			want: pointer(true),
		},
		{
			desc: "when the query string is false",
			args: []string{"proj:region:inst?private-ip=false"},
			want: pointer(false),
		},
		{
			desc: "when the query string is (short) f",
			args: []string{"proj:region:inst?private-ip=f"},
			want: pointer(false),
		},
		{
			desc: "when the query string is False",
			args: []string{"proj:region:inst?private-ip=False"},
			want: pointer(false),
		},
		{
			desc: "when the query string is (short) F",
			args: []string{"proj:region:inst?private-ip=F"},
			want: pointer(false),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c, err := invokeProxyCommand(tc.args)
			if err != nil {
				t.Fatalf("command.Execute: %v", err)
			}
			if tc.want == nil && c.conf.Instances[0].PrivateIP == nil {
				return
			}
			if got := c.conf.Instances[0].PrivateIP; *got != *tc.want {
				t.Errorf("args = %v, want = %v, got = %v", tc.args, *tc.want, *got)
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
			args: []string{"proj:region:inst?%=foo"},
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
		{
			desc: "when both token and credentials file are set",
			args: []string{
				"--token", "my-token",
				"--credentials-file", "/path/to/file", "proj:region:inst"},
		},
		{
			desc: "when both token and gcloud auth are set",
			args: []string{
				"--token", "my-token",
				"--gcloud-auth", "proj:region:inst"},
		},
		{
			desc: "when both gcloud auth and credentials file are set",
			args: []string{
				"--gcloud-auth",
				"--credentials-file", "/path/to/file", "proj:region:inst"},
		},
		{
			desc: "when both token and credentials JSON are set",
			args: []string{
				"--token", "a-token",
				"--json-credentials", `{"json":"here"}`, "proj:region:inst"},
		},
		{
			desc: "when both credentials file and credentials JSON are set",
			args: []string{
				"--credentials-file", "/a/file",
				"--json-credentials", `{"json":"here"}`, "proj:region:inst"},
		},
		{
			desc: "when both gcloud auth and credentials JSON are set",
			args: []string{
				"--gcloud-auth",
				"--json-credentials", `{"json":"here"}`, "proj:region:inst"},
		},
		{
			desc: "when the unix socket query param contains multiple values",
			args: []string{"proj:region:inst?unix-socket=/one&unix-socket=/two"},
		},
		{
			desc: "using the unix socket flag with addr",
			args: []string{"-u", "/path/to/dir/", "-a", "127.0.0.1", "proj:region:inst"},
		},
		{
			desc: "using the unix socket flag with port",
			args: []string{"-u", "/path/to/dir/", "-p", "5432", "proj:region:inst"},
		},
		{
			desc: "using the unix socket and addr query params",
			args: []string{"proj:region:inst?unix-socket=/path&address=127.0.0.1"},
		},
		{
			desc: "using the unix socket and port query params",
			args: []string{"proj:region:inst?unix-socket=/path&port=5000"},
		},
		{
			desc: "when the iam authn login query param contains multiple values",
			args: []string{"proj:region:inst?auto-iam-authn=true&auto-iam-authn=false"},
		},
		{
			desc: "when the iam authn login query param is bogus",
			args: []string{"proj:region:inst?auto-iam-authn=nope"},
		},
		{
			desc: "using an invalid url for sqladmin-api-endpoint",
			args: []string{"--sqladmin-api-endpoint", "https://user:abc{DEf1=ghi@example.com:5432/db?sslmode=require", "proj:region:inst"},
		},
		{
			desc: "using fuse-tmp-dir without fuse",
			args: []string{"--fuse-tmp-dir", "/mydir"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := invokeProxyCommand(tc.args)
			if err == nil {
				t.Fatal("want error != nil, got = nil")
			}
		})
	}
}

type spyDialer struct {
	mu  sync.Mutex
	got string
}

func (s *spyDialer) instance() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.got
	return i
}

func (*spyDialer) Dial(_ context.Context, _ string, _ ...cloudsqlconn.DialOption) (net.Conn, error) {
	return nil, errors.New("spy dialer does not dial")
}

func (s *spyDialer) EngineVersion(_ context.Context, inst string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = inst
	return "", nil
}

func (*spyDialer) Close() error {
	return nil
}

func TestCommandWithCustomDialer(t *testing.T) {
	want := "my-project:my-region:my-instance"
	s := &spyDialer{}
	c := NewCommand(WithDialer(s))
	// Keep the test output quiet
	c.SilenceUsage = true
	c.SilenceErrors = true
	c.SetArgs([]string{want})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := c.ExecuteContext(ctx); !errors.As(err, &errSigInt) {
		t.Fatalf("want errSigInt, got = %v", err)
	}

	if got := s.instance(); got != want {
		t.Fatalf("want = %v, got = %v", want, got)
	}
}

func tryDial(addr string) (*http.Response, error) {
	var (
		resp     *http.Response
		attempts int
		err      error
	)
	for {
		if attempts > 10 {
			return resp, err
		}
		resp, err = http.Get(addr)
		if err != nil {
			attempts++
			time.Sleep(time.Second)
			continue
		}
		return resp, err
	}
}

func TestPrometheusMetricsEndpoint(t *testing.T) {
	c := NewCommand(WithDialer(&spyDialer{}))
	// Keep the test output quiet
	c.SilenceUsage = true
	c.SilenceErrors = true
	c.SetArgs([]string{"--prometheus", "my-project:my-region:my-instance"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go c.ExecuteContext(ctx)

	// try to dial metrics server for a max of ~10s to give the proxy time to
	// start up.
	resp, err := tryDial("http://localhost:9090/metrics") // default port set by http-port flag
	if err != nil {
		t.Fatalf("failed to dial metrics endpoint: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected a 200 status, got = %v", resp.StatusCode)
	}
}

func TestPProfServer(t *testing.T) {
	c := NewCommand(WithDialer(&spyDialer{}))
	c.SilenceUsage = true
	c.SilenceErrors = true
	c.SetArgs([]string{"--debug", "--admin-port", "9191", "my-project:my-region:my-instance"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.ExecuteContext(ctx)
	resp, err := tryDial("http://localhost:9191/debug/pprof/")
	if err != nil {
		t.Fatalf("failed to dial endpoint: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected a 200 status, got = %v", resp.StatusCode)
	}
}

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
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
)

func TestNewCommandArguments(t *testing.T) {

	// saving true in a variable so we can take its address.
	trueValue := true

	withDefaults := func(c *proxy.Config) *proxy.Config {
		if c.UserAgent == "" {
			c.UserAgent = userAgent
		}
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
			// the query param's presence equates to true
			args: []string{"proj:region:inst?auto-iam-authn=true"},
			want: withDefaults(&proxy.Config{
				Instances: []proxy.InstanceConnConfig{{
					IAMAuthN: &trueValue,
				}},
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

func TestAutoIAMAuthNQueryParams(t *testing.T) {
	// saving true and false in a variable so we can take its address
	trueValue := true
	falseValue := false

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
			want: &trueValue,
		},
		{
			desc: "when the query string is (short) t",
			args: []string{"proj:region:inst?auto-iam-authn=t"},
			want: &trueValue,
		},
		{
			desc: "when the query string is false",
			args: []string{"proj:region:inst?auto-iam-authn=false"},
			want: &falseValue,
		},
		{
			desc: "when the query string is (short) f",
			args: []string{"proj:region:inst?auto-iam-authn=f"},
			want: &falseValue,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c := NewCommand()
			// Keep the test output quiet
			c.SilenceUsage = true
			c.SilenceErrors = true
			// Disable execute behavior
			c.RunE = func(*cobra.Command, []string) error { return nil }
			c.SetArgs(tc.args)

			err := c.Execute()
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
				"--credential-file", "/path/to/file", "proj:region:inst"},
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
			desc: "enabling a Prometheus port without a namespace",
			args: []string{"--http-port", "1111", "proj:region:inst"},
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

func (*spyDialer) Dial(_ context.Context, inst string, _ ...cloudsqlconn.DialOption) (net.Conn, error) {
	return nil, errors.New("spy dialer does not dial")
}

func (s *spyDialer) EngineVersion(ctx context.Context, inst string) (string, error) {
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

func TestPrometheusMetricsEndpoint(t *testing.T) {
	c := NewCommand(WithDialer(&spyDialer{}))
	// Keep the test output quiet
	c.SilenceUsage = true
	c.SilenceErrors = true
	c.SetArgs([]string{
		"--prometheus-namespace", "prometheus",
		"my-project:my-region:my-instance"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go c.ExecuteContext(ctx)

	// try to dial metrics server for a max of ~10s to give the proxy time to
	// start up.
	tryDial := func(addr string) (*http.Response, error) {
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
	resp, err := tryDial("http://localhost:9090/metrics") // default port set by http-port flag
	if err != nil {
		t.Fatalf("failed to dial metrics endpoint: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected a 200 status, got = %v", resp.StatusCode)
	}
}

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
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/spf13/cobra"
)

func TestNewCommandArgumentsOnLinux(t *testing.T) {
	defaultTmp := filepath.Join(os.TempDir(), "csql-tmp")
	tcs := []struct {
		desc        string
		args        []string
		wantDir     string
		wantTempDir string
	}{
		{
			desc:        "using the fuse flag",
			args:        []string{"--fuse", "/cloudsql"},
			wantDir:     "/cloudsql",
			wantTempDir: defaultTmp,
		},
		{
			desc:        "using the fuse temporary directory flag",
			args:        []string{"--fuse", "/cloudsql", "--fuse-tmp-dir", "/mycooldir"},
			wantDir:     "/cloudsql",
			wantTempDir: "/mycooldir",
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

			if got, want := c.conf.FUSEDir, tc.wantDir; got != want {
				t.Fatalf("FUSEDir: want = %v, got = %v", want, got)
			}

			if got, want := c.conf.FUSETempDir, tc.wantTempDir; got != want {
				t.Fatalf("FUSEDir: want = %v, got = %v", want, got)
			}
		})
	}
}

func TestSdNotifyOnLinux(t *testing.T) {
	tcs := []struct {
		desc          string
		proxyMustFail bool
		notifyState   string
	}{
		{
			desc:          "System with systemd Type=notify and proxy started successfully",
			proxyMustFail: false,
			notifyState:   daemon.SdNotifyReady,
		},
		{
			desc:          "System with systemd Type=notify and proxy failed to start",
			proxyMustFail: true,
			notifyState:   daemon.SdNotifyStopping,
		},
	}

	// Create a temp dir for the socket file.
	testDir, err := os.MkdirTemp("/tmp/", "test-")
	if err != nil {
		t.Fatalf("Fail to create the temp dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	//Set up the socket stream to listen for notifications.
	socketAddr := filepath.Join(testDir, "notify-socket.sock")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: socketAddr, Net: "unixgram"})
	if err != nil {
		t.Fatalf("net.ListenUnixgram error: %v", err)
	}

	// To simulate systemd behavior with Type=notify, set NOTIFY_SOCKET
	// to the name of the socket that listens for notifications.
	os.Setenv("NOTIFY_SOCKET", socketAddr)
	defer os.Unsetenv("NOTIFY_SOCKET")

	s := &spyDialer{}
	c := NewCommand(WithDialer(s))
	// Keep the test output quiet
	c.SilenceUsage = false
	c.SilenceErrors = false
	c.SetArgs([]string{"my-project:my-region:my-instance"})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {

			if tc.proxyMustFail {
				c.conf.FUSEDir = "invalid"
			}

			go c.ExecuteContext(ctx)

			stateReceived := make([]byte, 4096)
			length, _, err := conn.ReadFromUnix(stateReceived)
			if err != nil {
				t.Fatalf("conn.ReadFromUnix error: %s\n", err)
			}
			if string(stateReceived[0:length]) != tc.notifyState {
				t.Fatalf("Expected Notify State %v, got %v", tc.notifyState, string(stateReceived))
			}

		})
	}
}

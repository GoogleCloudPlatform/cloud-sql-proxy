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
	"os"
	"path/filepath"
	"testing"

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

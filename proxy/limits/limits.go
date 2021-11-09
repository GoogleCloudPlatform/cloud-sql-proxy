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

//go:build !windows && !freebsd
// +build !windows,!freebsd

// Package limits provides routines to check and enforce certain resource
// limits on the Cloud SQL client proxy process.
package limits

import (
	"fmt"
	"syscall"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/logging"
)

var (
	// For overriding in unittests.
	syscallGetrlimit = syscall.Getrlimit
	syscallSetrlimit = syscall.Setrlimit
)

// Each connection handled by the proxy requires two file descriptors, one
// for the local end of the connection and one for the remote. So, the proxy
// process should be able to open at least 8K file descriptors if it is to
// handle 4K connections to one instance.
const ExpectedFDs = 8500

// SetupFDLimits ensures that the process running the Cloud SQL proxy can have
// at least wantFDs number of open file descriptors. It returns an error if it
// cannot ensure the same.
func SetupFDLimits(wantFDs uint64) error {
	rlim := &syscall.Rlimit{}
	if err := syscallGetrlimit(syscall.RLIMIT_NOFILE, rlim); err != nil {
		return fmt.Errorf("failed to read rlimit for max file descriptors: %v", err)
	}

	if rlim.Cur >= wantFDs {
		logging.Verbosef("current FDs rlimit set to %d, wanted limit is %d. Nothing to do here.", rlim.Cur, wantFDs)
		return nil
	}

	// Linux man page:
	// The soft limit is the value that the kernel enforces for the corre‐
	// sponding resource. The hard limit acts as a ceiling for the soft limit:
	// an unprivileged process may set only its soft limit to a value in the
	// range from 0 up to the hard limit, and (irreversibly) lower its hard
	// limit. A privileged process (under Linux: one with the CAP_SYS_RESOURCE
	// capability in the initial user namespace) may make arbitrary changes to
	// either limit value.
	if rlim.Max < wantFDs {
		// When the hard limit is less than what is requested, let's just give it a
		// shot, and if we fail, we fallback and try just setting the softlimit.
		rlim2 := &syscall.Rlimit{}
		rlim2.Max = wantFDs
		rlim2.Cur = wantFDs
		if err := syscallSetrlimit(syscall.RLIMIT_NOFILE, rlim2); err == nil {
			logging.Verbosef("Rlimits for file descriptors set to {Current = %v, Max = %v}", rlim2.Cur, rlim2.Max)
			return nil
		}
	}

	rlim.Cur = wantFDs
	if err := syscallSetrlimit(syscall.RLIMIT_NOFILE, rlim); err != nil {
		return fmt.Errorf(
			`failed to set rlimit {Current = %v, Max = %v} for max file
descriptors. The hard limit on file descriptors (4096) is lower than the
requested rlimit. The proxy will only be able to handle ~2048
connections. To hide this message, please request a limit within the available range.`,
			rlim.Cur,
			rlim.Max,
		)
	}

	logging.Verbosef("Rlimits for file descriptors set to {Current = %v, Max = %v}", rlim.Cur, rlim.Max)
	return nil
}

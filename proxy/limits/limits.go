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

// +build !windows

// Package limits provides routines to check and enforce certain resource
// limits on the Cloud SQL client proxy process.
package limits

import (
	"fmt"
	"syscall"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
)

var (
	// For overriding in unittests.
	getRlimitFunc = syscall.Getrlimit
	setRlimitFunc = syscall.Setrlimit
)

// SetupFDLimits ensures that the process running the Cloud SQL proxy can have
// at least wantFDs number of open file descriptors. It returns an error if it
// cannot ensure the same.
func SetupFDLimits(wantFDs uint64) error {
	curr := &syscall.Rlimit{}
	if err := getRlimitFunc(syscall.RLIMIT_NOFILE, curr); err != nil {
		return fmt.Errorf("failed to read rlimit for max file descriptors: %v", err)
	}

	if curr.Max < wantFDs {
		return fmt.Errorf("max FDs rlimit set to %d, cannot support 4K connections. If you are the system administrator, consider increasing this limit.", curr.Max)
	}

	if curr.Cur > wantFDs {
		logging.Infof("current FDs rlimit set to %d, wanted limit is %d. Nothing to do here.", curr.Cur, wantFDs)
		return nil
	}

	rlim := &syscall.Rlimit{}
	rlim.Cur = wantFDs
	if err := setRlimitFunc(syscall.RLIMIT_NOFILE, rlim); err != nil {
		return fmt.Errorf("failed to set rlimit {%v} for max file descriptors: %v", rlim, err)
	}

	logging.Infof("Rlimits for file descriptors set to {%v}", rlim)
	return nil
}

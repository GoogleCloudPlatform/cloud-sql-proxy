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

//go:build !windows
// +build !windows

package limits

import (
	"errors"
	"math"
	"syscall"
	"testing"
)

type rlimitFunc func(int, *syscall.Rlimit) error

func TestSetupFDLimits(t *testing.T) {
	tests := []struct {
		desc    string
		getFunc rlimitFunc
		setFunc rlimitFunc
		wantFDs uint64
		wantErr bool
	}{
		{
			desc: "Getrlimit fails",
			getFunc: func(_ int, _ *syscall.Rlimit) error {
				return errors.New("failed to read rlimit for max file descriptors")
			},
			setFunc: func(_ int, _ *syscall.Rlimit) error {
				panic("shouldn't be called")
			},
			wantFDs: 0,
			wantErr: true,
		},
		{
			desc: "Getrlimit max is less than wantFDs",
			getFunc: func(_ int, rlim *syscall.Rlimit) error {
				rlim.Cur = 512
				rlim.Max = 512
				return nil
			},
			setFunc: func(_ int, rlim *syscall.Rlimit) error {
				if rlim.Cur != 1024 || rlim.Max != 1024 {
					return errors.New("setrlimit called with unexpected value")
				}
				return nil
			},
			wantFDs: 1024,
			wantErr: false,
		},
		{
			desc: "Getrlimit returns rlim_infinity",
			getFunc: func(_ int, rlim *syscall.Rlimit) error {
				rlim.Cur = math.MaxUint64
				rlim.Max = math.MaxUint64
				return nil
			},
			setFunc: func(_ int, _ *syscall.Rlimit) error {
				panic("shouldn't be called")
			},
			wantFDs: 1024,
			wantErr: false,
		},
		{
			desc: "Getrlimit cur is greater than wantFDs",
			getFunc: func(_ int, rlim *syscall.Rlimit) error {
				rlim.Cur = 512
				rlim.Max = 512
				return nil
			},
			setFunc: func(_ int, _ *syscall.Rlimit) error {
				panic("shouldn't be called")
			},
			wantFDs: 256,
			wantErr: false,
		},
		{
			desc: "Setrlimit fails",
			getFunc: func(_ int, rlim *syscall.Rlimit) error {
				rlim.Cur = 128
				rlim.Max = 512
				return nil
			},
			setFunc: func(_ int, _ *syscall.Rlimit) error {
				return errors.New("failed to set rlimit for max file descriptors")
			},
			wantFDs: 256,
			wantErr: true,
		},
		{
			desc: "Success",
			getFunc: func(_ int, rlim *syscall.Rlimit) error {
				rlim.Cur = 128
				rlim.Max = 512
				return nil
			},
			setFunc: func(_ int, _ *syscall.Rlimit) error {
				return nil
			},
			wantFDs: 256,
			wantErr: false,
		},
	}

	for _, test := range tests {
		oldGetFunc := syscallGetrlimit
		syscallGetrlimit = test.getFunc
		defer func() {
			syscallGetrlimit = oldGetFunc
		}()

		oldSetFunc := syscallSetrlimit
		syscallSetrlimit = test.setFunc
		defer func() {
			syscallSetrlimit = oldSetFunc
		}()

		gotErr := SetupFDLimits(test.wantFDs)
		if (gotErr != nil) != test.wantErr {
			t.Errorf("%s: limits.SetupFDLimits(%d) returned error %v, wantErr %v", test.desc, test.wantFDs, gotErr, test.wantErr)
		}
	}
}

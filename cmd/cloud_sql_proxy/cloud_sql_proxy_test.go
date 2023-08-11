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

package main

import (
	"testing"

	"golang.org/x/net/context"
)

func TestAuthenticatedClient(t *testing.T) {
	tcs := []struct {
		desc    string
		setup   func() func()
		wantErr bool
	}{
		{
			desc: "when just token is set",
			setup: func() func() {
				*token = "MYTOKEN"
				return func() {
					*token = ""
				}
			},
			wantErr: false,
		},
		{
			desc: "when just login_token is set",
			setup: func() func() {
				*loginToken = "MYTOKEN"
				return func() {
					*loginToken = ""
				}
			},
			wantErr: true,
		},
		{
			desc: "when token and enable_iam_login are set",
			setup: func() func() {
				*token = "MYTOKEN"
				*enableIAMLogin = true
				return func() {
					*token = ""
					*enableIAMLogin = false
				}
			},
			wantErr: true,
		},
		{
			desc: "when token, login_token, and enable_iam_login are set",
			setup: func() func() {
				*token = "MYTOKEN"
				*loginToken = "MYLOGINTOKEN"
				*enableIAMLogin = true
				return func() {
					*token = ""
					*loginToken = ""
					*enableIAMLogin = false
				}
			},
			wantErr: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			cleanup := tc.setup()
			defer cleanup()
			_, _, err := authenticatedClient(context.Background())
			gotErr := err != nil
			if tc.wantErr != gotErr {
				t.Fatalf("err: want = %v, got = %v", tc.wantErr, gotErr)
			}
		})
	}
}

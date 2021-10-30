// Copyright 2021 Google LLC
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

package certs_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/certs"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

func TestCertDurationConfiguration(t *testing.T) {
	testCases := []struct {
		in   time.Duration
		want time.Duration
		desc string
	}{
		{in: time.Duration(0), want: time.Hour, desc: "when no value is provided"},
		{in: 59 * time.Minute, want: time.Hour, desc: "when too short"},
		{in: 25 * time.Hour, want: time.Hour, desc: "when too long"},
		{in: time.Hour, want: time.Hour, desc: "when at the minimum"},
		{in: 24 * time.Hour, want: 24 * time.Hour, desc: "when at the maximum"},
	}
	for _, tc := range testCases {
		s := certs.NewCertSourceOpts(http.DefaultClient, certs.RemoteOpts{CertDuration: tc.in})
		if s.CertDuration != tc.want {
			t.Errorf("want = %v, got = %v", tc.want, s.CertDuration)
		}
	}
}

type spyRoundTripper struct {
	got *http.Request
}

func (rt *spyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.got = req
	return nil, errors.New("fail")
}

func (rt *spyRoundTripper) jsonBody(val interface{}) error {
	data, err := ioutil.ReadAll(rt.got.Body)
	if err != nil {
		return err
	}
	rt.got.Body.Close()
	if err := json.Unmarshal(data, val); err != nil {
		return err
	}
	return nil
}

func TestEphemeralCertRequestCertDuration(t *testing.T) {
	testCases := []struct {
		in   time.Duration
		want string
		desc string
	}{
		{in: time.Duration(0), want: "3600s", desc: "when no value is provided, use default"},
		{in: time.Hour, want: "3600s", desc: "1 hour"},
		{in: 24 * time.Hour, want: "86400s", desc: "24 hours"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rt := &spyRoundTripper{}
			cs := certs.NewCertSourceOpts(
				&http.Client{Transport: rt}, certs.RemoteOpts{CertDuration: tc.in},
			)

			// Trigger SQL Admin API request
			cs.Local("proj:reg:inst")

			var certReq sqladmin.GenerateEphemeralCertRequest
			if err := rt.jsonBody(&certReq); err != nil {
				t.Fatalf("failed to unmarshal JSON body: %v", err)
			}
			if got := certReq.ValidDuration; got != tc.want {
				t.Fatalf("want = %q, got = %q", tc.want, got)
			}
		})
	}
}

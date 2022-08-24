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

package gcloud_test

import (
	"testing"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/gcloud"
)

func TestGcloud(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping gcloud integration tests")
	}

	// gcloud is configured. Try to obtain a token from gcloud config
	// helper.
	ts, err := gcloud.TokenSource()
	if err != nil {
		t.Fatalf("failed to get token source: %v", err)
	}

	_, err = ts.Token()
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}
}

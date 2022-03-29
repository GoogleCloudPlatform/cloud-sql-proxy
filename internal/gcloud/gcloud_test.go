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
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/gcloud"
	exec "golang.org/x/sys/execabs"
)

func TestGcloud(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping gcloud integration tests")
	}

	configureGcloud := func(t *testing.T) func() {
		// The following configures gcloud using only GOOGLE_APPLICATION_CREDENTIALS.
		dir, err := ioutil.TempDir("", "cloudsdk*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		os.Setenv("CLOUDSDK_CONFIG", dir)

		gcloudCmd, err := gcloud.Cmd()
		if err != nil {
			t.Fatal(err)
		}

		keyFile, ok := os.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
		if !ok {
			t.Fatal("GOOGLE_APPLICATION_CREDENTIALS is not set in the environment")
		}
		os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

		buf := &bytes.Buffer{}
		cmd := exec.Command(gcloudCmd, "auth", "activate-service-account", "--key-file", keyFile)
		cmd.Stdout = buf

		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to active service account. err = %v, message = %v", err, buf.String())
		}

		return func() {
			os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyFile)
			os.Unsetenv("CLOUDSDK_CONFIG")
		}

	}
	cleanup := configureGcloud(t)
	defer cleanup()

	// gcloud is now configured. Try to obtain a token from gcloud config
	// helper.
	ts, err := gcloud.GcloudTokenSource(context.Background())
	if err != nil {
		t.Fatalf("failed to get token source: %v", err)
	}

	_, err = ts.Token()
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}
}

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

package testutil

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/gcloud"
)

// ConfigureGcloud configures gcloud using only GOOGLE_APPLICATION_CREDENTIALS
// and stores the resulting configuration in a temporary directory as set by
// CLOUDSDK_CONFIG, which changes the gcloud config directory from the
// default. We use a temporary directory to avoid trampling on any existing
// gcloud config.
func ConfigureGcloud(t *testing.T) func() {
	dir, err := ioutil.TempDir("", "cloudsdk*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	os.Setenv("CLOUDSDK_CONFIG", dir)

	gcloudCmd, err := gcloud.Path()
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

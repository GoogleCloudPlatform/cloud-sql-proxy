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

package gcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/logging"
	"golang.org/x/oauth2"
	exec "golang.org/x/sys/execabs"
)

// gcloudConfigData represents the data returned by `gcloud config config-helper`.
type gcloudConfigData struct {
	Configuration struct {
		Properties struct {
			Core struct {
				Project string
				Account string
			}
		}
	}
	Credential struct {
		AccessToken string    `json:"access_token"`
		TokenExpiry time.Time `json:"token_expiry"`
	}
}

func (cfg *gcloudConfigData) oauthToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken: cfg.Credential.AccessToken,
		Expiry:      cfg.Credential.TokenExpiry,
	}
}

type GcloudStatusCode int

const (
	GcloudOk GcloudStatusCode = iota
	GcloudNotFound
	// generic execution failure error not specified above.
	GcloudExecErr
)

type GcloudError struct {
	GcloudError error
	Status      GcloudStatusCode
}

func (e *GcloudError) Error() string {
	return e.GcloudError.Error()
}

// Cmd returns the gcloud command. If the command is not found it returns an
// error.
func Cmd() (string, error) {
	gcloud := "gcloud"
	if runtime.GOOS == "windows" {
		gcloud = gcloud + ".cmd"
	}

	if _, err := exec.LookPath(gcloud); err != nil {
		return "", &GcloudError{GcloudError: err, Status: GcloudNotFound}
	}
	return gcloud, nil

}

// gcloudConfig returns a gcloudConfigData object or an error of type *GcloudError.
func gcloudConfig() (*gcloudConfigData, error) {
	gcloudCmd, err := Cmd()
	if err != nil {
		return nil, err
	}
	buf, errbuf := new(bytes.Buffer), new(bytes.Buffer)
	cmd := exec.Command(gcloudCmd, "--format", "json", "config", "config-helper", "--min-expiry", "1h")
	cmd.Stdout = buf
	cmd.Stderr = errbuf

	if err := cmd.Run(); err != nil {
		err = fmt.Errorf("error reading config: %v; stderr was:\n%v", err, errbuf)
		logging.Errorf("GcloudConfig: %v", err)
		return nil, &GcloudError{err, GcloudExecErr}
	}

	data := &gcloudConfigData{}
	if err := json.Unmarshal(buf.Bytes(), data); err != nil {
		logging.Errorf("Failed to unmarshal bytes from gcloud: %v", err)
		logging.Errorf("   gcloud returned:\n%s", buf)
		return nil, &GcloudError{err, GcloudExecErr}
	}

	return data, nil
}

// gcloudTokenSource implements oauth2.TokenSource via the `gcloud config config-helper` command.
type gcloudTokenSource struct {
}

// Token helps gcloudTokenSource implement oauth2.TokenSource.
func (src *gcloudTokenSource) Token() (*oauth2.Token, error) {
	cfg, err := gcloudConfig()
	if err != nil {
		return nil, err
	}
	return cfg.oauthToken(), nil
}

func GcloudTokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	src := &gcloudTokenSource{}
	tok, err := src.Token()
	if err != nil {
		return nil, err
	}
	return oauth2.ReuseTokenSource(tok, src), nil
}
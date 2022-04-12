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
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	exec "golang.org/x/sys/execabs"
)

// config represents the credentials returned by `gcloud config config-helper`.
type config struct {
	Credential struct {
		AccessToken string    `json:"access_token"`
		TokenExpiry time.Time `json:"token_expiry"`
	}
}

func (c *config) Token() *oauth2.Token {
	return &oauth2.Token{
		AccessToken: c.Credential.AccessToken,
		Expiry:      c.Credential.TokenExpiry,
	}
}

// Path returns the absolute path to the gcloud command. If the command is not
// found it returns an error.
func Path() (string, error) {
	g := "gcloud"
	if runtime.GOOS == "windows" {
		g = g + ".cmd"
	}
	return exec.LookPath(g)
}

// configHelper implements oauth2.TokenSource via the `gcloud config config-helper` command.
type configHelper struct{}

// Token helps gcloudTokenSource implement oauth2.TokenSource.
func (configHelper) Token() (*oauth2.Token, error) {
	gcloudCmd, err := Path()
	if err != nil {
		return nil, err
	}
	buf, errbuf := new(bytes.Buffer), new(bytes.Buffer)
	cmd := exec.Command(gcloudCmd, "--format", "json", "config", "config-helper", "--min-expiry", "1h")
	cmd.Stdout = buf
	cmd.Stderr = errbuf

	if err := cmd.Run(); err != nil {
		err = fmt.Errorf("error reading config: %v; stderr was:\n%v", err, errbuf)
		return nil, err
	}

	c := &config{}
	if err := json.Unmarshal(buf.Bytes(), c); err != nil {
		return nil, err
	}
	return c.Token(), nil
}

// TokenSource returns an oauth2.TokenSource backed by the gcloud CLI.
func TokenSource() (oauth2.TokenSource, error) {
	h := configHelper{}
	tok, err := h.Token()
	if err != nil {
		return nil, err
	}
	return oauth2.ReuseTokenSource(tok, h), nil
}

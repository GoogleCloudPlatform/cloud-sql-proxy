// Copyright 2018 Google Inc. All Rights Reserved.
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

// Move clientFromCredentials method to common_open_source.go from common.go

package tests

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2"
)

func clientFromCredentials(ctx context.Context) (*http.Client, error) {
	if f := *credentialFile; f != "" {
		all, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("invalid json file %q: %v", f, err)
		}
		cfg, err := google.JWTConfigFromJSON(all, proxy.SQLScope)
		if err != nil {
			return nil, fmt.Errorf("invalid json file %q: %v", f, err)
		}
		return cfg.Client(ctx), nil
	} else if tok := *token; tok != "" {
		src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
		return oauth2.NewClient(ctx, src), nil
	}
	return google.DefaultClient(ctx, proxy.SQLScope)
}

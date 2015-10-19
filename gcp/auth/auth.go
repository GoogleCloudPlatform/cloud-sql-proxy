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

// Package auth provides an HTTP client that authenticates using OAuth2.
package auth

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/user"
	"time"

	"flag"
	"gcp/metadata"
	"log"
	"util/prettyprint"

	"golang.org/x/oauth2"
)

var (
	verifyHost = flag.Bool("verify_host", true, "Set to true to check hostnames when connecting with SSL. Only set to false for test environments.")

	fetchAccessToken = metadata.AccessToken
)

// CancelTripper represents a RoundTripper that supports timeouts via
// CancelRequest.
type CancelTripper interface {
	http.RoundTripper

	CancelRequest(*http.Request)
}

// NewGCEToken fetches a token retrieved from the GCE metadata server. It also
// returns the time at which the token should be considered expired.
func NewGCEToken() (string, time.Time, error) {
	var t struct {
		Token   string `json:"access_token"`
		Expires int64  `json:"expires_in"`
	}

	if err := fetchAccessToken.JSON(&t); err != nil {
		return "", time.Time{}, err
	}

	// It is safe to assume the timeout is more than a minute because the
	// metadata server will issue a new token 5 minutes in advance.  We do this
	// so that doing a request with this token never races with token
	// expiration.
	exp := time.Duration(t.Expires)*time.Second - 1*time.Minute
	return t.Token, time.Now().Add(exp), nil
}

var _ oauth2.TokenSource = tokenFunc(nil)

type tokenFunc func() (*oauth2.Token, error)

func (t tokenFunc) Token() (*oauth2.Token, error) {
	return t()
}

func gceToken() (*oauth2.Token, error) {
	tok, exp, err := NewGCEToken()
	if err != nil {
		return nil, err
	}

	return &oauth2.Token{
		AccessToken: tok,
		Expiry:      exp,
	}, nil
}

// sourceForToken returns a TokenSource based on the following rules:
//   - if token is not "" it is used as the bearer token for the source
//   - if the binary is running on GCE, tokens will be retrieved from the metadata server
//   - otherwise, a source is returned which returns an error each time it is used.
func sourceForToken(token string) oauth2.TokenSource {
	if token != "" {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	}

	tok, err := gceToken()
	if err == nil {
		return oauth2.ReuseTokenSource(tok, tokenFunc(gceToken))
	}
	log.Printf("Can't find GCE metadata server: %v", err)
	return tokenFunc(func() (*oauth2.Token, error) {
		return nil, errors.New("must provide a token on command line or run on GCE")
	})
}

func newAuthenticatedClient(tokenSrc oauth2.TokenSource, transport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: &oauth2.Transport{
			Base:   transport,
			Source: tokenSrc,
		},
	}
}

// WrapTransport optionally specifies a func to be called each time an
// http.Transport is constructed during calls to NewClient, etc. The returned
// CancelTripper is used in the chain of RoundTrippers for requests that run
// through the authenticated client. It is expected that the CancelTripper
// returned from this func eventually calls the RoundTrip method on the
// CancelTripper passed to this func.
var WrapTransport func(CancelTripper) CancelTripper

// transport returns an http.Transport that is configured according to flags
// passed to the binary. It sets whether TLS functions should verify the host
// as well as what HTTP requests should be retried.
func transport() CancelTripper {
	base := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !*verifyHost},
	}
	logging := &prettyprint.LoggingTransport{
		CancelTripper: base,
	}
	if false {
		logging.Logf = log.Printf
	}
	if WrapTransport != nil {
		return WrapTransport(logging)
	}
	return logging
}

// NewClientFrom returns an http client which will use the provided func
// as a source for oauth tokens. The underlying transport handles automatic
// retries and logging that is useful for integration tests and agents.
func NewClientFrom(src func() (*oauth2.Token, error)) *http.Client {
	// Wrapping in a ReuseTokenSource will cache the returned token so that src
	// is only called when a new token is needed.
	return newAuthenticatedClient(
		oauth2.ReuseTokenSource(nil, tokenFunc(src)),
		transport(),
	)
}

// NewAuthenticatedClient returns an http.Client authorized under the provided
// bearer token. If a token is passed, the client will use that token as
// its authentication credentials. If an empty string is given for the
// token one will be retrieved from the GCE Metadata server if possible.
func NewAuthenticatedClient(token string) *http.Client {
	return newAuthenticatedClient(sourceForToken(token), transport())
}

// GcloudToken returns the first unexpired access token it finds in the
// provided credential file. If the file parameter is "", the current user's
// gcloud credentials from the file "~/.config/gcloud/credentials" is used.
func GcloudToken(file string) (string, error) {
	if file == "" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("user.Current() error: %v", err)
		}
		file = usr.HomeDir + "/.config/gcloud/credentials"
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("ioutil.ReadFile(%v) error: %v", file, err)
	}

	return parseCredential(data)
}

func parseCredential(in []byte) (string, error) {
	var creds struct {
		Data []struct {
			Credential struct {
				AccessToken string    `json:"access_token"`
				Expires     time.Time `json:"token_expiry"`
			} `json:"credential"`
		} `json:"data"`
	}
	if err := json.Unmarshal(in, &creds); err != nil {
		return "", fmt.Errorf("json.Unmarshal(%s, %+v) error: %v", in, creds, err)
	}

	now := time.Now()
	for _, cred := range creds.Data {
		if cred.Credential.Expires.After(now) {
			return cred.Credential.AccessToken, nil
		}
	}

	return "", errors.New("no valid credentials found")
}

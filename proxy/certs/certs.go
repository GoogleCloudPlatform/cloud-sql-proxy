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

// Package certs implements a CertSource which speaks to the public Cloud SQL API endpoint.
package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	mrand "math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

const defaultUserAgent = "custom cloud_sql_proxy version >= 1.10"

// NewCertSource returns a CertSource which can be used to authenticate using
// the provided client, which must not be nil.
//
// This function is deprecated; use NewCertSourceOpts instead.
func NewCertSource(host string, c *http.Client, checkRegion bool) *RemoteCertSource {
	return NewCertSourceOpts(c, RemoteOpts{
		APIBasePath:  host,
		IgnoreRegion: !checkRegion,
		UserAgent:    defaultUserAgent,
	})
}

// RemoteOpts are a collection of options for NewCertSourceOpts. All fields are
// optional.
type RemoteOpts struct {
	// APIBasePath specifies the base path for the sqladmin API. If left blank,
	// the default from the autogenerated sqladmin library is used (which is
	// sufficient for nearly all users)
	APIBasePath string

	// IgnoreRegion specifies whether a missing or mismatched region in the
	// instance name should be ignored. In a future version this value will be
	// forced to 'false' by the RemoteCertSource.
	IgnoreRegion bool

	// A string for the RemoteCertSource to identify itself when contacting the
	// sqladmin API.
	UserAgent string

	// IP address type options
	IPAddrTypeOpts []string

	// Enable IAM proxy db authentication
	EnableIAMLogin bool

	// Token source for token information used in cert creation
	TokenSource oauth2.TokenSource

	// DelayKeyGenerate, if true, causes the RSA key to be generated lazily
	// on the first connection to a database. The default behavior is to generate
	// the key when the CertSource is created.
	DelayKeyGenerate bool
}

// NewCertSourceOpts returns a CertSource configured with the provided Opts.
// The provided http.Client must not be nil.
//
// Use this function instead of NewCertSource; it has a more forward-compatible
// signature.
func NewCertSourceOpts(c *http.Client, opts RemoteOpts) *RemoteCertSource {
	serv, err := sqladmin.New(c)
	if err != nil {
		panic(err) // Only will happen if the provided client is nil.
	}
	if opts.APIBasePath != "" {
		serv.BasePath = opts.APIBasePath
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	serv.UserAgent = ua

	// Set default value to be "PUBLIC,PRIVATE" if not specified
	if len(opts.IPAddrTypeOpts) == 0 {
		opts.IPAddrTypeOpts = []string{"PUBLIC", "PRIVATE"}
	}

	// Add "PUBLIC" as an alias for "PRIMARY"
	for index, ipAddressType := range opts.IPAddrTypeOpts {
		if strings.ToUpper(ipAddressType) == "PUBLIC" {
			opts.IPAddrTypeOpts[index] = "PRIMARY"
		}
	}

	certSource := &RemoteCertSource{
		serv:           serv,
		checkRegion:    !opts.IgnoreRegion,
		IPAddrTypes:    opts.IPAddrTypeOpts,
		EnableIAMLogin: opts.EnableIAMLogin,
		TokenSource:    opts.TokenSource,
	}
	if !opts.DelayKeyGenerate {
		// Generate the RSA key now, but don't block on it.
		go certSource.generateKey()
	}

	return certSource
}

// RemoteCertSource implements a CertSource, using Cloud SQL APIs to
// return Local certificates for identifying oneself as a specific user
// to the remote instance and Remote certificates for confirming the
// remote database's identity.
type RemoteCertSource struct {
	// keyOnce is used to create `key` lazily.
	keyOnce sync.Once
	// key is the private key used for certificates returned by Local.
	key *rsa.PrivateKey
	// serv is used to make authenticated API calls to Cloud SQL.
	serv *sqladmin.Service
	// If set, providing an incorrect region in their connection string will be
	// treated as an error. This is to provide the same functionality that will
	// occur when API calls require the region.
	checkRegion bool
	// a list of ip address types that users select
	IPAddrTypes []string
	// flag to enable IAM proxy db authentication
	EnableIAMLogin bool
	// token source for the token information used in cert creation
	TokenSource oauth2.TokenSource
}

// Constants for backoffAPIRetry. These cause the retry logic to scale the
// backoff delay from 200ms to around 3.5s.
const (
	baseBackoff    = float64(200 * time.Millisecond)
	backoffMult    = 1.618
	backoffRetries = 5
)

// now returns the current time in UTC. It is defined as a var so tests can
// replace it with a fixed return value.
var now = func() time.Time {
	return time.Now().UTC()
}

func backoffAPIRetry(desc, instance string, do func(staleRead time.Time) error) error {
	var (
		err error
		t   time.Time
	)
	for i := 0; i < backoffRetries; i++ {
		err = do(t)
		gErr, ok := err.(*googleapi.Error)
		switch {
		case !ok:
			// 'ok' will also be false if err is nil.
			return err
		case gErr.Code == 403 && len(gErr.Errors) > 0 && gErr.Errors[0].Reason == "insufficientPermissions":
			// The case where the admin API has not yet been enabled.
			return fmt.Errorf("ensure that the Cloud SQL API is enabled for your project (https://console.cloud.google.com/flows/enableapi?apiid=sqladmin). Error during %s %s: %v", desc, instance, err)
		case gErr.Code == 404 || gErr.Code == 403:
			return fmt.Errorf("ensure that the account has access to %q (and make sure there's no typo in that name). Error during %s %s: %v", instance, desc, instance, err)
		case gErr.Code < 500:
			// Only Server-level HTTP errors are immediately retryable.
			return err
		}

		// sleep = baseBackoff * backoffMult^(retries + randomFactor)
		exp := float64(i+1) + mrand.Float64()
		sleep := time.Duration(baseBackoff * math.Pow(backoffMult, exp))
		logging.Errorf("Error in %s %s: %v; retrying in %v", desc, instance, err, sleep)
		time.Sleep(sleep)
		// Create timestamp 30 seconds before now for stale read requests
		t = now().Add(-30 * time.Second)
	}
	return err
}

func refreshToken(ts oauth2.TokenSource, tok *oauth2.Token) (*oauth2.Token, error) {
	expiredToken := &oauth2.Token{
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		RefreshToken: tok.RefreshToken,
		Expiry:       time.Time{}.Add(1), // Expired
	}
	return oauth2.ReuseTokenSource(expiredToken, ts).Token()
}

// Local returns a certificate that may be used to establish a TLS
// connection to the specified instance.
func (s *RemoteCertSource) Local(instance string) (tls.Certificate, error) {
	pkix, err := x509.MarshalPKIXPublicKey(s.generateKey().Public())
	if err != nil {
		return tls.Certificate{}, err
	}

	p, r, n := util.SplitName(instance)
	regionName := fmt.Sprintf("%s~%s", r, n)
	pubKey := string(pem.EncodeToMemory(&pem.Block{Bytes: pkix, Type: "RSA PUBLIC KEY"}))
	generateEphemeralCertRequest := &sqladmin.GenerateEphemeralCertRequest{
		PublicKey: pubKey,
	}
	var tok *oauth2.Token
	// If IAM login is enabled, add the OAuth2 token into the ephemeral
	// certificate request.
	if s.EnableIAMLogin {
		var tokErr error
		tok, tokErr = s.TokenSource.Token()
		if tokErr != nil {
			return tls.Certificate{}, tokErr
		}
		// Always refresh the token to ensure its expiration is far enough in
		// the future.
		tok, tokErr = refreshToken(s.TokenSource, tok)
		if tokErr != nil {
			return tls.Certificate{}, tokErr
		}
		// TODO: remove this once issue with OAuth2 Tokens is resolved.
		// See https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/852.
		generateEphemeralCertRequest.AccessToken = strings.TrimRight(tok.AccessToken, ".")
	}
	req := s.serv.Connect.GenerateEphemeralCert(p, regionName, generateEphemeralCertRequest)

	var data *sqladmin.GenerateEphemeralCertResponse
	err = backoffAPIRetry("generateEphemeral for", instance, func(staleRead time.Time) error {
		if !staleRead.IsZero() {
			generateEphemeralCertRequest.ReadTime = staleRead.Format(time.RFC3339)
		}
		data, err = req.Do()
		return err
	})
	if err != nil {
		return tls.Certificate{}, err
	}

	c, err := parseCert(data.EphemeralCert.Cert)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("couldn't parse ephemeral certificate for instance %q: %v", instance, err)
	}

	if s.EnableIAMLogin {
		// Adjust the certificate's expiration to be the earlier of tok.Expiry or c.NotAfter
		if tok.Expiry.Before(c.NotAfter) {
			c.NotAfter = tok.Expiry
		}
	}
	return tls.Certificate{
		Certificate: [][]byte{c.Raw},
		PrivateKey:  s.generateKey(),
		Leaf:        c,
	}, nil
}

func parseCert(pemCert string) (*x509.Certificate, error) {
	bl, _ := pem.Decode([]byte(pemCert))
	if bl == nil {
		return nil, errors.New("invalid PEM: " + pemCert)
	}
	return x509.ParseCertificate(bl.Bytes)
}

// Return the RSA private key, which is lazily initialized.
func (s *RemoteCertSource) generateKey() *rsa.PrivateKey {
	s.keyOnce.Do(func() {
		start := time.Now()
		pkey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err) // very unexpected.
		}
		logging.Verbosef("Generated RSA key in %v", time.Since(start))
		s.key = pkey
	})
	return s.key
}

// Find the first matching IP address by user input IP address types
func (s *RemoteCertSource) findIPAddr(data *sqladmin.ConnectSettings, instance string) (ipAddrInUse string, err error) {
	for _, eachIPAddrTypeByUser := range s.IPAddrTypes {
		for _, eachIPAddrTypeOfInstance := range data.IpAddresses {
			if strings.ToUpper(eachIPAddrTypeOfInstance.Type) == strings.ToUpper(eachIPAddrTypeByUser) {
				ipAddrInUse = eachIPAddrTypeOfInstance.IpAddress
				return ipAddrInUse, nil
			}
		}
	}

	ipAddrTypesOfInstance := ""
	for _, eachIPAddrTypeOfInstance := range data.IpAddresses {
		ipAddrTypesOfInstance += fmt.Sprintf("(TYPE=%v, IP_ADDR=%v)", eachIPAddrTypeOfInstance.Type, eachIPAddrTypeOfInstance.IpAddress)
	}

	ipAddrTypeOfUser := fmt.Sprintf("%v", s.IPAddrTypes)

	return "", fmt.Errorf("User input IP address type %v does not match the instance %v, the instance's IP addresses are %v ", ipAddrTypeOfUser, instance, ipAddrTypesOfInstance)
}

// Remote returns the specified instance's CA certificate, address, and name.
func (s *RemoteCertSource) Remote(instance string) (cert *x509.Certificate, addr, name, version string, err error) {
	p, region, n := util.SplitName(instance)
	regionName := fmt.Sprintf("%s~%s", region, n)
	req := s.serv.Connect.Get(p, regionName)

	var data *sqladmin.ConnectSettings
	err = backoffAPIRetry("get instance", instance, func(staleRead time.Time) error {
		if !staleRead.IsZero() {
			req.ReadTime(staleRead.Format(time.RFC3339))
		}
		data, err = req.Do()
		return err
	})
	if err != nil {
		return nil, "", "", "", err
	}

	// TODO(chowski): remove this when us-central is removed.
	if data.Region == "us-central" {
		data.Region = "us-central1"
	}
	if data.Region != region {
		if region == "" {
			err = fmt.Errorf("instance %v doesn't provide region", instance)
		} else {
			err = fmt.Errorf(`for connection string "%s": got region %q, want %q`, instance, region, data.Region)
		}
		if s.checkRegion {
			return nil, "", "", "", err
		}
		logging.Errorf("%v", err)
		logging.Errorf("WARNING: specifying the correct region in an instance string will become required in a future version!")
	}

	if len(data.IpAddresses) == 0 {
		return nil, "", "", "", fmt.Errorf("no IP address found for %v", instance)
	}
	if data.BackendType == "FIRST_GEN" {
		logging.Errorf("WARNING: proxy client does not support first generation Cloud SQL instances.")
		return nil, "", "", "", fmt.Errorf("%q is a first generation instance", instance)
	}

	// Find the first matching IP address by user input IP address types
	ipAddrInUse := ""
	ipAddrInUse, err = s.findIPAddr(data, instance)
	if err != nil {
		return nil, "", "", "", err
	}

	c, err := parseCert(data.ServerCaCert.Cert)

	return c, ipAddrInUse, p + ":" + n, data.DatabaseVersion, err
}

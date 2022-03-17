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

package proxy_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"google.golang.org/api/sqladmin/v1"
)

// httpClient returns an *http.Client, URL, and cleanup function. The http.Client is
// configured to connect to test SSL Server at the returned URL. This server will
// respond to HTTP requests defined, or return a 5xx server error for unexpected ones.
// The cleanup function will close the server, and return an error if any expected calls
// weren't received.
func httpClient(requests ...*request) (*http.Client, string, func() error) {
	// Create a TLS Server that responses to the requests defined
	s := httptest.NewTLSServer(http.HandlerFunc(
		func(resp http.ResponseWriter, req *http.Request) {
			for _, r := range requests {
				if r.matches(req) {
					r.handle(resp, req)
					return
				}
			}
			// Unexpected requests should throw an error
			resp.WriteHeader(http.StatusNotImplemented)
			// TODO: follow error format better?
			resp.Write([]byte(fmt.Sprintf("unexpected request sent to mock client: %v", req)))
		},
	))
	// cleanup stops the test server and checks for uncalled requests
	cleanup := func() error {
		s.Close()
		for i, e := range requests {
			if e.reqCt > 0 {
				return fmt.Errorf("%d calls left for specified call in pos %d: %v", e.reqCt, i, e)
			}
		}
		return nil
	}

	return s.Client(), s.URL, cleanup

}

// request represents a HTTP request for a test Server to mock responses for.
//
// Use NewRequest to initialize new Requests.
type request struct {
	sync.Mutex

	reqMethod string
	reqPath   string
	reqCt     int

	handle func(resp http.ResponseWriter, req *http.Request)
}

// matches returns true if a given http.Request should be handled by this Request.
func (r *request) matches(hR *http.Request) bool {
	r.Lock()
	defer r.Unlock()
	if r.reqMethod != "" && r.reqMethod != hR.Method {
		return false
	}
	if r.reqPath != "" && r.reqPath != hR.URL.Path {
		return false
	}
	if r.reqCt <= 0 {
		return false
	}
	r.reqCt--
	return true
}

// fakeCSQLInstance represents settings for a specific Cloud SQL instance.
type fakeCSQLInstance struct {
	project   string
	region    string
	name      string
	dbVersion string
	// ipAddrs is a map of IP type (PUBLIC or PRIVATE) to IP address.
	ipAddrs     map[string]string
	backendType string
	Key         *rsa.PrivateKey
	Cert        *x509.Certificate
}

func (f fakeCSQLInstance) String() string {
	return fmt.Sprintf("%s:%s:%s", f.project, f.region, f.name)
}

// newFakeCSQLInstance returns a CloudSQLInst object for configuring mocks.
func newFakeCSQLInstance(project, region, name, dbVersion string) fakeCSQLInstance {
	key, cert, err := generateCerts(project, name)
	if err != nil {
		panic(err)
	}

	return fakeCSQLInstance{
		project:     project,
		region:      region,
		name:        name,
		ipAddrs:     map[string]string{"PUBLIC": "0.0.0.0"},
		dbVersion:   dbVersion,
		backendType: "SECOND_GEN",
		Key:         key,
		Cert:        cert,
	}
}

func (f fakeCSQLInstance) signedCert() ([]byte, error) {
	certBytes, err := x509.CreateCertificate(rand.Reader, f.Cert, f.Cert, &f.Key.PublicKey, f.Key)
	if err != nil {
		return nil, err
	}
	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	return certPEM.Bytes(), nil
}

func (f fakeCSQLInstance) clientCert(pubKey *rsa.PublicKey) ([]byte, error) {
	// Create a signed cert from the client's public key.
	cert := &x509.Certificate{ // TODO: Validate this format vs API
		SerialNumber: &big.Int{},
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"Google, Inc"},
			CommonName:   "Google Cloud SQL Client",
		},
		NotBefore:             time.Now(),
		NotAfter:              f.Cert.NotAfter,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, f.Cert, pubKey, f.Key)
	if err != nil {
		return nil, err
	}
	certPEM := new(bytes.Buffer)
	err = pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	if err != nil {
		return nil, err
	}
	return certPEM.Bytes(), nil
}

// generateCerts generates a private key, an X.509 certificate, and a TLS
// certificate for a particular fake Cloud SQL database instance.
func generateCerts(project, name string) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	cert := &x509.Certificate{
		SerialNumber: &big.Int{},
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s:%s", project, name),
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	return key, cert, nil
}

// instanceGetSuccess returns a Request that responds to the `instance.get` SQL Admin
// endpoint. It responds with a "StatusOK" and a DatabaseInstance object.
//
// https://cloud.google.com/sql/docs/mysql/admin-api/rest/v1beta4/instances/get
func instanceGetSuccess(i fakeCSQLInstance, ct int) *request {
	var ips []*sqladmin.IpMapping
	for ipType, addr := range i.ipAddrs {
		if ipType == "PUBLIC" {
			ips = append(ips, &sqladmin.IpMapping{IpAddress: addr, Type: "PRIMARY"})
			continue
		}
		if ipType == "PRIVATE" {
			ips = append(ips, &sqladmin.IpMapping{IpAddress: addr, Type: "PRIVATE"})
		}
	}
	certBytes, err := i.signedCert()
	if err != nil {
		panic(err)
	}
	db := &sqladmin.DatabaseInstance{
		BackendType:     i.backendType,
		ConnectionName:  fmt.Sprintf("%s:%s:%s", i.project, i.region, i.name),
		DatabaseVersion: i.dbVersion,
		Project:         i.project,
		Region:          i.region,
		Name:            i.name,
		IpAddresses:     ips,
		ServerCaCert:    &sqladmin.SslCert{Cert: string(certBytes)},
	}

	r := &request{
		reqMethod: http.MethodGet,
		reqPath:   fmt.Sprintf("/sql/v1beta4/projects/%s/instances/%s", i.project, i.name),
		reqCt:     ct,
		handle: func(resp http.ResponseWriter, req *http.Request) {
			b, err := db.MarshalJSON()
			if err != nil {
				http.Error(resp, err.Error(), http.StatusInternalServerError)
				return
			}
			resp.WriteHeader(http.StatusOK)
			resp.Write(b)
		},
	}
	return r
}

// createEphemeralSuccess returns a Request that responds to the
// `connect.generateEphemeralCert` SQL Admin endpoint. It responds with a
// "StatusOK" and a SslCerts object.
//
// https://cloud.google.com/sql/docs/mysql/admin-api/rest/v1beta4/connect/generateEphemeralCert
func createEphemeralSuccess(i fakeCSQLInstance, ct int) *request {
	r := &request{
		reqMethod: http.MethodPost,
		reqPath:   fmt.Sprintf("/sql/v1beta4/projects/%s/instances/%s:generateEphemeralCert", i.project, i.name),
		reqCt:     ct,
		handle: func(resp http.ResponseWriter, req *http.Request) {
			// Read the body from the request.
			b, err := ioutil.ReadAll(req.Body)
			defer req.Body.Close()
			if err != nil {
				http.Error(resp, fmt.Errorf("unable to read body: %w", err).Error(), http.StatusBadRequest)
				return
			}
			var eR sqladmin.GenerateEphemeralCertRequest
			err = json.Unmarshal(b, &eR)
			if err != nil {
				http.Error(resp, fmt.Errorf("invalid or unexpected json: %w", err).Error(), http.StatusBadRequest)
				return
			}
			// Extract the certificate from the request.
			bl, _ := pem.Decode([]byte(eR.PublicKey))
			if bl == nil {
				http.Error(resp, fmt.Errorf("unable to decode PublicKey: %w", err).Error(), http.StatusBadRequest)
				return
			}
			pubKey, err := x509.ParsePKIXPublicKey(bl.Bytes)
			if err != nil {
				http.Error(resp, fmt.Errorf("unable to decode PublicKey: %w", err).Error(), http.StatusBadRequest)
				return
			}

			certBytes, err := i.clientCert(pubKey.(*rsa.PublicKey))
			if err != nil {
				http.Error(resp, fmt.Errorf("failed to sign client certificate: %v", err).Error(), http.StatusBadRequest)
				return
			}

			// Return the signed cert to the client.
			c := &sqladmin.SslCert{
				Cert:           string(certBytes),
				CommonName:     "Google Cloud SQL Client",
				CreateTime:     time.Now().Format(time.RFC3339),
				ExpirationTime: i.Cert.NotAfter.Format(time.RFC3339),
				Instance:       i.name,
			}
			certResp := sqladmin.GenerateEphemeralCertResponse{
				EphemeralCert: c,
			}
			b, err = certResp.MarshalJSON()
			if err != nil {
				http.Error(resp, fmt.Errorf("unable to encode response: %w", err).Error(), http.StatusInternalServerError)
				return
			}
			resp.WriteHeader(http.StatusOK)
			resp.Write(b)
		},
	}
	return r
}

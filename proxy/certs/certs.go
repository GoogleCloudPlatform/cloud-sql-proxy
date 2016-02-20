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

// Package certs implements a CertSource which speaks to the public CLoud SQL API endpoint.
package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math"
	mrand "math/rand"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/googleapi"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// NewCertSource returns a CertSource which can be used to authenticate using
// the provided oauth token. The provided client must not be nil.
func NewCertSource(host string, c *http.Client, checkRegion bool) *RemoteCertSource {
	pkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err) // very unexpected.
	}
	serv, err := sqladmin.New(c)
	if err != nil {
		panic(err) // only possible if 'c' is nil.
	}
	return &RemoteCertSource{pkey, host + "projects/", serv, checkRegion}
}

// RemoteCertSource implements a CertSource, using Cloud SQL APIs to
// return Local certificates for identifying oneself as a specific user
// to the remote instance and Remote certificates for confirming the
// remote database's identity.
type RemoteCertSource struct {
	// key is the private key used for certificates returned by Local.
	key *rsa.PrivateKey
	// basepath is the URL prefix for Cloud SQL API calls.
	basepath string
	// serv is used to make authenticated API calls to Cloud SQL.
	serv *sqladmin.Service
	// If set, providing an incorrect region in their connection string will be
	// treated as an error. This is to provide the same functionality that will
	// occur when API calls require the region.
	checkRegion bool
}

// splitName splits a fully qualified instance into its project, region, and
// instance name components. While we make the transition to regionalized
// metadata, the region is optional.
//
// Examples:
//    "proj:region:my-db" -> ("proj", "region", "my-db")
//		"google.com:project:region:instance" -> ("google.com:project", "region", "instance")
//		"google.com:missing:part" -> ("google.com:missing", "", "part")
func splitName(instance string) (project, region, name string) {
	spl := strings.Split(instance, ":")
	if len(spl) < 2 {
		return "", "", instance
	}
	if dot := strings.Index(spl[0], "."); dot != -1 {
		spl[1] = spl[0] + ":" + spl[1]
		spl = spl[1:]
	}
	switch {
	case len(spl) < 2:
		return "", "", instance
	case len(spl) == 2:
		return spl[0], "", spl[1]
	default:
		return spl[0], spl[1], spl[2]
	}
}

const (
	baseBackoff = float64(time.Second / 5)
	backoffMult = 1.618
)

func backoffAPIRetry(iters int, desc string, do func() error) error {
	var err error
	for i := 0; i < iters; i++ {
		err = do()
		// Only Server-level HTTP errors are immediately retryable.
		// 'ok' will also be false if err is nil.
		gerr, ok := err.(*googleapi.Error)
		if !ok || gerr.Code < 500 {
			return err
		}

		exp := float64(i) * (1 - mrand.Float64()/10)
		sleep := time.Duration(baseBackoff * math.Pow(backoffMult, exp))
		log.Printf("Error in %s: %v; retrying in %v", desc, err, sleep)
		time.Sleep(sleep)
	}
	return err
}

// Local returns a certificate that may be used to establish a TLS
// connection to the specified instance.
func (s *RemoteCertSource) Local(instance string) (ret tls.Certificate, err error) {
	pkix, err := x509.MarshalPKIXPublicKey(&s.key.PublicKey)
	if err != nil {
		return ret, err
	}

	p, _, n := splitName(instance)
	req := s.serv.SslCerts.CreateEphemeral(p, n,
		&sqladmin.SslCertsCreateEphemeralRequest{
			PublicKey: string(pem.EncodeToMemory(&pem.Block{Bytes: pkix, Type: "RSA PUBLIC KEY"})),
		},
	)

	var data *sqladmin.SslCert
	err = backoffAPIRetry(5, "createEphemeral for "+instance, func() error {
		data, err = req.Do()
		return err
	})
	if err != nil {
		return ret, err
	}

	c, err := parseCert(data.Cert)
	if err != nil {
		return ret, fmt.Errorf("couldn't parse ephemeral certificate for instance %q: %v", instance, err)
	}
	return tls.Certificate{
		Certificate: [][]byte{c.Raw},
		PrivateKey:  s.key,
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

// Remote returns the specified instance's CA certificate, address, and name.
func (s *RemoteCertSource) Remote(instance string) (cert *x509.Certificate, addr, name string, err error) {
	p, region, n := splitName(instance)
	req := s.serv.Instances.Get(p, n)

	var data *sqladmin.DatabaseInstance
	err = backoffAPIRetry(5, "get instance "+instance, func() error {
		data, err = req.Do()
		return err
	})
	if err != nil {
		return nil, "", "", err
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
			return nil, "", "", err
		}
		log.Print(err)
	}
	if len(data.IpAddresses) == 0 || data.IpAddresses[0].IpAddress == "" {
		return nil, "", "", fmt.Errorf("no IP address found for %v", instance)
	}
	c, err := parseCert(data.ServerCaCert.Cert)
	return c, data.IpAddresses[0].IpAddress, p + ":" + n, err
}

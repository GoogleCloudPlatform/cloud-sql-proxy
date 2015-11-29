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
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// NewCertSource returns a CertSource which can be used to authenticate using
// the provided oauth token.
func NewCertSource(host string, c *http.Client, checkRegion bool) *RemoteCertSource {
	pkey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err) // very unexpected.
	}
	return &RemoteCertSource{pkey, host + "projects/", c, checkRegion}
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
	// client is used to make authenticated API calls to Cloud SQL.
	client *http.Client
	// If set, providing an incorrect region in their connection string will be
	// treated as an error. This is to provide the same functionality that will
	// occur when API calls require the region.
	checkRegion bool
}

// TODO(b/22249121): remove this method once I can use the auto-generated library.
func (s *RemoteCertSource) jsonReq(method, path string, src, dst interface{}) error {
	var data []byte
	switch src := src.(type) {
	case string:
		data = []byte(src)
	default:
		var err error
		if data, err = json.Marshal(src); err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, s.basepath+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	if err != nil || resp.StatusCode != 200 {
		return fmt.Errorf("%v %q: %v; Body=%q; read error: %v", req.Method, req.URL, resp.Status, buf.String(), err)
	}
	if err = json.Unmarshal(buf.Bytes(), dst); err != nil {
		err = fmt.Errorf("Decode(%q, %q, %#v) error: %v", req.URL, buf.String(), dst, err)
	}
	return err
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

// Local returns a certificate that may be used to establish a TLS
// connection to the specified instance.
func (s *RemoteCertSource) Local(instance string) (ret tls.Certificate, err error) {
	var (
		p, _, n = splitName(instance)
		url     = fmt.Sprintf("%s/instances/%s/createEphemeral", p, n)
		data    struct {
			Cert string
		}
	)
	pkix, err := x509.MarshalPKIXPublicKey(&s.key.PublicKey)
	if err != nil {
		return ret, err
	}

	var request = struct {
		Key string `json:"public_key"`
	}{
		string(pem.EncodeToMemory(&pem.Block{Bytes: pkix, Type: "RSA PUBLIC KEY"})),
	}
	if err := s.jsonReq("POST", url, request, &data); err != nil {
		return ret, err
	}
	c, err := parseCert(data.Cert)
	if err != nil {
		return ret, fmt.Errorf("coudln't parse ephemeral certificate for instance %q: %v", instance, err)
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
	// This represents a DatabaseInstance retrieved from the standard InstancesService Get.
	// As a part of b/22249121 this will be moved to also use the autogenerated library for
	// accessing the API.
	var data struct {
		Region string `json:"region"`

		IPAddresses []struct {
			IPAddress string `json:"ipAddress"`
		} `json:"ipAddresses"`
		ServerCaCert struct {
			Cert string
		}
	}

	p, region, n := splitName(instance)
	if err := s.jsonReq("GET", fmt.Sprintf("%s/instances/%s", p, n), "", &data); err != nil {
		return nil, "", "", err
	}
	// TODO(chowski): remove this when us-central is removed.
	if data.Region == "us-central" {
		data.Region = "us-central1"
	}
	if data.Region != region {
		var err error
		if region == "" {
			err = fmt.Errorf("instance %v doesn't provide region", instance)
		} else {
			err = fmt.Errorf(`for connection string "%s": got region %q, want %q`, instance, region, data.Region)
		}
		if err != nil {
			if s.checkRegion {
				return nil, "", "", err
			}
			log.Print(err)
		}
	}
	c, err := parseCert(data.ServerCaCert.Cert)
	return c, data.IPAddresses[0].IPAddress, p + ":" + n, err
}

// Copyright 2026 Google LLC
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
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// pqcTestPKI holds ephemeral test CA and server certificates matching Cloud SQL
// mTLS verification rules.
type pqcTestPKI struct {
	serverCACert     *x509.Certificate
	serverCAPEM      string
	serverTLSChain   tls.Certificate
	clientSigningCA  *x509.Certificate
	clientSigningKey *rsa.PrivateKey
	clientCAPool     *x509.CertPool
}

func newPQCTestPKI(t *testing.T, project, instance string) *pqcTestPKI {
	t.Helper()
	now := time.Now().Add(-time.Minute)
	notAfter := now.Add(time.Hour)

	serverCAKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey(serverCA): %v", err)
	}
	serverCATmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Cloud SQL Server CA"},
		NotBefore:             now,
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	serverCADER, err := x509.CreateCertificate(rand.Reader, serverCATmpl, serverCATmpl, &serverCAKey.PublicKey, serverCAKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(serverCA): %v", err)
	}
	serverCACert, err := x509.ParseCertificate(serverCADER)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(serverCA): %v", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey(server): %v", err)
	}
	serverTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: fmt.Sprintf("%s:%s", project, instance)},
		NotBefore:             now,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTmpl, serverCACert, &serverKey.PublicKey, serverCAKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(server): %v", err)
	}
	serverLeaf, err := x509.ParseCertificate(serverDER)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(server): %v", err)
	}

	var caPEM bytes.Buffer
	if err := pem.Encode(&caPEM, &pem.Block{Type: "CERTIFICATE", Bytes: serverCADER}); err != nil {
		t.Fatalf("pem.Encode(serverCA): %v", err)
	}

	clientCAKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey(clientCA): %v", err)
	}
	clientCATmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: "Test Cloud SQL Client Signing CA"},
		NotBefore:             now,
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	clientCADER, err := x509.CreateCertificate(rand.Reader, clientCATmpl, clientCATmpl, &clientCAKey.PublicKey, clientCAKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(clientCA): %v", err)
	}
	clientCACert, err := x509.ParseCertificate(clientCADER)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(clientCA): %v", err)
	}
	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(clientCACert)

	return &pqcTestPKI{
		serverCACert: serverCACert,
		serverCAPEM:  caPEM.String(),
		serverTLSChain: tls.Certificate{
			Certificate: [][]byte{serverDER, serverCADER},
			PrivateKey:  serverKey,
			Leaf:        serverLeaf,
		},
		clientSigningCA:  clientCACert,
		clientSigningKey: clientCAKey,
		clientCAPool:     clientCAPool,
	}
}

func (p *pqcTestPKI) signClientPublicKey(pubKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode client public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("ParsePKIXPublicKey: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("unexpected public key type %T", pub)
	}
	now := time.Now().Add(-time.Minute)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(100),
		Subject:               pkix.Name{CommonName: "Google Cloud SQL Client"},
		NotBefore:             now,
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, p.clientSigningCA, rsaPub, p.clientSigningKey)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := pem.Encode(&out, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return "", err
	}
	return out.String(), nil
}

func startMockSQLAdminServer(t *testing.T, pki *pqcTestPKI, project, region, instance string) *httptest.Server {
	t.Helper()
	connectPath := fmt.Sprintf("/sql/v1beta4/projects/%s/instances/%s/connectSettings", project, instance)
	ephemeralPath := fmt.Sprintf("/sql/v1beta4/projects/%s/instances/%s:generateEphemeralCert", project, instance)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == connectPath:
			cs := &sqladmin.ConnectSettings{
				BackendType:     "SECOND_GEN",
				DatabaseVersion: "POSTGRES_15",
				Region:          region,
				IpAddresses: []*sqladmin.IpMapping{
					{IpAddress: "127.0.0.1", Type: "PRIMARY"},
				},
				ServerCaCert: &sqladmin.SslCert{Cert: pki.serverCAPEM},
			}
			b, err := cs.MarshalJSON()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)
		case r.Method == http.MethodPost && r.URL.Path == ephemeralPath:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var req sqladmin.GenerateEphemeralCertRequest
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			certPEM, err := pki.signClientPublicKey(req.PublicKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			resp := &sqladmin.GenerateEphemeralCertResponse{
				EphemeralCert: &sqladmin.SslCert{
					Cert:           certPEM,
					CommonName:     "Google Cloud SQL Client",
					CreateTime:     time.Now().Format(time.RFC3339),
					ExpirationTime: time.Now().Add(time.Hour).Format(time.RFC3339),
					Instance:       instance,
				},
			}
			b, err := resp.MarshalJSON()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)
		default:
			http.Error(w, fmt.Sprintf("unexpected request: %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

type recordedTLSHandshake struct {
	mu              sync.Mutex
	supportedCurves []tls.CurveID
	connState       tls.ConnectionState
	handshakeErr    error
	done            chan struct{}
	once            sync.Once
}

func startMockCloudSQLTLSServer(t *testing.T, pki *pqcTestPKI, curvePrefs []tls.CurveID) (string, *recordedTLSHandshake) {
	t.Helper()
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	rec := &recordedTLSHandshake{done: make(chan struct{})}
	baseCfg := &tls.Config{
		Certificates:     []tls.Certificate{pki.serverTLSChain},
		ClientCAs:        pki.clientCAPool,
		ClientAuth:       tls.RequireAndVerifyClientCert,
		MinVersion:       tls.VersionTLS13,
		MaxVersion:       tls.VersionTLS13,
		CurvePreferences: curvePrefs,
	}
	baseCfg.GetConfigForClient = func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
		rec.mu.Lock()
		rec.supportedCurves = slices.Clone(chi.SupportedCurves)
		rec.mu.Unlock()
		return nil, nil
	}

	tlsLn := tls.NewListener(rawLn, baseCfg)
	t.Cleanup(func() { _ = tlsLn.Close() })

	go func() {
		for {
			conn, aErr := tlsLn.Accept()
			if aErr != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				tlsConn, ok := c.(*tls.Conn)
				if !ok {
					return
				}
				hErr := tlsConn.Handshake()
				rec.mu.Lock()
				rec.handshakeErr = hErr
				if hErr == nil {
					rec.connState = tlsConn.ConnectionState()
				}
				rec.mu.Unlock()
				rec.once.Do(func() { close(rec.done) })
				if hErr != nil {
					return
				}
				r := bufio.NewReader(tlsConn)
				line, rErr := r.ReadString('\n')
				if rErr != nil {
					return
				}
				_, _ = tlsConn.Write([]byte("pong:" + line))
			}(conn)
		}
	}()

	return tlsLn.Addr().String(), rec
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestProxyPQCKeyExchange(t *testing.T) {
	// IANA codepoint for X25519MLKEM768 hybrid post-quantum key exchange.
	const wantX25519MLKEM768Codepoint = tls.CurveID(0x11ec)
	if tls.X25519MLKEM768 != wantX25519MLKEM768Codepoint {
		t.Fatalf("tls.X25519MLKEM768 = %#x, want %#x", tls.X25519MLKEM768, wantX25519MLKEM768Codepoint)
	}

	const (
		project  = "proj"
		region   = "region"
		instance = "pqc-inst"
		icn      = project + ":" + region + ":" + instance
	)
	pki := newPQCTestPKI(t, project, instance)
	adminSrv := startMockSQLAdminServer(t, pki, project, region, instance)

	tcs := []struct {
		desc             string
		godebug          string
		serverCurves     []tls.CurveID
		wantClientMLKEM  bool
		wantNegotiatedID tls.CurveID
		wantHandshakeErr bool
	}{
		{
			desc:             "default hybrid PQC handshake negotiates X25519MLKEM768",
			serverCurves:     nil, // Go 1.24+ default server curves include X25519MLKEM768 and classical curves
			wantClientMLKEM:  true,
			wantNegotiatedID: tls.X25519MLKEM768,
		},
		{
			desc:             "PQC-only server succeeds by default",
			serverCurves:     []tls.CurveID{tls.X25519MLKEM768},
			wantClientMLKEM:  true,
			wantNegotiatedID: tls.X25519MLKEM768,
		},
		{
			desc:             "classical-only server falls back cleanly to X25519",
			serverCurves:     []tls.CurveID{tls.X25519, tls.CurveP256},
			wantClientMLKEM:  true,
			wantNegotiatedID: tls.X25519,
		},
		{
			desc:             "P-256-only server falls back cleanly via HelloRetryRequest",
			serverCurves:     []tls.CurveID{tls.CurveP256},
			wantClientMLKEM:  true,
			wantNegotiatedID: tls.CurveP256,
		},
		{
			desc:    "GODEBUG=tlsmlkem=0 opt-out negotiates classical X25519 against PQC server",
			godebug: "tlsmlkem=0",
			// Explicitly configure X25519MLKEM768 first on the server so process-wide
			// GODEBUG=tlsmlkem=0 does not disable PQC on the mock server itself.
			serverCurves:     []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256},
			wantClientMLKEM:  false,
			wantNegotiatedID: tls.X25519,
		},
		{
			desc:             "GODEBUG=tlsmlkem=1 explicit enable negotiates X25519MLKEM768",
			godebug:          "tlsmlkem=1",
			serverCurves:     []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256},
			wantClientMLKEM:  true,
			wantNegotiatedID: tls.X25519MLKEM768,
		},
		{
			desc:             "GODEBUG=tlsmlkem=0 fails against PQC-only server",
			godebug:          "tlsmlkem=0",
			serverCurves:     []tls.CurveID{tls.X25519MLKEM768},
			wantClientMLKEM:  false,
			wantHandshakeErr: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.godebug != "" {
				t.Setenv("GODEBUG", tc.godebug)
			} else {
				// Ensure inherited environment does not disable tlsmlkem for default cases.
				t.Setenv("GODEBUG", "")
			}

			tlsAddr, rec := startMockCloudSQLTLSServer(t, pki, tc.serverCurves)
			proxyPort := freeTCPPort(t)

			conf := &proxy.Config{
				Addr:           "127.0.0.1",
				Port:           proxyPort,
				APIEndpointURL: adminSrv.URL,
				Token:          "test-oauth2-token",
				UserAgent:      "cloud-sql-proxy/test",
				Instances: []proxy.InstanceConnConfig{
					{Name: icn},
				},
			}

			dialerOpts, err := conf.DialerOptions(testLogger)
			if err != nil {
				t.Fatalf("conf.DialerOptions: %v", err)
			}
			// Redirect the server-side proxy TCP dial (127.0.0.1:3307) to our
			// ephemeral mock Cloud SQL TLS 1.3 listener to avoid port 3307 collisions.
			dialerOpts = append(dialerOpts, cloudsqlconn.WithDialFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
				if addr != "127.0.0.1:3307" {
					return nil, fmt.Errorf("unexpected backend dial address %q, want 127.0.0.1:3307", addr)
				}
				var d net.Dialer
				return d.DialContext(ctx, network, tlsAddr)
			}))

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			dialer, err := cloudsqlconn.NewDialer(ctx, dialerOpts...)
			if err != nil {
				t.Fatalf("cloudsqlconn.NewDialer: %v", err)
			}

			client, err := proxy.NewClient(ctx, dialer, testLogger, conf, nil)
			if err != nil {
				_ = dialer.Close()
				t.Fatalf("proxy.NewClient: %v", err)
			}
			defer func() { _ = client.Close() }()

			ready := make(chan struct{})
			go func() {
				_ = client.Serve(ctx, func() { close(ready) })
			}()
			select {
			case <-ready:
			case <-ctx.Done():
				t.Fatal("timed out waiting for proxy Serve() readiness")
			}

			localAddr := fmt.Sprintf("127.0.0.1:%d", proxyPort)
			conn, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
			if err != nil {
				t.Fatalf("net.DialTimeout(%s): %v", localAddr, err)
			}
			defer func() { _ = conn.Close() }()
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

			if _, err := conn.Write([]byte("ping-pqc\n")); err != nil {
				t.Fatalf("conn.Write: %v", err)
			}

			select {
			case <-rec.done:
			case <-ctx.Done():
				t.Fatal("timed out waiting for TLS server handshake")
			}

			rec.mu.Lock()
			gotCurves := slices.Clone(rec.supportedCurves)
			gotState := rec.connState
			gotHandshakeErr := rec.handshakeErr
			rec.mu.Unlock()

			gotHasMLKEM := slices.Contains(gotCurves, tls.X25519MLKEM768)
			if gotHasMLKEM != tc.wantClientMLKEM {
				t.Fatalf("ClientHello SupportedCurves = %v: contains X25519MLKEM768 (%#x) = %v, want %v",
					gotCurves, tls.X25519MLKEM768, gotHasMLKEM, tc.wantClientMLKEM)
			}
			if !slices.Contains(gotCurves, tls.X25519) {
				t.Fatalf("ClientHello SupportedCurves = %v: expected classical X25519 fallback curve", gotCurves)
			}

			if tc.wantHandshakeErr {
				if gotHandshakeErr == nil {
					t.Fatalf("expected TLS handshake to fail against PQC-only server with GODEBUG=%q, but succeeded with CurveID=%v",
						tc.godebug, gotState.CurveID)
				}
				var buf [1]byte
				if _, rErr := conn.Read(buf[:]); rErr == nil {
					t.Fatal("expected local client connection to close when backend TLS handshake fails")
				}
				return
			}
			if gotHandshakeErr != nil {
				t.Fatalf("unexpected TLS server handshake error: %v", gotHandshakeErr)
			}

			if gotState.Version != tls.VersionTLS13 {
				t.Fatalf("TLS Version = %#x, want TLS 1.3 (%#x)", gotState.Version, tls.VersionTLS13)
			}
			if gotState.CurveID != tc.wantNegotiatedID {
				t.Fatalf("negotiated CurveID = %v (%#x), want %v (%#x)",
					gotState.CurveID, uint16(gotState.CurveID), tc.wantNegotiatedID, uint16(tc.wantNegotiatedID))
			}

			reply, err := bufio.NewReader(conn).ReadString('\n')
			if err != nil {
				t.Fatalf("reading proxied reply: %v", err)
			}
			if wantReply := "pong:ping-pqc\n"; reply != wantReply {
				t.Fatalf("proxied reply = %q, want %q", reply, wantReply)
			}
		})
	}
}

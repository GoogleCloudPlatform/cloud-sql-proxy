package certs

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/api/option"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

const fakeCert = `-----BEGIN CERTIFICATE-----
MIICgTCCAWmgAwIBAgIBADANBgkqhkiG9w0BAQsFADAAMCIYDzAwMDEwMTAxMDAw
MDAwWhgPMDAwMTAxMDEwMDAwMDBaMAAwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw
ggEKAoIBAQCvN0H6/ecloIfNyRu8KKtVSIK0JaW1lB1C1/ZI9iZmihqiUrxeyKTb
9hWuMPJ3u9NfSn1Vlwuj0bw7/T8e3Ol5BImcGxYxWMefkqFtqnjCafo2wnIea/eQ
JFLt4wXYkeveHReUseGtaBzpCo4wYOiqgxyIrGiQ/rq4Xjr2hXuqTg4TTgxv+0Iv
nrJwn61pitGvLPjsl9quzSQ6CdM3tWfb6cwozF5uJatbxRCZDsp1qUBXX9/zYqmx
8regdRG95btNgXLCfNS0iX0jopl00vGwYRGGKjfPZ5AkpuxX9M4Ys3X7pOspaQMC
Zf4VjXdwOljqZxIOGhOBbrXQacSywTLjAgMBAAGjAjAAMA0GCSqGSIb3DQEBCwUA
A4IBAQAXj/0iiU2AQGztlFstLVwQ9yz+7/pfqAr26DYu9hpI/QvrZsJWjwNUNlX+
7gwhrwiJs7xsLZqnEr2qvj6at/MtxIEVgQd43sOsWW9de8R5WNQNzsCb+5npWcx7
vtcKXD9jFFLDDCIYjAf9+6m/QrMJtIf++zBmjguShccjZzY+GQih78oWqNTYqRQs
//wOP15vFQ/gB4DcJ0UyO9icVgbJha66yzG7XABDEepha5uhpLhwFaONU8jMxW7A
fOx52xqIUu3m4M3Ci0ZIp22TeGVuJ/Dy1CPbDOshcb0dXTE+mU5T91SHKRF4jz77
+9TQIXHGk7lJyVVhbed8xm/p727f
-----END CERTIFICATE-----`

func patchNowFunc(t *testing.T, nowTime string) func() {
	oldNow := now

	aTime, err := time.Parse(time.RFC3339, nowTime)
	if err != nil {
		t.Fatal(err)
	}
	now = func() time.Time { return aTime }
	return func() { now = oldNow }
}

func TestLocalCertSupportsStaleReads(t *testing.T) {
	cleanup := patchNowFunc(t, "2006-01-02T15:00:30Z")
	defer cleanup()

	var (
		gotReadTimes []string
		ok           bool
	)
	handleEphemeralCert := func(w http.ResponseWriter, r *http.Request) {
		var actual sqladmin.GenerateEphemeralCertRequest
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		defer r.Body.Close()
		if err = json.Unmarshal(data, &actual); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}
		gotReadTimes = append(gotReadTimes, actual.ReadTime)
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, `{"message":"the first request fails"}`)
			ok = true
			return
		}
		// the second request succeeds
		fmt.Fprintln(w, fmt.Sprintf(`{"ephemeralCert":{"cert": %q}}`, fakeCert))
	}
	ts := httptest.NewServer(http.HandlerFunc(handleEphemeralCert))
	defer ts.Close()

	cs := NewCertSourceOpts(ts.Client(), RemoteOpts{})
	// replace SQL Admin API client with client backed by test server
	var err error
	cs.serv, err = sqladmin.NewService(context.Background(),
		option.WithEndpoint(ts.URL), option.WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("failed to replace SQL Admin client: %v", err)
	}

	// Send request to generate a cert
	_, err = cs.Local("my-proj:reg:my-inst")
	if err != nil {
		t.Fatal(err)
	}

	// Verify read time is not present for first request
	// and is 30 seconds before "now" for second request
	if len(gotReadTimes) != 2 {
		t.Fatalf("expected two results, got = %v", len(gotReadTimes))
	}
	if gotReadTimes[0] != "" {
		t.Fatalf("expected empty ReadTime for first request, got = %v", gotReadTimes[0])
	}
	want := "2006-01-02T15:00:00Z" // 30 seconds earlier
	if gotReadTimes[1] != want {
		t.Fatal("expected non-empty ReadTime for second request, got empty value")
	}
}

func TestRemoteCertSupportsStaleReads(t *testing.T) {
	cleanup := patchNowFunc(t, "2006-01-02T15:00:30Z")
	defer cleanup()

	var (
		gotReadTimes []string
		ok           bool
	)
	handleConnectSettings := func(w http.ResponseWriter, r *http.Request) {
		rt := r.URL.Query()["readTime"]
		// if the URL parameter isn't nil, record its value; otherwise add an
		// empty string to indicate no query param was set
		if rt != nil {
			gotReadTimes = append(gotReadTimes, rt[0])
		} else {
			gotReadTimes = append(gotReadTimes, "")
		}
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, `{"message":"the first request fails"}`)
			ok = true
			return
		}
		fmt.Fprintln(w, fmt.Sprintf(`{
			"region":"us-central1",
			"ipAddresses": [
				{"type":"PRIMARY", "ipAddress":"127.0.0.1"}
			],
			"serverCaCert": {"cert": %q}
		}`, fakeCert))
	}
	ts := httptest.NewServer(http.HandlerFunc(handleConnectSettings))
	defer ts.Close()

	cs := NewCertSourceOpts(ts.Client(), RemoteOpts{})
	var err error
	// replace SQL Admin API client with client backed by test server
	cs.serv, err = sqladmin.NewService(context.Background(),
		option.WithEndpoint(ts.URL), option.WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatalf("failed to replace SQL Admin client: %v", err)
	}

	// Send request to retrieve instance metadata
	_, _, _, _, err = cs.Remote("my-proj:us-central1:my-inst")
	if err != nil {
		t.Fatal(err)
	}

	// Verify read time is not present for first request
	// and is 30 seconds before "now" for second request
	if len(gotReadTimes) != 2 {
		t.Fatalf("expected two results, got = %v", len(gotReadTimes))
	}
	if gotReadTimes[0] != "" {
		t.Fatalf("expected empty ReadTime for first request, got = %v", gotReadTimes[0])
	}
	want := "2006-01-02T15:00:00Z" // 30 seconds earlier
	if gotReadTimes[1] != want {
		t.Fatal("expected non-empty ReadTime for second request, got empty value")
	}
}

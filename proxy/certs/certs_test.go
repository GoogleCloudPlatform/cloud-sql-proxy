package certs_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/certs"
)

func TestCertDurationConfiguration(t *testing.T) {
	testCases := []struct {
		in   time.Duration
		want time.Duration
		desc string
	}{
		{in: time.Duration(0), want: time.Hour, desc: "when no value is provided"},
		{in: 59 * time.Minute, want: time.Hour, desc: "when too short"},
		{in: 25 * time.Hour, want: time.Hour, desc: "when too long"},
		{in: time.Hour, want: time.Hour, desc: "when at the minimum"},
		{in: 24 * time.Hour, want: 24 * time.Hour, desc: "when at the maximum"},
	}
	for _, tc := range testCases {
		s := certs.NewCertSourceOpts(http.DefaultClient, certs.RemoteOpts{CertDuration: tc.in})
		if s.CertDuration != tc.want {
			t.Errorf("want = %v, got = %v", tc.want, s.CertDuration)
		}
	}
}

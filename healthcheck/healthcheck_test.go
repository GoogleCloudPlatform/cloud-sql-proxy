package healthcheck

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
)

func newClient(mc uint64) *proxy.Client {
	return &proxy.Client{
		MaxConnections: mc,
	}
}

func newHealthCheck(l bool, r bool, s bool) *HealthCheck {
	return &HealthCheck{
		live: l,
		ready: r,
		started: s,
	}
}

func TestLiveness(t *testing.T) {
	proxyClient := newClient(0)
	InitHealthCheck(proxyClient)

	resp, err := http.Get("/liveness")
	if err != nil {
		t.Errorf("failed to GET from /liveness")
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}
	if resp.Status != "ok\n" {
		t.Errorf("got status %v instead of \"ok\\n\"", resp.Status)
	}
}

func TestUnexpectedTermination(t *testing.T) {

}

func TestBadStartup(t *testing.T) {
	proxyClient := newClient(0)
	InitHealthCheck(proxyClient)

	resp, err := http.Get("/readiness")
	if err != nil {
		t.Errorf("failed to GET from /readiness")
	}
	if resp.StatusCode != 500 {
		t.Errorf("got status code %v instead of 500", resp.StatusCode)
	}
	if resp.Status != "error\n" {
		t.Errorf("got status %v instead of \"error\\n\"", resp.Status)
	}
}

func TestSuccessfulStartup(t *testing.T) {
	proxyClient := newClient(0)
	hc := InitHealthCheck(proxyClient)
	NotifyReady(hc)

	resp, err := http.Get("/readiness")
	if err != nil {
		t.Errorf("failed to GET from /readiness")
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}
	if resp.Status != "ok\n" {
		t.Errorf("got status %v instead of \"ok\\n\"", resp.Status)
	}
}

func TestReadiness(t *testing.T) {

}

func TestMaxConnections(t *testing.T) {

}
package healthcheck

import (
	"net/http"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
)

func newClient(mc uint64) *proxy.Client {
	return &proxy.Client{
		MaxConnections: mc,
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
	if resp.Status != "ok\n" {
		t.Errorf("got status %v instead of \"ok\\n\"", resp.Status)
	}
}

func TestSuccessfulStartup(t *testing.T) {
	proxyClient := newClient(0)
	hc := InitHealthCheck(proxyClient)
	NotifyReadyForConnections(hc)

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
	proxyClient := newClient(0) // No specified limit for MaxConnections (MaxConnections check always passes)
	hc := InitHealthCheck(proxyClient)
	NotifyReadyForConnections(hc) // Set hc.started = true

	resp, err := http.Get("http://localhost:8080/readiness")
	if err != nil {
		t.Errorf("failed to GET from /readiness")
	}
	if resp.StatusCode != 200 {
		t.Errorf("got status code %v instead of 200", resp.StatusCode)
	}
}

func TestMaxConnections(t *testing.T) {
	proxyClient := newClient(10) // MaxConnections == 10
	hc := InitHealthCheck(proxyClient)
	NotifyReadyForConnections(hc)

	proxyClient.ConnectionsCounter = proxyClient.MaxConnections // Simulate reaching the limit for maximum number of connections

	resp, err := http.Get("http://localhost:8080/readiness")
	if err != nil {
		t.Errorf("failed to GET from /readiness")
	}

	if resp.StatusCode != 500 {
		t.Errorf("got status code %v instead of 500", resp.StatusCode)
	}
}

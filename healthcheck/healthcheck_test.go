package healthcheck

import (
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
)

var handler = func(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "something failed", http.StatusInternalServerError)
}

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
	InitHealthCheck(newClient(0))

	req, err := http.NewRequest("GET", "127.0.0.1:8080/liveness", nil)
	if err != nil {
		log.Fatal(err)
	}

	resp := httptest.NewRecorder()
	handler(resp, req)

	if resp.Code != 200 {
		t.Errorf("got status code %v instead of 200", resp.Code)
	}
}

func TestUnexpectedTermination(t *testing.T) {

}

func TestBadStartup(t *testing.T) {
	InitHealthCheck(newClient(0))

	req, err := http.NewRequest("GET", "/readiness", nil)
	if err != nil {
		log.Fatal(err)
	}

	resp := httptest.NewRecorder()
	handler(resp, req)

	if resp.Code != 500 {
		t.Errorf("got status code %v instead of 500", resp.Code)
	}
}

func TestSuccessfulStartup(t *testing.T) {
	hc := InitHealthCheck(newClient(0))
	NotifyReady(hc)

	req, err := http.NewRequest("GET", "/readiness", nil)
	if err != nil {
		log.Fatal(err)
	}

	resp := httptest.NewRecorder()
	handler(resp, req)

	if resp.Code != 200 {
		t.Errorf("got status code %v instead of 200", resp.Code)
	}
}

func TestReadiness(t *testing.T) {

}

func TestMaxConnections(t *testing.T) {

}
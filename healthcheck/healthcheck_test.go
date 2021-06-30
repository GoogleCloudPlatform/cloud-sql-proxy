package healthcheck

import (
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
	
}

func TestUnexpectedTermination(t *testing.T) {
	
}

func TestBadStartup(t *testing.T) {

}

func TestNotifyReady(t *testing.T) {

}

func TestReadiness(t *testing.T) {

}

func TestMaxConnections(t *testing.T) {

}
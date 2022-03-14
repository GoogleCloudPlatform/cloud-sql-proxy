package proxy_test

import (
	"context"
	"net"
	"testing"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
	"github.com/spf13/cobra"
)

func TestClientInitialization(t *testing.T) {
	tcs := []struct {
		desc      string
		in        *proxy.Config
		wantAddrs []string
	}{
		{
			desc: "multiple instances",
			in: &proxy.Config{
				Addr: "127.0.0.1",
				Instances: []proxy.InstanceConnConfig{
					{Name: "proj:region:inst1"},
					{Name: "proj:region:inst2"},
				},
			},
			wantAddrs: []string{"127.0.0.1:5000", "127.0.0.1:5001"},
		},
		{
			desc: "with instance address",
			in: &proxy.Config{
				Addr: "1.1.1.1", // bad address, binding shouldn't happen here.
				Instances: []proxy.InstanceConnConfig{
					{Addr: "0.0.0.0", Name: "proj:region:inst1"},
				},
			},
			wantAddrs: []string{"0.0.0.0:5000"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			c, err := proxy.NewClient(context.Background(), &cobra.Command{}, tc.in)
			if err != nil {
				t.Fatalf("want error = nil, got = %v", err)
			}
			defer c.Close()
			for _, addr := range tc.wantAddrs {
				conn, err := net.Dial("tcp4", addr)
				if err != nil {
					t.Fatalf("want error = nil, got = %v", err)
				}
				defer conn.Close()
			}
		})
	}
}

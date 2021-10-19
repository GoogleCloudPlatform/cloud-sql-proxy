//go:build go1.17
// +build go1.17

package proxy

import (
	"context"
	"crypto/tls"
	"net"
)

// connectTLS returns a new TLS client side connection
// using conn as the underlying transport.
//
// The returned connection has already completed its TLS handshake.
func (c *Client) connectTLS(
	ctx context.Context,
	conn net.Conn,
	instance string,
	cfg *tls.Config,
) (net.Conn, error) {
	ret := tls.Client(conn, cfg)
	// HandshakeContext was introduced in Go 1.17, hence
	// this file is conditionally compiled on only Go versions >= 1.17.
	if err := ret.HandshakeContext(ctx); err != nil {
		_ = ret.Close()
		c.invalidateCfg(cfg, instance)
		return nil, err
	}
	return ret, nil
}

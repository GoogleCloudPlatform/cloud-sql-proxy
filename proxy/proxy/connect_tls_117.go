// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		c.invalidateCfg(cfg, instance, err)
		return nil, err
	}
	return ret, nil
}

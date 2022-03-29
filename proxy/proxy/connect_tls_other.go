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

//go:build !go1.17
// +build !go1.17

package proxy

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"
)

type cancelationWatcher struct {
	done chan struct{} // closed when the caller requests shutdown by calling stop().
	wg   sync.WaitGroup
}

// newCancelationWatcher starts a goroutine that will monitor
// ctx for cancelation. If ctx is canceled, the I/O
// deadline on conn is set to some point in the past, canceling
// ongoing I/O and refusing new I/O.
//
// The caller must call stop() on the returned struct to
// release resources associated with this.
func newCancelationWatcher(ctx context.Context, conn net.Conn) *cancelationWatcher {
	cw := &cancelationWatcher{
		done: make(chan struct{}),
	}
	// Monitor for context cancelation.
	cw.wg.Add(1)
	go func() {
		defer cw.wg.Done()

		select {
		case <-ctx.Done():
			// Set the deadline to some point in the past, but not
			// the zero value. This will cancel ongoing requests
			// and refuse future ones.
			_ = conn.SetDeadline(time.Time{}.Add(1))
		case <-cw.done:
			return
		}
	}()
	return cw
}

// stop shuts down this cancelationWatcher and releases
// the resources associated with it.
//
// Once stop has returned, the provided context is no longer
// watched for cancelation and the deadline on the
// provided net.Conn is no longer manipulated.
func (cw *cancelationWatcher) stop() {
	close(cw.done)
	cw.wg.Wait()
}

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
	// For the purposes of this Handshake, manipulate the I/O
	// deadlines on this connection inline. We have to do this
	// manual dance because we don't have HandshakeContext in this
	// version of Go.

	defer func() {
		// The connection didn't originally have a read deadline (we
		// just created it). So no matter what happens here, restore
		// the lack-of-deadline.
		//
		// In other words, only apply the deadline while dialing,
		// not during subsequent usage.
		_ = conn.SetDeadline(time.Time{})
	}()

	// If we have a context deadline, apply it.
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	cw := newCancelationWatcher(ctx, conn)
	defer cw.stop() // Always free the context watcher.

	ret := tls.Client(conn, cfg)
	if err := ret.Handshake(); err != nil {
		_ = ret.Close()
		c.invalidateCfg(cfg, instance, err)
		return nil, err
	}
	return ret, nil
}

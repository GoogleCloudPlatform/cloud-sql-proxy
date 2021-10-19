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

type cancellationWatcher struct {
	done chan struct{} // closed when the caller requests shutdown by calling stop().
	wg   sync.WaitGroup
}

// newCancellationWatcher starts a goroutine that will monitor
// ctx for cancellation. If ctx is cancelled, the I/O
// deadline on conn is set to some point in the past, cancelling
// ongoing I/O and refusing new I/O.
//
// The caller must call stop() on the returned struct to
// release resources associated with this.
func newCancellationWatcher(ctx context.Context, conn net.Conn) *cancellationWatcher {
	cw := &cancellationWatcher{
		done: make(chan struct{}),
	}
	// Monitor for context cancellation.
	cw.wg.Add(1)
	go func() {
		defer cw.wg.Done()

		select {
		case <-ctx.Done():
			// Set the deadline to some point in the past, but not
			// the zero value. This will cancel ongoing requests
			// and refuse future ones.
			_ = conn.SetDeadline(time.Now().Add(-time.Hour))
		case <-cw.done:
			return
		}
	}()
	return cw
}

// stop shuts down this cancellationWatcher and releases
// the resources associated with it.
//
// Once stop has returned, the provided context is no longer
// watched for cancellation and the deadline on the
// provided net.Conn is no longer manipulated.
func (cw *cancellationWatcher) stop() {
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
		_ = conn.SetDeadline(time.Time{})
	}()

	// If we have a context deadline, apply it.
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	cw := newCancellationWatcher(ctx, conn)
	defer cw.stop() // Always free the context watcher.

	ret := tls.Client(conn, cfg)
	if err := ret.Handshake(); err != nil {
		_ = ret.Close()
		c.invalidateCfg(cfg, instance)
		return nil, err
	}
	return ret, nil
}

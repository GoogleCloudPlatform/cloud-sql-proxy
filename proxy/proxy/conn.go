package proxy

import (
	"net"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
)

type IdleTrackingConn interface {
	net.Conn
	idleDuration() time.Duration
}

type TrackedConn struct {
	net.Conn
	sync.Mutex
	lastActivity time.Time
}

func (t *TrackedConn) Write(b []byte) (int, error) {
	x, y := t.Conn.Write(b)
	t.Lock()
	t.lastActivity = time.Now()
	t.Unlock()
	return x, y
}

func (t *TrackedConn) Read(b []byte) (int, error) {
	x, y := t.Conn.Read(b)
	t.Lock()
	t.lastActivity = time.Now()
	t.Unlock()
	return x, y
}

func (t *TrackedConn) idleDuration() time.Duration {
	t.Lock()
	defer t.Unlock()
	return time.Since(t.lastActivity)
}

type dbgConn struct {
	IdleTrackingConn
}

func (d dbgConn) Write(b []byte) (int, error) {
	x, y := d.IdleTrackingConn.Write(b)
	logging.Verbosef("write(%q) => (%v, %v)", b, x, y)
	return x, y
}

func (d dbgConn) Read(b []byte) (int, error) {
	x, y := d.IdleTrackingConn.Read(b)
	logging.Verbosef("read: (%v, %v) => %q", x, y, b[:x])
	return x, y
}

func (d dbgConn) Close() error {
	err := d.IdleTrackingConn.Close()
	logging.Verbosef("close: %v", err)
	return err
}

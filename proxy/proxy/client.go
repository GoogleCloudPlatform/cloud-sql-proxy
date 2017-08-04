// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
)

const (
	DefaultRefreshCfgThrottle = time.Minute
	keepAlivePeriod           = time.Minute
)

// errNotCached is returned when the instance was not found in the Client's
// cache. It is an internal detail and is not actually ever returned to the
// user.
var errNotCached = errors.New("instance was not found in cache")

// Conn represents a connection from a client to a specific instance.
type Conn struct {
	Instance string
	Conn     net.Conn
}

// CertSource is how a Client obtains various certificates required for operation.
type CertSource interface {
	// Local returns a certificate that can be used to authenticate with the
	// provided instance.
	Local(instance string) (tls.Certificate, error)
	// Remote returns the instance's CA certificate, address, and name.
	Remote(instance string) (cert *x509.Certificate, addr, name string, err error)
}

// Client is a type to handle connecting to a Server. All fields are required
// unless otherwise specified.
type Client struct {
	// Port designates which remote port should be used when connecting to
	// instances. This value is defined by the server-side code, but for now it
	// should always be 3307.
	Port int
	// Required; specifies how certificates are obtained.
	Certs CertSource
	// Optionally tracks connections through this client. If nil, connections
	// are not tracked and will not be closed before method Run exits.
	Conns *ConnSet
	// Dialer should return a new connection to the provided address. It is
	// called on each new connection to an instance. net.Dial will be used if
	// left nil.
	Dialer func(net, addr string) (net.Conn, error)

	// RefreshCfgThrottle is the amount of time to wait between configuration
	// refreshes. If not set, it defaults to 1 minute.
	//
	// This is to prevent quota exhaustion in the case of client-side
	// malfunction.
	RefreshCfgThrottle time.Duration

	// The cfgCache holds the most recent connection configuration keyed by
	// instance. Relevant functions are refreshCfg and cachedCfg. It is
	// protected by cfgL.
	cfgCache map[string]cacheEntry
	cfgL     sync.RWMutex

	// MaxConnections is the maximum number of connections to establish
	// before refusing new connections. 0 means no limit.
	MaxConnections uint64

	// ConnectionsCounter is used to enforce the optional maxConnections limit
	ConnectionsCounter uint64
}

type cacheEntry struct {
	lastRefreshed time.Time
	// If err is not nil, the addr and cfg are not valid.
	err  error
	addr string
	cfg  *tls.Config
}

// Run causes the client to start waiting for new connections to connSrc and
// proxy them to the destination instance. It blocks until connSrc is closed.
func (c *Client) Run(connSrc <-chan Conn) {
	for conn := range connSrc {
		go c.handleConn(conn)
	}

	if err := c.Conns.Close(); err != nil {
		logging.Errorf("closing client had error: %v", err)
	}
}

func (c *Client) handleConn(conn Conn) {
	// Track connections count only if a maximum connections limit is set to avoid useless overhead
	if c.MaxConnections > 0 {
		active := atomic.AddUint64(&c.ConnectionsCounter, 1)

		// Deferred decrement of ConnectionsCounter upon connection closing
		defer atomic.AddUint64(&c.ConnectionsCounter, ^uint64(0))

		if active > c.MaxConnections {
			logging.Errorf("too many open connections (max %d)", c.MaxConnections)
			conn.Conn.Close()
			return
		}
	}

	server, err := c.Dial(conn.Instance)
	if err != nil {
		logging.Errorf("couldn't connect to %q: %v", conn.Instance, err)
		conn.Conn.Close()
		return
	}

	if false {
		// Log the connection's traffic via the debug connection if we're in a
		// verbose mode. Note that this is the unencrypted traffic stream.
		conn.Conn = dbgConn{conn.Conn}
	}

	c.Conns.Add(conn.Instance, conn.Conn)
	copyThenClose(server, conn.Conn, conn.Instance, "local connection on "+conn.Conn.LocalAddr().String())

	if err := c.Conns.Remove(conn.Instance, conn.Conn); err != nil {
		logging.Errorf("%s", err)
	}
}

// refreshCfg uses the CertSource inside the Client to find the instance's
// address as well as construct a new tls.Config to connect to the instance. It
// caches the result.
func (c *Client) refreshCfg(instance string) (addr string, cfg *tls.Config, err error) {
	c.cfgL.Lock()
	defer c.cfgL.Unlock()

	throttle := c.RefreshCfgThrottle
	if throttle == 0 {
		throttle = DefaultRefreshCfgThrottle
	}

	if old := c.cfgCache[instance]; time.Since(old.lastRefreshed) < throttle {
		logging.Errorf("Throttling refreshCfg(%s): it was only called %v ago", instance, time.Since(old.lastRefreshed))
		// Refresh was called too recently, just reuse the result.
		return old.addr, old.cfg, old.err
	}

	if c.cfgCache == nil {
		c.cfgCache = make(map[string]cacheEntry)
	}

	defer func() {
		c.cfgCache[instance] = cacheEntry{
			lastRefreshed: time.Now(),

			err:  err,
			addr: addr,
			cfg:  cfg,
		}
	}()

	mycert, err := c.Certs.Local(instance)
	if err != nil {
		return "", nil, err
	}

	scert, addr, name, err := c.Certs.Remote(instance)
	if err != nil {
		return "", nil, err
	}
	certs := x509.NewCertPool()
	certs.AddCert(scert)

	cfg = &tls.Config{
		ServerName:   name,
		Certificates: []tls.Certificate{mycert},
		RootCAs:      certs,
	}
	return fmt.Sprintf("%s:%d", addr, c.Port), cfg, nil
}

func (c *Client) cachedCfg(instance string) (string, *tls.Config) {
	c.cfgL.RLock()
	ret, ok := c.cfgCache[instance]
	c.cfgL.RUnlock()

	// Don't waste time returning an expired/invalid cert.
	if !ok || ret.err != nil || time.Now().After(ret.cfg.Certificates[0].Leaf.NotAfter) {
		return "", nil
	}
	return ret.addr, ret.cfg
}

// Dial uses the configuration stored in the client to connect to an instance.
// If this func returns a nil error the connection is correctly authenticated
// to connect to the instance.
func (c *Client) Dial(instance string) (net.Conn, error) {
	if addr, cfg := c.cachedCfg(instance); cfg != nil {
		ret, err := c.tryConnect(addr, cfg)
		if err == nil {
			return ret, err
		}
	}

	addr, cfg, err := c.refreshCfg(instance)
	if err != nil {
		return nil, err
	}
	return c.tryConnect(addr, cfg)
}

func (c *Client) tryConnect(addr string, cfg *tls.Config) (net.Conn, error) {
	d := c.Dialer
	if d == nil {
		d = net.Dial
	}
	conn, err := d("tcp", addr)
	if err != nil {
		return nil, err
	}
	type setKeepAliver interface {
		SetKeepAlive(keepalive bool) error
		SetKeepAlivePeriod(d time.Duration) error
	}

	if s, ok := conn.(setKeepAliver); ok {
		if err := s.SetKeepAlive(true); err != nil {
			logging.Verbosef("Couldn't set KeepAlive to true: %v", err)
		} else if err := s.SetKeepAlivePeriod(keepAlivePeriod); err != nil {
			logging.Verbosef("Couldn't set KeepAlivePeriod to %v", keepAlivePeriod)
		}
	} else {
		logging.Verbosef("KeepAlive not supported: long-running tcp connections may be killed by the OS.")
	}

	ret := tls.Client(conn, cfg)
	if err := ret.Handshake(); err != nil {
		ret.Close()
		return nil, err
	}
	return ret, nil
}

// NewConnSrc returns a chan which can be used to receive connections
// on the passed Listener. All requests sent to the returned chan will have the
// instance name provided here. The chan will be closed if the Listener returns
// an error.
func NewConnSrc(instance string, l net.Listener) <-chan Conn {
	ch := make(chan Conn)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				logging.Errorf("listener (%#v) had error: %v", l, err)
				l.Close()
				close(ch)
				return
			}
			ch <- Conn{instance, c}
		}
	}()
	return ch
}

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
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
	"golang.org/x/net/proxy"
)

const (
	DefaultRefreshCfgThrottle = time.Minute
	keepAlivePeriod           = time.Minute
	defaultRefreshCfgBuffer   = 5 * time.Minute
)

var (
	// errNotCached is returned when the instance was not found in the Client's
	// cache. It is an internal detail and is not actually ever returned to the
	// user.
	errNotCached = errors.New("instance was not found in cache")
)

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
	Remote(instance string) (cert *x509.Certificate, addr, name, version string, err error)
}

// Client is a type to handle connecting to a Server. All fields are required
// unless otherwise specified.
type Client struct {
	// ConnectionsCounter is used to enforce the optional maxConnections limit
	ConnectionsCounter uint64

	// MaxConnections is the maximum number of connections to establish
	// before refusing new connections. 0 means no limit.
	MaxConnections uint64

	// Port designates which remote port should be used when connecting to
	// instances. This value is defined by the server-side code, but for now it
	// should always be 3307.
	Port int
	// Required; specifies how certificates are obtained.
	Certs CertSource
	// Optionally tracks connections through this client. If nil, connections
	// are not tracked and will not be closed before method Run exits.
	Conns *ConnSet
	// ContextDialer should return a new connection to the provided address.
	// It is called on each new connection to an instance.
	// If left nil, Dialer will be tried first, and if that one is nil too then net.Dial will be used.
	ContextDialer func(ctx context.Context, net, addr string) (net.Conn, error)
	// Dialer should return a new connection to the provided address. It will be used only if ContextDialer is nil.
	Dialer func(net, addr string) (net.Conn, error)

	// The cfgCache holds the most recent connection configuration keyed by
	// instance. Relevant functions are refreshCfg and cachedCfg. It is
	// protected by cacheL.
	cfgCache map[string]cacheEntry
	cacheL   sync.RWMutex

	// refreshCfgL prevents multiple goroutines from contacting the Cloud SQL API at once.
	refreshCfgL sync.Mutex

	// RefreshCfgThrottle is the amount of time to wait between configuration
	// refreshes. If not set, it defaults to 1 minute.
	//
	// This is to prevent quota exhaustion in the case of client-side
	// malfunction.
	RefreshCfgThrottle time.Duration

	// RefreshCertBuffer is the amount of time before the configuration expires to
	// attempt to refresh it. If not set, it defaults to 5 minutes.
	RefreshCfgBuffer time.Duration
}

type cacheEntry struct {
	lastRefreshed time.Time
	// If err is not nil, the addr and cfg are not valid.
	err     error
	addr    string
	version string
	cfg     *tls.Config
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
	active := atomic.AddUint64(&c.ConnectionsCounter, 1)

	// Deferred decrement of ConnectionsCounter upon connection closing
	defer atomic.AddUint64(&c.ConnectionsCounter, ^uint64(0))

	if c.MaxConnections > 0 && active > c.MaxConnections {
		logging.Errorf("too many open connections (max %d)", c.MaxConnections)
		conn.Conn.Close()
		return
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
func (c *Client) refreshCfg(instance string) (addr string, cfg *tls.Config, version string, err error) {
	c.refreshCfgL.Lock()
	defer c.refreshCfgL.Unlock()

	throttle := c.RefreshCfgThrottle
	if throttle == 0 {
		throttle = DefaultRefreshCfgThrottle
	}

	refreshCfgBuffer := c.RefreshCfgBuffer
	if refreshCfgBuffer == 0 {
		refreshCfgBuffer = defaultRefreshCfgBuffer
	}

	c.cacheL.Lock()
	if c.cfgCache == nil {
		c.cfgCache = make(map[string]cacheEntry)
	}
	old, oldok := c.cfgCache[instance]
	c.cacheL.Unlock()
	if oldok && time.Since(old.lastRefreshed) < throttle {
		logging.Errorf("Throttling refreshCfg(%s): it was only called %v ago", instance, time.Since(old.lastRefreshed))
		// Refresh was called too recently, just reuse the result.
		return old.addr, old.cfg, old.version, old.err
	}

	defer func() {
		// if we failed to refresh cfg do not throw out potentially valid one
		if err != nil && !isExpired(old.cfg) {
			logging.Errorf("failed to refresh the ephemeral certificate for %s, returning previous cert instead: %v", instance, err)
			addr, cfg, version, err = old.addr, old.cfg, old.version, old.err
		}

		c.cacheL.Lock()
		c.cfgCache[instance] = cacheEntry{
			lastRefreshed: time.Now(),
			err:           err,
			addr:          addr,
			version:       version,
			cfg:           cfg,
		}
		c.cacheL.Unlock()
	}()

	mycert, err := c.Certs.Local(instance)
	if err != nil {
		return "", nil, "", err
	}

	scert, addr, name, version, err := c.Certs.Remote(instance)
	if err != nil {
		return "", nil, "", err
	}
	certs := x509.NewCertPool()
	certs.AddCert(scert)

	cfg = &tls.Config{
		ServerName:   name,
		Certificates: []tls.Certificate{mycert},
		RootCAs:      certs,
		// We need to set InsecureSkipVerify to true due to
		// https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/194
		// https://tip.golang.org/doc/go1.11#crypto/x509
		//
		// Since we have a secure channel to the Cloud SQL API which we use to retrieve the
		// certificates, we instead need to implement our own VerifyPeerCertificate function
		// that will verify that the certificate is OK.
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: genVerifyPeerCertificateFunc(name, certs),
	}

	expire := mycert.Leaf.NotAfter
	now := time.Now()
	timeToRefresh := expire.Sub(now) - refreshCfgBuffer
	if timeToRefresh <= 0 {
		err = fmt.Errorf("new ephemeral certificate expires too soon: current time: %v, certificate expires: %v", expire, now)
		logging.Errorf("ephemeral certificate (%+v) error: %v", mycert, err)
		return "", nil, "", err
	}
	go c.refreshCertAfter(instance, timeToRefresh)

	return fmt.Sprintf("%s:%d", addr, c.Port), cfg, version, nil
}

// refreshCertAfter refreshes the epehemeral certificate of the instance after timeToRefresh.
func (c *Client) refreshCertAfter(instance string, timeToRefresh time.Duration) {
	<-time.After(timeToRefresh)
	logging.Verbosef("ephemeral certificate for instance %s will expire soon, refreshing now.", instance)
	if _, _, _, err := c.refreshCfg(instance); err != nil {
		logging.Errorf("failed to refresh the ephemeral certificate for %s before expiring: %v", instance, err)
	}
}

// genVerifyPeerCertificateFunc creates a VerifyPeerCertificate func that verifies that the peer
// certificate is in the cert pool. We need to define our own because of our sketchy non-standard
// CNs.
func genVerifyPeerCertificateFunc(instanceName string, pool *x509.CertPool) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("no certificate to verify")
		}

		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("x509.ParseCertificate(rawCerts[0]) returned error: %v", err)
		}

		opts := x509.VerifyOptions{Roots: pool}
		if _, err = cert.Verify(opts); err != nil {
			return err
		}

		if cert.Subject.CommonName != instanceName {
			return fmt.Errorf("certificate had CN %q, expected %q", cert.Subject.CommonName, instanceName)
		}
		return nil
	}
}

func isExpired(cfg *tls.Config) bool {
	if cfg == nil {
		return true
	}
	return time.Now().After(cfg.Certificates[0].Leaf.NotAfter)
}

func (c *Client) cachedCfg(instance string) (string, *tls.Config, string) {
	c.cacheL.RLock()
	ret, ok := c.cfgCache[instance]
	c.cacheL.RUnlock()

	// Don't waste time returning an expired/invalid cert.
	if !ok || ret.err != nil || isExpired(ret.cfg) {
		return "", nil, ""
	}
	return ret.addr, ret.cfg, ret.version
}

// DialContext uses the configuration stored in the client to connect to an instance.
// If this func returns a nil error the connection is correctly authenticated
// to connect to the instance.
func (c *Client) DialContext(ctx context.Context, instance string) (net.Conn, error) {
	if addr, cfg, _ := c.cachedCfg(instance); cfg != nil {
		ret, err := c.tryConnect(ctx, addr, cfg)
		if err == nil {
			return ret, err
		}
	}

	addr, cfg, _, err := c.refreshCfg(instance)
	if err != nil {
		return nil, err
	}
	return c.tryConnect(ctx, addr, cfg)
}

// Dial does the same as DialContext but using context.Background() as the context.
func (c *Client) Dial(instance string) (net.Conn, error) {
	return c.DialContext(context.Background(), instance)
}

func (c *Client) tryConnect(ctx context.Context, addr string, cfg *tls.Config) (net.Conn, error) {
	dial := c.selectDialer()
	conn, err := dial(ctx, "tcp", addr)
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

func (c *Client) selectDialer() func(context.Context, string, string) (net.Conn, error) {
	if c.ContextDialer != nil {
		return c.ContextDialer
	}

	if c.Dialer != nil {
		return func(_ context.Context, net, addr string) (net.Conn, error) {
			return c.Dialer(net, addr)
		}
	}

	dialer := proxy.FromEnvironment()
	if ctxDialer, ok := dialer.(proxy.ContextDialer); ok {
		// although proxy.FromEnvironment() returns a Dialer interface which only has a Dial method,
		// it happens in fact that method often returns ContextDialers.
		return ctxDialer.DialContext
	}

	return func(_ context.Context, net, addr string) (net.Conn, error) {
		return dialer.Dial(net, addr)
	}
}

// NewConnSrc returns a chan which can be used to receive connections
// on the passed Listener. All requests sent to the returned chan will have the
// instance name provided here. The chan will be closed if the Listener returns
// an error.
func NewConnSrc(instance string, l net.Listener) <-chan Conn {
	ch := make(chan Conn)
	go func() {
		for {
			start := time.Now()
			c, err := l.Accept()
			if err != nil {
				logging.Errorf("listener (%#v) had error: %v", l, err)
				if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
					d := 10*time.Millisecond - time.Since(start)
					if d > 0 {
						time.Sleep(d)
					}
					continue
				}
				l.Close()
				close(ch)
				return
			}
			ch <- Conn{instance, c}
		}
	}()
	return ch
}

// InstanceVersion uses client cache to return instance version string.
func (c *Client) InstanceVersion(instance string) (string, error) {
	if _, cfg, version := c.cachedCfg(instance); cfg != nil {
		return version, nil
	}
	_, _, version, err := c.refreshCfg(instance)
	if err != nil {
		return "", err
	}
	return version, nil
}

// Shutdown waits up to a given amount of time for all active connections to
// close. Returns an error if there are still active connections after waiting
// for the whole length of the timeout.
func (c *Client) Shutdown(termTimeout time.Duration) error {
	term, ticker := time.After(termTimeout), time.NewTicker(100*time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if atomic.LoadUint64(&c.ConnectionsCounter) > 0 {
				continue
			}
		case <-term:
		}
		break
	}

	active := atomic.LoadUint64(&c.ConnectionsCounter)
	if active == 0 {
		return nil
	}
	return fmt.Errorf("%d active connections still exist after waiting for %v", active, termTimeout)
}

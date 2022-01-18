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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/util"
	"golang.org/x/net/proxy"
	"golang.org/x/time/rate"
)

const (
	// DefaultRefreshCfgThrottle is the time a refresh attempt must wait since
	// the last attempt.
	DefaultRefreshCfgThrottle = time.Minute
	// IAMLoginRefreshThrottle is the time a refresh attempt must wait since the
	// last attempt when using IAM login.
	IAMLoginRefreshThrottle = 30 * time.Second
	keepAlivePeriod         = time.Minute
	// DefaultRefreshCfgBuffer is the minimum amount of time for which a
	// certificate must be valid to ensure the next refresh attempt has adequate
	// time to complete.
	DefaultRefreshCfgBuffer = 5 * time.Minute
	// IAMLoginRefreshCfgBuffer is the minimum amount of time for which a
	// certificate holding an Access Token must be valid. Because some token
	// sources (e.g., ouath2.ComputeTokenSource) are refreshed with only ~60
	// seconds before expiration, this value must be smaller than the
	// DefaultRefreshCfgBuffer.
	IAMLoginRefreshCfgBuffer = 55 * time.Second
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
	// limiters holds a rate limiter keyed by instance. It is protected by
	// cacheL.
	limiters map[string]*rate.Limiter

	// refreshCfgL prevents multiple goroutines from contacting the Cloud SQL API at once.
	refreshCfgL sync.Mutex

	// RefreshCfgThrottle is the amount of time to wait between configuration
	// refreshes. If not set, it defaults to 1 minute.
	//
	// This is to prevent quota exhaustion in the case of client-side
	// malfunction.
	RefreshCfgThrottle time.Duration

	// RefreshCertBuffer is the amount of time before the configuration expires
	// to attempt to refresh it. If not set, it defaults to 5 minutes. When IAM
	// Login is enabled, this value should be set to IAMLoginRefreshCfgBuffer.
	RefreshCfgBuffer time.Duration
}

type cacheEntry struct {
	lastRefreshed time.Time
	// If err is not nil, the addr and cfg are not valid.
	err     error
	addr    string
	version string
	cfg     *tls.Config
	// done represents the status of any pending refresh operation related to this instance.
	// If unset the op hasn't started, if open the op is still pending, and if closed the op has finished.
	done chan struct{}
}

// Run causes the client to start waiting for new connections to connSrc and
// proxy them to the destination instance. It blocks until connSrc is closed.
func (c *Client) Run(connSrc <-chan Conn) {
	c.RunContext(context.Background(), connSrc)
}

func (c *Client) run(ctx context.Context, connSrc <-chan Conn) {
	for {
		select {
		case conn, ok := <-connSrc:
			if !ok {
				return
			}
			go c.handleConn(ctx, conn)
		case <-ctx.Done():
			return
		}
	}
}

// RunContext is like Run with an additional context.Context argument.
func (c *Client) RunContext(ctx context.Context, connSrc <-chan Conn) {
	c.run(ctx, connSrc)

	if err := c.Conns.Close(); err != nil {
		logging.Errorf("closing client had error: %v", err)
	}
}

func (c *Client) handleConn(ctx context.Context, conn Conn) {
	active := atomic.AddUint64(&c.ConnectionsCounter, 1)

	// Deferred decrement of ConnectionsCounter upon connection closing
	defer atomic.AddUint64(&c.ConnectionsCounter, ^uint64(0))

	if c.MaxConnections > 0 && active > c.MaxConnections {
		logging.Errorf("too many open connections (max %d)", c.MaxConnections)
		conn.Conn.Close()
		return
	}

	server, err := c.DialContext(ctx, conn.Instance)
	if err != nil {
		logging.Errorf("couldn't connect to %q: %v", conn.Instance, err)
		conn.Conn.Close()
		return
	}

	c.Conns.Add(conn.Instance, conn.Conn)
	copyThenClose(server, conn.Conn, conn.Instance, "local connection on "+conn.Conn.LocalAddr().String())

	if err := c.Conns.Remove(conn.Instance, conn.Conn); err != nil {
		logging.Errorf("%s", err)
	}
}

// refreshCfg uses the CertSource inside the Client to find the instance's
// address as well as construct a new tls.Config to connect to the instance.
// This function should only be called from the scope of "cachedCfg", which
// controls the logic around throttling.
func (c *Client) refreshCfg(instance string) (addr string, cfg *tls.Config, version string, err error) {
	c.refreshCfgL.Lock()
	defer c.refreshCfgL.Unlock()
	logging.Verbosef("refreshing ephemeral certificate for instance %s", instance)

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
		MinVersion:            tls.VersionTLS13,
	}

	return fmt.Sprintf("%s:%d", addr, c.Port), cfg, version, nil
}

// refreshCertAfter refreshes the epehemeral certificate of the instance after timeToRefresh.
func (c *Client) refreshCertAfter(instance string, timeToRefresh time.Duration) {
	<-time.After(timeToRefresh)
	logging.Verbosef("ephemeral certificate for instance %s will expire soon, refreshing now.", instance)
	if _, _, _, err := c.cachedCfg(context.Background(), instance); err != nil {
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

// startRefresh kicks off a refreshCfg asynchronously, that updates the cacheEntry and closes the returned channel once the refresh is completed. This function
// should only be called from the scope of "cachedCfg", which controls the logic around throttling refreshes.
func (c *Client) startRefresh(instance string, refreshCfgBuffer time.Duration) chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		addr, cfg, ver, err := c.refreshCfg(instance)

		c.cacheL.Lock()
		old := c.cfgCache[instance]
		// if we failed to refresh cfg do not throw out potentially valid one
		if err != nil && !isExpired(old.cfg) {
			logging.Errorf("failed to refresh the ephemeral certificate for %s, returning previous cert instead: %v", instance, err)
			addr, cfg, ver, err = old.addr, old.cfg, old.version, old.err
		}
		e := cacheEntry{
			lastRefreshed: time.Now(),
			err:           err,
			addr:          addr,
			version:       ver,
			cfg:           cfg,
			done:          done,
		}
		c.cfgCache[instance] = e
		c.cacheL.Unlock()

		if !isValid(e) {
			// Note: Future refreshes will not be scheduled unless another
			// connection attempt is made.
			logging.Errorf("failed to refresh the ephemeral certificate for %v: %v", instance, err)
			return
		}

		certExpiration := cfg.Certificates[0].Leaf.NotAfter
		now := time.Now()
		timeToRefresh := certExpiration.Sub(now) - refreshCfgBuffer
		if timeToRefresh <= 0 {
			// If a new certificate expires before our buffer has expired, we should wait a bit and schedule a new refresh to much closer to the expiration's date
			// This situation probably only occurs when the oauth2 token isn't refreshed before the cert is, so by scheduling closer to the expiration we can hope the oauth2 token is newer.
			timeToRefresh = certExpiration.Sub(now) - (5 * time.Second)
			logging.Errorf("new ephemeral certificate expires sooner than expected (adjusting refresh time to compensate): current time: %v, certificate expires: %v", now, certExpiration)
		}
		logging.Infof("Scheduling refresh of ephemeral certificate in %s", timeToRefresh)
		go c.refreshCertAfter(instance, timeToRefresh)
	}()
	return done
}

// isValid returns true if the cacheEntry is still useable
func isValid(c cacheEntry) bool {
	// the entry is only valid there wasn't an error retrieving it and it has a cfg
	return c.err == nil && c.cfg != nil
}

// InvalidError is an error from an instance connection that is invalid because
// its recent refresh attempt has failed, its TLS config is invalid, etc.
type InvalidError struct {
	// instance is the instance connection name
	instance string
	// err is what makes the instance invalid
	err error
	// hasTLS reports whether the instance has a valid TLS config
	hasTLS bool
}

func (e *InvalidError) Error() string {
	if e.hasTLS {
		return e.instance + ": " + e.err.Error()
	}
	return e.instance + ": missing TLS config, " + e.err.Error()
}

// InvalidInstances reports whether the existing connections have valid
// configuration.
func (c *Client) InvalidInstances() []*InvalidError {
	c.cacheL.RLock()
	defer c.cacheL.RUnlock()

	var invalid []*InvalidError
	for instance, entry := range c.cfgCache {
		var refreshInProgress bool
		select {
		case <-entry.done:
			// refresh has already completed
		default:
			refreshInProgress = true
		}
		if !isValid(entry) && !refreshInProgress {
			invalid = append(invalid, &InvalidError{
				instance: instance,
				err:      entry.err,
				hasTLS:   entry.cfg != nil,
			})
		}
	}
	return invalid
}

func needsRefresh(e cacheEntry, refreshCfgBuffer time.Duration) bool {
	if e.done == nil { // no refresh started
		return true
	}
	if !isValid(e) || e.cfg.Certificates[0].Leaf.NotAfter.Sub(time.Now()) <= refreshCfgBuffer {
		// if the entry is invalid or close enough to expiring check
		// use the entry's done channel to determine if a refresh has started yet
		select {
		case <-e.done: // last refresh completed, so it's time for a new one
			return true
		default: // new refresh already started, so we can wait on that
			return false
		}
	}
	return false
}

func (c *Client) cachedCfg(ctx context.Context, instance string) (string, *tls.Config, string, error) {
	c.cacheL.RLock()

	throttle := c.RefreshCfgThrottle
	if throttle == 0 {
		throttle = DefaultRefreshCfgThrottle
	}
	refreshCfgBuffer := c.RefreshCfgBuffer
	if refreshCfgBuffer == 0 {
		refreshCfgBuffer = DefaultRefreshCfgBuffer
	}

	e := c.cfgCache[instance]
	c.cacheL.RUnlock()
	if needsRefresh(e, refreshCfgBuffer) {
		// Reenter the critical section with intent to make changes
		c.cacheL.Lock()
		if c.cfgCache == nil {
			c.cfgCache = make(map[string]cacheEntry)
		}
		if c.limiters == nil {
			c.limiters = make(map[string]*rate.Limiter)
		}
		// the state may have changed between critical sections, so double check
		e = c.cfgCache[instance]
		limiter := c.limiters[instance]
		if limiter == nil {
			limiter = rate.NewLimiter(rate.Every(throttle), 2)
			c.limiters[instance] = limiter
		}
		if needsRefresh(e, refreshCfgBuffer) {
			if limiter.Allow() {
				// start a new refresh and update the cachedEntry to reflect that
				e.done = c.startRefresh(instance, refreshCfgBuffer)
				e.lastRefreshed = time.Now()
				c.cfgCache[instance] = e
			} else {
				// TODO: Investigate returning this as an error instead of just logging
				logging.Infof("refresh operation throttled for %s: reusing config from last refresh (%s ago)", instance, time.Since(e.lastRefreshed))
			}
		}
		c.cacheL.Unlock()
	}

	if !isValid(e) {
		// if the previous result was invalid, wait for the next result to complete
		select {
		case <-ctx.Done():
			return "", nil, "", ctx.Err()
		case <-e.done:
		}

		c.cacheL.RLock()
		// the state may have changed between critical sections, so double check
		e = c.cfgCache[instance]
		c.cacheL.RUnlock()
	}
	return e.addr, e.cfg, e.version, e.err
}

// DialContext uses the configuration stored in the client to connect to an instance.
// If this func returns a nil error the connection is correctly authenticated
// to connect to the instance.
func (c *Client) DialContext(ctx context.Context, instance string) (net.Conn, error) {
	addr, cfg, _, err := c.cachedCfg(ctx, instance)
	if err != nil {
		return nil, err
	}

	// TODO: attempt an early refresh if an connect fails?
	return c.tryConnect(ctx, addr, instance, cfg)
}

// Dial does the same as DialContext but using context.Background() as the context.
func (c *Client) Dial(instance string) (net.Conn, error) {
	return c.DialContext(context.Background(), instance)
}

// ErrUnexpectedFailure indicates the internal refresh operation failed unexpectedly.
var ErrUnexpectedFailure = errors.New("ErrUnexpectedFailure")

func (c *Client) tryConnect(ctx context.Context, addr, instance string, cfg *tls.Config) (net.Conn, error) {
	// When multiple dial attempts start in quick succession, the internal
	// refresh logic is sometimes subject to a race condition. If the first
	// attempt fails on a handshake error, it will invalidate the cached config.
	// In some cases, a second dial attempt will initiate a connection with an
	// invalid config. This check fails fast in such cases.
	if addr == "" {
		return nil, ErrUnexpectedFailure
	}
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

	return c.connectTLS(ctx, conn, instance, cfg)
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

func (c *Client) invalidateCfg(cfg *tls.Config, instance string) {
	c.cacheL.RLock()
	e := c.cfgCache[instance]
	c.cacheL.RUnlock()
	if e.cfg != cfg {
		return
	}
	c.cacheL.Lock()
	defer c.cacheL.Unlock()
	e = c.cfgCache[instance]
	// the state may have changed between critical sections, so double check
	if e.cfg != cfg {
		return
	}
	c.cfgCache[instance] = cacheEntry{
		done:          e.done,
		lastRefreshed: e.lastRefreshed,
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
//
// Deprecated: Use Client.InstanceVersionContext instead.
func (c *Client) InstanceVersion(instance string) (string, error) {
	return c.InstanceVersionContext(context.Background(), instance)
}

// InstanceVersionContext uses client cache to return instance version string.
func (c *Client) InstanceVersionContext(ctx context.Context, instance string) (string, error) {
	_, _, version, err := c.cachedCfg(ctx, instance)
	if err != nil {
		return "", err
	}
	return version, nil
}

// ParseInstanceConnectionName verifies that instances are in the expected format and include
// the necessary components.
func ParseInstanceConnectionName(instance string) (string, string, string, []string, error) {
	args := strings.Split(instance, "=")
	if len(args) > 2 {
		return "", "", "", nil, fmt.Errorf("invalid instance argument: must be either form - `<instance_connection_string>` or `<instance_connection_string>=<options>`; invalid arg was %q", instance)
	}
	// Parse the instance connection name - everything before the "=".
	proj, region, name := util.SplitName(args[0])
	if proj == "" || region == "" || name == "" {
		return "", "", "", nil, fmt.Errorf("invalid instance connection string: must be in the form `project:region:instance-name`; invalid name was %q", args[0])
	}
	return proj, region, name, args, nil
}

// GetInstances iterates through the client cache, returning a list of previously dialed
// instances.
func (c *Client) GetInstances() []string {
	var insts []string
	c.cacheL.Lock()
	cfgCache := c.cfgCache
	c.cacheL.Unlock()
	for i := range cfgCache {
		insts = append(insts, i)
	}
	return insts
}

// AvailableConn returns false if MaxConnections has been reached, true otherwise.
// When MaxConnections is 0, there is no limit.
func (c *Client) AvailableConn() bool {
	return c.MaxConnections == 0 || atomic.LoadUint64(&c.ConnectionsCounter) < c.MaxConnections
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

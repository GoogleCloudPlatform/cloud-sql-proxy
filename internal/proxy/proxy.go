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

package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/connection"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/gcloud"
	"golang.org/x/oauth2"
)

// InstanceConnConfig holds the configuration for an individual instance
// connection.
type InstanceConnConfig struct {
	// Name is the instance connection name.
	Name string
	// Addr is the address on which to bind a listener for the instance.
	Addr string
	// Port is the port on which to bind a listener for the instance.
	Port int
	// UnixSocket is the directory where a Unix socket will be created,
	// connected to the Cloud SQL instance. If set, takes precedence over Addr
	// and Port.
	UnixSocket string
	// IAMAuthN enables automatic IAM DB Authentication for the instance.
	// Postgres-only. If it is nil, the value was not specified.
	IAMAuthN *bool

	// PrivateIP tells the proxy to attempt to connect to the db instance's
	// private IP address instead of the public IP address
	PrivateIP *bool
}

// Config contains all the configuration provided by the caller.
type Config struct {
	// UserAgent is the user agent to use when connecting to the cloudsql instance
	UserAgent string

	// Token is the Bearer token used for authorization.
	Token string

	// CredentialsFile is the path to a service account key.
	CredentialsFile string

	// GcloudAuth set whether to use Gcloud's config helper to retrieve a
	// token for authentication.
	GcloudAuth bool

	// Addr is the address on which to bind all instances.
	Addr string

	// Port is the initial port to bind to. Subsequent instances bind to
	// increments from this value.
	Port int

	// APIEndpointURL is the URL of the google cloud sql api. When left blank,
	// the proxy will use the main public api: https://sqladmin.googleapis.com/
	APIEndpointURL string

	// UnixSocket is the directory where Unix sockets will be created,
	// connected to any Instances. If set, takes precedence over Addr and Port.
	UnixSocket string

	// IAMAuthN enables automatic IAM DB Authentication for all instances.
	// Postgres-only.
	IAMAuthN bool

	// MaxConnections are the maximum number of connections the Client may
	// establish to the Cloud SQL server side proxy before refusing additional
	// connections. A zero-value indicates no limit.
	MaxConnections uint64

	// WaitOnClose sets the duration to wait for connections to close before
	// shutting down. Not setting this field means to close immediately
	// regardless of any open connections.
	WaitOnClose time.Duration

	// PrivateIP enables connections via the database server's private IP address
	// for all instances.
	PrivateIP bool

	// Instances are configuration for individual instances. Instance
	// configuration takes precedence over global configuration.
	Instances []InstanceConnConfig

	// QuotaProject is the ID of the Google Cloud project to use to track
	// API request quotas.
	QuotaProject string

	// StructuredLogs sets all output to use JSON in the LogEntry format.
	// See https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry
	StructuredLogs bool

	// Dialer specifies the dialer to use when connecting to Cloud SQL
	// instances.
	Dialer cloudsql.Dialer
}

// DialOptions interprets appropriate dial options for a particular instance
// configuration
func (c *Config) DialOptions(i InstanceConnConfig) []cloudsqlconn.DialOption {
	var opts []cloudsqlconn.DialOption

	if i.IAMAuthN != nil {
		opts = append(opts, cloudsqlconn.WithDialIAMAuthN(*i.IAMAuthN))
	}

	if i.PrivateIP != nil && *i.PrivateIP || i.PrivateIP == nil && c.PrivateIP {
		opts = append(opts, cloudsqlconn.WithPrivateIP())
	} else {
		opts = append(opts, cloudsqlconn.WithPublicIP())
	}

	return opts
}

// DialerOptions builds appropriate list of options from the Config
// values for use by cloudsqlconn.NewClient()
func (c *Config) DialerOptions(l cloudsql.Logger) ([]cloudsqlconn.Option, error) {
	opts := []cloudsqlconn.Option{
		cloudsqlconn.WithUserAgent(c.UserAgent),
	}
	switch {
	case c.Token != "":
		l.Infof("Authorizing with the -token flag")
		opts = append(opts, cloudsqlconn.WithTokenSource(
			oauth2.StaticTokenSource(&oauth2.Token{AccessToken: c.Token}),
		))
	case c.CredentialsFile != "":
		l.Infof("Authorizing with the credentials file at %q", c.CredentialsFile)
		opts = append(opts, cloudsqlconn.WithCredentialsFile(
			c.CredentialsFile,
		))
	case c.GcloudAuth:
		l.Infof("Authorizing with gcloud user credentials")
		ts, err := gcloud.TokenSource()
		if err != nil {
			return nil, err
		}
		opts = append(opts, cloudsqlconn.WithTokenSource(ts))
	default:
		l.Infof("Authorizing with Application Default Credentials")
	}

	if c.APIEndpointURL != "" {
		opts = append(opts, cloudsqlconn.WithAdminAPIEndpoint(c.APIEndpointURL))
	}

	if c.IAMAuthN {
		opts = append(opts, cloudsqlconn.WithIAMAuthN())
	}

	if c.QuotaProject != "" {
		opts = append(opts, cloudsqlconn.WithQuotaProject(c.QuotaProject))
	}

	return opts, nil
}

type portConfig struct {
	global    int
	postgres  int
	mysql     int
	sqlserver int
}

func newPortConfig(global int) *portConfig {
	return &portConfig{
		global:    global,
		postgres:  5432,
		mysql:     3306,
		sqlserver: 1433,
	}
}

// nextPort returns the next port based on the initial global value.
func (c *portConfig) nextPort() int {
	p := c.global
	c.global++
	return p
}

func (c *portConfig) nextDBPort(version string) int {
	switch {
	case strings.HasPrefix(version, "MYSQL"):
		p := c.mysql
		c.mysql++
		return p
	case strings.HasPrefix(version, "POSTGRES"):
		p := c.postgres
		c.postgres++
		return p
	case strings.HasPrefix(version, "SQLSERVER"):
		p := c.sqlserver
		c.sqlserver++
		return p
	default:
		// Unexpected engine version, use global port setting instead.
		return c.nextPort()
	}
}

// socketProxy is a server that uses TCP or Unix domain sockets to proxy traffic
// to and from a Cloud SQL insteand and the corresponding socket.
type socketProxy struct {
	sockets []*socket
	conf    *Config
	dialer  cloudsql.Dialer
	logger  cloudsql.Logger
	counter *connection.Counter
}

// newSocketProxy creates a socketProxy.
func newSocketProxy(ctx context.Context, d cloudsql.Dialer, l cloudsql.Logger, c *connection.Counter, conf *Config) (*socketProxy, error) {
	var mnts []*socket
	pc := newPortConfig(conf.Port)
	for _, inst := range conf.Instances {
		version, err := d.EngineVersion(ctx, inst.Name)
		if err != nil {
			return nil, err
		}

		m, err := newSocket(ctx, conf, pc, inst, version)
		if err != nil {
			for _, m := range mnts {
				mErr := m.listener.Close()
				if mErr != nil {
					l.Errorf("failed to close mount: %v", mErr)
				}
			}
			return nil, fmt.Errorf("[%v] Unable to mount socket: %v", inst.Name, err)
		}

		l.Infof("[%s] Listening on %s", inst.Name, m.listener.Addr())
		mnts = append(mnts, m)
	}
	return &socketProxy{
		sockets: mnts,
		conf:    conf,
		dialer:  d,
		logger:  l,
		counter: c,
	}, nil
}

func (s *socketProxy) Serve(ctx context.Context, exitCh chan<- error) {
	for _, m := range s.sockets {
		go func(mnt *socket) {
			err := connection.AcceptAndHandle(ctx, mnt.listener, s.logger, s.dialer, s.counter, mnt.inst, mnt.dialOpts...)
			if err != nil {
				select {
				// Best effort attempt to send error.
				// If this send fails, it means the reading goroutine has
				// already pulled a value out of the channel and is no longer
				// reading any more values. In other words, we report only the
				// first error.
				case exitCh <- err:
				default:
					return
				}
			}
		}(m)
	}
}

// CheckConnections verifies that each socket can reach its backing Cloud SQL
// instance.
func (s *socketProxy) CheckConnections(ctx context.Context) error {
	var (
		wg    sync.WaitGroup
		errCh = make(chan error, len(s.sockets))
	)
	for _, m := range s.sockets {
		wg.Add(1)
		go func(inst string) {
			defer wg.Done()
			conn, err := s.dialer.Dial(ctx, inst)
			if err != nil {
				errCh <- err
				return
			}
			cErr := conn.Close()
			if err != nil {
				errCh <- fmt.Errorf("%v: %v", inst, cErr)
			}
		}(m.inst)
	}
	wg.Wait()

	var mErr MultiErr
	for i := 0; i < len(s.sockets); i++ {
		select {
		case err := <-errCh:
			mErr = append(mErr, err)
		default:
			continue
		}
	}
	if len(mErr) > 0 {
		return mErr
	}
	return nil
}

// Close closes all open listeners and stops the dialer from refreshing any
// further.
func (s *socketProxy) Close() error {
	var mErr MultiErr
	// First, close all open socket listeners to prevent additional connections.
	for _, m := range s.sockets {
		err := m.listener.Close()
		if err != nil {
			// TODO
			mErr = append(mErr, err)
		}
	}
	// Next, close the dialer to prevent any additional refreshes.
	cErr := s.dialer.Close()
	if cErr != nil {
		mErr = append(mErr, cErr)
	}
	if len(mErr) > 0 {
		return mErr
	}
	return nil
}

// ProxyServer serves local listeners that proxy traffic to remote Cloud SQL
// instances.
type ProxyServer interface {
	Serve(ctx context.Context, exitCh chan<- error)
	CheckConnections(context.Context) error
	io.Closer
}

// Session represents a single invocation of the Cloud SQL Auth proxy and
// orchestrates local listeners connected to remote Cloud SQL instances.
type Session struct {
	ps     ProxyServer
	dialer cloudsql.Dialer

	// counter tracks the number of open connections
	counter *connection.Counter

	// waitOnClose is the maximum duration to wait for open connections to close
	// when shutting down.
	waitOnClose time.Duration

	logger cloudsql.Logger
}

// NewSession completes the initial setup for a Session based on the provided
// configuration.
func NewSession(ctx context.Context, d cloudsql.Dialer, l cloudsql.Logger, conf *Config) (*Session, error) {
	// Check if the caller has configured a dialer.
	// Otherwise, initialize a new one.
	if d == nil {
		dialerOpts, err := conf.DialerOptions(l)
		if err != nil {
			return nil, fmt.Errorf("error initializing dialer: %v", err)
		}
		d, err = cloudsqlconn.NewDialer(ctx, dialerOpts...)
		if err != nil {
			return nil, fmt.Errorf("error initializing dialer: %v", err)
		}
	}

	for _, inst := range conf.Instances {
		// Initiate refresh operation and warm the cache.
		go func(name string) { d.EngineVersion(ctx, name) }(inst.Name)
	}

	counter := connection.NewCounter(conf.MaxConnections)
	var (
		ps  ProxyServer
		err error
	)
	ps, err = newSocketProxy(ctx, d, l, counter, conf)
	if err != nil {
		return nil, err
	}

	c := &Session{
		ps:          ps,
		logger:      l,
		dialer:      d,
		counter:     counter,
		waitOnClose: conf.WaitOnClose,
	}
	return c, nil
}

// CheckConnections dials each registered instance and reports any errors that
// may have occurred.
func (s *Session) CheckConnections(ctx context.Context) error {
	return s.ps.CheckConnections(ctx)
}

// ConnCount returns the number of open connections and the maximum allowed
// connections. Returns 0 when the maximum allowed connections have not been set.
func (s *Session) ConnCount() (uint64, uint64) {
	return s.counter.Count()
}

// Start starts proxying connections for all configured instances using the
// associated socket.
func (s *Session) Start(ctx context.Context, notify func()) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	exitCh := make(chan error)
	s.ps.Serve(ctx, exitCh)
	notify()
	return <-exitCh
}

// MultiErr is a group of errors wrapped into one.
type MultiErr []error

// Error returns a single string representing one or more errors.
func (m MultiErr) Error() string {
	l := len(m)
	if l == 1 {
		return m[0].Error()
	}
	var errs []string
	for _, e := range m {
		errs = append(errs, e.Error())
	}
	return strings.Join(errs, ", ")
}

// Close concludes the session by closing the proxy server. If waitOnClose is
// configured, Close waits up to waitOnClose duration for connections to close
// and then shuts down.
func (s *Session) Close() error {
	var (
		mErr MultiErr
		ok   bool
	)
	if err := s.ps.Close(); err != nil {
		mErr, ok = err.(MultiErr)
		// If it's not a MultiErr, just append it.
		if !ok {
			mErr = append(mErr, err)
		}
	}
	if s.waitOnClose == 0 {
		if len(mErr) > 0 {
			return mErr
		}
		return nil
	}
	timeout := time.After(s.waitOnClose)
	tick := time.Tick(100 * time.Millisecond)
	for {
		select {
		case <-tick:
			if !s.counter.IsZero() {
				continue
			}
		case <-timeout:
		}
		break
	}
	open, _ := s.counter.Count()
	if open > 0 {
		mErr = append(mErr, fmt.Errorf("%d connection(s) still open after waiting %v", open, s.waitOnClose))
	}
	if len(mErr) > 0 {
		return mErr
	}
	return nil
}

// socket is a TCP or UNIX domain socket that proxies traffic to a remove Cloud
// SQL instance.
type socket struct {
	inst     string
	dialOpts []cloudsqlconn.DialOption
	listener net.Listener
}

// newSocket initializes the socket based on the provided configure and
// instance-level overrides.
func newSocket(ctx context.Context, c *Config, pc *portConfig, inst InstanceConnConfig, version string) (*socket, error) {
	var (
		// network is one of "tcp" or "unix"
		network string
		// address is either a TCP host port, or a Unix socket
		address string
	)
	// IF
	//   a global Unix socket directory is NOT set AND
	//   an instance-level Unix socket is NOT set
	//   (e.g.,  I didn't set a Unix socket globally or for this instance)
	// OR
	//   an instance-level TCP address or port IS set
	//   (e.g., I'm overriding any global settings to use TCP for this
	//   instance)
	// use a TCP listener.
	// Otherwise, use a Unix socket.
	if (c.UnixSocket == "" && inst.UnixSocket == "") ||
		(inst.Addr != "" || inst.Port != 0) {
		network = "tcp"

		a := c.Addr
		if inst.Addr != "" {
			a = inst.Addr
		}

		var np int
		switch {
		case inst.Port != 0:
			np = inst.Port
		case c.Port != 0:
			np = pc.nextPort()
		default:
			np = pc.nextDBPort(version)
		}

		address = net.JoinHostPort(a, fmt.Sprint(np))
	} else {
		network = "unix"

		dir := c.UnixSocket
		if dir == "" {
			dir = inst.UnixSocket
		}
		if _, err := os.Stat(dir); err != nil {
			return nil, err
		}
		address = UnixAddress(dir, inst.Name)
		// When setting up a listener for Postgres, create address as a
		// directory, and use the Postgres-specific socket name
		// .s.PGSQL.5432.
		if strings.HasPrefix(version, "POSTGRES") {
			// Make the directory only if it hasn't already been created.
			if _, err := os.Stat(address); err != nil {
				if err = os.Mkdir(address, 0777); err != nil {
					return nil, err
				}
			}
			address = UnixAddress(address, ".s.PGSQL.5432")
		}
	}

	lc := net.ListenConfig{KeepAlive: 30 * time.Second}
	ln, err := lc.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}
	opts := c.DialOptions(inst)
	m := &socket{inst: inst.Name, dialOpts: opts, listener: ln}
	return m, nil
}

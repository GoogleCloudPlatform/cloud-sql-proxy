// Copyright 2022 Google LLC
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
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/gcloud"
	"golang.org/x/oauth2"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1"
)

var (
	// Instance connection name is the format <PROJECT>:<REGION>:<INSTANCE>
	// Additionally, we have to support legacy "domain-scoped" projects (e.g. "google.com:PROJECT")
	connNameRegex = regexp.MustCompile("([^:]+(:[^:]+)?):([^:]+):([^:]+)")
)

// connName represents the "instance connection name", in the format
// "project:region:name". Use the "parseConnName" method to initialize this
// struct.
type connName struct {
	project string
	region  string
	name    string
}

func (c *connName) String() string {
	return fmt.Sprintf("%s:%s:%s", c.project, c.region, c.name)
}

// parseConnName initializes a new connName struct.
func parseConnName(cn string) (connName, error) {
	b := []byte(cn)
	m := connNameRegex.FindSubmatch(b)
	if m == nil {
		return connName{}, fmt.Errorf(
			"invalid instance connection name, want = PROJECT:REGION:INSTANCE, got = %v",
			cn,
		)
	}

	c := connName{
		project: string(m[1]),
		region:  string(m[3]),
		name:    string(m[4]),
	}
	return c, nil
}

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

	// CredentialsJSON is a JSON representation of the service account key.
	CredentialsJSON string

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

	// FUSEDir enables a file system in user space at the provided path that
	// connects to the requested instance only when a client requests it.
	FUSEDir string

	// FUSETempDir sets the temporary directory where the FUSE mount will place
	// Unix domain sockets connected to Cloud SQL instances. The temp directory
	// is not accessed directly.
	FUSETempDir string

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

	// ImpersonationChain is a comma separated list of one or more service
	// accounts. The first entry in the chain is the impersonation target. Any
	// additional service accounts after the target are delegates. The
	// roles/iam.serviceAccountTokenCreator must be configured for each account
	// that will be impersonated.
	ImpersonationChain string

	// StructuredLogs sets all output to use JSON in the LogEntry format.
	// See https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry
	StructuredLogs bool
	// Quiet controls whether only error messages are logged.
	Quiet bool

	// TelemetryProject enables sending metrics and traces to the specified project.
	TelemetryProject string
	// TelemetryPrefix sets a prefix for all emitted metrics.
	TelemetryPrefix string
	// TelemetryTracingSampleRate sets the rate at which traces are
	// samples. A higher value means fewer traces.
	TelemetryTracingSampleRate int
	// DisableTraces disables tracing when TelemetryProject is set.
	DisableTraces bool
	// DisableMetrics disables metrics when TelemetryProject is set.
	DisableMetrics bool

	// Prometheus enables a Prometheus endpoint served at the address and
	// port specified by HTTPAddress and HTTPPort.
	Prometheus bool
	// PrometheusNamespace configures the namespace underwhich metrics are written.
	PrometheusNamespace string

	// HealthCheck enables a health check server. It's address and port are
	// specified by HTTPAddress and HTTPPort.
	HealthCheck bool

	// HTTPAddress sets the address for the health check and prometheus server.
	HTTPAddress string
	// HTTPPort sets the port for the health check and prometheus server.
	HTTPPort string
	// AdminPort configures the port for the localhost-only admin server.
	AdminPort string

	// Debug enables a debug handler on localhost.
	Debug bool

	// OtherUserAgents is a list of space separate user agents that will be
	// appended to the default user agent.
	OtherUserAgents string
}

// dialOptions interprets appropriate dial options for a particular instance
// configuration
func dialOptions(c Config, i InstanceConnConfig) []cloudsqlconn.DialOption {
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

func parseImpersonationChain(chain string) (string, []string) {
	accts := strings.Split(chain, ",")
	target := accts[0]
	// Assign delegates if the chain is more than one account. Delegation
	// goes from last back towards target, e.g., With sa1,sa2,sa3, sa3
	// delegates to sa2, which impersonates the target sa1.
	var delegates []string
	if l := len(accts); l > 1 {
		for i := l - 1; i > 0; i-- {
			delegates = append(delegates, accts[i])
		}
	}
	return target, delegates
}

const iamLoginScope = "https://www.googleapis.com/auth/sqlservice.login"

func credentialsOpt(c Config, l cloudsql.Logger) (cloudsqlconn.Option, error) {
	// If service account impersonation is configured, set up an impersonated
	// credentials token source.
	if c.ImpersonationChain != "" {
		var iopts []option.ClientOption
		switch {
		case c.Token != "":
			l.Infof("Impersonating service account with OAuth2 token")
			iopts = append(iopts, option.WithTokenSource(
				oauth2.StaticTokenSource(&oauth2.Token{AccessToken: c.Token}),
			))
		case c.CredentialsFile != "":
			l.Infof("Impersonating service account with the credentials file at %q", c.CredentialsFile)
			iopts = append(iopts, option.WithCredentialsFile(c.CredentialsFile))
		case c.CredentialsJSON != "":
			l.Infof("Impersonating service account with JSON credentials environment variable")
			iopts = append(iopts, option.WithCredentialsJSON([]byte(c.CredentialsJSON)))
		case c.GcloudAuth:
			l.Infof("Impersonating service account with gcloud user credentials")
			ts, err := gcloud.TokenSource()
			if err != nil {
				return nil, err
			}
			iopts = append(iopts, option.WithTokenSource(ts))
		default:
			l.Infof("Impersonating service account with Application Default Credentials")
		}
		target, delegates := parseImpersonationChain(c.ImpersonationChain)
		ts, err := impersonate.CredentialsTokenSource(
			context.Background(),
			impersonate.CredentialsConfig{
				TargetPrincipal: target,
				Delegates:       delegates,
				Scopes:          []string{sqladmin.SqlserviceAdminScope},
			},
			iopts...,
		)
		if err != nil {
			return nil, err
		}
		if c.IAMAuthN {
			iamLoginTS, err := impersonate.CredentialsTokenSource(
				context.Background(),
				impersonate.CredentialsConfig{
					TargetPrincipal: target,
					Delegates:       delegates,
					Scopes:          []string{iamLoginScope},
				},
				iopts...,
			)
			if err != nil {
				return nil, err
			}
			return cloudsqlconn.WithIAMAuthNTokenSources(ts, iamLoginTS), nil
		}
		return cloudsqlconn.WithTokenSource(ts), nil
	}

	// Otherwise, configure credentials as usual.
	switch {
	case c.Token != "":
		l.Infof("Authorizing with OAuth2 token")
		return cloudsqlconn.WithTokenSource(
			oauth2.StaticTokenSource(&oauth2.Token{AccessToken: c.Token}),
		), nil
	case c.CredentialsFile != "":
		l.Infof("Authorizing with the credentials file at %q", c.CredentialsFile)
		return cloudsqlconn.WithCredentialsFile(c.CredentialsFile), nil
	case c.CredentialsJSON != "":
		l.Infof("Authorizing with JSON credentials environment variable")
		return cloudsqlconn.WithCredentialsJSON([]byte(c.CredentialsJSON)), nil
	case c.GcloudAuth:
		l.Infof("Authorizing with gcloud user credentials")
		ts, err := gcloud.TokenSource()
		if err != nil {
			return nil, err
		}
		return cloudsqlconn.WithTokenSource(ts), nil
	default:
		l.Infof("Authorizing with Application Default Credentials")
		// Return no-op options to avoid having to handle nil in caller code
		return cloudsqlconn.WithOptions(), nil
	}
}

// DialerOptions builds appropriate list of options from the Config
// values for use by cloudsqlconn.NewClient()
func (c *Config) DialerOptions(l cloudsql.Logger) ([]cloudsqlconn.Option, error) {
	opts := []cloudsqlconn.Option{
		cloudsqlconn.WithUserAgent(c.UserAgent),
	}
	co, err := credentialsOpt(*c, l)
	if err != nil {
		return nil, err
	}
	opts = append(opts, co)

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

// Client proxies connections from a local client to the remote server side
// proxy for multiple Cloud SQL instances.
type Client struct {
	// connCount tracks the number of all open connections from the Client to
	// all Cloud SQL instances.
	connCount uint64

	// maxConns is the maximum number of allowed connections tracked by
	// connCount. If not set, there is no limit.
	maxConns uint64

	dialer cloudsql.Dialer

	// mnts is a list of all mounted sockets for this client
	mnts []*socketMount

	// waitOnClose is the maximum duration to wait for open connections to close
	// when shutting down.
	waitOnClose time.Duration

	logger cloudsql.Logger

	fuseMount
}

// NewClient completes the initial setup required to get the proxy to a "steady"
// state.
func NewClient(ctx context.Context, d cloudsql.Dialer, l cloudsql.Logger, conf *Config) (*Client, error) {
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

	c := &Client{
		logger:      l,
		dialer:      d,
		maxConns:    conf.MaxConnections,
		waitOnClose: conf.WaitOnClose,
	}

	if conf.FUSEDir != "" {
		return configureFUSE(c, conf)
	}

	for _, inst := range conf.Instances {
		// Initiate refresh operation and warm the cache.
		go func(name string) { d.EngineVersion(ctx, name) }(inst.Name)
	}

	var mnts []*socketMount
	pc := newPortConfig(conf.Port)
	for _, inst := range conf.Instances {
		version, err := d.EngineVersion(ctx, inst.Name)
		if err != nil {
			return nil, err
		}

		m, err := newSocketMount(ctx, conf, pc, inst, version)
		if err != nil {
			for _, m := range mnts {
				mErr := m.Close()
				if mErr != nil {
					l.Errorf("failed to close mount: %v", mErr)
				}
			}
			return nil, fmt.Errorf("[%v] Unable to mount socket: %v", inst.Name, err)
		}

		l.Infof("[%s] Listening on %s", inst.Name, m.Addr())
		mnts = append(mnts, m)
	}
	c.mnts = mnts
	return c, nil
}

// CheckConnections dials each registered instance and reports the number of
// connections checked and any errors that may have occurred.
func (c *Client) CheckConnections(ctx context.Context) (int, error) {
	var (
		wg    sync.WaitGroup
		errCh = make(chan error, len(c.mnts))
		mnts  = c.mnts
	)
	if c.fuseDir != "" {
		mnts = c.fuseMounts()
	}
	for _, mnt := range mnts {
		wg.Add(1)
		go func(m *socketMount) {
			defer wg.Done()
			conn, err := c.dialer.Dial(ctx, m.inst, m.dialOpts...)
			if err != nil {
				errCh <- err
				return
			}
			cErr := conn.Close()
			if cErr != nil {
				c.logger.Errorf(
					"connection check failed to close connection for %v: %v",
					m.inst, cErr,
				)
			}
		}(mnt)
	}
	wg.Wait()

	var mErr MultiErr
	for i := 0; i < len(mnts); i++ {
		select {
		case err := <-errCh:
			mErr = append(mErr, err)
		default:
			continue
		}
	}
	mLen := len(mnts)
	if len(mErr) > 0 {
		return mLen, mErr
	}
	return mLen, nil
}

// ConnCount returns the number of open connections and the maximum allowed
// connections. Returns 0 when the maximum allowed connections have not been set.
func (c *Client) ConnCount() (uint64, uint64) {
	return atomic.LoadUint64(&c.connCount), c.maxConns
}

// Serve starts proxying connections for all configured instances using the
// associated socket.
func (c *Client) Serve(ctx context.Context, notify func()) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if c.fuseDir != "" {
		return c.serveFuse(ctx, notify)
	}

	exitCh := make(chan error)
	for _, m := range c.mnts {
		go func(mnt *socketMount) {
			err := c.serveSocketMount(ctx, mnt)
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

// Close triggers the proxyClient to shutdown.
func (c *Client) Close() error {
	mnts := c.mnts

	var mErr MultiErr

	if c.fuseDir != "" {
		if err := c.unmountFUSE(); err != nil {
			mErr = append(mErr, err)
		}
		mnts = c.fuseMounts()
	}

	// First, close all open socket listeners to prevent additional connections.
	for _, m := range mnts {
		err := m.Close()
		if err != nil {
			mErr = append(mErr, err)
		}
	}
	if c.fuseDir != "" {
		c.waitForFUSEMounts()
	}
	// Next, close the dialer to prevent any additional refreshes.
	cErr := c.dialer.Close()
	if cErr != nil {
		mErr = append(mErr, cErr)
	}
	if c.waitOnClose == 0 {
		if len(mErr) > 0 {
			return mErr
		}
		return nil
	}
	timeout := time.After(c.waitOnClose)
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if atomic.LoadUint64(&c.connCount) > 0 {
				continue
			}
		case <-timeout:
		}
		break
	}
	open := atomic.LoadUint64(&c.connCount)
	if open > 0 {
		mErr = append(mErr, fmt.Errorf("%d connection(s) still open after waiting %v", open, c.waitOnClose))
	}
	if len(mErr) > 0 {
		return mErr
	}
	return nil
}

// serveSocketMount persistently listens to the socketMounts listener and proxies connections to a
// given Cloud SQL instance.
func (c *Client) serveSocketMount(_ context.Context, s *socketMount) error {
	for {
		cConn, err := s.Accept()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				c.logger.Errorf("[%s] Error accepting connection: %v", s.inst, err)
				// For transient errors, wait a small amount of time to see if it resolves itself
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return err
		}
		// handle the connection in a separate goroutine
		go func() {
			c.logger.Infof("[%s] accepted connection from %s", s.inst, cConn.RemoteAddr())

			// A client has established a connection to the local socket. Before
			// we initiate a connection to the Cloud SQL backend, increment the
			// connection counter. If the total number of connections exceeds
			// the maximum, refuse to connect and close the client connection.
			count := atomic.AddUint64(&c.connCount, 1)
			defer atomic.AddUint64(&c.connCount, ^uint64(0))

			if c.maxConns > 0 && count > c.maxConns {
				c.logger.Infof("max connections (%v) exceeded, refusing new connection", c.maxConns)
				_ = cConn.Close()
				return
			}

			// give a max of 30 seconds to connect to the instance
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			sConn, err := c.dialer.Dial(ctx, s.inst, s.dialOpts...)
			if err != nil {
				c.logger.Errorf("[%s] failed to connect to instance: %v", s.inst, err)
				cConn.Close()
				return
			}
			c.proxyConn(s.inst, cConn, sConn)
		}()
	}
}

// socketMount is a tcp/unix socket that listens for a Cloud SQL instance.
type socketMount struct {
	inst     string
	listener net.Listener
	dialOpts []cloudsqlconn.DialOption
}

func newSocketMount(ctx context.Context, conf *Config, pc *portConfig, inst InstanceConnConfig, version string) (*socketMount, error) {
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
	if (conf.UnixSocket == "" && inst.UnixSocket == "") ||
		(inst.Addr != "" || inst.Port != 0) {
		network = "tcp"

		a := conf.Addr
		if inst.Addr != "" {
			a = inst.Addr
		}

		var np int
		switch {
		case inst.Port != 0:
			np = inst.Port
		case conf.Port != 0:
			np = pc.nextPort()
		default:
			np = pc.nextDBPort(version)
		}

		address = net.JoinHostPort(a, fmt.Sprint(np))
	} else {
		network = "unix"

		dir := conf.UnixSocket
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
	// Change file permisions to allow access for user, group, and other.
	if network == "unix" {
		// Best effort. If this call fails, group and other won't have write
		// access.
		_ = os.Chmod(address, 0777)
	}
	opts := dialOptions(*conf, inst)
	m := &socketMount{inst: inst.Name, dialOpts: opts, listener: ln}
	return m, nil
}

func (s *socketMount) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *socketMount) Accept() (net.Conn, error) {
	return s.listener.Accept()
}

// close stops the mount from listening for any more connections
func (s *socketMount) Close() error {
	return s.listener.Close()
}

// proxyConn sets up a bidirectional copy between two open connections
func (c *Client) proxyConn(inst string, client, server net.Conn) {
	// only allow the first side to give an error for terminating a connection
	var o sync.Once
	cleanup := func(errDesc string, isErr bool) {
		o.Do(func() {
			client.Close()
			server.Close()
			if isErr {
				c.logger.Errorf(errDesc)
			} else {
				c.logger.Infof(errDesc)
			}
		})
	}

	// copy bytes from client to server
	go func() {
		buf := make([]byte, 8*1024) // 8kb
		for {
			n, cErr := client.Read(buf)
			var sErr error
			if n > 0 {
				_, sErr = server.Write(buf[:n])
			}
			switch {
			case cErr == io.EOF:
				cleanup(fmt.Sprintf("[%s] client closed the connection", inst), false)
				return
			case cErr != nil:
				cleanup(fmt.Sprintf("[%s] connection aborted - error reading from client: %v", inst, cErr), true)
				return
			case sErr == io.EOF:
				cleanup(fmt.Sprintf("[%s] instance closed the connection", inst), false)
				return
			case sErr != nil:
				cleanup(fmt.Sprintf("[%s] connection aborted - error writing to instance: %v", inst, cErr), true)
				return
			}
		}
	}()

	// copy bytes from server to client
	buf := make([]byte, 8*1024) // 8kb
	for {
		n, sErr := server.Read(buf)
		var cErr error
		if n > 0 {
			_, cErr = client.Write(buf[:n])
		}
		switch {
		case sErr == io.EOF:
			cleanup(fmt.Sprintf("[%s] instance closed the connection", inst), false)
			return
		case sErr != nil:
			cleanup(fmt.Sprintf("[%s] connection aborted - error reading from instance: %v", inst, sErr), true)
			return
		case cErr == io.EOF:
			cleanup(fmt.Sprintf("[%s] client closed the connection", inst), false)
			return
		case cErr != nil:
			cleanup(fmt.Sprintf("[%s] connection aborted - error writing to client: %v", inst, sErr), true)
			return
		}
	}
}

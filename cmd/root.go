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

package cmd

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/healthcheck"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/log"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.opencensus.io/trace"
)

var (
	// versionString indicates the version of this library.
	//go:embed version.txt
	versionString    string
	defaultUserAgent string
)

func init() {
	versionString = strings.TrimSpace(versionString)
	defaultUserAgent = "cloud-sql-proxy/" + versionString
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := NewCommand().Execute(); err != nil {
		exit := 1
		if terr, ok := err.(*exitError); ok {
			exit = terr.Code
		}
		os.Exit(exit)
	}
}

// Command represents an invocation of the Cloud SQL Auth Proxy.
type Command struct {
	*cobra.Command
	conf    *proxy.Config
	logger  cloudsql.Logger
	dialer  cloudsql.Dialer
	cleanup func() error
}

// Option is a function that configures a Command.
type Option func(*Command)

// WithLogger overrides the default logger.
func WithLogger(l cloudsql.Logger) Option {
	return func(c *Command) {
		c.logger = l
	}
}

// WithDialer configures the Command to use the provided dialer to connect to
// Cloud SQL instances.
func WithDialer(d cloudsql.Dialer) Option {
	return func(c *Command) {
		c.dialer = d
	}
}

var longHelp = `
Overview

    The Cloud SQL Auth proxy is a utility for ensuring secure connections to
    your Cloud SQL instances. It provides IAM authorization, allowing you to
    control who can connect to your instance through IAM permissions, and TLS
    1.3 encryption, without having to manage certificates.

    NOTE: The proxy does not configure the network. You MUST ensure the proxy
    can reach your Cloud SQL instance, either by deploying it in a VPC that has
    access to your Private IP instance, or by configuring Public IP.

    For every provided instance connection name, the proxy creates:

    - a socket that mimics a database running locally, and
    - an encrypted connection using TLS 1.3 back to your Cloud SQL instance.

    The proxy uses an ephemeral certificate to establish a secure connection to
    your Cloud SQL instance. The proxy will refresh those certificates on an
    hourly basis. Existing client connections are unaffected by the refresh
    cycle.

Starting the Proxy

    To start the proxy, you will need your instance connection name, which may
    be found in the Cloud SQL instance overview page or by using gcloud with the
    following command:

        gcloud sql instances describe INSTANCE --format='value(connectionName)'

    For example, if your instance connection name is
    "my-project:us-central1:my-db-server", starting the proxy will be:

        ./cloud-sql-proxy my-project:us-central1:my-db-server

    By default, the proxy will determine the database engine and start a
    listener on localhost using the default database engine's port, i.e., MySQL
    is 3306, Postgres is 5432, SQL Server is 1433. If multiple instances are
    specified which all use the same database engine, the first will be started
    on the default database port and subsequent instances will be incremented
    from there (e.g., 3306, 3307, 3308, etc). To disable this behavior (and
    reduce startup time), use the --port flag. All subsequent listeners will
    increment from the provided value.

    All socket listeners use the localhost network interface. To override this
    behavior, use the --address flag.

Instance Level Configuration

    The proxy supports overriding configuration on an instance-level with an
    optional query string syntax using the corresponding full flag name. The
    query string takes the form of a URL query string and should be appended to
    the INSTANCE_CONNECTION_NAME, e.g.,

        'my-project:us-central1:my-db-server?key1=value1&key2=value2'

    When using the optional query string syntax, quotes must wrap the instance
    connection name and query string to prevent conflicts with the shell. For
    example, to override the address and port for one instance but otherwise use
    the default behavior, use:

        ./cloud-sql-proxy \
    	    my-project:us-central1:my-db-server \
    	    'my-project:us-central1:my-other-server?address=0.0.0.0&port=7000'

Health checks

    When enabling the --health-check flag, the proxy will start an HTTP server
    on localhost with three endpoints:

    - /startup: Returns 200 status when the proxy has finished starting up.
    Otherwise returns 503 status.

    - /readiness: Returns 200 status when the proxy has started, has available
    connections if max connections have been set with the --max-connections
    flag, and when the proxy can connect to all registered instances. Otherwise,
    returns a 503 status. Optionally supports a min-ready query param (e.g.,
    /readiness?min-ready=3) where the proxy will return a 200 status if the
    proxy can connect successfully to at least min-ready number of instances. If
    min-ready exceeds the number of registered instances, returns a 400.

    - /liveness: Always returns 200 status. If this endpoint is not responding,
    the proxy is in a bad state and should be restarted.

    To configure the address, use --http-address. To configure the port, use
    --http-port.

Service Account Impersonation

    The proxy supports service account impersonation with the
    --impersonate-service-account flag and matches gcloud's flag. When enabled,
    all API requests are made impersonating the supplied service account. The
    IAM principal must have the iam.serviceAccounts.getAccessToken permission or
    the role roles/iam.serviceAccounts.serviceAccountTokenCreator.

    For example:

        ./cloud-sql-proxy \
            --impersonate-service-account=impersonated@my-project.iam.gserviceaccount.com
            my-project:us-central1:my-db-server

    In addition, the flag supports an impersonation delegation chain where the
    value is a comma-separated list of service accounts. The first service
    account in the list is the impersonation target. Each subsequent service
    account is a delegate to the previous service account. When delegation is
    used, each delegate must have the permissions named above on the service
    account it is delegating to.

    For example:

        ./cloud-sql-proxy \
            --impersonate-service-account=SERVICE_ACCOUNT_1,SERVICE_ACCOUNT_2,SERVICE_ACCOUNT_3
            my-project:us-central1:my-db-server

    In this example, the environment's IAM principal impersonates
    SERVICE_ACCOUNT_3 which impersonates SERVICE_ACCOUNT_2 which then
    impersonates the target SERVICE_ACCOUNT_1.

Configuration using environment variables

    Instead of using CLI flags, the proxy may be configured using environment
    variables. Each environment variable uses "CSQL_PROXY" as a prefix and is
    the uppercase version of the flag using underscores as word delimiters. For
    example, the --auto-iam-authn flag may be set with the environment variable
    CSQL_PROXY_AUTO_IAM_AUTHN. An invocation of the proxy using environment
    variables would look like the following:

        CSQL_PROXY_AUTO_IAM_AUTHN=true \
            ./cloud-sql-proxy my-project:us-central1:my-db-server

    In addition to CLI flags, instance connection names may also be specified
    with environment variables. If invoking the proxy with only one instance
    connection name, use CSQL_PROXY_INSTANCE_CONNECTION_NAME. For example:

        CSQL_PROXY_INSTANCE_CONNECTION_NAME=my-project:us-central1:my-db-server \
            ./cloud-sql-proxy

    If multiple instance connection names are used, add the index of the
    instance connection name as a suffix. For example:

        CSQL_PROXY_INSTANCE_CONNECTION_NAME_0=my-project:us-central1:my-db-server \
        CSQL_PROXY_INSTANCE_CONNECTION_NAME_1=my-other-project:us-central1:my-other-server \
            ./cloud-sql-proxy

Localhost Admin Server

    The Proxy includes support for an admin server on localhost. By default,
    the admin server is not enabled. To enable the server, pass the --debug
    flag. This will start the server on localhost at port 9091. To change the
    port, use the --admin-port flag.

    The admin server includes Go's pprof tool and is available at
    /debug/pprof/.

    See the documentation on pprof for details on how to use the
    profiler at https://pkg.go.dev/net/http/pprof.

(*) indicates a flag that may be used as a query parameter

`

const envPrefix = "CSQL_PROXY"

func instanceFromEnv(args []string) []string {
	// This supports naming the first instance first with:
	//     INSTANCE_CONNECTION_NAME
	// or if that's not defined, with:
	//     INSTANCE_CONNECTION_NAME_0
	inst := os.Getenv(fmt.Sprintf("%s_INSTANCE_CONNECTION_NAME", envPrefix))
	if inst == "" {
		inst = os.Getenv(fmt.Sprintf("%s_INSTANCE_CONNECTION_NAME_0", envPrefix))
		if inst == "" {
			return nil
		}
	}
	args = append(args, inst)

	i := 1
	for {
		instN := os.Getenv(fmt.Sprintf("%s_INSTANCE_CONNECTION_NAME_%d", envPrefix, i))
		// if the next instance connection name is not defined, stop checking
		// environment variables.
		if instN == "" {
			break
		}
		args = append(args, instN)
		i++
	}
	return args
}

// NewCommand returns a Command object representing an invocation of the proxy.
func NewCommand(opts ...Option) *Command {
	cmd := &cobra.Command{
		Use:     "cloud-sql-proxy INSTANCE_CONNECTION_NAME...",
		Version: versionString,
		Short:   "cloud-sql-proxy authorizes and encrypts connections to Cloud SQL.",
		Long:    longHelp,
	}

	logger := log.NewStdLogger(os.Stdout, os.Stderr)
	c := &Command{
		Command: cmd,
		logger:  logger,
		cleanup: func() error { return nil },
		conf: &proxy.Config{
			UserAgent: defaultUserAgent,
		},
	}
	for _, o := range opts {
		o(c)
	}

	cmd.Args = func(cmd *cobra.Command, args []string) error {
		// If args is not already populated, try to read from the environment.
		if len(args) == 0 {
			args = instanceFromEnv(args)
		}
		// Handle logger separately from config
		if c.conf.StructuredLogs {
			c.logger, c.cleanup = log.NewStructuredLogger()
		}
		if c.conf.Quiet {
			c.logger = log.NewStdLogger(io.Discard, os.Stderr)
		}
		err := parseConfig(c, c.conf, args)
		if err != nil {
			return err
		}
		// The arguments are parsed. Usage is no longer needed.
		cmd.SilenceUsage = true
		// Errors will be handled by logging from here on.
		cmd.SilenceErrors = true
		return nil
	}

	cmd.RunE = func(*cobra.Command, []string) error { return runSignalWrapper(c) }

	pflags := cmd.PersistentFlags()

	// Override Cobra's default messages.
	pflags.BoolP("help", "h", false, "Display help information for cloud-sql-proxy")
	pflags.BoolP("version", "v", false, "Print the cloud-sql-proxy version")

	// Global-only flags
	pflags.StringVar(&c.conf.OtherUserAgents, "user-agent", "",
		"Space separated list of additional user agents, e.g. cloud-sql-proxy-operator/0.0.1")
	pflags.StringVarP(&c.conf.Token, "token", "t", "",
		"Use bearer token as a source of IAM credentials.")
	pflags.StringVarP(&c.conf.CredentialsFile, "credentials-file", "c", "",
		"Use service account key file as a source of IAM credentials.")
	pflags.StringVarP(&c.conf.CredentialsJSON, "json-credentials", "j", "",
		"Use service account key JSON as a source of IAM credentials.")
	pflags.BoolVarP(&c.conf.GcloudAuth, "gcloud-auth", "g", false,
		"Use gcloud's user credentials as a source of IAM credentials.")
	pflags.BoolVarP(&c.conf.StructuredLogs, "structured-logs", "l", false,
		"Enable structured logging with LogEntry format")
	pflags.Uint64Var(&c.conf.MaxConnections, "max-connections", 0,
		"Limit the number of connections. Default is no limit.")
	pflags.DurationVar(&c.conf.WaitOnClose, "max-sigterm-delay", 0,
		"Maximum number of seconds to wait for connections to close after receiving a TERM signal.")
	pflags.StringVar(&c.conf.TelemetryProject, "telemetry-project", "",
		"Enable Cloud Monitoring and Cloud Trace with the provided project ID.")
	pflags.BoolVar(&c.conf.DisableTraces, "disable-traces", false,
		"Disable Cloud Trace integration (used with --telemetry-project)")
	pflags.IntVar(&c.conf.TelemetryTracingSampleRate, "telemetry-sample-rate", 10_000,
		"Set the Cloud Trace sample rate. A smaller number means more traces.")
	pflags.BoolVar(&c.conf.DisableMetrics, "disable-metrics", false,
		"Disable Cloud Monitoring integration (used with --telemetry-project)")
	pflags.StringVar(&c.conf.TelemetryPrefix, "telemetry-prefix", "",
		"Prefix for Cloud Monitoring metrics.")
	pflags.BoolVar(&c.conf.Prometheus, "prometheus", false,
		"Enable Prometheus HTTP endpoint /metrics on localhost")
	pflags.StringVar(&c.conf.PrometheusNamespace, "prometheus-namespace", "",
		"Use the provided Prometheus namespace for metrics")
	pflags.StringVar(&c.conf.HTTPAddress, "http-address", "localhost",
		"Address for Prometheus and health check server")
	pflags.StringVar(&c.conf.HTTPPort, "http-port", "9090",
		"Port for Prometheus and health check server")
	pflags.BoolVar(&c.conf.Debug, "debug", false,
		"Enable the admin server on localhost")
	pflags.StringVar(&c.conf.AdminPort, "admin-port", "9091",
		"Port for localhost-only admin server")
	pflags.BoolVar(&c.conf.HealthCheck, "health-check", false,
		"Enables health check endpoints /startup, /liveness, and /readiness on localhost.")
	pflags.StringVar(&c.conf.APIEndpointURL, "sqladmin-api-endpoint", "",
		"API endpoint for all Cloud SQL Admin API requests. (default: https://sqladmin.googleapis.com)")
	pflags.StringVar(&c.conf.QuotaProject, "quota-project", "",
		`Specifies the project to use for Cloud SQL Admin API quota tracking.
The IAM principal must have the "serviceusage.services.use" permission
for the given project. See https://cloud.google.com/service-usage/docs/overview and
https://cloud.google.com/storage/docs/requester-pays`)
	pflags.StringVar(&c.conf.FUSEDir, "fuse", "",
		"Mount a directory at the path using FUSE to access Cloud SQL instances.")
	pflags.StringVar(&c.conf.FUSETempDir, "fuse-tmp-dir",
		filepath.Join(os.TempDir(), "csql-tmp"),
		"Temp dir for Unix sockets created with FUSE")
	pflags.StringVar(&c.conf.ImpersonationChain, "impersonate-service-account", "",
		`Comma separated list of service accounts to impersonate. Last value
is the target account.`)
	cmd.PersistentFlags().BoolVar(&c.conf.Quiet, "quiet", false, "Log error messages only")

	// Global and per instance flags
	pflags.StringVarP(&c.conf.Addr, "address", "a", "127.0.0.1",
		"(*) Address to bind Cloud SQL instance listeners.")
	pflags.IntVarP(&c.conf.Port, "port", "p", 0,
		"(*) Initial port for listeners. Subsequent listeners increment from this value.")
	pflags.StringVarP(&c.conf.UnixSocket, "unix-socket", "u", "",
		`(*) Enables Unix sockets for all listeners with the provided directory.`)
	pflags.BoolVarP(&c.conf.IAMAuthN, "auto-iam-authn", "i", false,
		"(*) Enables Automatic IAM Authentication for all instances")
	pflags.BoolVar(&c.conf.PrivateIP, "private-ip", false,
		"(*) Connect to the private ip address for all instances")

	v := viper.NewWithOptions(viper.EnvKeyReplacer(strings.NewReplacer("-", "_")))
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	// Ignoring the error here since its only occurence is if one of the pflags
	// is nil which is never the case here.
	_ = v.BindPFlags(pflags)

	pflags.VisitAll(func(f *pflag.Flag) {
		// Override any unset flags with Viper values to use the pflags
		// object as a single source of truth.
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			pflags.Set(f.Name, fmt.Sprintf("%v", val))
		}
	})

	return c
}

func parseConfig(cmd *Command, conf *proxy.Config, args []string) error {
	// If no instance connection names were provided AND FUSE isn't enabled,
	// error.
	if len(args) == 0 && conf.FUSEDir == "" {
		return newBadCommandError("missing instance_connection_name (e.g., project:region:instance)")
	}

	if conf.FUSEDir != "" {
		if err := proxy.SupportsFUSE(); err != nil {
			return newBadCommandError(
				fmt.Sprintf("--fuse is not supported: %v", err),
			)
		}
	}

	if len(args) == 0 && conf.FUSEDir == "" && conf.FUSETempDir != "" {
		return newBadCommandError("cannot specify --fuse-tmp-dir without --fuse")
	}

	userHasSet := func(f string) bool {
		return cmd.PersistentFlags().Lookup(f).Changed
	}
	if userHasSet("address") && userHasSet("unix-socket") {
		return newBadCommandError("cannot specify --unix-socket and --address together")
	}
	if userHasSet("port") && userHasSet("unix-socket") {
		return newBadCommandError("cannot specify --unix-socket and --port together")
	}
	if ip := net.ParseIP(conf.Addr); ip == nil {
		return newBadCommandError(fmt.Sprintf("not a valid IP address: %q", conf.Addr))
	}

	// If more than one auth method is set, error.
	if conf.Token != "" && conf.CredentialsFile != "" {
		return newBadCommandError("cannot specify --token and --credentials-file flags at the same time")
	}
	if conf.Token != "" && conf.GcloudAuth {
		return newBadCommandError("cannot specify --token and --gcloud-auth flags at the same time")
	}
	if conf.CredentialsFile != "" && conf.GcloudAuth {
		return newBadCommandError("cannot specify --credentials-file and --gcloud-auth flags at the same time")
	}
	if conf.CredentialsJSON != "" && conf.Token != "" {
		return newBadCommandError("cannot specify --json-credentials and --token flags at the same time")
	}
	if conf.CredentialsJSON != "" && conf.CredentialsFile != "" {
		return newBadCommandError("cannot specify --json-credentials and --credentials-file flags at the same time")
	}
	if conf.CredentialsJSON != "" && conf.GcloudAuth {
		return newBadCommandError("cannot specify --json-credentials and --gcloud-auth flags at the same time")
	}

	if userHasSet("http-port") && !userHasSet("prometheus") && !userHasSet("health-check") {
		cmd.logger.Infof("Ignoring --http-port because --prometheus or --health-check was not set")
	}

	if !userHasSet("telemetry-project") && userHasSet("telemetry-prefix") {
		cmd.logger.Infof("Ignoring --telementry-prefix because --telemetry-project was not set")
	}
	if !userHasSet("telemetry-project") && userHasSet("disable-metrics") {
		cmd.logger.Infof("Ignoring --disable-metrics because --telemetry-project was not set")
	}
	if !userHasSet("telemetry-project") && userHasSet("disable-traces") {
		cmd.logger.Infof("Ignoring --disable-traces because --telemetry-project was not set")
	}

	if userHasSet("user-agent") {
		defaultUserAgent += " " + cmd.conf.OtherUserAgents
		conf.UserAgent = defaultUserAgent
	}

	if userHasSet("sqladmin-api-endpoint") && conf.APIEndpointURL != "" {
		_, err := url.Parse(conf.APIEndpointURL)
		if err != nil {
			return newBadCommandError(fmt.Sprintf(
				"the value provided for --sqladmin-api-endpoint is not a valid URL, %v",
				conf.APIEndpointURL,
			))
		}

		// add a trailing '/' if omitted
		if !strings.HasSuffix(conf.APIEndpointURL, "/") {
			conf.APIEndpointURL = conf.APIEndpointURL + "/"
		}
	}

	var ics []proxy.InstanceConnConfig
	for _, a := range args {
		// Assume no query params initially
		ic := proxy.InstanceConnConfig{
			Name: a,
		}
		// If there are query params, update instance config.
		if res := strings.SplitN(a, "?", 2); len(res) > 1 {
			ic.Name = res[0]
			q, err := url.ParseQuery(res[1])
			if err != nil {
				return newBadCommandError(fmt.Sprintf("could not parse query: %q", res[1]))
			}

			a, aok := q["address"]
			p, pok := q["port"]
			u, uok := q["unix-socket"]

			if aok && uok {
				return newBadCommandError("cannot specify both address and unix-socket query params")
			}
			if pok && uok {
				return newBadCommandError("cannot specify both port and unix-socket query params")
			}

			if aok {
				if len(a) != 1 {
					return newBadCommandError(fmt.Sprintf("address query param should be only one value: %q", a))
				}
				if ip := net.ParseIP(a[0]); ip == nil {
					return newBadCommandError(
						fmt.Sprintf("address query param is not a valid IP address: %q",
							a[0],
						))
				}
				ic.Addr = a[0]
			}

			if pok {
				if len(p) != 1 {
					return newBadCommandError(fmt.Sprintf("port query param should be only one value: %q", a))
				}
				pp, err := strconv.Atoi(p[0])
				if err != nil {
					return newBadCommandError(
						fmt.Sprintf("port query param is not a valid integer: %q",
							p[0],
						))
				}
				ic.Port = pp
			}

			if uok {
				if len(u) != 1 {
					return newBadCommandError(fmt.Sprintf("unix query param should be only one value: %q", a))
				}
				ic.UnixSocket = u[0]
			}

			ic.IAMAuthN, err = parseBoolOpt(q, "auto-iam-authn")
			if err != nil {
				return err
			}

			ic.PrivateIP, err = parseBoolOpt(q, "private-ip")
			if err != nil {
				return err
			}

		}
		ics = append(ics, ic)
	}

	conf.Instances = ics
	return nil
}

// parseBoolOpt parses a boolean option from the query string, returning
//
//	true if the value is "t" or "true" case-insensitive
//	false if the value is "f" or "false" case-insensitive
func parseBoolOpt(q url.Values, name string) (*bool, error) {
	iam, ok := q[name]
	if !ok {
		return nil, nil
	}

	if len(iam) != 1 {
		return nil, newBadCommandError(fmt.Sprintf("%v param should be only one value: %q", name, iam))
	}

	switch strings.ToLower(iam[0]) {
	case "true", "t", "":
		enable := true
		return &enable, nil
	case "false", "f":
		disable := false
		return &disable, nil
	default:
		// value is not recognized
		return nil, newBadCommandError(
			fmt.Sprintf("%v query param should be true or false, got: %q",
				name, iam[0],
			))
	}

}

// runSignalWrapper watches for SIGTERM and SIGINT and interupts execution if necessary.
func runSignalWrapper(cmd *Command) error {
	defer cmd.cleanup()
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Configure collectors before the proxy has started to ensure we are
	// collecting metrics before *ANY* Cloud SQL Admin API calls are made.
	enableMetrics := !cmd.conf.DisableMetrics
	enableTraces := !cmd.conf.DisableTraces
	if cmd.conf.TelemetryProject != "" && (enableMetrics || enableTraces) {
		sd, err := stackdriver.NewExporter(stackdriver.Options{
			ProjectID:    cmd.conf.TelemetryProject,
			MetricPrefix: cmd.conf.TelemetryPrefix,
		})
		if err != nil {
			return err
		}
		if enableMetrics {
			err = sd.StartMetricsExporter()
			if err != nil {
				return err
			}
		}
		if enableTraces {
			s := trace.ProbabilitySampler(1 / float64(cmd.conf.TelemetryTracingSampleRate))
			trace.ApplyConfig(trace.Config{DefaultSampler: s})
			trace.RegisterExporter(sd)
		}
		defer func() {
			sd.Flush()
			sd.StopMetricsExporter()
		}()
	}

	var (
		needsHTTPServer bool
		mux             = http.NewServeMux()
	)
	if cmd.conf.Prometheus {
		needsHTTPServer = true
		e, err := prometheus.NewExporter(prometheus.Options{
			Namespace: cmd.conf.PrometheusNamespace,
		})
		if err != nil {
			return err
		}
		mux.Handle("/metrics", e)
	}

	shutdownCh := make(chan error)
	// watch for sigterm / sigint signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		var s os.Signal
		select {
		case s = <-signals:
		case <-ctx.Done():
			// this should only happen when the context supplied in tests in canceled
			s = syscall.SIGINT
		}
		switch s {
		case syscall.SIGINT:
			shutdownCh <- errSigInt
		case syscall.SIGTERM:
			shutdownCh <- errSigTerm
		}
	}()

	// Start the proxy asynchronously, so we can exit early if a shutdown signal is sent
	startCh := make(chan *proxy.Client)
	go func() {
		defer close(startCh)
		p, err := proxy.NewClient(ctx, cmd.dialer, cmd.logger, cmd.conf)
		if err != nil {
			shutdownCh <- fmt.Errorf("unable to start: %v", err)
			return
		}
		startCh <- p
	}()
	// Wait for either startup to finish or a signal to interupt
	var p *proxy.Client
	select {
	case err := <-shutdownCh:
		cmd.logger.Errorf("The proxy has encountered a terminal error: %v", err)
		return err
	case p = <-startCh:
		cmd.logger.Infof("The proxy has started successfully and is ready for new connections!")
	}
	defer func() {
		if cErr := p.Close(); cErr != nil {
			cmd.logger.Errorf("error during shutdown: %v", cErr)
		}
	}()

	notify := func() {}
	if cmd.conf.HealthCheck {
		needsHTTPServer = true
		cmd.logger.Infof("Starting health check server at %s",
			net.JoinHostPort(cmd.conf.HTTPAddress, cmd.conf.HTTPPort))
		hc := healthcheck.NewCheck(p, cmd.logger)
		mux.HandleFunc("/startup", hc.HandleStartup)
		mux.HandleFunc("/readiness", hc.HandleReadiness)
		mux.HandleFunc("/liveness", hc.HandleLiveness)
		notify = hc.NotifyStarted
	}

	go func() {
		if !cmd.conf.Debug {
			return
		}
		m := http.NewServeMux()
		m.HandleFunc("/debug/pprof/", pprof.Index)
		m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		m.HandleFunc("/debug/pprof/profile", pprof.Profile)
		m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		m.HandleFunc("/debug/pprof/trace", pprof.Trace)
		addr := net.JoinHostPort("localhost", cmd.conf.AdminPort)
		cmd.logger.Infof("Starting admin server on %v", addr)
		if lErr := http.ListenAndServe(addr, m); lErr != nil {
			cmd.logger.Errorf("Failed to start admin HTTP server: %v", lErr)
		}
	}()
	// Start the HTTP server if anything requiring HTTP is specified.
	if needsHTTPServer {
		server := &http.Server{
			Addr:    net.JoinHostPort(cmd.conf.HTTPAddress, cmd.conf.HTTPPort),
			Handler: mux,
		}
		// Start the HTTP server.
		go func() {
			err := server.ListenAndServe()
			if err == http.ErrServerClosed {
				return
			}
			if err != nil {
				shutdownCh <- fmt.Errorf("failed to start HTTP server: %v", err)
			}
		}()
		// Handle shutdown of the HTTP server gracefully.
		go func() {
			<-ctx.Done()
			// Give the HTTP server a second to shutdown cleanly.
			ctx2, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := server.Shutdown(ctx2); err != nil {
				cmd.logger.Errorf("failed to shutdown Prometheus HTTP server: %v\n", err)
			}
		}()
	}

	go func() { shutdownCh <- p.Serve(ctx, notify) }()

	err := <-shutdownCh
	switch {
	case errors.Is(err, errSigInt):
		cmd.logger.Errorf("SIGINT signal received. Shutting down...")
	case errors.Is(err, errSigTerm):
		cmd.logger.Errorf("SIGTERM signal received. Shutting down...")
	default:
		cmd.logger.Errorf("The proxy has encountered a terminal error: %v", err)
	}
	return err
}

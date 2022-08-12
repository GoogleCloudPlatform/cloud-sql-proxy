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

package cmd

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"net/http"
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
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/healthcheck"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/log"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
	"github.com/spf13/cobra"
	"go.opencensus.io/trace"
)

var (
	// versionString indicates the version of this library.
	//go:embed version.txt
	versionString string
	userAgent     string
)

func init() {
	versionString = strings.TrimSpace(versionString)
	userAgent = "cloud-sql-auth-proxy/" + versionString
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
	conf   *proxy.Config
	logger cloudsql.Logger
	dialer cloudsql.Dialer

	cleanup                    func() error
	disableTraces              bool
	telemetryTracingSampleRate int
	disableMetrics             bool
	telemetryProject           string
	telemetryPrefix            string
	prometheus                 bool
	prometheusNamespace        string
	healthCheck                bool
	httpPort                   string
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

// NewCommand returns a Command object representing an invocation of the proxy.
func NewCommand(opts ...Option) *Command {
	cmd := &cobra.Command{
		Use:     "cloud_sql_proxy instance_connection_name...",
		Version: versionString,
		Short:   "cloud_sql_proxy provides a secure way to authorize connections to Cloud SQL.",
		Long: `The Cloud SQL Auth proxy provides IAM-based authorization and encryption when
connecting to Cloud SQL instances. It listens on a local port and forwards connections
to your instance's IP address, providing a secure connection without having to manage
any client SSL certificates.`,
	}

	logger := log.NewStdLogger(os.Stdout, os.Stderr)
	c := &Command{
		Command: cmd,
		logger:  logger,
		cleanup: func() error { return nil },
		conf: &proxy.Config{
			UserAgent: userAgent,
		},
	}
	for _, o := range opts {
		o(c)
	}

	cmd.Args = func(cmd *cobra.Command, args []string) error {
		// Handle logger separately from config
		if c.conf.StructuredLogs {
			c.logger, c.cleanup = log.NewStructuredLogger()
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

	// Global-only flags
	cmd.PersistentFlags().StringVarP(&c.conf.Token, "token", "t", "",
		"Bearer token used for authorization.")
	cmd.PersistentFlags().StringVarP(&c.conf.CredentialsFile, "credentials-file", "c", "",
		"Path to a service account key to use for authentication.")
	cmd.PersistentFlags().BoolVarP(&c.conf.GcloudAuth, "gcloud-auth", "g", false,
		"Use gcloud's user configuration to retrieve a token for authentication.")
	cmd.PersistentFlags().BoolVarP(&c.conf.StructuredLogs, "structured-logs", "l", false,
		"Enable structured logs using the LogEntry format")
	cmd.PersistentFlags().Uint64Var(&c.conf.MaxConnections, "max-connections", 0,
		`Limits the number of connections by refusing any additional connections.
When this flag is not set, there is no limit.`)
	cmd.PersistentFlags().DurationVar(&c.conf.WaitOnClose, "max-sigterm-delay", 0,
		`Maximum amount of time to wait after for any open connections
to close after receiving a TERM signal. The proxy will shut
down when the number of open connections reaches 0 or when
the maximum time has passed. Defaults to 0s.`)
	cmd.PersistentFlags().StringVar(&c.telemetryProject, "telemetry-project", "",
		"Enable Cloud Monitoring and Cloud Trace integration with the provided project ID.")
	cmd.PersistentFlags().BoolVar(&c.disableTraces, "disable-traces", false,
		"Disable Cloud Trace integration (used with telemetry-project)")
	cmd.PersistentFlags().IntVar(&c.telemetryTracingSampleRate, "telemetry-sample-rate", 10_000,
		"Configure the denominator of the probabilistic sample rate of traces sent to Cloud Trace\n(e.g., 10,000 traces 1/10,000 calls).")
	cmd.PersistentFlags().BoolVar(&c.disableMetrics, "disable-metrics", false,
		"Disable Cloud Monitoring integration (used with telemetry-project)")
	cmd.PersistentFlags().StringVar(&c.telemetryPrefix, "telemetry-prefix", "",
		"Prefix to use for Cloud Monitoring metrics.")
	cmd.PersistentFlags().BoolVar(&c.prometheus, "prometheus", false,
		"Enable Prometheus HTTP endpoint /metrics")
	cmd.PersistentFlags().StringVar(&c.prometheusNamespace, "prometheus-namespace", "",
		"Use the provided Prometheus namespace for metrics")
	cmd.PersistentFlags().StringVar(&c.httpPort, "http-port", "9090",
		"Port for the Prometheus server to use")
	cmd.PersistentFlags().BoolVar(&c.healthCheck, "health-check", false,
		`Enables HTTP endpoints /startup, /liveness, and /readiness
that report on the proxy's health. Endpoints are available on localhost
only. Uses the port specified by the http-port flag.`)
	cmd.PersistentFlags().StringVar(&c.conf.APIEndpointURL, "sqladmin-api-endpoint", "",
		"When set, the proxy uses this url as the API endpoint for all Cloud SQL Admin API requests.\nExample: https://sqladmin.googleapis.com")
	cmd.PersistentFlags().StringVar(&c.conf.QuotaProject, "quota-project", "",
		`Specifies the project to use for Cloud SQL Admin API quota tracking.
The IAM principal must have the "serviceusage.services.use" permission
for the given project. See https://cloud.google.com/service-usage/docs/overview and
https://cloud.google.com/storage/docs/requester-pays`)
	cmd.PersistentFlags().StringVar(&c.conf.FUSE, "fuse", "",
		"Mount a directory at the path using FUSE to access Cloud SQL instances.")
	cmd.PersistentFlags().StringVar(&c.conf.FUSETempDir, "fuse-tmp-dir",
		filepath.Join(os.TempDir(), "csql-tmp"),
		"Temp dir for Unix sockets created with FUSE")

	// Global and per instance flags
	cmd.PersistentFlags().StringVarP(&c.conf.Addr, "address", "a", "127.0.0.1",
		"Address on which to bind Cloud SQL instance listeners.")
	cmd.PersistentFlags().IntVarP(&c.conf.Port, "port", "p", 0,
		"Initial port to use for listeners. Subsequent listeners increment from this value.")
	cmd.PersistentFlags().StringVarP(&c.conf.UnixSocket, "unix-socket", "u", "",
		`Enables Unix sockets for all listeners using the provided directory.`)
	cmd.PersistentFlags().BoolVarP(&c.conf.IAMAuthN, "auto-iam-authn", "i", false,
		"Enables Automatic IAM Authentication for all instances")
	cmd.PersistentFlags().BoolVar(&c.conf.PrivateIP, "private-ip", false,
		"Connect to the private ip address for all instances")

	return c
}

func parseConfig(cmd *Command, conf *proxy.Config, args []string) error {
	// If no instance connection names were provided AND FUSE isn't enabled,
	// error.
	if len(args) == 0 && conf.FUSE == "" {
		return newBadCommandError("missing instance_connection_name (e.g., project:region:instance)")
	}

	if conf.FUSE != "" {
		if err := proxy.SupportsFUSE(); err != nil {
			return newBadCommandError(
				fmt.Sprintf("--fuse is not supported: %v", err),
			)
		}
	}

	if len(args) == 0 && conf.FUSE == "" && conf.FUSETempDir != "" {
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

	if userHasSet("sqladmin-api-endpoint") && conf.APIEndpointURL != "" {
		_, err := url.Parse(conf.APIEndpointURL)
		if err != nil {
			return newBadCommandError(fmt.Sprintf("the value provided for --sqladmin-api-endpoint is not a valid URL, %v", conf.APIEndpointURL))
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
//   true if the value is "t" or "true" case-insensitive
//   false if the value is "f" or "false" case-insensitive
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
	enableMetrics := !cmd.disableMetrics
	enableTraces := !cmd.disableTraces
	if cmd.telemetryProject != "" && (enableMetrics || enableTraces) {
		sd, err := stackdriver.NewExporter(stackdriver.Options{
			ProjectID:    cmd.telemetryProject,
			MetricPrefix: cmd.telemetryPrefix,
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
			s := trace.ProbabilitySampler(1 / float64(cmd.telemetryTracingSampleRate))
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
	if cmd.prometheus {
		needsHTTPServer = true
		e, err := prometheus.NewExporter(prometheus.Options{
			Namespace: cmd.prometheusNamespace,
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
	if cmd.healthCheck {
		needsHTTPServer = true
		hc := healthcheck.NewCheck(p, cmd.logger)
		mux.HandleFunc("/startup", hc.HandleStartup)
		mux.HandleFunc("/readiness", hc.HandleReadiness)
		mux.HandleFunc("/liveness", hc.HandleLiveness)
		notify = hc.NotifyStarted
	}

	// Start the HTTP server if anything requiring HTTP is specified.
	if needsHTTPServer {
		server := &http.Server{
			Addr:    fmt.Sprintf("localhost:%s", cmd.httpPort),
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
			select {
			case <-ctx.Done():
				// Give the HTTP server a second to shutdown cleanly.
				ctx2, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := server.Shutdown(ctx2); err != nil {
					cmd.logger.Errorf("failed to shutdown Prometheus HTTP server: %v\n", err)
				}
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

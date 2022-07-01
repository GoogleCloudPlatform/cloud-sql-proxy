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
	"strconv"
	"strings"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/cloudsql"
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
	conf *proxy.Config

	disableTraces              bool
	telemetryTracingSampleRate int
	disableMetrics             bool
	telemetryProject           string
	telemetryPrefix            string
	prometheusNamespace        string
	httpPort                   string
}

// Option is a function that configures a Command.
type Option func(*proxy.Config)

// WithDialer configures the Command to use the provided dialer to connect to
// Cloud SQL instances.
func WithDialer(d cloudsql.Dialer) Option {
	return func(c *proxy.Config) {
		c.Dialer = d
	}
}

// NewCommand returns a Command object representing an invocation of the proxy.
func NewCommand(opts ...Option) *Command {
	c := &Command{
		conf: &proxy.Config{},
	}
	for _, o := range opts {
		o(c.conf)
	}

	cmd := &cobra.Command{
		Use:     "cloud_sql_proxy instance_connection_name...",
		Version: versionString,
		Short:   "cloud_sql_proxy provides a secure way to authorize connections to Cloud SQL.",
		Long: `The Cloud SQL Auth proxy provides IAM-based authorization and encryption when
connecting to Cloud SQL instances. It listens on a local port and forwards connections
to your instance's IP address, providing a secure connection without having to manage
any client SSL certificates.`,
		Args: func(cmd *cobra.Command, args []string) error {
			err := parseConfig(cmd, c.conf, args)
			if err != nil {
				return err
			}
			// The arguments are parsed. Usage is no longer needed.
			cmd.SilenceUsage = true
			return nil
		},
		RunE: func(*cobra.Command, []string) error {
			return runSignalWrapper(c)
		},
	}

	// Global-only flags
	cmd.PersistentFlags().StringVarP(&c.conf.Token, "token", "t", "",
		"Bearer token used for authorization.")
	cmd.PersistentFlags().StringVarP(&c.conf.CredentialsFile, "credentials-file", "c", "",
		"Path to a service account key to use for authentication.")
	cmd.PersistentFlags().BoolVarP(&c.conf.GcloudAuth, "gcloud-auth", "g", false,
		"Use gcloud's user configuration to retrieve a token for authentication.")
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
	cmd.PersistentFlags().StringVar(&c.prometheusNamespace, "prometheus-namespace", "",
		"Enable Prometheus for metric collection using the provided namespace")
	cmd.PersistentFlags().StringVar(&c.httpPort, "http-port", "9090",
		"Port for the Prometheus server to use")
	cmd.PersistentFlags().StringVar(&c.conf.APIEndpointURL, "sqladmin-api-endpoint", "",
		"When set, the proxy uses this url as the API endpoint for all Cloud SQL Admin API requests.\nExample: https://sqladmin.googleapis.com")

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

	c.Command = cmd
	return c
}

func parseConfig(cmd *cobra.Command, conf *proxy.Config, args []string) error {
	// If no instance connection names were provided, error.
	if len(args) == 0 {
		return newBadCommandError("missing instance_connection_name (e.g., project:region:instance)")
	}

	conf.UserAgent = userAgent

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

	if userHasSet("http-port") && !userHasSet("prometheus-namespace") {
		return newBadCommandError("cannot specify --http-port without --prometheus-namespace")
	}

	if !userHasSet("telemetry-project") && userHasSet("telemetry-prefix") {
		cmd.Println("Ignoring telementry-prefix as telemetry-project was not set")
	}
	if !userHasSet("telemetry-project") && userHasSet("disable-metrics") {
		cmd.Println("Ignoring disable-metrics as telemetry-project was not set")
	}
	if !userHasSet("telemetry-project") && userHasSet("disable-traces") {
		cmd.Println("Ignoring disable-traces as telemetry-project was not set")
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
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Configure Cloud Trace and/or Cloud Monitoring based on command
	// invocation. If a project has not been enabled, no traces or metrics are
	// enabled.
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

	shutdownCh := make(chan error)

	if cmd.prometheusNamespace != "" {
		e, err := prometheus.NewExporter(prometheus.Options{
			Namespace: cmd.prometheusNamespace,
		})
		if err != nil {
			return err
		}
		mux := http.NewServeMux()
		mux.Handle("/metrics", e)
		addr := fmt.Sprintf("localhost:%s", cmd.httpPort)
		server := &http.Server{Addr: addr, Handler: mux}
		go func() {
			select {
			case <-ctx.Done():
				// Give the HTTP server a second to shutdown cleanly.
				ctx2, _ := context.WithTimeout(context.Background(), time.Second)
				if err := server.Shutdown(ctx2); err != nil {
					cmd.Printf("failed to shutdown Prometheus HTTP server: %v\n", err)
				}
			}
		}()
		go func() {
			err := server.ListenAndServe()
			if err == http.ErrServerClosed {
				return
			}
			if err != nil {
				shutdownCh <- fmt.Errorf("failed to start prometheus HTTP server: %v", err)
			}
		}()
	}

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
		p, err := proxy.NewClient(ctx, cmd.Command, cmd.conf)
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
		return err
	case p = <-startCh:
	}
	cmd.Println("The proxy has started successfully and is ready for new connections!")
	defer p.Close()

	go func() {
		shutdownCh <- p.Serve(ctx)
	}()

	err := <-shutdownCh
	switch {
	case errors.Is(err, errSigInt):
		cmd.PrintErrln("SIGINT signal received. Shuting down...")
	case errors.Is(err, errSigTerm):
		cmd.PrintErrln("SIGTERM signal received. Shuting down...")
	default:
		cmd.PrintErrf("The proxy has encountered a terminal error: %v\n", err)
	}
	return err
}

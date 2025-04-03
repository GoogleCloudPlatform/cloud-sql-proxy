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
	"sync"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/healthcheck"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/log"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.opencensus.io/trace"
)

var (
	// versionString indicates the version of this library.
	//go:embed version.txt
	versionString string
	// metadataString indicates additional build or distribution metadata.
	metadataString string
	userAgent      string
)

func init() {
	versionString = semanticVersion()
	userAgent = "cloud-sql-proxy/" + versionString
}

// semanticVersion returns the version of the proxy including a compile-time
// metadata.
func semanticVersion() string {
	v := strings.TrimSpace(versionString)
	if metadataString != "" {
		v += "+" + metadataString
	}
	return v
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := NewCommand().Execute(); err != nil {
		exit := 1
		var terr *exitError
		if errors.As(err, &terr) {
			exit = terr.Code
		}
		os.Exit(exit)
	}
}

// Command represents an invocation of the Cloud SQL Auth Proxy.
type Command struct {
	*cobra.Command
	conf             *proxy.Config
	logger           cloudsql.Logger
	dialer           cloudsql.Dialer
	cleanup          func() error
	connRefuseNotify func()
}

var longHelp = `
Overview

  The Cloud SQL Auth Proxy is a utility for ensuring secure connections to
  your Cloud SQL instances. It provides IAM authorization, allowing you to
  control who can connect to your instance through IAM permissions, and TLS
  1.3 encryption, without having to manage certificates.

  NOTE: The Proxy does not configure the network. You MUST ensure the Proxy
  can reach your Cloud SQL instance, either by deploying it in a VPC that has
  access to your Private IP instance, or by configuring Public IP.

  For every provided instance connection name, the Proxy creates:

  - a socket that mimics a database running locally, and
  - an encrypted connection using TLS 1.3 back to your Cloud SQL instance.

  The Proxy uses an ephemeral certificate to establish a secure connection to
  your Cloud SQL instance. The Proxy will refresh those certificates on an
  hourly basis. Existing client connections are unaffected by the refresh
  cycle.

Starting the Proxy

  To start the Proxy, you will need your instance connection name, which may
  be found in the Cloud SQL instance overview page or by using gcloud with the
  following command:

      gcloud sql instances describe INSTANCE --format='value(connectionName)'

  For example, if your instance connection name is
  "my-project:us-central1:my-db-server", starting the Proxy will be:

      ./cloud-sql-proxy my-project:us-central1:my-db-server

  By default, the Proxy will determine the database engine and start a
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

  The Proxy supports overriding configuration on an instance-level with an
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

  When necessary, you may specify the full path to a Unix socket. Set the
  unix-socket-path query parameter to the absolute path of the Unix socket for
  the database instance. The parent directory of the unix-socket-path must
  exist when the Proxy starts or else socket creation will fail. For Postgres
  instances, the Proxy will ensure that the last path element is
  '.s.PGSQL.5432' appending it if necessary. For example,

      ./cloud-sql-proxy \
        'my-project:us-central1:my-db-server?unix-socket-path=/path/to/socket'

Health checks

  When enabling the --health-check flag, the Proxy will start an HTTP server
  on localhost with three endpoints:

  - /startup: Returns 200 status when the Proxy has finished starting up.
  Otherwise returns 503 status.

  - /readiness: Returns 200 status when the Proxy has started, has available
  connections if max connections have been set with the --max-connections
  flag, and when the Proxy can connect to all registered instances. Otherwise,
  returns a 503 status.

  - /liveness: Always returns 200 status. If this endpoint is not responding,
  the Proxy is in a bad state and should be restarted.

  To configure the address, use --http-address. To configure the port, use
  --http-port.

Service Account Impersonation

  The Proxy supports service account impersonation with the
  --impersonate-service-account flag and matches gclouds flag. When enabled,
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

  Instead of using CLI flags, the Proxy may be configured using environment
  variables. Each environment variable uses "CSQL_PROXY" as a prefix and is
  the uppercase version of the flag using underscores as word delimiters. For
  example, the --auto-iam-authn flag may be set with the environment variable
  CSQL_PROXY_AUTO_IAM_AUTHN. An invocation of the Proxy using environment
  variables would look like the following:

      CSQL_PROXY_AUTO_IAM_AUTHN=true \
          ./cloud-sql-proxy my-project:us-central1:my-db-server

  In addition to CLI flags, instance connection names may also be specified
  with environment variables. If invoking the Proxy with only one instance
  connection name, use CSQL_PROXY_INSTANCE_CONNECTION_NAME. For example:

      CSQL_PROXY_INSTANCE_CONNECTION_NAME=my-project:us-central1:my-db-server \
          ./cloud-sql-proxy

  If multiple instance connection names are used, add the index of the
  instance connection name as a suffix. For example:

      CSQL_PROXY_INSTANCE_CONNECTION_NAME_0=my-project:us-central1:my-db-server \
      CSQL_PROXY_INSTANCE_CONNECTION_NAME_1=my-other-project:us-central1:my-other-server \
          ./cloud-sql-proxy

Configuration using a configuration file

  Instead of using CLI flags, the Proxy may be configured using a configuration
  file. The configuration file is a TOML, YAML or JSON file with the same keys
  as the environment variables. The configuration file is specified with the
  --config-file flag. An invocation of the Proxy using a configuration file
  would look like the following:

      ./cloud-sql-proxy --config-file=config.toml

  The configuration file may look like the following:

      instance-connection-name = "my-project:us-central1:my-server-instance"
      auto-iam-authn = true

  If multiple instance connection names are used, add the index of the
  instance connection name as a suffix. For example:

      instance-connection-name-0 = "my-project:us-central1:my-db-server"
      instance-connection-name-1 = "my-other-project:us-central1:my-other-server"

  The configuration file may also contain the same keys as the environment
  variables and flags. For example:

      auto-iam-authn = true
      debug = true
      max-connections = 5

Localhost Admin Server

  The Proxy includes support for an admin server on localhost. By default,
  the admin server is not enabled. To enable the server, pass the --debug or
  --quitquitquit flag. This will start the server on localhost at port 9091.
  To change the port, use the --admin-port flag.

  When --debug is set, the admin server enables Go's profiler available at
  /debug/pprof/.

  See the documentation on pprof for details on how to use the
  profiler at https://pkg.go.dev/net/http/pprof.

  When --quitquitquit is set, the admin server adds an endpoint at
  /quitquitquit. The admin server exits gracefully when it receives a GET or POST
  request at /quitquitquit.

Debug logging

  On occasion, it can help to enable debug logging which will report on
  internal certificate refresh operations. To enable debug logging, use:

      ./cloud-sql-proxy <INSTANCE_CONNECTION_NAME> --debug-logs

Waiting for Startup

  See the wait subcommand's help for details.

(*) indicates a flag that may be used as a query parameter

Third Party Licenses

  To view all licenses for third party dependencies used within this
  distribution please see:

  https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.15.2/third_party/licenses.tar.gz {x-release-please-version}
`

var waitHelp = `
Waiting for Proxy Startup

  Sometimes it is necessary to wait for the Proxy to start.

  To help ensure the Proxy is up and ready, the Proxy includes a wait
  subcommand with an optional --max flag to set the maximum time to wait.
  The wait command uses a separate Proxy's startup endpoint to determine
  if the other Proxy process is ready.

  Invoke the wait command, like this:

  # waits for another Proxy process' startup endpoint to respond
  ./cloud-sql-proxy wait

Configuration

  By default, the Proxy will wait up to the maximum time for the startup
  endpoint to respond. The wait command requires that the Proxy be started in
  another process with the HTTP health check or Prometheus enabled. If an
  alternate health check port or address is used, as in:

  ./cloud-sql-proxy <INSTANCE_CONNECTION_NAME> \
    --http-address 0.0.0.0 \
    --http-port 9191 \
    --health-check 

  Then the wait command must also be told to use the same custom values:

  ./cloud-sql-proxy wait \
    --http-address 0.0.0.0 \
    --http-port 9191

  By default the wait command will wait 30 seconds. To alter this value,
  use:

  ./cloud-sql-proxy wait --max 10s
`

const (
	waitMaxFlag     = "max"
	httpAddressFlag = "http-address"
	httpPortFlag    = "http-port"
)

func runWaitCmd(c *cobra.Command, _ []string) error {
	a, _ := c.Flags().GetString(httpAddressFlag)
	p, _ := c.Flags().GetString(httpPortFlag)
	addr := fmt.Sprintf("http://%v:%v/startup", a, p)

	wait, err := c.Flags().GetDuration(waitMaxFlag)
	if err != nil {
		// This error should always be nil. If the error occurs, it means the
		// wait flag name has changed where it was registered.
		return err
	}
	c.SilenceUsage = true

	t := time.After(wait)
	for {
		select {
		case <-t:
			return errors.New("command failed to complete successfully")
		default:
			resp, err := http.Get(addr)
			if err != nil || resp.StatusCode != http.StatusOK {
				time.Sleep(time.Second)
				break
			}
			return nil
		}
	}
}

const envPrefix = "CSQL_PROXY"

// NewCommand returns a Command object representing an invocation of the proxy.
func NewCommand(opts ...Option) *Command {
	rootCmd := &cobra.Command{
		Use:     "cloud-sql-proxy INSTANCE_CONNECTION_NAME...",
		Version: versionString,
		Short:   "cloud-sql-proxy authorizes and encrypts connections to Cloud SQL.",
		//remove the inline annotation required by release-please to update version.
		Long: strings.ReplaceAll(longHelp, "{x-release-please-version}", ""),
	}

	logger := log.NewStdLogger(os.Stdout, os.Stderr)
	c := &Command{
		Command: rootCmd,
		logger:  logger,
		cleanup: func() error { return nil },
		conf: &proxy.Config{
			UserAgent: userAgent,
		},
	}
	var waitCmd = &cobra.Command{
		Use:   "wait",
		Short: "Wait for another Proxy process to start",
		Long:  waitHelp,
		RunE:  runWaitCmd,
	}
	waitFlags := waitCmd.Flags()
	waitFlags.DurationP(
		waitMaxFlag, "m",
		30*time.Second,
		"maximum amount of time to wait for startup",
	)
	rootCmd.AddCommand(waitCmd)

	rootCmd.Args = func(_ *cobra.Command, args []string) error {
		// Load the configuration file before running the command. This should
		// ensure that the configuration is loaded in the correct order:
		//
		//   flags > environment variables > configuration files
		//
		// See https://github.com/carolynvs/stingoftheviper for more info
		return loadConfig(c, args, opts)
	}
	rootCmd.RunE = func(*cobra.Command, []string) error { return runSignalWrapper(c) }

	// Flags that apply only to the root command
	localFlags := rootCmd.Flags()
	// Flags that apply to all sub-commands
	globalFlags := rootCmd.PersistentFlags()

	localFlags.BoolP("help", "h", false, "Display help information for cloud-sql-proxy")
	localFlags.BoolP("version", "v", false, "Print the cloud-sql-proxy version")

	localFlags.StringVar(&c.conf.Filepath, "config-file", c.conf.Filepath,
		"Path to a TOML file containing configuration options.")
	localFlags.StringVar(&c.conf.OtherUserAgents, "user-agent", "",
		"Space separated list of additional user agents, e.g. cloud-sql-proxy-operator/0.0.1")
	localFlags.StringVarP(&c.conf.Token, "token", "t", "",
		"Use bearer token as a source of IAM credentials.")
	localFlags.StringVar(&c.conf.LoginToken, "login-token", "",
		"Use bearer token as a database password (used with token and auto-iam-authn only)")
	localFlags.StringVarP(&c.conf.CredentialsFile, "credentials-file", "c", "",
		"Use service account key file as a source of IAM credentials.")
	localFlags.StringVarP(&c.conf.CredentialsJSON, "json-credentials", "j", "",
		"Use service account key JSON as a source of IAM credentials.")
	localFlags.BoolVarP(&c.conf.GcloudAuth, "gcloud-auth", "g", false,
		`Use gclouds user credentials as a source of IAM credentials.
NOTE: this flag is a legacy feature and generally should not be used.
Instead prefer Application Default Credentials
(enabled with: gcloud auth application-default login) which
the Proxy will then pick-up automatically.`)
	localFlags.BoolVarP(&c.conf.StructuredLogs, "structured-logs", "l", false,
		"Enable structured logging with LogEntry format")
	localFlags.BoolVar(&c.conf.DebugLogs, "debug-logs", false,
		"Enable debug logging")
	localFlags.Uint64Var(&c.conf.MaxConnections, "max-connections", 0,
		"Limit the number of connections. Default is no limit.")
	localFlags.DurationVar(&c.conf.WaitBeforeClose, "min-sigterm-delay", 0,
		"The number of seconds to accept new connections after receiving a TERM signal.")
	localFlags.DurationVar(&c.conf.WaitOnClose, "max-sigterm-delay", 0,
		"Maximum number of seconds to wait for connections to close after receiving a TERM signal.")
	localFlags.StringVar(&c.conf.TelemetryProject, "telemetry-project", "",
		"Enable Cloud Monitoring and Cloud Trace with the provided project ID.")
	localFlags.BoolVar(&c.conf.DisableTraces, "disable-traces", false,
		"Disable Cloud Trace integration (used with --telemetry-project)")
	localFlags.IntVar(&c.conf.TelemetryTracingSampleRate, "telemetry-sample-rate", 10_000,
		"Set the Cloud Trace sample rate. A smaller number means more traces.")
	localFlags.BoolVar(&c.conf.DisableMetrics, "disable-metrics", false,
		"Disable Cloud Monitoring integration (used with --telemetry-project)")
	localFlags.StringVar(&c.conf.TelemetryPrefix, "telemetry-prefix", "",
		"Prefix for Cloud Monitoring metrics.")
	localFlags.BoolVar(&c.conf.ExitZeroOnSigterm, "exit-zero-on-sigterm", false,
		"Exit with 0 exit code when Sigterm received (default is 143)")
	localFlags.BoolVar(&c.conf.Prometheus, "prometheus", false,
		"Enable Prometheus HTTP endpoint /metrics on localhost")
	localFlags.StringVar(&c.conf.PrometheusNamespace, "prometheus-namespace", "",
		"Use the provided Prometheus namespace for metrics")
	globalFlags.StringVar(&c.conf.HTTPAddress, httpAddressFlag, "localhost",
		"Address for Prometheus and health check server")
	globalFlags.StringVar(&c.conf.HTTPPort, httpPortFlag, "9090",
		"Port for Prometheus and health check server")
	localFlags.BoolVar(&c.conf.Debug, "debug", false,
		"Enable pprof on the localhost admin server")
	localFlags.BoolVar(&c.conf.QuitQuitQuit, "quitquitquit", false,
		"Enable quitquitquit endpoint on the localhost admin server")
	localFlags.StringVar(&c.conf.AdminPort, "admin-port", "9091",
		"Port for localhost-only admin server")
	localFlags.BoolVar(&c.conf.HealthCheck, "health-check", false,
		"Enables health check endpoints /startup, /liveness, and /readiness on localhost.")
	localFlags.StringVar(&c.conf.APIEndpointURL, "sqladmin-api-endpoint", "",
		"API endpoint for all Cloud SQL Admin API requests. (default: https://sqladmin.googleapis.com)")
	localFlags.StringVar(&c.conf.UniverseDomain, "universe-domain", "",
		"Universe Domain for non-GDU environments. (default: googleapis.com)")
	localFlags.StringVar(&c.conf.QuotaProject, "quota-project", "",
		`Specifies the project to use for Cloud SQL Admin API quota tracking.
The IAM principal must have the "serviceusage.services.use" permission
for the given project. See https://cloud.google.com/service-usage/docs/overview and
https://cloud.google.com/storage/docs/requester-pays`)
	localFlags.StringVar(&c.conf.FUSEDir, "fuse", "",
		"Mount a directory at the path using FUSE to access Cloud SQL instances.")
	localFlags.StringVar(&c.conf.FUSETempDir, "fuse-tmp-dir",
		filepath.Join(os.TempDir(), "csql-tmp"),
		"Temp dir for Unix sockets created with FUSE")
	localFlags.StringVar(&c.conf.ImpersonationChain, "impersonate-service-account", "",
		`Comma separated list of service accounts to impersonate. Last value
is the target account.`)
	localFlags.BoolVar(&c.conf.Quiet, "quiet", false, "Log error messages only")
	localFlags.BoolVar(&c.conf.AutoIP, "auto-ip", false,
		`Supports legacy behavior of v1 and will try to connect to first IP
address returned by the SQL Admin API. In most cases, this flag should not be used.
Prefer default of public IP or use --private-ip instead.`)
	localFlags.BoolVar(&c.conf.LazyRefresh, "lazy-refresh", false,
		`Configure a lazy refresh where connection info is retrieved only if
the cached copy has expired. Use this setting in environments where the
CPU may be throttled and a background refresh cannot run reliably
(e.g., Cloud Run)`,
	)

	localFlags.BoolVar(&c.conf.RunConnectionTest, "run-connection-test", false, `Runs a connection test
against all specified instances. If an instance is unreachable, the Proxy exits with a failure
status code.`)

	// Global and per instance flags
	localFlags.StringVarP(&c.conf.Addr, "address", "a", "127.0.0.1",
		"(*) Address to bind Cloud SQL instance listeners.")
	localFlags.IntVarP(&c.conf.Port, "port", "p", 0,
		"(*) Initial port for listeners. Subsequent listeners increment from this value.")
	localFlags.StringVarP(&c.conf.UnixSocket, "unix-socket", "u", "",
		`(*) Enables Unix sockets for all listeners with the provided directory.`)
	localFlags.BoolVarP(&c.conf.IAMAuthN, "auto-iam-authn", "i", false,
		"(*) Enables Automatic IAM Authentication for all instances")
	localFlags.BoolVar(&c.conf.PrivateIP, "private-ip", false,
		"(*) Connect to the private ip address for all instances")
	localFlags.BoolVar(&c.conf.PSC, "psc", false,
		"(*) Connect to the PSC endpoint for all instances")

	return c
}

func loadConfig(c *Command, args []string, opts []Option) error {
	v, err := initViper(c)
	if err != nil {
		return err
	}

	c.Flags().VisitAll(func(f *pflag.Flag) {
		// Override any unset flags with Viper values to use the pflags
		// object as a single source of truth.
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			_ = c.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})

	// If args is not already populated, try to read from the environment.
	if len(args) == 0 {
		args = instanceFromEnv(args)
	}

	// If no environment args are present, try to read from the config file.
	if len(args) == 0 {
		args = instanceFromConfigFile(v)
	}

	for _, o := range opts {
		o(c)
	}

	// Handle logger separately from config
	if c.conf.StructuredLogs {
		c.logger, c.cleanup = log.NewStructuredLogger(c.conf.Quiet)
	}

	if c.conf.Quiet {
		c.logger = log.NewStdLogger(io.Discard, os.Stderr)
	}

	err = parseConfig(c, c.conf, args)
	if err != nil {
		return err
	}

	// The arguments are parsed. Usage is no longer needed.
	c.SilenceUsage = true

	// Errors will be handled by logging from here on.
	c.SilenceErrors = true

	return nil
}

func initViper(c *Command) (*viper.Viper, error) {
	v := viper.New()

	if c.conf.Filepath != "" {
		// Setup Viper configuration file. Viper will attempt to load
		// configuration from the specified file if it exists. Otherwise, Viper
		// will source all configuration from flags and then environment
		// variables.
		ext := filepath.Ext(c.conf.Filepath)

		badExtErr := newBadCommandError(
			fmt.Sprintf("config file %v should have extension of "+
				"toml, yaml, or json", c.conf.Filepath,
			))

		if ext == "" {
			return nil, badExtErr
		}

		if ext != ".toml" && ext != ".yaml" && ext != ".yml" && ext != ".json" {
			return nil, badExtErr
		}

		conf := filepath.Base(c.conf.Filepath)
		noExt := strings.ReplaceAll(conf, ext, "")
		// argument must be the name of config file without extension
		v.SetConfigName(noExt)
		v.AddConfigPath(filepath.Dir(c.conf.Filepath))

		// Attempt to load configuration from a file. If no file is found,
		// assume configuration is provided by flags or environment variables.
		if err := v.ReadInConfig(); err != nil {
			// If the error is a ConfigFileNotFoundError, then ignore it.
			// Otherwise, report the error to the user.
			var cErr viper.ConfigFileNotFoundError
			if !errors.As(err, &cErr) {
				return nil, newBadCommandError(fmt.Sprintf(
					"failed to load configuration from %v: %v",
					c.conf.Filepath, err,
				))
			}
		}
	}

	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	return v, nil
}

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

func instanceFromConfigFile(v *viper.Viper) []string {
	var args []string
	inst := v.GetString("instance-connection-name")

	if inst == "" {
		inst = v.GetString("instance-connection-name-0")
		if inst == "" {
			return nil
		}
	}
	args = append(args, inst)

	i := 1
	for {
		instN := v.GetString(fmt.Sprintf("instance-connection-name-%d", i))
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

func userHasSetLocal(cmd *Command, f string) bool {
	return cmd.LocalFlags().Lookup(f).Changed
}

func userHasSetGlobal(cmd *Command, f string) bool {
	return cmd.PersistentFlags().Lookup(f).Changed
}

func parseConfig(cmd *Command, conf *proxy.Config, args []string) error {
	// If no instance connection names were provided AND FUSE isn't enabled,
	// error.
	if len(args) == 0 && conf.FUSEDir == "" {
		return newBadCommandError("missing instance_connection_name (e.g., project:region:instance)")
	}

	if conf.FUSEDir != "" {
		if conf.RunConnectionTest {
			return newBadCommandError("cannot run connection tests in FUSE mode")
		}

		if err := proxy.SupportsFUSE(); err != nil {
			return newBadCommandError(
				fmt.Sprintf("--fuse is not supported: %v", err),
			)
		}
	}

	if len(args) == 0 && conf.FUSEDir == "" && conf.FUSETempDir != "" {
		return newBadCommandError("cannot specify --fuse-tmp-dir without --fuse")
	}

	if userHasSetLocal(cmd, "address") && userHasSetLocal(cmd, "unix-socket") {
		return newBadCommandError("cannot specify --unix-socket and --address together")
	}
	if userHasSetLocal(cmd, "port") && userHasSetLocal(cmd, "unix-socket") {
		return newBadCommandError("cannot specify --unix-socket and --port together")
	}
	if ip := net.ParseIP(conf.Addr); ip == nil {
		return newBadCommandError(fmt.Sprintf("not a valid IP address: %q", conf.Addr))
	}
	if userHasSetLocal(cmd, "private-ip") && userHasSetLocal(cmd, "auto-ip") {
		return newBadCommandError("cannot specify --private-ip and --auto-ip together")
	}

	// If more than one IP type is set, error.
	if conf.PrivateIP && conf.PSC {
		return newBadCommandError("cannot specify --private-ip and --psc flags at the same time")
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

	// When using token with auto-iam-authn, login-token must also be set.
	// All three are required together.
	if conf.IAMAuthN && conf.Token != "" && conf.LoginToken == "" {
		return newBadCommandError("cannot specify --auto-iam-authn and --token without --login-token")
	}
	if conf.IAMAuthN && conf.GcloudAuth {
		return newBadCommandError(`cannot use --auto-iam-authn with --gcloud-auth.
Instead use Application Default Credentials (enabled with: gcloud auth application-default login)
and re-try with just --auto-iam-authn`)
	}
	if conf.LoginToken != "" && (conf.Token == "" || !conf.IAMAuthN) {
		return newBadCommandError("cannot specify --login-token without --token and --auto-iam-authn")
	}

	if userHasSetGlobal(cmd, "http-port") && !userHasSetLocal(cmd, "prometheus") && !userHasSetLocal(cmd, "health-check") {
		cmd.logger.Infof("Ignoring --http-port because --prometheus or --health-check was not set")
	}

	if !userHasSetLocal(cmd, "telemetry-project") && userHasSetLocal(cmd, "telemetry-prefix") {
		cmd.logger.Infof("Ignoring --telementry-prefix because --telemetry-project was not set")
	}
	if !userHasSetLocal(cmd, "telemetry-project") && userHasSetLocal(cmd, "disable-metrics") {
		cmd.logger.Infof("Ignoring --disable-metrics because --telemetry-project was not set")
	}
	if !userHasSetLocal(cmd, "telemetry-project") && userHasSetLocal(cmd, "disable-traces") {
		cmd.logger.Infof("Ignoring --disable-traces because --telemetry-project was not set")
	}

	if userHasSetLocal(cmd, "user-agent") {
		userAgent += " " + cmd.conf.OtherUserAgents
		conf.UserAgent = userAgent
	}

	if userHasSetLocal(cmd, "sqladmin-api-endpoint") && userHasSetLocal(cmd, "universe-domain") {
		return newBadCommandError("cannot specify --sqladmin-api-endpoint and --universe-domain at the same time")
	}
	if userHasSetLocal(cmd, "sqladmin-api-endpoint") && conf.APIEndpointURL != "" {
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
			up, upok := q["unix-socket-path"]

			if aok && uok {
				return newBadCommandError("cannot specify both address and unix-socket query params")
			}
			if pok && uok {
				return newBadCommandError("cannot specify both port and unix-socket query params")
			}
			if aok && upok {
				return newBadCommandError("cannot specify both address and unix-socket-path query params")
			}
			if pok && upok {
				return newBadCommandError("cannot specify both port and unix-socket-path query params")
			}
			if uok && upok {
				return newBadCommandError("cannot specify both unix-socket-path and unix-socket query params")
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

			if upok {
				if len(up) != 1 {
					return newBadCommandError(fmt.Sprintf("unix-socket-path query param should be only one value: %q", a))
				}
				ic.UnixSocketPath = up[0]
			}

			ic.IAMAuthN, err = parseBoolOpt(q, "auto-iam-authn")
			if err != nil {
				return err
			}

			ic.PrivateIP, err = parseBoolOpt(q, "private-ip")
			if err != nil {
				return err
			}
			if ic.PrivateIP != nil && *ic.PrivateIP && conf.AutoIP {
				return newBadCommandError("cannot use --auto-ip with private-ip")
			}

			ic.PSC, err = parseBoolOpt(q, "psc")
			if err != nil {
				return err
			}

			if ic.PrivateIP != nil && ic.PSC != nil {
				return newBadCommandError("cannot specify both private-ip and psc query params")
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
	v, ok := q[name]
	if !ok {
		return nil, nil
	}

	if len(v) != 1 {
		return nil, newBadCommandError(fmt.Sprintf("%v param should be only one value: %q", name, v))
	}

	switch strings.ToLower(v[0]) {
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
				name, v[0],
			))
	}

}

// runSignalWrapper watches for SIGTERM and SIGINT and interupts execution if necessary.
func runSignalWrapper(cmd *Command) (err error) {
	defer func() { _ = cmd.cleanup() }()
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
			cmd.logger.Debugf("Sending SIGINT signal for proxy to shutdown.")
			shutdownCh <- errSigInt
		case syscall.SIGTERM:
			cmd.logger.Debugf("Sending SIGTERM signal for proxy to shutdown.")
			if cmd.conf.ExitZeroOnSigterm {
				shutdownCh <- errSigTermZero
			} else {
				shutdownCh <- errSigTerm
			}
		}
	}()

	// Start the proxy asynchronously, so we can exit early if a shutdown signal is sent
	startCh := make(chan *proxy.Client)
	go func() {
		defer close(startCh)
		p, err := proxy.NewClient(ctx, cmd.dialer, cmd.logger, cmd.conf, cmd.connRefuseNotify)
		if err != nil {
			cmd.logger.Debugf("Error starting proxy: %v", err)
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
		// If running under systemd with Type=notify, it will send a message to the
		// service manager that a failure occurred, and it is terminating.
		go func() {
			if _, err := daemon.SdNotify(false, daemon.SdNotifyStopping); err != nil {
				cmd.logger.Errorf("Failed to notify systemd of termination: %v", err)
			}
		}()
		return err
	case p = <-startCh:
		cmd.logger.Infof("The proxy has started successfully and is ready for new connections!")
		// If running under systemd with Type=notify, it will send a message to the
		// service manager that it is ready to handle connections now.
		go func() {
			if _, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
				cmd.logger.Errorf("Failed to notify systemd of readiness: %v", err)
			}
		}()
	}
	defer func() {
		if cErr := p.Close(); cErr != nil {
			cmd.logger.Errorf("error during shutdown: %v", cErr)
			// Capture error from close to propagate it to the caller.
			err = cErr
		}
	}()

	var (
		needsHTTPServer bool
		mux             = http.NewServeMux()
		notifyStarted   = func() {}
		notifyStopped   = func() {}
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

	if cmd.conf.HealthCheck {
		needsHTTPServer = true
		cmd.logger.Infof("Starting health check server at %s",
			net.JoinHostPort(cmd.conf.HTTPAddress, cmd.conf.HTTPPort))
		hc := healthcheck.NewCheck(p, cmd.logger)
		mux.HandleFunc("/startup", hc.HandleStartup)
		mux.HandleFunc("/readiness", hc.HandleReadiness)
		mux.HandleFunc("/liveness", hc.HandleLiveness)
		notifyStarted = hc.NotifyStarted
		notifyStopped = hc.NotifyStopped
	}
	defer notifyStopped()
	// Start the HTTP server if anything requiring HTTP is specified.
	if needsHTTPServer {
		go startHTTPServer(
			ctx,
			cmd.logger,
			net.JoinHostPort(cmd.conf.HTTPAddress, cmd.conf.HTTPPort),
			mux,
			shutdownCh,
		)
	}

	var (
		needsAdminServer bool
		m                = http.NewServeMux()
	)
	if cmd.conf.QuitQuitQuit {
		needsAdminServer = true
		cmd.logger.Infof("Enabling quitquitquit endpoint at localhost:%v", cmd.conf.AdminPort)
		// quitquitquit allows for shutdown on localhost only.
		var quitOnce sync.Once
		m.HandleFunc("/quitquitquit", quitquitquit(&quitOnce, shutdownCh))
	}
	if cmd.conf.Debug {
		needsAdminServer = true
		cmd.logger.Infof("Enabling pprof endpoints at localhost:%v", cmd.conf.AdminPort)
		// pprof standard endpoints
		m.HandleFunc("/debug/pprof/", pprof.Index)
		m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		m.HandleFunc("/debug/pprof/profile", pprof.Profile)
		m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		m.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	if needsAdminServer {
		go startHTTPServer(
			ctx,
			cmd.logger,
			net.JoinHostPort("localhost", cmd.conf.AdminPort),
			m,
			shutdownCh,
		)
	}

	go func() {
		err := p.Serve(ctx, notifyStarted)
		cmd.logger.Debugf("proxy server error: %v", err)
		shutdownCh <- err
	}()

	err = <-shutdownCh
	switch {
	case errors.Is(err, errSigInt):
		cmd.logger.Infof("SIGINT signal received. Shutting down...")
		time.Sleep(cmd.conf.WaitBeforeClose)
	case errors.Is(err, errSigTerm):
		cmd.logger.Infof("SIGTERM signal received. Shutting down...")
		time.Sleep(cmd.conf.WaitBeforeClose)
	case errors.Is(err, errSigTermZero):
		cmd.logger.Infof("SIGTERM signal received. Shutting down...")
		time.Sleep(cmd.conf.WaitBeforeClose)
	case errors.Is(err, errQuitQuitQuit):
		cmd.logger.Infof("/quitquitquit received request. Shutting down...")
		time.Sleep(cmd.conf.WaitBeforeClose)
	default:
		cmd.logger.Errorf("The proxy has encountered a terminal error: %v", err)
	}
	return err
}

func quitquitquit(quitOnce *sync.Once, shutdownCh chan<- error) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost && req.Method != http.MethodGet {
			rw.WriteHeader(400)
			return
		}
		quitOnce.Do(func() {
			select {
			case shutdownCh <- errQuitQuitQuit:
			default:
				// The write attempt to shutdownCh failed and
				// the proxy is already exiting.
			}
		})
	}
}

func startHTTPServer(ctx context.Context, l cloudsql.Logger, addr string, mux *http.ServeMux, shutdownCh chan<- error) {
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	// Start the HTTP server.
	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		if err != nil {
			shutdownCh <- fmt.Errorf("failed to start HTTP server: %v", err)
		}
	}()
	// Handle shutdown of the HTTP server gracefully.
	<-ctx.Done()
	// Give the HTTP server a second to shut down cleanly.
	ctx2, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx2); err != nil {
		l.Errorf("failed to shutdown HTTP server: %v\n", err)
	}
}

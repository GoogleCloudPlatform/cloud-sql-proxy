# Structure

Configuration defined in cmd/root.go

| cli option            | per instance | global config field            | instance override config field      |
|-----------------------|--------------|--------------------------------|-------------------------------------|
| telemetry-project     |              | cmd.telemetryProject           |                                     |
| disable-traces        |              | cmd.disableTraces              |                                     |
| telemetry-sample-rate |              | cmd.telemetryTracingSampleRate |                                     |
| disable-metrics       |              | cmd.disableMetrics             |                                     |
| telemetry-project     |              | cmd.telemetryProject           |                                     |
| telemetry-prefix      |              | cmd.telemetryPrefix            |                                     |
| prometheus-namespace  |              | cmd.prometheusNamespace        |                                     |
| http-port             |              | cmd.httpPort                   |                                     |
| token (t)             |              | proxy.Config.Token             |                                     |
| credentials-file (c)  |              | proxy.Config.CredentialsFile   |                                     |
| gcloud-auth(g)        |              | proxy.Config.GcloudAuth        |                                     |
| address (a)           | ✅            | proxy.Config.Addr              | proxy.InstanceConnConfig.Addr       |
| port (p)              | ✅            | proxy.Config.Port              | proxy.InstanceConnConfig.Port       |
| unix-socket (u)       | ✅            | proxy.Config.UnixSocket        | proxy.InstanceConnConfig.UnixSocket |
| auto-iam-authn (i)    | ✅            | proxy.Config.IAMAuthN          | proxy.InstanceConnConfig.IAMAuthN   |
| private-ip            | ✅            | proxy.Config.PrivateIP         | proxy.InstanceConnConfig.PrivateIP  |


`proxy.NewClient()` in proxy.go:151 should be refactored to separate concerns:

- proxy.NewConfiguration(Config): transforming a Config into a
  cloud.google.com/go/cloudsqlconn
- expose the intended state of
- proxy.Listen()

proxy.go:243 Passing a &Command in exposes too much interface for use as a
simple logging. Could we create a CommandLogger interface that has PrintF and
PrintErr?

proxy.go  `30*time.Second` different kinds of timeouts should be configurable.

## Separating Concerns / Single Responsibility Principle

Computers only do two things: store data and orchestrate machine instructions to
transform data.

Software engineers define the structure of the data and transformations in a way
that allows us to think about how the software works and whether we did built
the software correctly to get the desired results.

The Separation of Concerns and Single Responsibility Principle are lofty
engineering goals, but are vague. We need heuristics on how to write code that
will accomplish these goals. These are my heuristics I use to keep the cognitive
load reasonable.

I like to define a single concern with a single responsibility as one of these
three:

- a data format describing a single conceptual domain (see domain-driven design)
- a transformation between 2 data formats
- an orchestration of 7 or fewer subservient concerns

This applies regardless of the programming language, style, and idiom.

Ways to hint at a concern Boundry in go: 
- a short function < 30 lines of code
- a section of code ~15 lines of code with a comment at the top within a long function
- a source file containing structs
- a package

### Contracts and Testing

I like each concern to define a contract with the outside world on its function,
parameters, and data.

I like each concern to have unit tests that exercise it independent of external
usage. I like each collection of concerns to have integration tests that make
sure that several related concerns work correctly together.

Config --> ServerlessConfig --> socketMount

## Separated Concerns in the cloudsql-proxy

### Orchestrate the process lifecycle of the proxy process

- main.main() – the standard go binary entrypoint, runs cmd.Execute()
    - cmd.Execute() – Configures and runs the commands
        - cmd.NewCommand() – builds a cobra.Command object
        - cobra.Execute() - cobra library parses cli and invokes the command
            - cmd.parseConfig() – transforms parsed CLI args flags into a
              proxy.Config
            - cmd.runSignalWrapper() – runs the proxy, transforms between proxy
              and OS signals

### Transform between OS process and proxy.Config

- proxy.Config – data format describing the desired behavior the proxy listeners
- cmd.parseConfig() – transforms parsed CLI args flags into a proxy.Config

### Transform between proxy.Config and cloudsqlconn library concepts

- cloudsqlconn.DialOption configuration for cloudsqlconn
- proxy.NewClient(ctx context.Context, conf *Config) - transforms proxy.Config
  into an internal state ready to perform the proxy work

### Orchestrate the lifecycle of the proxy listeners and processors

- cloudsqlconn.Dialer builds a net.Conn to cloudsql instances
- cloudsqlconn.
- proxy.socketMount - data describing a single proxy connection and its listener
- proxy.Serve() – orchestrate all proxy listeners
    - proxy.serveSocketMount() – orchestrate handling of an inbound connection
      to the proxy
        - proxy.proxyConn() – orchestrate movement of a single connection's
          proxy data between listener and cloudsql instance

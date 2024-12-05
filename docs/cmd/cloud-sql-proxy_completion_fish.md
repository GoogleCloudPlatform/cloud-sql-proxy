## cloud-sql-proxy completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	cloud-sql-proxy completion fish | source

To load completions for every new session, execute once:

	cloud-sql-proxy completion fish > ~/.config/fish/completions/cloud-sql-proxy.fish

You will need to start a new shell for this setup to take effect.


```
cloud-sql-proxy completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --http-address string   Address for Prometheus and health check server (default "localhost")
      --http-port string      Port for Prometheus and health check server (default "9090")
```

### SEE ALSO

* [cloud-sql-proxy completion](cloud-sql-proxy_completion.md)	 - Generate the autocompletion script for the specified shell


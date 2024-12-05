## cloud-sql-proxy completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(cloud-sql-proxy completion zsh)

To load completions for every new session, execute once:

#### Linux:

	cloud-sql-proxy completion zsh > "${fpath[1]}/_cloud-sql-proxy"

#### macOS:

	cloud-sql-proxy completion zsh > $(brew --prefix)/share/zsh/site-functions/_cloud-sql-proxy

You will need to start a new shell for this setup to take effect.


```
cloud-sql-proxy completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --http-address string   Address for Prometheus and health check server (default "localhost")
      --http-port string      Port for Prometheus and health check server (default "9090")
```

### SEE ALSO

* [cloud-sql-proxy completion](cloud-sql-proxy_completion.md)	 - Generate the autocompletion script for the specified shell


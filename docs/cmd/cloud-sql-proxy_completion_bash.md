## cloud-sql-proxy completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(cloud-sql-proxy completion bash)

To load completions for every new session, execute once:

#### Linux:

	cloud-sql-proxy completion bash > /etc/bash_completion.d/cloud-sql-proxy

#### macOS:

	cloud-sql-proxy completion bash > $(brew --prefix)/etc/bash_completion.d/cloud-sql-proxy

You will need to start a new shell for this setup to take effect.


```
cloud-sql-proxy completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --http-address string   Address for Prometheus and health check server (default "localhost")
      --http-port string      Port for Prometheus and health check server (default "9090")
```

### SEE ALSO

* [cloud-sql-proxy completion](cloud-sql-proxy_completion.md)	 - Generate the autocompletion script for the specified shell


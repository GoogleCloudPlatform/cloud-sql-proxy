## cloud-sql-proxy completion powershell

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	cloud-sql-proxy completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
cloud-sql-proxy completion powershell [flags]
```

### Options

```
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --http-address string   Address for Prometheus and health check server (default "localhost")
      --http-port string      Port for Prometheus and health check server (default "9090")
```

### SEE ALSO

* [cloud-sql-proxy completion](cloud-sql-proxy_completion.md)	 - Generate the autocompletion script for the specified shell


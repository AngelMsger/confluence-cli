## confluence-cli config

Manage confluence-cli configuration

### Options

```
  -h, --help   help for config
```

### Options inherited from parent commands

```
      --base-url string      Confluence site URL (overrides config)
      --config string        config directory (default ~/.confluence)
      --fields string        comma-separated dot-path fields to keep
      --flavor string        backend flavor: cloud, datacenter or auto
  -f, --format string        output format: json, table or ndjson
      --timeout string       request timeout, e.g. 30s
      --use-context string   use a named context for this invocation
  -v, --verbose              verbose diagnostics on stderr
```

### SEE ALSO

* [confluence-cli](confluence-cli.md)	 - Use a Confluence instance as a knowledge base for coding agents
* [confluence-cli config delete-context](confluence-cli_config_delete-context.md)	 - Delete a context and its stored credential
* [confluence-cli config get-contexts](confluence-cli_config_get-contexts.md)	 - List the configured contexts
* [confluence-cli config init](confluence-cli_config_init.md)	 - Interactively set up server URL and credentials
* [confluence-cli config path](confluence-cli_config_path.md)	 - Print the config file path
* [confluence-cli config show](confluence-cli_config_show.md)	 - Show the resolved configuration
* [confluence-cli config use-context](confluence-cli_config_use-context.md)	 - Switch the current context


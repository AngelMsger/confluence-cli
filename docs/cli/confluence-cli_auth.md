## confluence-cli auth

Inspect and manage stored credentials

### Options

```
  -h, --help   help for auth
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
* [confluence-cli auth login](confluence-cli_auth_login.md)	 - Store a credential for the configured server
* [confluence-cli auth logout](confluence-cli_auth_logout.md)	 - Remove the stored credential for the configured server
* [confluence-cli auth status](confluence-cli_auth_status.md)	 - Show whether a usable credential is configured


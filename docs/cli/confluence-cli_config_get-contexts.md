## confluence-cli config get-contexts

List the configured contexts

### Synopsis

List every context in the config file. The current context — the one
used when --use-context is not given — is marked.

```
confluence-cli config get-contexts [flags]
```

### Examples

```
  confluence-cli config get-contexts
  confluence-cli config get-contexts --format table
```

### Options

```
  -h, --help   help for get-contexts
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

* [confluence-cli config](confluence-cli_config.md)	 - Manage confluence-cli configuration


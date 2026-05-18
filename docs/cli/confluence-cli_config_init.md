## confluence-cli config init

Interactively set up server URL and credentials

### Synopsis

Run the interactive setup wizard. It collects a server URL, detects
the flavor, validates a credential and stores it. The wizard can also
configure additional named contexts for working with several servers.

```
confluence-cli config init [flags]
```

### Examples

```
  confluence-cli config init
```

### Options

```
  -h, --help   help for init
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


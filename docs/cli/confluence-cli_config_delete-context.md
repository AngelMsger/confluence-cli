## confluence-cli config delete-context

Delete a context and its stored credential

```
confluence-cli config delete-context <name> [flags]
```

### Examples

```
  confluence-cli config delete-context staging
```

### Options

```
  -h, --help   help for delete-context
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


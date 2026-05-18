## confluence-cli doctor

Diagnose configuration, credentials and connectivity

```
confluence-cli doctor [flags]
```

### Examples

```
  confluence-cli doctor
  confluence-cli doctor --no-update-check
```

### Options

```
  -h, --help              help for doctor
      --no-update-check   skip the check for a newer confluence-cli release
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


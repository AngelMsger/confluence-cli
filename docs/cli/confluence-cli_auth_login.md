## confluence-cli auth login

Store a credential for the configured server

### Synopsis

Prompt for a secret and store it securely. Run `config init` first if the server URL is not set.

```
confluence-cli auth login [flags]
```

### Examples

```
  confluence-cli auth login
  confluence-cli --use-context staging auth login
```

### Options

```
  -h, --help   help for login
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

* [confluence-cli auth](confluence-cli_auth.md)	 - Inspect and manage stored credentials


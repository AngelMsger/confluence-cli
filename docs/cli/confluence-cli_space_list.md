## confluence-cli space list

List spaces

```
confluence-cli space list [flags]
```

### Options

```
      --all           fetch every page of results
  -h, --help          help for list
      --limit int     page size (default from config)
      --type string   filter by type: global or personal
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

* [confluence-cli space](confluence-cli_space.md)	 - List and inspect Confluence spaces


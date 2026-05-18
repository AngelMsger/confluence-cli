## confluence-cli page children

List the direct child pages of a page

```
confluence-cli page children <id|url> [flags]
```

### Examples

```
  confluence-cli page children 123456
  confluence-cli page children 123456 --all --format table
```

### Options

```
      --all         fetch every page of results
  -h, --help        help for children
      --limit int   page size (default from config)
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

* [confluence-cli page](confluence-cli_page.md)	 - Read and write Confluence pages


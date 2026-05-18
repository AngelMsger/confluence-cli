## confluence-cli attachment list

List the attachments of a page

```
confluence-cli attachment list <id|url> [flags]
```

### Examples

```
  confluence-cli attachment list 123456
```

### Options

```
      --all         fetch every page of results
  -h, --help        help for list
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

* [confluence-cli attachment](confluence-cli_attachment.md)	 - List and download page attachments


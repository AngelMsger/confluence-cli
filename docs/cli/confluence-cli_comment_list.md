## confluence-cli comment list

List the footer comments of a page

```
confluence-cli comment list <id|url> [flags]
```

### Examples

```
  confluence-cli comment list 123456
  confluence-cli comment list 123456 --all --as text
```

### Options

```
      --all         fetch every page of results
      --as string   render comment bodies as markdown or text (default "markdown")
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

* [confluence-cli comment](confluence-cli_comment.md)	 - Read and post page comments


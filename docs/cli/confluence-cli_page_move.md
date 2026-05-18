## confluence-cli page move

Move a page under a new parent and/or space

```
confluence-cli page move <id|url> [flags]
```

### Examples

```
  # reparent a page
  confluence-cli page move 123456 --target-parent 999

  # move a page into another space
  confluence-cli page move 123456 --target-space DOCS
```

### Options

```
      --dry-run                print the request without sending it
  -h, --help                   help for move
      --target-parent string   new parent page ID or URL
      --target-space string    new space key
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


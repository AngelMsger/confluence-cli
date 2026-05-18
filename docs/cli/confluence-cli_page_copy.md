## confluence-cli page copy

Copy a page's title and body to a new page

### Synopsis

Create a new page from an existing one. The copy is shallow: it
carries the title and body only, not child pages or attachments.

```
confluence-cli page copy <id|url> --title <TITLE> [flags]
```

### Examples

```
  confluence-cli page copy 123456 --title "Copy of the spec"
  confluence-cli page copy 123456 --title Draft --space SANDBOX
```

### Options

```
      --dry-run         print the request without sending it
  -h, --help            help for copy
      --parent string   parent page for the copy (default: source parent)
      --space string    space key for the copy (default: source space)
      --title string    title for the copied page
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


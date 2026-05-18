## confluence-cli attachment download

Download an attachment's content

### Synopsis

Download an attachment by its content ID. Use --output - to stream to stdout.

```
confluence-cli attachment download <attachment-id|url> [flags]
```

### Examples

```
  confluence-cli attachment download att12345 --output spec.pdf
  confluence-cli attachment download att12345 --output -
```

### Options

```
  -h, --help            help for download
  -o, --output string   output path ('-' for stdout)
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


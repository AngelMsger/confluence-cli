## confluence-cli page delete

Delete a page (move it to the trash)

### Synopsis

Move a page to the trash. With --purge the trashed page is then
permanently removed. Deletion requires --yes, or an interactive
confirmation when stdin is a terminal.

```
confluence-cli page delete <id|url> [flags]
```

### Examples

```
  confluence-cli page delete 123456 --yes
  confluence-cli page delete 123456 --purge --yes
```

### Options

```
      --dry-run   print the request without sending it
  -h, --help      help for delete
      --purge     permanently delete (removes the trashed page)
      --yes       skip the deletion confirmation
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


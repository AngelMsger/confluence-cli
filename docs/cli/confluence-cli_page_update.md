## confluence-cli page update

Update a page's title and/or body

### Synopsis

Update an existing page. Omitted fields are kept as-is. The new
version is the current version + 1; pass --version to assert the
version you last read (a mismatch fails with a conflict error).

```
confluence-cli page update <id|url> [flags]
```

### Examples

```
  # retitle a page, keeping its body
  confluence-cli page update 123456 --title "New Title"

  # replace the body from Markdown, asserting the version last read
  confluence-cli page update 123456 --format markdown --body-file body.md --version 7
```

### Options

```
      --body string        body text
      --body-file string   read body from a file ('-' for stdin)
      --dry-run            print the request without sending it
      --format string      body format: storage, wiki or markdown (default "storage")
  -h, --help               help for update
      --message string     version comment for the edit
      --minor              mark the edit as minor
      --title string       new page title (kept when omitted)
      --version int        expected current version (fetched when omitted)
```

### Options inherited from parent commands

```
      --base-url string      Confluence site URL (overrides config)
      --config string        config directory (default ~/.confluence)
      --fields string        comma-separated dot-path fields to keep
      --flavor string        backend flavor: cloud, datacenter or auto
      --timeout string       request timeout, e.g. 30s
      --use-context string   use a named context for this invocation
  -v, --verbose              verbose diagnostics on stderr
```

### SEE ALSO

* [confluence-cli page](confluence-cli_page.md)	 - Read and write Confluence pages


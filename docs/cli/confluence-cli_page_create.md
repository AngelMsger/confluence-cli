## confluence-cli page create

Create a new page

### Synopsis

Create a page in a space. Use --parent to nest it under an existing
page. The body may be storage-format XHTML, Confluence wiki markup or
Markdown (--format markdown, converted on a best-effort basis).

```
confluence-cli page create --space <KEY> --title <TITLE> [flags]
```

### Examples

```
  # create a page from a Markdown file, nested under a parent
  confluence-cli page create --space ENG --title "Release Notes" \
    --parent 123456 --format markdown --body-file notes.md

  # preview the request without sending it
  confluence-cli page create --space ENG --title Draft --body "<p>hi</p>" --dry-run
```

### Options

```
      --body string        body text
      --body-file string   read body from a file ('-' for stdin)
      --dry-run            print the request without sending it
      --format string      body format: storage, wiki or markdown (default "storage")
  -h, --help               help for create
      --parent string      parent page ID or URL
      --space string       space key for the new page
      --title string       page title
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


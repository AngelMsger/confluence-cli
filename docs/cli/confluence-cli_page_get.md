## confluence-cli page get

Fetch a page and render its body

### Synopsis

Fetch a single page. Use --scope to read only part of the body:
  outline  list the headings (start here when the structure is unknown)
  section  one section, identified by --section <id> from the outline
  keyword  blocks matching --keyword, with their heading for context
  full     the entire body (default)

```
confluence-cli page get <id|url> [flags]
```

### Examples

```
  # render the whole page as Markdown
  confluence-cli page get 123456

  # list the headings, then read just one section
  confluence-cli page get 123456 --scope outline
  confluence-cli page get 123456 --scope section --section sec-2

  # a page URL works in place of an ID
  confluence-cli page get https://wiki.example.com/pages/viewpage.action?pageId=123456
```

### Options

```
      --as string            render body as markdown or text (default "markdown")
      --body-format string   source body format: storage or view (default "storage")
      --detail string        block detail: simple, with-ids or full (default "simple")
  -h, --help                 help for get
      --keyword string       keyword (with --scope keyword)
      --no-body              fetch metadata only, skip the body
      --scope string         read scope: full, outline, section or keyword (default "full")
      --section string       section ID (with --scope section)
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


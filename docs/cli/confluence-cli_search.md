## confluence-cli search

Search pages with CQL or filter flags

### Synopsis

Search Confluence content. Provide a raw CQL string as the argument,
or build one from filter flags (--text, --author, --space, ...).

```
confluence-cli search [cql] [flags]
```

### Examples

```
  # filter flags are combined into a CQL query
  confluence-cli search --text "release process" --space ENG --type page

  # or pass raw CQL directly
  confluence-cli search 'creator = "jdoe" AND created >= "2025-01-01"' --all
```

### Options

```
      --after string         modified on/after date, e.g. 2025-01-01
      --all                  fetch every page of results
      --author string        original creator (CQL: creator =)
      --before string        modified on/before date, e.g. 2025-12-31
      --contributor string   any contributor (CQL: contributor =)
  -h, --help                 help for search
      --label string         label (CQL: label =)
      --limit int            page size (default from config)
      --space string         space key (CQL: space =)
      --text string          free-text match (CQL: text ~)
      --type string          content type: page, blogpost, comment, attachment
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

* [confluence-cli](confluence-cli.md)	 - Use a Confluence instance as a knowledge base for coding agents


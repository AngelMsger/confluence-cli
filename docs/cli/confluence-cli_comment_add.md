## confluence-cli comment add

Post a comment on a page

### Synopsis

Post a footer comment on a page. Use --parent to reply to an existing
comment.

```
confluence-cli comment add <id|url> [flags]
```

### Examples

```
  confluence-cli comment add 123456 --body "Looks good to me."

  # reply to a comment, reading the body from stdin
  echo "Agreed." | confluence-cli comment add 123456 --parent 789 --body-file -
```

### Options

```
      --body string        comment body text
      --body-file string   read body from a file ('-' for stdin)
      --format string      body format: storage or wiki (default "storage")
  -h, --help               help for add
      --parent string      parent comment ID, to post a reply
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

* [confluence-cli comment](confluence-cli_comment.md)	 - Read and post page comments


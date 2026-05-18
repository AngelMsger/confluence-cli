## confluence-cli

Use a Confluence instance as a knowledge base for coding agents

### Synopsis

confluence-cli reads Confluence pages, searches via CQL, creates and
edits pages, and manages comments. It supports Confluence Cloud and
Data Center / Server, and emits agent-friendly JSON with structured errors.

### Options

```
      --base-url string      Confluence site URL (overrides config)
      --config string        config directory (default ~/.confluence)
      --fields string        comma-separated dot-path fields to keep
      --flavor string        backend flavor: cloud, datacenter or auto
  -f, --format string        output format: json, table or ndjson
  -h, --help                 help for confluence-cli
      --timeout string       request timeout, e.g. 30s
      --use-context string   use a named context for this invocation
  -v, --verbose              verbose diagnostics on stderr
```

### SEE ALSO

* [confluence-cli attachment](confluence-cli_attachment.md)	 - List and download page attachments
* [confluence-cli auth](confluence-cli_auth.md)	 - Inspect and manage stored credentials
* [confluence-cli comment](confluence-cli_comment.md)	 - Read and post page comments
* [confluence-cli config](confluence-cli_config.md)	 - Manage confluence-cli configuration
* [confluence-cli doctor](confluence-cli_doctor.md)	 - Diagnose configuration, credentials and connectivity
* [confluence-cli page](confluence-cli_page.md)	 - Read and write Confluence pages
* [confluence-cli search](confluence-cli_search.md)	 - Search pages with CQL or filter flags
* [confluence-cli skill](confluence-cli_skill.md)	 - Install the companion Skill for coding agents (Claude Code, Codex)
* [confluence-cli space](confluence-cli_space.md)	 - List and inspect Confluence spaces
* [confluence-cli version](confluence-cli_version.md)	 - Print version information


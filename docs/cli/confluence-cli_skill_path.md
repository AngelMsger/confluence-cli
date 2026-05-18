## confluence-cli skill path

Print where the Skill would be installed, and whether it is

```
confluence-cli skill path [flags]
```

### Options

```
      --agent strings   limit to specific agents (claude-code, codex)
      --dir string      explicit skills base directory
  -h, --help            help for path
      --project         use the project skills directories instead of $HOME
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

* [confluence-cli skill](confluence-cli_skill.md)	 - Install the companion Skill for coding agents (Claude Code, Codex)


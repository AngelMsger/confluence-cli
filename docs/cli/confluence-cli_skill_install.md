## confluence-cli skill install

Deploy the embedded Skill into a coding agent's skills directory

### Synopsis

Write the companion `confluence` Skill — bundled inside this binary —
into a coding agent's skills directory. With no flags it probes for
installed agents (Claude Code, Codex) and installs into each one found.
Re-run it after upgrading the CLI to refresh the Skill to the matching
version.

```
confluence-cli skill install [flags]
```

### Options

```
      --agent strings   target agents instead of auto-detecting (claude-code, codex)
      --dir string      explicit skills base directory; installs into <dir>/confluence
  -h, --help            help for install
      --project         install into the project (./.claude/skills, ./.agents/skills) instead of $HOME
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


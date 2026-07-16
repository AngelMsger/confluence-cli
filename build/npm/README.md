# @angelmsger/confluence-cli

npm distribution of [`confluence-cli`](https://github.com/AngelMsger/confluence-cli)
— a command-line tool that reads, searches and maintains Confluence pages
from the terminal, built for coding agents (Claude Code and others) and humans
alike. Supports Confluence Cloud and Data Center / Server.

```bash
npm install -g @angelmsger/confluence-cli
confluence-cli config init --pretty       # interactive TUI: server URL + credentials
confluence-cli skill install              # deploy the companion agent Skill
confluence-cli search --text "runbook"    # find the page you need
```

Installing this package downloads the prebuilt binary for your platform from the
matching GitHub Release and verifies its SHA-256 checksum. If your npm setup
disables install scripts, the binary is fetched on first run instead.

The companion `confluence` Skill for coding agents is embedded in the binary;
`confluence-cli skill install` deploys a copy that always matches the installed
CLI version.

See the [project README](https://github.com/AngelMsger/confluence-cli) and the
[installation guide](https://github.com/AngelMsger/confluence-cli/blob/main/docs/installation.md)
for full documentation.

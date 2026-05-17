# confluence-cli

![confluence-cli — use Confluence as a knowledge base from your terminal](docs/image.png)

A command-line tool that lets coding agents (Claude Code and others) use a
Confluence instance as an external knowledge base. It reads pages, searches with
CQL, browses spaces and page trees, and reads or posts comments.

- Supports **Confluence Cloud** and **Confluence Data Center / Server**.
- **Agent-friendly**: JSON output by default, structured errors with recovery
  hints, and partial page reads (`outline` / `section` / `keyword`) so an agent
  spends minimal context.
- Configurable via CLI flags, environment variables, a `.env` file, a YAML
  config file, or an interactive `config init` wizard.
- Ships with a companion `confluence` Skill (`skills/confluence/`).

## Install

```bash
npm install -g @angelmsger/confluence-cli                                   # npm
go install github.com/angelmsger/confluence-cli/cmd/confluence-cli@latest   # go
make install                                                                # from source
```

Or download a prebuilt binary from the
[Releases page](https://github.com/angelmsger/confluence-cli/releases). Then
enable shell completion and install the companion Skill — see the full
[installation & setup guide](docs/installation.md).

## Quick start

```bash
confluence-cli config init     # interactive setup
confluence-cli doctor          # verify configuration and connectivity

confluence-cli search --text "release process"
confluence-cli page get <id|url> --scope outline
confluence-cli page get <id|url> --scope section --section sec-2
confluence-cli comment list <id|url>
```

## Configuration

Settings resolve in precedence order (highest first): CLI flags → environment
variables (`CONFLUENCE_*`) → `.env` → `~/.confluence/config.yaml` → defaults.
See `.env.example` and `skills/confluence/references/getting-started.md`.
Secrets are stored in the OS keychain (with a `0600` file fallback) and never
written to the config file.

## Commands

| Command | Purpose |
|---------|---------|
| `page get` | fetch a page; render body with `--scope`/`--detail`/`--as` |
| `page children` / `page descendants` | browse the page tree |
| `search` | CQL search, raw or built from `--text`/`--author`/`--space`/... |
| `space list` / `space get` | inspect spaces |
| `comment list` / `comment add` | read or post comments (`add` is the only write) |
| `attachment list` / `attachment download` | inspect and fetch attachments |
| `config` / `auth` / `doctor` / `version` | setup and diagnostics |

## Shell completion

`confluence-cli` completes subcommands, enum flag values (`--format`, `--flavor`,
`--scope`, `--detail`, `--as`, `--type`, ...) and live space keys for
`space get <key>`. Each shell needs the completion script loaded once:

```bash
source <(confluence-cli completion bash)     # bash, current shell
confluence-cli completion zsh > "${fpath[1]}/_confluence-cli"   # zsh, persistent
```

See [docs/installation.md](docs/installation.md#2-enable-shell-completion) for
every shell (bash / zsh / fish / PowerShell) and persistent setup.

## Companion Skill

The `confluence` Skill in [`skills/confluence/`](skills/confluence) teaches a
coding agent how to drive this CLI. Install it with the
[`skills`](https://github.com/vercel-labs/skills) tool:

```bash
npx skills add angelmsger/confluence-cli --skill confluence       # this project
npx skills add angelmsger/confluence-cli --skill confluence -g    # all projects
npx skills update confluence                                   # refresh after a CLI upgrade
```

No Node? `make install-skill` copies it into `~/.claude/skills/`. Full
instructions, including how to keep the Skill in sync with CLI updates, are in
[docs/installation.md](docs/installation.md#3-install-the-companion-skill).

## Development

```bash
make test       # unit + integration tests
make e2e        # build + run against an in-repo mock Confluence server
make e2e-live   # additionally run read-only checks against the real server
make lint       # gofmt + go vet
```

See `docs/technical-design.md` for the architecture and `internal/` package
layout.

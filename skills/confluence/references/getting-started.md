# Getting started

Before any Confluence command works, `confluence-cli` needs a server URL and a
credential.

## Check the current state

```bash
confluence-cli doctor
```

`doctor` runs three checks — configuration, credentials, connectivity — and
prints a JSON report. If `healthy` is `true`, you are ready. Otherwise each
failing check's `detail` explains what to fix.

```bash
confluence-cli auth status   # is a usable credential resolvable?
confluence-cli config show   # the resolved, non-secret configuration
confluence-cli config show --explain   # ...annotated with each value's source
```

## Configuration sources

Settings are resolved in this precedence order (highest first):

1. CLI flags (`--base-url`, `--flavor`, `--format`, `--timeout`)
2. Environment variables (`CONFLUENCE_*`)
3. A `.env` file in the working directory
4. `~/.confluence/config.yaml`
5. Built-in defaults

Key environment variables:

| Variable | Meaning |
|----------|---------|
| `CONFLUENCE_SERVER` | Site URL |
| `CONFLUENCE_FLAVOR` | `cloud`, `datacenter` or `auto` |
| `CONFLUENCE_PERSONAL_ACCESS_TOKEN` | Data Center PAT (Bearer auth) |
| `CONFLUENCE_USERNAME` + `CONFLUENCE_PASSWORD` | Basic auth (Data Center) |
| `CONFLUENCE_USERNAME` + `CONFLUENCE_API_TOKEN` | Basic auth (Cloud) |
| `CONFLUENCE_CONTEXT` | Select a named context (multi-server setups) |

## Interactive setup

For a human running the CLI in a terminal, point them at the TUI:

```bash
confluence-cli config init --pretty
```

The plain `config init` form (no flag) is line-by-line — use it from
scripts, dotfiles bootstrap, and non-TTY environments where the TUI
cannot render. Both forms ask for the server URL, detect the flavor,
collect a credential, validate it live, and store the secret in the OS
keychain (falling back to a `0600` file). Non-secret settings go to
`~/.confluence/config.yaml`; secrets are never written there.

> **Cloud auth note.** On Atlassian Cloud (`*.atlassian.net`) the auth
> scheme must be `basic` with the user's Atlassian email + an API token
> from id.atlassian.com — the wizard defaults to this when it sees a
> Cloud tenant. `pat` (Bearer) is Data Center 7.9+ only; Cloud REST
> endpoints reject Bearer with 403 even when the token itself is valid.

## Multiple servers (contexts)

Most setups use a single server and need none of this. To work with several
Confluence servers, `config init` can save them as named *contexts*:

```bash
confluence-cli config get-contexts        # list contexts; the current is marked
confluence-cli config use-context prod    # switch the persistent current context
confluence-cli --use-context prod doctor  # override the context for one command
```

The active context is chosen by, in order: the `--use-context` flag, the
`CONFLUENCE_CONTEXT` env var, the file's `current_context`. Legacy single-server
config files keep working without change.

## Flavors

- **cloud** — Confluence Cloud (`*.atlassian.net`). REST API under `/wiki`.
- **datacenter** — self-hosted Confluence Data Center / Server. REST API under
  the site root.

Leave `--flavor` unset (or `auto`) to let the CLI probe the server and decide.

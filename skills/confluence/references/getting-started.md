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

## Interactive setup

```bash
confluence-cli config init
```

The wizard asks for the server URL, detects the flavor, collects a credential,
validates it live, and stores the secret in the OS keychain (falling back to a
`0600` file). Non-secret settings go to `~/.confluence/config.yaml`; secrets are
never written there.

## Flavors

- **cloud** — Confluence Cloud (`*.atlassian.net`). REST API under `/wiki`.
- **datacenter** — self-hosted Confluence Data Center / Server. REST API under
  the site root.

Leave `--flavor` unset (or `auto`) to let the CLI probe the server and decide.

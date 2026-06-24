# Agent Guide

This file orients coding agents (Claude Code and others) working in this
repository. It is intentionally short — the real guidance lives elsewhere.

## Start here

1. Read [`CONTRIBUTING.md`](CONTRIBUTING.md) first. It covers the project
   structure, the build/test/lint commands, coding and testing conventions, and
   the commit/PR expectations every change must follow.

2. Then read, **only as the task needs them**, the documents under
   [`docs/`](docs/):

   - [`docs/technical-design.md`](docs/technical-design.md) — architecture, the
     `internal/` package layout, the API-client/flavor abstraction, the config
     and error models, and the rendering pipeline. Read before changing core
     behavior.
   - [`docs/installation.md`](docs/installation.md) — install methods, shell
     completion, and the companion Skill. Read for distribution/UX changes.
   - [`docs/releasing.md`](docs/releasing.md) — versioning, the changelog step,
     tagging, and the release/CI workflows. Read before cutting a release or
     touching `.github/workflows/`.

Pull in a document when the task touches its area; do not read all of `docs/`
up front.

## Ground rules

- Run `make test` and `make e2e` before claiming a change is complete.
- Keep commits scoped to one logical change; follow the commit and PR
  conventions in `CONTRIBUTING.md`.
- Never commit `.env`, credentials, tokens, or build artifacts.

## Cloud / Data Center parity — touch both branches, always

This CLI targets **two backends** (Confluence Cloud and Data Center / Server).
Writes go through the v1 REST API on both, but the flavors still diverge on
endpoints, pagination, and field shapes — and the recurring bug class is a
branch (often Cloud) honouring an input the other branch silently drops. A real
example: Cloud user search ignored the page cursor and capped at the first page
while the DC branch paged correctly.

When you touch any path that branches on `c.flavor`:

1. **Handle both branches or consciously diverge** — every input the command
   accepts (filters, cursors, version, expand) must be consumed by both flavor
   branches, or the gap must be deliberate and documented. Compiling proves
   nothing; a branch that never reads a field is silently wrong.
2. **Verify pagination + optimistic-lock symmetry.** If one branch returns a
   `next` cursor or sends a `version`, the other almost certainly must too.
3. **Add a test that exercises both flavors** for the changed path so a
   one-sided change is caught.

## Discoverability — no dead-end inputs

**Every non-trivial identifier a command accepts as input must be discoverable
through another command in this CLI.** Examples of inputs covered by this
rule: space keys, page IDs, comment IDs, attachment IDs, label names, version
numbers, user identifiers (accountId on Cloud, username on Data Center), and
file paths within an attachment.

The rule is symmetric: an input is *only* discoverable if (a) the CLI has a
command that lists / searches values of that kind, **and** (b) that listing
command itself is reachable without already knowing some other value the CLI
can't tell you about. A `comment list` that demands a page ID is only useful
if `search` (or another path-to-page command) also exists.

When you add a command or flag that takes a new kind of input:

1. Walk every parameter the new surface accepts. For each, answer:
   *"Where does the caller (especially an AI agent) get this value?"*
2. If the answer is **another command in this CLI**, you are done.
3. If the answer is **an existing identifier the user already had in hand**
   (e.g. they pasted a Confluence page URL), that counts too — `pkg/urlref`
   already parses page / comment IDs out of URLs.
4. If the answer is **out-of-band** (a web UI, another tool, the API
   directly), that is a gap. Add the missing discovery command in the same
   PR, or surface it as a follow-up issue and document the dead end in
   `CHANGELOG.md` under *Known gaps*.

The same rule applies to error messages. When a command rejects an
invocation for missing a required identifier, the resulting `CLIError`'s
`next_steps` must include the discovery command (e.g. `space list`,
`search`, `comment list`, `attachment list`). Errors that say
"Pass `--space <key>`" without showing the user how to find a valid `<key>`
are defects.

The e2e harness should exercise this contract: when adding a "missing
input" error path, also add a `scripts/e2e.sh` assertion that the error
output contains the discovery command's name (use a stderr-capturing helper
when needed, since structured errors are written to stderr).

## Safety modes — `--dry-run` and read-only posture

Two orthogonal protections guard every operation that mutates remote state:

1. **`--dry-run`** is a per-command flag on every mutating command. It must
   resolve the request via `Client.DescribeWrite(ctx, op)` and emit the
   resulting `WriteRequestPlan{Method, URL, Payload}` instead of sending the
   HTTP request. Use the shared `emitDryRun(s, client, ctx, op)` helper —
   never re-implement the dispatch inline. Every write request type that
   reaches a command must also have a `DescribeWrite` case; the read path
   (build helper) must be the same code path the live write uses so the
   preview cannot drift from the actual request.
2. **Read-only posture** is a session-level switch: `defaults.read_only`
   in the config file, or `CONFLUENCE_CLI_READ_ONLY` in the environment.
   When active, `appState.newClient()` wraps the client in
   `apiclient.NewReadOnly(...)`, which returns a structured
   `READONLY_BLOCKED` (`category=permission`) error from every mutating
   method *before* any HTTP request is sent. `--allow-writes` (root
   persistent flag) overrides the posture for one invocation.

When you add a new mutating method on `Client`:

- Add the method override on `readOnlyClient` in `internal/apiclient/readonly.go`
  so the wrapper actually blocks it.
- Add a `DescribeWrite` case + a `--dry-run` branch on the calling command.
- Add an e2e assertion for both the `--dry-run` happy path and the
  `READONLY_BLOCKED` rejection (see `scripts/e2e.sh`).
- Add a row to the wrapper's table test in
  `internal/apiclient/readonly_test.go`.

`--dry-run` must *not* be blocked by read-only mode — `DescribeWrite` sends
no HTTP and is the right tool to inspect what a write would look like under
a read-only session. The wrapper intentionally does not override it.

## Documentation — keep it current

- **Actively maintain the docs.** When a change affects architecture,
  installation, commands, flags, or the release process, update the relevant
  file under [`docs/`](docs/) in the same commit. Stale docs are a defect.
- **This includes the GitHub Pages site.** [`docs/index.html`](docs/index.html)
  is the published landing page (served at
  <https://angelmsger.github.io/confluence-cli/>) and
  `.github/workflows/pages.yml` redeploys it on every push to `main` that
  touches `docs/`. When commands, the feature
  list, or install instructions change, update `docs/index.html` to match — do
  not let the landing page drift from the README and the CLI.

## Changelog & versioning — required

- **Actively maintain [`CHANGELOG.md`](CHANGELOG.md).** Whenever a change is
  user-facing (a flag, command, output, behavior, or bug fix), add an entry to
  the `[Unreleased]` section in the same commit — do not leave it for later.
- **If you bump the version, you must tag the commit.** "Bumping the version"
  means renaming `[Unreleased]` in `CHANGELOG.md` to the new version with
  today's date and updating `build/npm/package.json`. The CLI's own version is
  derived from the git tag via `-ldflags`, so a version bump is not real until
  the commit carrying it is tagged:

  ```bash
  git tag vX.Y.Z <commit>
  git push origin vX.Y.Z
  ```

  See [`docs/releasing.md`](docs/releasing.md) for the full release procedure.

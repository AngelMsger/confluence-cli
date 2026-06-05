# Changelog

All notable changes to `confluence-cli` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.0] - 2026-06-05

### Changed

- **`auth login` fails fast without a TTY.** Rather than blocking on the secret
  prompt when stdin is not an interactive terminal (a sandboxed agent, CI without a
  PTY), it now returns a structured `AUTH_LOGIN_NEEDS_TTY` error that points at the
  non-interactive paths — run it in a real terminal, or supply `CONFLUENCE_*`
  credentials via the environment.
- **`--pretty` clarified as human-only.** The flag help now states it is for
  interactive terminal use and that agents/scripts should omit it, and error
  `next_steps` / hints no longer suggest `config init --pretty` — plain
  `config init` is the non-TTY-safe form.

### Skill

- AI attribution guidance for agent writes: mark AI-authored pages (Info macro for
  `storage` bodies, a markdown callout for markdown/wiki bodies) and comments
  (storage XHTML `<a>` anchor) with a link back to `confluence-cli`, written in the
  user's language.
- New "For agents and sandboxes" guidance: reuse the user's existing config and
  credentials, request elevation rather than giving up or re-initializing inside a
  sandbox, and never run interactive `config init` / `auth login` or pass
  `--pretty`. Skill bumped to `1.7.0`.

## [0.7.0] - 2026-05-28

### Changed

- **Default config location moved to `~/.angelmsger/confluence/`.** New
  installs and `config init` now write `config.yaml` (and the
  credentials fallback file) under `~/.angelmsger/confluence/`, grouping
  every angelmsger CLI under one shared dotfile root. The legacy
  `~/.confluence/` directory is still honored — if it has a
  `config.yaml` and the new location does not, the CLI reads and writes
  there as before, so existing installations keep working without a
  migration step. To migrate manually:
  `mkdir -p ~/.angelmsger && mv ~/.confluence ~/.angelmsger/confluence`.
  Keychain entries are unaffected (the service key has not changed).

## [0.6.0] - 2026-05-27

### Added

- **Read-only mode.** A session-level safety switch that blocks every
  mutating client method before any HTTP request is sent. Enable it via
  `defaults.read_only: true` in `~/.confluence/config.yaml` or
  `CONFLUENCE_CLI_READ_ONLY=1` in the environment. Blocked writes return a
  structured `READONLY_BLOCKED` error (`category=permission`, exit code 5)
  whose `next_steps[0]` is `--allow-writes`. The new root-level
  `--allow-writes` persistent flag overrides the posture for a single
  invocation, so `CONFLUENCE_CLI_READ_ONLY=1 confluence-cli --allow-writes
  page delete <id> --yes` is the documented escape hatch. `--dry-run`
  remains usable under read-only — it sends no HTTP, so the wrapper passes
  `DescribeWrite` through unchanged.
- `confluence-cli user search` / `user get` / `user me` close the
  discoverability gap for `search --author` and `search --contributor`.
  Cloud uses the CQL-driven `/wiki/rest/api/search/user?cql=user.fullname~"..."`
  (so `--query` is required there); Data Center uses the global
  `/rest/api/1.0/users` directory (`--query` is optional). `user me`
  mirrors the top-level `whoami` inside the subtree, so a single
  `bitbucket-cli`-style mental model works across both projects.
- The `search --author` / `search --contributor` flag descriptions now
  point at `user search` as the discovery path.

### Changed

- Error `next_steps`, `--help` examples, and the long descriptions
  that point users at the setup wizard now uniformly suggest
  `confluence-cli config init --pretty`. The plain `config init`
  form is still kept as the second example for scripted / non-TTY
  setups, but the recommended path for a human at a terminal is
  the TUI wizard. Touched: the global hint defaults in
  `internal/errors/hints.go`, the `auth login` / `config init` /
  `config show` long help, `apiclient.Build`'s no-base-url error,
  `doctor`'s unhealthy `next_steps`, the README's multi-context
  section, the companion Skill's `errors-and-exit-codes.md`, and
  the regenerated `docs/cli/` reference.

## [0.5.1] - 2026-05-25

### Changed

- Documentation now steers human users toward
  `config init --pretty` (the TUI wizard) as the recommended
  first-time setup, with the plain line-by-line `config init` called
  out as the form to use from scripts and non-TTY environments.
  README, the installation guide, the landing page, the npm
  package README, and the companion Skill's getting-started guide
  all reflect this. The companion guide also gained a short note
  explaining that Cloud must use `basic` auth (email + API token) —
  Cloud has no Bearer-style PAT.
- `config show --explain` now annotates `auth.scheme` and `auth.user`
  with their source (e.g. `pat (from env)`, `basic (from file)`).
  Previously these fields were emitted without provenance, so
  env-variable inference — most often `CONFLUENCE_PERSONAL_ACCESS_TOKEN`
  silently forcing `auth.scheme` to `pat` over a Cloud context's
  `basic` — was invisible to anyone diagnosing a failed auth.
- Context-name lookup is now case-insensitive everywhere
  (`--use-context`, `CONFLUENCE_CONTEXT`, `current_context`,
  `config use-context`, `config delete-context`). The canonical
  (as-stored) name is what gets persisted and surfaced, so a CI lookup
  against a legacy mixed-case file never leaves `current_context`
  pointing at a spelling that is not in the contexts list. The wizard
  additionally lowercases new context names at write time, so
  freshly-saved configs are uniformly lowercase. Mixed-case names in
  legacy files keep working until they are re-saved.
- Email-shaped usernames (anything containing `@`) are lowercased at
  write time. Atlassian Cloud authenticates by email + API token and
  treats the address case-insensitively, so a typo like `Alice@…` no
  longer maps to a different keychain identity than `alice@…`.
  Non-email usernames (Data Center LDAP / AD identifiers, which may
  be case-sensitive server-side) are only trimmed.

### Fixed

- `config init` no longer defaults the auth scheme to `pat` for Cloud
  tenants. The wizard now defaults to `basic` whenever the explicitly
  chosen or detected flavor is Cloud — Atlassian Cloud's
  id.atlassian.com API tokens authenticate via HTTP Basic
  (`email:token`); using them as a Bearer/PAT token returns 403
  FORBIDDEN even when the token is valid. Data Center continues to
  default to `pat`. In `--pretty` mode the wizard now runs flavor
  detection between the URL/flavor question and the auth question so
  the same default applies on `auto`-detected Cloud tenants.
- `--use-context <name>` with an unknown name no longer surfaces as a
  generic `CONFIG_LOAD "failed to load configuration"`. The underlying
  `UNKNOWN_CONTEXT` error from the loader was being blanket-wrapped by
  `appState.load()`, which stripped its code and hint. The wrapper now
  passes structured `*CLIError` values through untouched, so callers
  see the real reason and the recovery hint.
- The `UNKNOWN_CONTEXT` hint is now actionable. It lists every
  available context inline (`Available contexts: alpha, beta.`), so
  users do not have to run a second command to recover from a typo or
  an unset `current_context`. (The historical "Did you mean X?"
  case-mismatch suggestion is now mostly a defensive belt — see the
  case-insensitive lookup item above.)

## [0.5.0] - 2026-05-24

### Added

- New global `--pretty` flag opts a human user into a richer UX without
  changing the default Agent-friendly behaviour. When set:
  - `config init` runs a `charmbracelet/huh`-based interactive TUI with
    arrow-key selection, password masking, placeholder examples, and
    Shift-Tab back-navigation across the whole form. The wizard first asks
    whether to edit an existing context, add a new one, or replace the
    configuration, and pre-fills fields with the chosen context's stored
    values so pressing Enter keeps them. The historical line-by-line
    prompt remains the default and is used for non-TTY scripted setup.
  - JSON, NDJSON, and error output written to a terminal is ANSI-colored
    via `neilotoole/jsoncolor`. When stdout is a pipe (`… | jq`,
    redirection, CI) the output silently falls back to plain JSON so
    consumers see byte-identical bytes.
- `config init` now detects an existing config and offers to edit a
  specific context, add another, or replace the file outright (previously
  it would only initialize a fresh `default` context).
- `config init` now offers "press Enter to keep the current value" on the
  secret prompt when editing an existing context whose auth scheme has
  not changed.

### Fixed

- `config init --pretty` no longer appends an empty-named context when the
  config file holds exactly one context but omits `current_context` (e.g. a
  hand-edited or migrated file). The huh wizard now mirrors the plain
  wizard's fallback to the sole context's name; `config init` additionally
  refuses to persist any context with an empty name as a defensive depth.
- Confluence Cloud flavor auto-detection on `*.atlassian.net` tenants no
  longer fails when the tenant's REST endpoints 302-redirect anonymous
  requests to the Atlassian SSO login page (the redirected HTML response
  used to be rejected by the JSON-only probe). Detection now short-circuits
  to Cloud on any `*.atlassian.net` host and additionally probes the
  unauthenticated `_edge/tenant_info` sentinel before falling back to the
  REST endpoints.

### Changed

- `--pretty` is refused with a structured `PRETTY_NEEDS_TTY` error when
  stdin is not an interactive terminal, so the user does not silently
  fall through to a UX they did not ask for.
- `config init` now persists in a safer order: every credential is
  saved into the keychain (or file fallback) first, then `config.yaml`
  is written, and only after both succeed are orphaned old credentials
  (from a context whose base URL or auth scheme changed) cleaned up.
  Previously a credential-store or write failure could leave a
  half-written config pointing at a nonexistent credential, or delete
  the credential still referenced by an unchanged config. The worst
  case now is an orphan secret in the keychain — harmless storage,
  never a broken auth.

## [0.4.0] - 2026-05-19

### Added

- `page get --as raw` emits the page body's untouched source — storage-format
  XHTML, or server-rendered HTML with `--body-format view` — with no
  markdown/text rendering. Use it to inspect macros, round-trip-edit a page or
  debug. It requires `--scope full`.
- `page get` now reports a `render_notes` field when markdown/text rendering
  drops or degrades content (macros without a native rendering, images shown
  as placeholders). Rendering loss was previously silent; when `render_notes`
  is present, re-read with `--as raw` for the full source.

## [0.3.1] - 2026-05-19

### Fixed

- Page and comment writes against Confluence Data Center no longer fail with a
  spurious `parse` / `DECODE` error. A write response expands the `container`
  object with a numeric `id`, but the client expected a string — so a write
  that had actually succeeded was reported as a failure and the new page's
  id/url was lost. Container IDs are now decoded loosely (string or number),
  matching how space IDs were already handled.
- Decode failures now surface the underlying parser error and a snippet of the
  response body instead of an opaque "failed to decode server response", so a
  response-shape mismatch is diagnosable rather than a dead end.

## [0.3.0] - 2026-05-19

### Changed

- **Breaking:** list commands (`search`, `page children`/`descendants`/`history`,
  `comment list`, `attachment list`, `label list`, `space list`) now emit a
  `{items, next, has_more}` envelope instead of a bare array. A new `--cursor`
  flag resumes from a prior page's `next`, so an agent can page deterministically
  without `--all`.
- **Breaking:** the body-format flag on `page create`/`update` and
  `comment add`/`update` was renamed from `--format` to `--body-format`; it no
  longer shadows the global `--format` (json/table/ndjson) output flag.
- **Breaking:** `config path`/`use-context`/`delete-context`, `config init`,
  `auth login`/`logout` and `skill install`/`uninstall`/`path` now emit JSON on
  stdout like every other command; interactive prompts moved to stderr.

### Fixed

- **Security:** `comment` and `attachment` commands no longer mis-resolve a
  page URL to a page ID. Passing a plain page URL to `comment delete` /
  `attachment delete` could previously delete the wrong content; comment
  commands now read `focusedCommentId` from a comment permalink and reject
  ambiguous page URLs, and attachment commands require a bare attachment ID.

## [0.2.0] - 2026-05-18

### Added

- Comment write commands: `comment update` edits a footer comment's body and
  `comment delete` removes one. `update` supports `--dry-run`; `delete`
  supports `--dry-run` and requires `--yes`.
- `whoami` prints the user the configured credentials authenticate as, and
  `doctor` now reports that user as an informational `current-user` check.
- Page version history: `page history` lists a page's versions, and
  `page restore --version N` rolls a page back by republishing that version's
  body as a new version (non-destructive — the history is kept). `restore`
  supports `--dry-run`.
- Page watch commands: `page watch` / `page unwatch` subscribe or unsubscribe
  the authenticated user, and `page watch-status` reports whether you watch a
  page. `watch` / `unwatch` support `--dry-run`.
- Attachment write commands: `attachment upload` attaches a file to a page,
  `attachment update` replaces an attachment's content with a new version, and
  `attachment delete` removes one. Uploads use `multipart/form-data`; `--file -`
  reads from stdin. All three support `--dry-run`, and `delete` requires `--yes`.
- Label commands: `label list`, `label add` (one or more labels at once) and
  `label remove`. `add` and `remove` support `--dry-run`.
- A generated CLI reference. `cmd/gen-docs` renders the cobra command tree
  into a single styled HTML page (`docs/cli/index.html`, served by GitHub
  Pages with a per-module sidebar and flag/example tables) and a
  module-grouped Markdown index (`docs/cli/README.md`). Both come from the
  same command tree, so they always match `--help`; `make docs` regenerates
  them and CI fails if the committed output drifts.
- `Example` sections on the common commands, shown in both `--help` and the
  generated reference.

## [0.1.0] - 2026-05-18

### Added

- Page write commands: `page create`, `page update`, `page delete`,
  `page move` and `page copy`. Bodies accept storage-format XHTML, Confluence
  wiki markup or Markdown (`--format markdown`, converted client-side).
- Every write command supports `--dry-run`, which prints the HTTP request that
  would be sent without sending it. `page delete` additionally requires `--yes`
  (or an interactive confirmation when stdin is a terminal).
- New `conflict` error category (exit code 11) for version conflicts (HTTP 409)
  on `page update`.
- Multiple named contexts (kubectl-style). The config file can hold several
  Confluence servers; `config use-context` switches the current one,
  `config get-contexts` lists them, `config delete-context` removes one. The
  `--use-context` flag and `CONFLUENCE_CONTEXT` env var override per invocation,
  and `config init` offers to configure additional contexts. Legacy flat config
  files keep working unchanged — single-context users see no difference.

## [0.0.4] - 2026-05-18

### Added

- `skill install` now supports **Codex** alongside Claude Code. With no flags it
  probes for installed agents (`~/.claude`, `~/.codex`, or project markers) and
  installs the Skill into each one found; `--agent claude-code,codex` targets
  agents explicitly.
- `skill uninstall` removes a previously installed Skill, taking the same
  `--agent` / `--project` / `--dir` flags as `skill install`.

### Changed

- `skill path` lists every known agent's install location and status.

## [0.0.3] - 2026-05-18

### Added

- `doctor` now checks GitHub for a newer `confluence-cli` release and reports it
  in an `update` block. The check is informational only — it never changes the
  `healthy` verdict or the exit code — and can be skipped with
  `--no-update-check`.

## [0.0.2] - 2026-05-18

### Added

- `--version` flag on the root command, mirroring the `version` subcommand
  output via a shared `versionString` helper.

## [0.0.1] - 2026-05-18

Initial release.

### Added

- Flavor-agnostic Confluence client supporting both **Cloud** and
  **Data Center / Server**, with automatic backend detection.
- `page get` / `page children` / `page descendants` — fetch pages and browse
  the page tree, with partial reads (`--scope full|outline|section|keyword`)
  and detail levels (`--detail simple|with-ids|full`).
- `search` — CQL search, raw or built from `--text` / `--author` /
  `--space` / `--label` / `--type` / `--after` / `--before`.
- `space list` / `space get` — inspect spaces.
- `comment list` / `comment add` — read and post comments (`add` is the only
  write operation).
- `attachment list` / `attachment download` — inspect and fetch attachments.
- `config` / `auth` / `doctor` — layered configuration (CLI flags > env >
  `.env` > YAML > defaults), an interactive `config init` wizard, OS keychain
  credential storage with a `0600` file fallback, and connectivity diagnostics.
- Agent-friendly output: JSON by default, `table` and `ndjson` formats,
  `--fields` projection, and structured errors with categories, exit codes and
  recovery hints.
- Storage-format XHTML rendering to Markdown or plain text.
- Shell completion for bash, zsh, fish and PowerShell, including live space
  keys and enum flag values.
- Companion `confluence` Skill, embedded in the binary and deployable with
  `skill install`.
- Distribution via npm (`@angelmsger/confluence-cli`), `go install`, prebuilt
  release binaries and `make install`.

[Unreleased]: https://github.com/angelmsger/confluence-cli/compare/v0.5.1...HEAD
[0.5.1]: https://github.com/angelmsger/confluence-cli/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/angelmsger/confluence-cli/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/angelmsger/confluence-cli/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/angelmsger/confluence-cli/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/angelmsger/confluence-cli/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/angelmsger/confluence-cli/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/angelmsger/confluence-cli/compare/v0.0.4...v0.1.0
[0.0.4]: https://github.com/angelmsger/confluence-cli/compare/v0.0.3...v0.0.4
[0.0.3]: https://github.com/angelmsger/confluence-cli/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/angelmsger/confluence-cli/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/angelmsger/confluence-cli/releases/tag/v0.0.1

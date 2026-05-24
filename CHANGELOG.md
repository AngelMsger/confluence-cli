# Changelog

All notable changes to `confluence-cli` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/angelmsger/confluence-cli/compare/v0.5.0...HEAD
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

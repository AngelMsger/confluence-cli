# Changelog

All notable changes to `confluence-cli` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/angelmsger/confluence-cli/compare/v0.0.3...HEAD
[0.0.3]: https://github.com/angelmsger/confluence-cli/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/angelmsger/confluence-cli/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/angelmsger/confluence-cli/releases/tag/v0.0.1

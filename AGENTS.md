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

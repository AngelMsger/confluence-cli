# Releasing (maintainer guide)

`confluence-cli` is distributed from **GitHub Releases**. Everything else — the
npm package, `go install`, the `doctor` update check — points back to the
release assets, so a release is the single source of truth.

## Publishing setup

This is **already configured** — the section is kept so the setup, and the
non-obvious constraints behind it, are not lost.

- The repository is public at `github.com/AngelMsger/confluence-cli`.
- The npm account owns the `@angelmsger` scope; `@angelmsger/confluence-cli`
  exists on the registry.
- **npm publishing uses trusted publishing (OIDC)** — no long-lived token, no
  repository secret. The release workflow grants `id-token: write` and the npm
  CLI authenticates automatically. The trusted publisher is configured on
  npmjs.com → the `@angelmsger/confluence-cli` **package** → Settings → Trusted
  Publisher → **GitHub Actions**:

  | Field | Value |
  |-------|-------|
  | Organization or user | `AngelMsger` |
  | Repository | `confluence-cli` |
  | Workflow filename | `release.yml` |
  | Environment | *(leave blank)* |

  If you see *"There are security risks with this option"* while creating a
  classic automation token — that prompt is steering you here; you do not need
  a token.

### Constraints that must hold for OIDC publishing

These are subtle and each one silently breaks `npm publish`:

1. **No `registry-url` on `actions/setup-node`.** It writes an `.npmrc` with
   `_authToken=${NODE_AUTH_TOKEN}`; with no token that is an empty string, and
   npm then takes the token-auth path and skips the OIDC exchange entirely.
2. **npm ≥ 11.5.1.** Trusted publishing needs it. The workflow gets it from
   Node 24's bundled npm — *not* from an `npm install -g npm@latest` self-
   upgrade, which intermittently corrupts the install.
3. **`repository.url` casing must match the GitHub repo exactly.** Provenance
   verification compares `build/npm/package.json`'s `repository.url` against the
   repo reported by the OIDC provenance (`AngelMsger/confluence-cli`); a casing
   mismatch fails the publish with `E422`.
4. The trusted publisher must be configured on the **package**, not the
   account — npm's OIDC token exchange returns `404 package not found` when no
   per-package trusted publisher exists.

## Cutting a release

Before tagging, update [`CHANGELOG.md`](../CHANGELOG.md): rename the
`[Unreleased]` section to the new version with today's date, add a fresh empty
`[Unreleased]` heading, and update the comparison links at the bottom. Bump the
`version` field in `build/npm/package.json` to match. Commit both, then tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Pushing a `v*` tag triggers `.github/workflows/release.yml`, which:

1. runs the unit tests;
2. cross-compiles every platform via `make cross` → `dist/` (the binary version
   is taken from the git tag through `-ldflags`);
3. writes `dist/checksums.txt` (SHA-256 of every binary);
4. creates the GitHub Release for the tag with all `dist/` assets attached, or
   re-uploads the assets if the release already exists;
5. sets the npm package version to the tag (minus the `v`) and runs
   `npm publish --access public` for `@angelmsger/confluence-cli`, skipping the
   publish when that version is already on the registry.

Use an annotated tag and semantic versioning (`vMAJOR.MINOR.PATCH`).

The workflow is **idempotent**: steps 4 and 5 tolerate a partial previous run,
so if a release fails halfway you can fix the cause and re-run it — either
re-run the failed run from the Actions tab, or move the tag to the fixed commit
(`git tag -f` / delete and re-push) to trigger a fresh run.

## Continuous integration

`.github/workflows/ci.yml` runs on every push to `main` and every pull request:
`gofmt` check, `go vet`, a `docs/cli/` drift check (`go run ./cmd/gen-docs`,
then fail if the committed reference differs), `go test ./...`, and the
mock-server end-to-end suite (`scripts/e2e.sh`). The live e2e checks are not run
in CI — they require a real server and credentials.

The CLI reference under `docs/cli/` is generated from the cobra command tree
(`cmd/gen-docs`); run `make docs` after any command or flag change and commit
the result, or CI will fail.

`.github/workflows/pages.yml` publishes `docs/` (the landing page
`docs/index.html` plus the markdown guides) to GitHub Pages on every push to
`main` that touches `docs/`. Enable it once: repository Settings → Pages →
Source → **GitHub Actions**. The site is served at
<https://angelmsger.github.io/confluence-cli/>.

## Release artifact contract

The release asset names are **stable** and must not change — the npm installer
and the `doctor` release-update check both depend on them:

```
confluence-cli-darwin-amd64    confluence-cli-linux-amd64    confluence-cli-windows-amd64.exe
confluence-cli-darwin-arm64    confluence-cli-linux-arm64    confluence-cli-windows-arm64.exe
checksums.txt
```

Download URL pattern:
`https://github.com/angelmsger/confluence-cli/releases/download/v<version>/<asset>`

## Companion Skill

The `confluence` Skill is **embedded into the binary** at build time
(`//go:embed skills/confluence`, see `assets.go`), so every release ships a
Skill that matches the CLI version; users deploy it with `confluence-cli skill
install`. The Skill is also published in the git repository for the `npx skills`
workflow.

The Skill is versioned independently via the `version:` field in
`skills/confluence/SKILL.md`. Bump it whenever the Skill or its `references/`
change — see [installation.md](installation.md#3-install-the-companion-skill).

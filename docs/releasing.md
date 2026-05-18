# Releasing (maintainer guide)

`confluence-cli` is distributed from **GitHub Releases**. Everything else — the
npm package, `go install`, the auto-update feature (planned) — points back to
the release assets, so a release is the single source of truth.

## One-time setup

1. The repository must be public at `github.com/angelmsger/confluence-cli`.
2. The npm account must own the `@angelmsger` scope.
3. **npm publishing uses trusted publishing (OIDC)** — no long-lived token, no
   repository secret. The release workflow already grants `id-token: write` and
   the npm CLI authenticates automatically. Two things must be set up once:

   **a. Bootstrap the package.** Trusted publishing can only be configured on a
   package that already exists, so do the very first publish manually from your
   machine:

   ```bash
   cd build/npm
   npm login
   npm version 0.0.1 --no-git-tag-version
   npm publish --access public
   ```

   **b. Configure the trusted publisher.** On npmjs.com → the
   `@angelmsger/confluence-cli` package → Settings → Trusted Publisher → choose
   **GitHub Actions** and enter:

   | Field | Value |
   |-------|-------|
   | Organization or user | `angelmsger` |
   | Repository | `confluence-cli` |
   | Workflow filename | `release.yml` |
   | Environment | *(leave blank)* |

   From then on every `v*` tag publishes automatically with no token. If you see
   *"There are security risks with this option"* while creating a classic
   automation token — that prompt is steering you here; you do not need a token.

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
4. creates a GitHub Release for the tag with all `dist/` assets attached;
5. sets the npm package version to the tag (minus the `v`) and runs
   `npm publish --access public` for `@angelmsger/confluence-cli`.

Use an annotated tag and semantic versioning (`vMAJOR.MINOR.PATCH`).

## Continuous integration

`.github/workflows/ci.yml` runs on every push to `main` and every pull request:
`gofmt` check, `go vet`, `go test ./...`, and the mock-server end-to-end suite
(`scripts/e2e.sh`). The live e2e checks are not run in CI — they require a real
server and credentials.

`.github/workflows/pages.yml` publishes `docs/` (the landing page
`docs/index.html` plus the markdown guides) to GitHub Pages on every push to
`main` that touches `docs/`. Enable it once: repository Settings → Pages →
Source → **GitHub Actions**. The site is served at
<https://angelmsger.github.io/confluence-cli/>.

## Release artifact contract

The release asset names are **stable** and must not change — the npm installer
and the planned in-CLI auto-update feature both depend on them:

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

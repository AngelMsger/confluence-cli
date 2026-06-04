# Installation & setup guide

This guide covers three things:

1. [Installing the `confluence-cli` binary](#1-install-the-cli)
2. [Enabling shell completion](#2-enable-shell-completion)
3. [Installing & updating the companion `confluence` Skill](#3-install-the-companion-skill)

---

## 1. Install the CLI

**npm is the recommended way to install** — it downloads the prebuilt binary
for your platform, verifies its checksum, and keeps upgrades a single
`npm update -g` away. The *Other methods* below are alternatives.

### npm (recommended)

```bash
npm install -g @angelmsger/confluence-cli
```

Installing downloads the prebuilt binary for your platform from the matching
GitHub Release and verifies its SHA-256 checksum. If your npm setup disables
install scripts (`--ignore-scripts`, some pnpm setups), the binary is fetched on
first run instead.

### Other methods

Prefer not to use npm? Any of these also work.

#### go install

```bash
go install github.com/angelmsger/confluence-cli/cmd/confluence-cli@latest
```

Installs into `go env GOBIN` (or `$GOPATH/bin`). Requires Go 1.24+.

#### Prebuilt binary

Download the binary for your platform from the
[Releases page](https://github.com/angelmsger/confluence-cli/releases), verify
it against `checksums.txt`, then put it on your `PATH`:

```bash
chmod +x confluence-cli-* && mv confluence-cli-* /usr/local/bin/confluence-cli
```

#### From source

```bash
git clone https://github.com/angelmsger/confluence-cli.git && cd conflunce-cli
make install        # builds and installs into `go env GOBIN` or $GOPATH/bin
```

`make install` prints the install path. Make sure that directory is on your
`PATH`:

```bash
echo 'export PATH="$(go env GOPATH)/bin:$PATH"' >> ~/.zshrc   # or ~/.bashrc
```

Other build targets: `make build` (to `./bin/`), `make cross` (every platform
into `./dist/`).

### First-time configuration

```bash
confluence-cli config init --pretty   # interactive TUI: server URL, flavor, credentials
confluence-cli doctor                 # verify configuration and connectivity
```

The `--pretty` flag opts into a `huh`-based TUI with arrow-key selection,
masked password input, and Shift-Tab back-navigation. Without it,
`config init` runs as a plain line-by-line wizard — keep that form for
scripted setup, dotfiles bootstrap, and non-TTY environments where a TUI
cannot render.

When the server URL is on `*.atlassian.net` (Cloud), the wizard now
defaults the auth scheme to **basic** and asks for your Atlassian email
plus an API token from
[id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens).
Cloud's REST API only accepts those tokens via HTTP Basic — `pat`
(Bearer) is Data Center only and 403s on Cloud, so the wizard saves you
from picking the wrong scheme.

---

## 2. Enable shell completion

`confluence-cli` completes subcommands, enum flag values (`--format`, `--flavor`,
`--scope`, `--detail`, `--as`, `--type`, ...) and **live space keys** for
`space get <key>`.

The CLI ships the completion *logic*, but every shell needs the completion
*script* loaded once. Pick your shell below.

### bash

```bash
# try it in the current shell
source <(confluence-cli completion bash)

# make it permanent (Linux)
confluence-cli completion bash | sudo tee /etc/bash_completion.d/confluence-cli >/dev/null

# make it permanent (macOS, Homebrew bash-completion)
confluence-cli completion bash > "$(brew --prefix)/etc/bash_completion.d/confluence-cli"
```

bash needs the `bash-completion` package installed and sourced from your
`~/.bashrc`.

### zsh

```bash
# ensure compinit runs — add this to ~/.zshrc if it is not there already:
#   autoload -Uz compinit && compinit

# install the completion into a directory on $fpath
confluence-cli completion zsh > "${fpath[1]}/_confluence-cli"
```

Open a new shell afterwards. If completions still do not appear, run
`rm -f ~/.zcompdump*` and start a new shell.

### fish

```bash
confluence-cli completion fish > ~/.config/fish/completions/confluence-cli.fish
```

### PowerShell

```powershell
# current session
confluence-cli completion powershell | Out-String | Invoke-Expression

# persistent — append to your profile
confluence-cli completion powershell >> $PROFILE
```

Run `confluence-cli completion --help` for the authoritative per-shell notes.

### Verifying

After loading the script, type `confluence-cli page get x --scope ` and press
`<TAB>` — you should see `full outline section keyword`. For live space-key
completion, `confluence-cli space get <TAB>` queries the configured server
(best-effort; it shows nothing if the CLI is not configured yet).

---

## 3. Install the companion Skill

The `confluence` Skill teaches a coding agent — **Claude Code** and **Codex** —
how to drive this CLI. It is **embedded in the `confluence-cli` binary**, so
whichever way you installed the CLI — npm, `go install`, a prebuilt binary —
you already have a version-matched copy of the Skill.

### Recommended: `confluence-cli skill install`

With no flags, `skill install` **probes for installed agents** and installs the
Skill into every one it finds:

```bash
confluence-cli skill install              # auto-detect; install for each agent found
confluence-cli skill install --agent codex          # only Codex
confluence-cli skill install --agent claude-code,codex
confluence-cli skill install --project    # project dirs instead of $HOME
confluence-cli skill install --dir <path> # explicit base -> <path>/confluence

confluence-cli skill path                 # show every agent's location + status
confluence-cli skill show                 # print SKILL.md to stdout
```

Install locations per agent:

| Agent | Global (default) | Project (`--project`) |
|-------|------------------|-----------------------|
| Claude Code | `~/.claude/skills/confluence` | `./.claude/skills/confluence` |
| Codex | `~/.codex/skills/confluence` | `./.agents/skills/confluence` |

Auto-detection looks for `~/.claude` / `~/.codex` (global) or `./.claude` /
`./.agents` / `./AGENTS.md` (project). If nothing is detected, pass `--agent`
or `--dir` explicitly.

Because the Skill ships inside the binary, **updating is automatic**: upgrade
the CLI (`npm update -g @angelmsger/confluence-cli`, `go install ...@latest`,
etc.) and re-run `confluence-cli skill install` — the deployed Skill always
matches the CLI version.

### Alternative: the `skills` CLI

If you manage agent skills with the [`skills` tool](https://github.com/vercel-labs/skills)
(`npx skills`), you can install the Skill straight from the repository:

```bash
npx skills add angelmsger/confluence-cli --skill confluence       # this project
npx skills add angelmsger/confluence-cli --skill confluence -g    # all projects
npx skills add ./skills/confluence                                # local checkout
npx skills update confluence                                      # refresh later
```

Useful flags: `-a claude-code` targets a specific agent, `-y` runs
non-interactively, `--list` previews a repo's skills.

> **Maintainers:** bump `version:` in `skills/confluence/SKILL.md` on every
> change to the Skill or its `references/`, so both `confluence-cli skill show`
> and `npx skills update` report the new version.

### Removing the Skill

```bash
confluence-cli skill uninstall          # auto-detect; remove from each agent found
confluence-cli skill uninstall --agent codex
confluence-cli skill uninstall --dir <path>
npx skills remove confluence            # if installed via the skills CLI
```

`skill uninstall` takes the same `--agent` / `--project` / `--dir` flags as
`skill install`; removing a Skill that is not installed is a no-op.

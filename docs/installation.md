# Installation & setup guide

This guide covers three things:

1. [Installing the `confluence-cli` binary](#1-install-the-cli)
2. [Enabling shell completion](#2-enable-shell-completion)
3. [Installing & updating the companion `confluence` Skill](#3-install-the-companion-skill)

---

## 1. Install the CLI

Pick whichever method suits you.

### npm

```bash
npm install -g @angelmsger/confluence-cli
```

Installing downloads the prebuilt binary for your platform from the matching
GitHub Release and verifies its SHA-256 checksum. If your npm setup disables
install scripts (`--ignore-scripts`, some pnpm setups), the binary is fetched on
first run instead.

### go install

```bash
go install github.com/angelmsger/confluence-cli/cmd/confluence-cli@latest
```

Installs into `go env GOBIN` (or `$GOPATH/bin`). Requires Go 1.24+.

### Prebuilt binary

Download the binary for your platform from the
[Releases page](https://github.com/angelmsger/confluence-cli/releases), verify
it against `checksums.txt`, then put it on your `PATH`:

```bash
chmod +x confluence-cli-* && mv confluence-cli-* /usr/local/bin/confluence-cli
```

### From source

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
confluence-cli config init     # interactive: server URL, flavor, credentials
confluence-cli doctor          # verify configuration and connectivity
```

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

The repository ships a `confluence` Skill at [`skills/confluence/`](../skills/confluence)
that teaches a coding agent (Claude Code and others) how to drive this CLI.

### Recommended: the `skills` CLI

The [`skills` tool](https://github.com/vercel-labs/skills) (`npx skills`)
installs and updates agent skills for 18+ agents.

```bash
# install for the current project (-> .claude/skills/confluence)
npx skills add angelmsger/confluence-cli --skill confluence

# install globally for every project (-> ~/.claude/skills/confluence)
npx skills add angelmsger/confluence-cli --skill confluence -g

# from a specific subdirectory URL
npx skills add https://github.com/angelmsger/confluence-cli/tree/main/skills/confluence

# from a local checkout (no network)
npx skills add ./skills/confluence
```

Useful flags: `-a claude-code` targets a specific agent, `-y` runs
non-interactively, `--list` previews the skills in a repo without installing.

### Keeping the Skill up to date

The Skill and the CLI evolve together. When you upgrade the binary, refresh the
Skill so its guidance matches:

```bash
npx skills update confluence     # re-pull just this skill
npx skills update                # re-pull every installed skill
npx skills list                  # show installed skills and versions
```

`update` compares the `version` field in `SKILL.md`; this project bumps that
field whenever the Skill's content changes, so `npx skills update` reliably
picks up new guidance.

> **Maintainers:** bump `version:` in `skills/confluence/SKILL.md` on every
> change to the Skill or its `references/`, so installed copies update cleanly.

### Manual install (no Node / npx)

```bash
make install-skill                       # copies to ~/.claude/skills/confluence
# or, project-local:
mkdir -p .claude/skills
cp -R skills/confluence .claude/skills/
```

To update a manually installed Skill, re-run the same copy command (or
`make install-skill`) after pulling the latest repository changes.

### Removing the Skill

```bash
npx skills remove confluence            # if installed via the skills CLI
rm -rf ~/.claude/skills/confluence      # if installed manually
```

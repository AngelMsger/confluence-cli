# @angelmsger/confluence-cli

npm distribution of [`confluence-cli`](https://github.com/angelmsger/confluence-cli)
— a command-line tool that lets coding agents use a Confluence instance as an
external knowledge base.

```bash
npm install -g @angelmsger/confluence-cli
confluence-cli config init        # set up server URL + credentials
confluence-cli skill install      # deploy the companion agent Skill
```

Installing this package downloads the prebuilt binary for your platform from the
matching GitHub Release and verifies its SHA-256 checksum. If your npm setup
disables install scripts, the binary is fetched on first run instead.

The companion `confluence` Skill for coding agents is embedded in the binary;
`confluence-cli skill install` deploys a copy that always matches the installed
CLI version.

See the [project README](https://github.com/angelmsger/confluence-cli) and the
[installation guide](https://github.com/angelmsger/confluence-cli/blob/main/docs/installation.md)
for full documentation.

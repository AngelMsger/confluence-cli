# confluence-cli command reference

This index is generated from the CLI command tree — do not edit it by
hand; run `make docs`. The full reference, with every flag and example,
is published at <https://angelmsger.github.io/confluence-cli/cli/>.

## attachment

| Command | Description |
| --- | --- |
| [`confluence-cli attachment`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-attachment) | List and download page attachments |
| [`confluence-cli attachment download`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-attachment-download) | Download an attachment's content |
| [`confluence-cli attachment list`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-attachment-list) | List the attachments of a page |

## auth

| Command | Description |
| --- | --- |
| [`confluence-cli auth`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-auth) | Inspect and manage stored credentials |
| [`confluence-cli auth login`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-auth-login) | Store a credential for the configured server |
| [`confluence-cli auth logout`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-auth-logout) | Remove the stored credential for the configured server |
| [`confluence-cli auth status`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-auth-status) | Show whether a usable credential is configured |

## comment

| Command | Description |
| --- | --- |
| [`confluence-cli comment`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-comment) | Read and post page comments |
| [`confluence-cli comment add`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-comment-add) | Post a comment on a page |
| [`confluence-cli comment list`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-comment-list) | List the footer comments of a page |

## config

| Command | Description |
| --- | --- |
| [`confluence-cli config`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-config) | Manage confluence-cli configuration |
| [`confluence-cli config delete-context`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-config-delete-context) | Delete a context and its stored credential |
| [`confluence-cli config get-contexts`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-config-get-contexts) | List the configured contexts |
| [`confluence-cli config init`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-config-init) | Interactively set up server URL and credentials |
| [`confluence-cli config path`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-config-path) | Print the config file path |
| [`confluence-cli config show`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-config-show) | Show the resolved configuration |
| [`confluence-cli config use-context`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-config-use-context) | Switch the current context |

## doctor

| Command | Description |
| --- | --- |
| [`confluence-cli doctor`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-doctor) | Diagnose configuration, credentials and connectivity |

## page

| Command | Description |
| --- | --- |
| [`confluence-cli page`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-page) | Read and write Confluence pages |
| [`confluence-cli page children`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-page-children) | List the direct child pages of a page |
| [`confluence-cli page copy`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-page-copy) | Copy a page's title and body to a new page |
| [`confluence-cli page create`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-page-create) | Create a new page |
| [`confluence-cli page delete`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-page-delete) | Delete a page (move it to the trash) |
| [`confluence-cli page descendants`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-page-descendants) | List all descendant pages of a page |
| [`confluence-cli page get`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-page-get) | Fetch a page and render its body |
| [`confluence-cli page move`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-page-move) | Move a page under a new parent and/or space |
| [`confluence-cli page update`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-page-update) | Update a page's title and/or body |

## search

| Command | Description |
| --- | --- |
| [`confluence-cli search`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-search) | Search pages with CQL or filter flags |

## skill

| Command | Description |
| --- | --- |
| [`confluence-cli skill`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-skill) | Install the companion Skill for coding agents (Claude Code, Codex) |
| [`confluence-cli skill install`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-skill-install) | Deploy the embedded Skill into a coding agent's skills directory |
| [`confluence-cli skill path`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-skill-path) | Print where the Skill would be installed, and whether it is |
| [`confluence-cli skill show`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-skill-show) | Print the embedded SKILL.md to stdout |
| [`confluence-cli skill uninstall`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-skill-uninstall) | Remove the companion Skill from a coding agent's skills directory |

## space

| Command | Description |
| --- | --- |
| [`confluence-cli space`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-space) | List and inspect Confluence spaces |
| [`confluence-cli space get`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-space-get) | Fetch a single space by key |
| [`confluence-cli space list`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-space-list) | List spaces |

## version

| Command | Description |
| --- | --- |
| [`confluence-cli version`](https://angelmsger.github.io/confluence-cli/cli/#confluence-cli-version) | Print version information |


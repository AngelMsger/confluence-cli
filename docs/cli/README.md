# confluence-cli command reference

This reference is generated from the CLI command tree, so it always
matches `--help`. Do not edit these files by hand — run `make docs`.

- [`confluence-cli`](confluence-cli.md) — Use a Confluence instance as a knowledge base for coding agents
  - [`confluence-cli attachment`](confluence-cli_attachment.md) — List and download page attachments
    - [`confluence-cli attachment download`](confluence-cli_attachment_download.md) — Download an attachment's content
    - [`confluence-cli attachment list`](confluence-cli_attachment_list.md) — List the attachments of a page
  - [`confluence-cli auth`](confluence-cli_auth.md) — Inspect and manage stored credentials
    - [`confluence-cli auth login`](confluence-cli_auth_login.md) — Store a credential for the configured server
    - [`confluence-cli auth logout`](confluence-cli_auth_logout.md) — Remove the stored credential for the configured server
    - [`confluence-cli auth status`](confluence-cli_auth_status.md) — Show whether a usable credential is configured
  - [`confluence-cli comment`](confluence-cli_comment.md) — Read and post page comments
    - [`confluence-cli comment add`](confluence-cli_comment_add.md) — Post a comment on a page
    - [`confluence-cli comment list`](confluence-cli_comment_list.md) — List the footer comments of a page
  - [`confluence-cli config`](confluence-cli_config.md) — Manage confluence-cli configuration
    - [`confluence-cli config delete-context`](confluence-cli_config_delete-context.md) — Delete a context and its stored credential
    - [`confluence-cli config get-contexts`](confluence-cli_config_get-contexts.md) — List the configured contexts
    - [`confluence-cli config init`](confluence-cli_config_init.md) — Interactively set up server URL and credentials
    - [`confluence-cli config path`](confluence-cli_config_path.md) — Print the config file path
    - [`confluence-cli config show`](confluence-cli_config_show.md) — Show the resolved configuration
    - [`confluence-cli config use-context`](confluence-cli_config_use-context.md) — Switch the current context
  - [`confluence-cli doctor`](confluence-cli_doctor.md) — Diagnose configuration, credentials and connectivity
  - [`confluence-cli page`](confluence-cli_page.md) — Read and write Confluence pages
    - [`confluence-cli page children`](confluence-cli_page_children.md) — List the direct child pages of a page
    - [`confluence-cli page copy`](confluence-cli_page_copy.md) — Copy a page's title and body to a new page
    - [`confluence-cli page create`](confluence-cli_page_create.md) — Create a new page
    - [`confluence-cli page delete`](confluence-cli_page_delete.md) — Delete a page (move it to the trash)
    - [`confluence-cli page descendants`](confluence-cli_page_descendants.md) — List all descendant pages of a page
    - [`confluence-cli page get`](confluence-cli_page_get.md) — Fetch a page and render its body
    - [`confluence-cli page move`](confluence-cli_page_move.md) — Move a page under a new parent and/or space
    - [`confluence-cli page update`](confluence-cli_page_update.md) — Update a page's title and/or body
  - [`confluence-cli search`](confluence-cli_search.md) — Search pages with CQL or filter flags
  - [`confluence-cli skill`](confluence-cli_skill.md) — Install the companion Skill for coding agents (Claude Code, Codex)
    - [`confluence-cli skill install`](confluence-cli_skill_install.md) — Deploy the embedded Skill into a coding agent's skills directory
    - [`confluence-cli skill path`](confluence-cli_skill_path.md) — Print where the Skill would be installed, and whether it is
    - [`confluence-cli skill show`](confluence-cli_skill_show.md) — Print the embedded SKILL.md to stdout
    - [`confluence-cli skill uninstall`](confluence-cli_skill_uninstall.md) — Remove the companion Skill from a coding agent's skills directory
  - [`confluence-cli space`](confluence-cli_space.md) — List and inspect Confluence spaces
    - [`confluence-cli space get`](confluence-cli_space_get.md) — Fetch a single space by key
    - [`confluence-cli space list`](confluence-cli_space_list.md) — List spaces
  - [`confluence-cli version`](confluence-cli_version.md) — Print version information

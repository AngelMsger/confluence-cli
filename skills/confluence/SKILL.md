---
name: confluence
version: 1.7.2
description: "Use a Confluence wiki as an external knowledge base. Search, read and summarise pages, browse spaces and page trees, create/update/delete/move/copy pages, view history and restore versions, read and post/edit/delete comments, manage attachments and page labels, and watch pages. Every mutating command accepts --dry-run, and a session read-only posture (defaults.read_only / CONFLUENCE_CLI_READ_ONLY=1, overridable via --allow-writes) blocks writes before they leave the CLI. Use this skill when the user gives a Confluence page URL or ID or mentions a Confluence/wiki page; asks to find, read, summarise or extract a page; browse a space or list child pages; create/update/delete/move/copy a page; view history or restore a version; read or post/edit/delete a comment; upload/replace/delete an attachment; add/remove labels; watch/unwatch a page; check which Confluence user they are; or wants a dry-run / read-only / safe-mode session. Works with both Confluence Cloud and Data Center / Server."
metadata:
  requires:
    bins: ["confluence-cli"]
  cliHelp: "confluence-cli --help; confluence-cli page get --help; confluence-cli search --help"
---

# confluence

`confluence-cli` reads and writes a Confluence instance for you. Output is JSON
by default; errors are JSON on stderr with a `category`, a `hint` and
`next_steps`.

## Golden rule — resolve to an ID first

Confluence operations act on a **page ID**. If the user gives a URL, pass it
directly: every command accepts a page URL or a bare ID as its argument and
resolves the ID itself. If the user gives only a *name* or *topic*, do **not**
guess an ID — run `confluence-cli search` first, then act on the ID from the hit.

## Decision tree

- User gave a **page URL or ID** and wants its content → `page get` (see
  [reading-pages.md](references/reading-pages.md) to pick a `--scope`).
- User describes a topic / keywords but no ID → `confluence-cli search`
  (see [searching-cql.md](references/searching-cql.md)), then `page get`.
- User wants the structure under a page → `page children` / `page descendants`.
- User wants to **create, edit, delete, move or copy** a page → `page create` /
  `update` / `delete` / `move` / `copy` (see [writing-pages.md](references/writing-pages.md)).
- User wants a page's **version history**, or to **roll back** a page →
  `page history` / `page restore --version N` (see
  [writing-pages.md](references/writing-pages.md)).
- User wants to **watch / unwatch** a page, or check if they watch it →
  `page watch` / `page unwatch` / `page watch-status`.
- User wants the **comments** on a page → `comment list`; to post one →
  `comment add`; to edit or delete one → `comment update` / `comment delete`
  (see [comments.md](references/comments.md)).
- User asks **who they are** / which Confluence account is in use → `whoami`.
- User wants files on a page → `attachment list` / `attachment download`; to
  put a file on a page → `attachment upload`; to replace one → `attachment
  update`; to remove one → `attachment delete` (see
  [attachments.md](references/attachments.md)).
- User wants to tag / categorise a page → `label list` / `label add` /
  `label remove`.
- A command fails → read the JSON error on stderr and follow `next_steps`
  (see [errors-and-exit-codes.md](references/errors-and-exit-codes.md)).
- Nothing is configured yet → [getting-started.md](references/getting-started.md).

## Commands

```
confluence-cli page get <id|url>          # fetch + render a page body
confluence-cli page children <id|url>     # direct child pages
confluence-cli page descendants <id|url>  # all descendant pages
confluence-cli page create                # create a page (--space --title)
confluence-cli page update <id|url>       # edit a page's title / body
confluence-cli page delete <id|url>       # trash a page (needs --yes)
confluence-cli page move <id|url>         # reparent / move to another space
confluence-cli page copy <id|url>         # shallow-copy a page
confluence-cli page history <id|url>      # list a page's version history
confluence-cli page restore <id|url>      # restore an old version (--version N)
confluence-cli page watch <id|url>        # watch / unwatch / check watch status
confluence-cli search [cql]               # CQL search, or use --text/--author/...
confluence-cli space list                 # list spaces
confluence-cli space get <key>            # one space
confluence-cli comment list <id|url>      # page comments
confluence-cli comment add <id|url>       # post a comment
confluence-cli comment update <id|url>    # edit a comment's body
confluence-cli comment delete <id|url>    # delete a comment (needs --yes)
confluence-cli attachment list <id|url>   # page attachments
confluence-cli attachment download <id>   # download an attachment
confluence-cli attachment upload <id|url> # attach a file (--file)
confluence-cli attachment update <id>     # replace an attachment's content
confluence-cli attachment delete <id>     # delete an attachment (needs --yes)
confluence-cli label list <id|url>        # labels on a page
confluence-cli label add <id|url> <l>...  # add labels to a page
confluence-cli label remove <id|url> <l>  # remove a label from a page
confluence-cli config init|show           # configuration
confluence-cli auth status                # credential check
confluence-cli whoami                      # the user the credentials act as
confluence-cli doctor                     # diagnose setup + connectivity
```

## Reading efficiently — do not slurp whole pages

`page get` defaults to the full body. For anything but a short page, read in
stages so you spend the least context:

1. `page get <id> --scope outline` — see the section headings and their IDs.
2. `page get <id> --scope section --section <sec-N>` — read just the section
   you need.
3. `page get <id> --scope keyword --keyword "<term>"` — when you only have a
   fuzzy term, get matching blocks plus their heading.

Only fall back to `--scope full` when the whole page is genuinely needed.
[reading-pages.md](references/reading-pages.md) has the full decision tree.

## Large result sets

`search`, `page children/descendants`, `page history`, `comment list`,
`attachment list` and `label list` return a `{items, next, has_more}` envelope.
By default they return one page; when `has_more` is true, pass `--cursor` with
the `next` value to read the following page. Use `--all` to fetch every page in
one call, or `--limit N` to size each request. For very large outputs use
`--format ndjson` (one JSON object per line, items only).

## AI attribution (agent writes)

When you, as an AI agent, write to Confluence on the user's behalf, mark the content
as AI-authored with a link back to the tool. This applies **only** to agent-driven
writes — `page create` / `page update` and `comment add` — never to anything a human
authored directly. Include the marker exactly once per page/comment.

- **Pages** — prepend a one-line banner at the top of the body. With `--body-format
  storage`, use an Info macro; with markdown/wiki, a leading callout line. See
  [writing-pages.md](references/writing-pages.md).
- **Comments** — prefix the body with a clickable **`[AI]`** tag. Comment bodies are
  storage XHTML by default, so use an `<a>` anchor with literal brackets in its text —
  `<a href="url">[AI]</a>`, **not** `[AI](url)`. See
  [comments.md](references/comments.md).

Write the attribution sentence in the **same language as the content** (the user's
language); keep the plain-ASCII `[AI]` marker (never an emoji — some Data Center
databases can't store 4-byte characters), the URL
`https://angelmsger.github.io/confluence-cli/`, and the `confluence-cli` label constant.

## Configuration & credentials (agents)

The user has normally already configured `confluence-cli`. **Reuse their existing
config and credentials** from `~/.angelmsger/confluence/config.yaml` + the OS keychain
— do not run `config init` to create a fresh setup, and never pass `--pretty` (a
human-only flag for the interactive TUI / colorized JSON; it errors without a TTY and
agents never need it).

If you run inside a **sandbox** that cannot read the user's home or keychain you'll
see a `config` (3) or `auth` (4) error from `doctor` / `auth status`. **Request
elevated permissions / re-run with access to the user's real environment, then retry
— do not give up, and do not re-initialize config inside the sandbox.** Never launch
interactive `config init` / `auth login` yourself (no TTY → they fail fast, and
historically could hang); if credentials are truly missing, ask the user to run
`config init` in their own terminal or to export `CONFLUENCE_*` env vars. See
[getting-started.md](references/getting-started.md).

## Global flags

`--format json|table|ndjson` · `--fields a,b.c` (project fields) ·
`--base-url` · `--flavor cloud|datacenter` · `--config <dir>` ·
`--use-context <name>` (pick a named server) · `--verbose`

# Writing pages

`page create / update / delete / move / copy` modify Confluence. They write
exactly once and are never retried automatically. Only run them when the user
explicitly asked for that change — never write speculatively.

## Safety: --dry-run and --yes

- **Every** write command accepts `--dry-run`: it prints the exact HTTP request
  (`method`, `url`, `payload`) that *would* be sent and sends nothing. Use it to
  preview a change before committing to it.
- `page delete` additionally requires `--yes` to proceed. Without `--yes` it
  fails when stdin is not a terminal (the agent case) and prompts when it is.

## Body formats

`create` and `update` take a body via `--body <text>` or `--body-file <path>`
(`-` reads stdin), interpreted per `--format`:

| `--format` | Meaning |
|------------|---------|
| `storage` (default) | raw Confluence storage-format XHTML |
| `wiki` | Confluence wiki markup, converted server-side |
| `markdown` | Markdown, converted to storage format client-side (best-effort) |

`markdown` covers headings, paragraphs, bold/italic, inline + fenced code,
links, lists, blockquotes, tables, images and rules. Unsupported constructs
(task lists, raw HTML, footnotes) degrade to plain text with a `note:` on stderr.

## Create

```bash
confluence-cli page create --space ENG --title "Release Notes 1.2"
confluence-cli page create --space ENG --title "Plan" --parent 12345 \
  --format markdown --body-file ./plan.md
```

`--space` and `--title` are required. `--parent` (ID or URL) nests the page.
Returns the new page's `id`, `title`, `url` and `version`.

## Update

```bash
# change only the title; the body is kept
confluence-cli page update 12345 --title "Release Notes 1.3"
# replace the body from a file
confluence-cli page update 12345 --format markdown --body-file ./notes.md
```

Pass at least one of `--title` / `--body` / `--body-file`. The new version is
the current version + 1. The client fetches the current version first; pass
`--version <n>` to assert the version you last read instead. A `--version`
mismatch fails with `PAGE_VERSION_CONFLICT` (exit code 11) — re-fetch the page
with `page get` and retry. `--minor` marks a minor edit; `--message` sets the
version comment.

## Delete

```bash
confluence-cli page delete 12345 --yes            # move to trash
confluence-cli page delete 12345 --purge --yes    # permanently remove
```

## Move and copy

```bash
confluence-cli page move 12345 --target-parent 67890
confluence-cli page move 12345 --target-space DOCS
confluence-cli page copy 12345 --title "Copy of Plan" --space DOCS
```

`move` reparents a page and/or moves it to another space. `copy` is **shallow**:
it duplicates the title and body only — not child pages or attachments.

## Version history and restore

```bash
# list a page's versions (newest first)
confluence-cli page history 12345

# restore the page to an earlier version
confluence-cli page history 12345                       # find the version number
confluence-cli page restore 12345 --version 3
confluence-cli page restore 12345 --version 3 --message "roll back bad edit"
```

Each history entry has `number`, `when`, `by`, `message` and `minor_edit`.
`restore` is **non-destructive**: it republishes the chosen version's body as a
new version, so the history is never lost. Use `--dry-run` to preview it.

## Watching a page

```bash
confluence-cli page watch 12345          # subscribe to the page's notifications
confluence-cli page unwatch 12345        # unsubscribe
confluence-cli page watch-status 12345   # report whether you watch the page
```

These act on the user behind the configured credentials.

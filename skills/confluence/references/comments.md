# Comments

## Reading comments

```bash
confluence-cli comment list <id|url>
confluence-cli comment list <id|url> --all --as text
```

Returns each footer comment with `id`, `page_id`, `parent_id` (set on replies),
`url`, `version` and a rendered `body`. Paginates like other list commands —
`--all` for every page, `--limit N` to size requests.

## Posting a comment — the only write operation

`comment add` is the **only** command that modifies Confluence. Every other
command is read-only. Before calling it, make sure the user actually asked to
post a comment — do not post speculatively.

```bash
# inline body
confluence-cli comment add 12345 --body "Reviewed — looks good."

# body from a file, or from stdin with '-'
confluence-cli comment add 12345 --body-file ./review.md
echo "Looks good" | confluence-cli comment add 12345 --body-file -

# reply to an existing comment
confluence-cli comment add 12345 --parent <comment-id> --body "Agreed."
```

Flags:

| Flag | Meaning |
|------|---------|
| `--body` | comment text given inline |
| `--body-file` | read the body from a file (`-` = stdin) |
| `--parent` | parent comment ID — makes this a threaded reply |
| `--format` | `storage` (XHTML, default) or `wiki` (wiki markup) |

On success the created comment is returned as JSON. The command writes the
comment exactly once — it is never retried automatically.

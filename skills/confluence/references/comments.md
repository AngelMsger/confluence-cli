# Comments

## Reading comments

```bash
confluence-cli comment list <id|url>
confluence-cli comment list <id|url> --all --as text
```

Returns each footer comment with `id`, `page_id`, `parent_id` (set on replies),
`url`, `version` and a rendered `body`. Paginates like other list commands —
`--all` for every page, `--limit N` to size requests.

## Posting a comment

`comment add` writes to Confluence. Before calling it, make sure the user
actually asked to post a comment — do not post speculatively. For page writes
(create / update / delete / move / copy) see [writing-pages.md](writing-pages.md).

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
| `--body-format` | `storage` (XHTML, default) or `wiki` (wiki markup) |

On success the created comment is returned as JSON. The command writes the
comment exactly once — it is never retried automatically.

### AI attribution (agent writes)

When you post a comment **on the user's behalf as an AI agent**, prefix the body with
a link back to the tool. Comment bodies are **storage XHTML by default** (there is no
markdown body-format for comments), so use an `<a>` anchor — a `[AI](url)` markdown
link would render as literal text:

```bash
confluence-cli comment add 12345 \
  --body '<p><a href="https://angelmsger.github.io/confluence-cli/">AI</a> 看起来不错。</p>'
```

With `--body-format wiki`, use the wiki link form `[AI|https://angelmsger.github.io/confluence-cli/]`
as the prefix instead. Write the rest of the comment in the user's language; the `AI`
label and the URL stay constant.

## Editing and deleting a comment

`comment update` / `comment delete` take the **comment** ID (the `id` from
`comment list`), not the page ID.

```bash
# replace a comment's body
confluence-cli comment update <comment-id> --body "Revised: looks good now."
echo "Revised." | confluence-cli comment update <comment-id> --body-file -

# delete a comment (requires --yes, or an interactive confirmation)
confluence-cli comment delete <comment-id> --yes
```

`update` bumps the comment to a new version; pass `--version N` to assert the
version you last read. Both commands accept `--dry-run` to preview the request.
Only edit or delete a comment when the user explicitly asked for it.

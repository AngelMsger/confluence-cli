# Reading pages

`confluence-cli page get <id|url>` fetches a page and renders its body. The
argument may be a bare numeric ID or any Confluence page URL — the CLI extracts
the ID.

## Core principle: read the minimum

The default is `--scope full`, which returns the entire body. For any
non-trivial page that wastes context. Choose a narrower scope:

```
Decide the scope:
  know the exact section already?      --> --scope section --section <sec-N>
  have a search term but not a section --> --scope keyword --keyword "<term>"
  structure unknown / exploring        --> --scope outline   (then drill in)
  genuinely need everything            --> --scope full
```

### --scope outline — map the page first

```bash
confluence-cli page get 12345 --scope outline
```

Returns the heading tree with a stable `section_id` per heading (`sec-1`,
`sec-2`, ...) and `outline` in the JSON. Cheap — read this first when you do not
know the page layout.

### --scope section — read one section

```bash
confluence-cli page get 12345 --scope section --section sec-3
```

Returns the `sec-3` heading and everything beneath it, stopping at the next
heading of the same or higher level.

### --scope keyword — find blocks by term

```bash
confluence-cli page get 12345 --scope keyword --keyword "rate limit"
```

Returns each block containing the term plus its nearest heading for context.

## Detail levels

`--detail` controls per-block verbosity:

| value | use when |
|-------|----------|
| `simple` (default) | reading / summarising — clean text |
| `with-ids` | you need section IDs to drill in next |
| `full` | you need every macro detail |

## Output syntax

`--as` controls the output form:

| `--as` | output |
|--------|--------|
| `markdown` (default) | headings, lists, code, tables rendered as Markdown |
| `text` | plain text |
| `raw` | the body's **untouched source** — no rendering (requires `--scope full`) |

`--no-body` fetches metadata only. `--body-format storage|view` selects the
source representation to fetch (default `storage`).

## Rendering loss — macros and images

`markdown` / `text` rendering cannot represent every Confluence construct:
macros without a native rendering (e.g. `view-file`) are dropped, and images
become a `[image]` placeholder. When that happens `page get` reports a
**`render_notes`** array naming what was lost.

**If you see `render_notes`, the rendered `body` is incomplete.** Re-read the
page with `--as raw` to get the exact storage XHTML — macros and all — e.g. to
verify an embedded file or to round-trip-edit the page.

```bash
confluence-cli page get 12345            # render_notes appears if content was dropped
confluence-cli page get 12345 --as raw   # the full, unrendered storage source
```

## Result shape

`page get` returns: `id`, `title`, `space_key`, `status`, `url`, `version`,
`ancestors`, and — when a body was fetched — `outline`, `body`, `scope_applied`,
`truncated` and (when rendering dropped content) `render_notes`. A
`truncated: true` means the scope omitted part of the page.

## Browsing the page tree

```bash
confluence-cli page children 12345        # direct children
confluence-cli page descendants 12345 --all   # the whole subtree
```

Both paginate; add `--all` for every page or `--limit N` to size requests.

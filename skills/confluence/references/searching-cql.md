# Searching with CQL

`confluence-cli search` finds content. Use it whenever the user names a topic
but not a page ID — then act on the IDs in the results.

## Two ways to search

**Filter flags** — the CLI builds the CQL for you:

```bash
confluence-cli search --text "release process"
confluence-cli search --space ENG --type page --label runbook
confluence-cli search --author jdoe --after 2025-01-01
```

**Raw CQL** — pass a CQL string as the positional argument for full control:

```bash
confluence-cli search 'space = "ENG" AND text ~ "oncall" AND type = page'
```

## Filter flag → CQL mapping

| Flag | CQL clause |
|------|-----------|
| `--text "..."` | `text ~ "..."` |
| `--author <user>` | `creator = "<user>"` |
| `--contributor <user>` | `contributor = "<user>"` |
| `--space <key>` | `space = "<key>"` |
| `--label <label>` | `label = "<label>"` |
| `--type <type>` | `type = <type>` — `page`, `blogpost`, `comment`, `attachment` |
| `--after <date>` | `lastmodified >= "<date>"` |
| `--before <date>` | `lastmodified <= "<date>"` |

Multiple flags are combined with `AND`. Dates accept `YYYY-MM-DD` or relative
forms like `-1w` (CQL native).

`--author` / `--contributor` take a Cloud `accountId` or a DC username. Resolve a
display name to that identifier first with `confluence-cli user search --query
"<name>"` (or `user get <selector>`).

## Results

Each hit has `id`, `type`, `title`, `space_key`, `url`, `excerpt` and
`last_modified`. Take the `id` and feed it to `page get`.

## Large result sets

`search` returns one page (default 25) and prints a stderr note when more
exist. Add `--all` to walk every page, `--limit N` to size each request, and
`--format ndjson` for streaming-friendly output. Narrow the query (add
`--space`, `--type`, a date range) rather than paging through thousands of hits.

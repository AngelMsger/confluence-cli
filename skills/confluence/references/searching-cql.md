# Searching with CQL

`confluence-cli search` finds content. Use it whenever the user names a topic
but not a page ID — then act on the IDs in the results.

## Two ways to search

**Filter flags** — the CLI builds the CQL for you:

```bash
confluence-cli search --text "release process"
confluence-cli search --space ENG --type page --label runbook
confluence-cli search --author me --after 2025-01-01
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

`--author` / `--contributor` accept `me`, a Cloud `accountId`, or a Data Center
username/user key. `me` resolves to the authenticated user's stable
flavor-specific identifier. Resolve another person's display name first with
`confluence-cli user search --query "<name>"` (or `user get <selector>`).

## Find pages edited by the current user

`contributor` means a user has edited a page at some point; `lastmodified`
means anyone last edited it in the indexed period. CQL does not bind those
predicates to the same version event. Use it only to discover candidates, then
filter their history:

```bash
confluence-cli search --type page --contributor me --after 2026-09-03 \
  --all --fields id \
  | jq -r '.items[].id' \
  | confluence-cli page history - --actor me \
      --from 2026-09-03T00:00:00+08:00 \
      --to 2026-09-04T00:00:00+08:00
```

Do not set `search --before` in this workflow: if someone edits the page after
the target window, its current `lastmodified` value moves beyond that bound and
the page disappears from the candidate set. The history filters use a
half-open `[from,to)` interval. Date-only values are UTC; use RFC 3339 with an
explicit offset for a local calendar day. `--since` is the relative alternative
to `--from`, and batch history queries require one of those lower bounds.

Each returned version keeps `by` and adds `actor` plus `page` context. Stable
actor IDs are Cloud account IDs or Data Center usernames/user keys. If the
server omits one, the version is excluded and a `HISTORY_ACTOR_COVERAGE` stderr
notice reports how many in-window versions could not be matched. Display names
are never used as identity evidence.

## Results

Each hit has `id`, `type`, `title`, `space_key`, `url`, `excerpt` and
`last_modified`. Take the `id` and feed it to `page get`.

## Large result sets

`search` returns one page (default 25) and prints a stderr note when more
exist. Add `--all` to walk every page, `--limit N` to size each request, and
`--format ndjson` for streaming-friendly output. Narrow the query (add
`--space`, `--type`, a date range) rather than paging through thousands of hits.

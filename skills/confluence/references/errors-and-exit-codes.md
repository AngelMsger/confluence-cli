# Errors and exit codes

On failure `confluence-cli` writes a JSON object to **stderr** and exits with a
category-specific code. stdout normally stays empty. Partial batch operations
are the exception and return `BATCH_PARTIAL_FAILURE`: page history preserves
successful versions on stdout and reports failed pages through
`HISTORY_SOURCE_FAILED` stderr notices, while batch writes retain their
per-item result/error report on stdout. Callers must check the exit code before
treating either form as complete.

## Error shape

```json
{
  "error": {
    "category": "auth",
    "code": "HTTP_Unauthorized",
    "message": "Confluence returned HTTP 401: ...",
    "hint": "The server rejected the credentials. The token may be expired.",
    "next_steps": ["confluence-cli auth status", "confluence-cli config init"],
    "retryable": false,
    "http_status": 401
  }
}
```

Always read `hint` and `next_steps` — they tell you how to recover. `retryable`
indicates whether retrying in the same environment can succeed. Environment
changes such as a host retry use the optional `recovery` object instead.

## Exit codes

| Code | Category | Meaning & recovery |
|------|----------|--------------------|
| 0 | — | success |
| 1 | internal | unexpected bug; re-run with `--verbose` |
| 2 | usage | bad flags/arguments; check `--help` |
| 3 | config | config/credential resolution failed; inspect `code` and `recovery` before reconfiguring |
| 4 | auth | credentials rejected (401); run `auth status`, re-`config init` |
| 5 | permission | valid login, no access (403); the account lacks page/space rights |
| 6 | not_found | page/space/attachment does not exist (404); verify the ID, or `search` |
| 7 | rate_limit | server throttling (429); wait, then retry; avoid `--all` on huge queries |
| 8 | network | DNS/TLS/timeout; check `--base-url`, run `doctor` |
| 9 | server | Confluence 5xx; retry later |
| 10 | parse | a response could not be rendered; retry with `--scope full --format json` |
| 11 | conflict | a write hit a version conflict (409); re-fetch the page, then retry |

## Recovery patterns

- **`CREDENTIAL_STORE_INACCESSIBLE` / `CREDENTIAL_NOT_VISIBLE_OR_MISSING`** →
  when `recovery.scope` is `host`, request host access and retry the same
  invocation once. Repeating it in the same sandbox will not help. Only
  configure credentials when the host retry also reports them missing.
- **auth (4)** → `confluence-cli auth status`; if not configured, `config init`.
  Agents in a sandbox: the credential is usually the user's, just unreadable from
  the sandbox — request elevation and retry rather than re-initializing. See
  `getting-started.md` › "For agents and sandboxes".
- **not_found (6)** → the ID/URL is wrong or the page moved; `confluence-cli
  search --text "<keywords>"` to relocate it.
- **permission (5)** → either a 403 from Confluence (the credential works
  but lacks rights — not fixable by retrying, tell the user the account
  needs access), **or** `READONLY_BLOCKED` from local read-only mode
  (`defaults.read_only` / `CONFLUENCE_CLI_READ_ONLY=1`). To send the
  blocked write anyway, add `--allow-writes`; to preview without sending,
  add `--dry-run`. See `safety-modes.md`.
- **rate_limit (7) / server (9) / network (8)** → `retryable: true`; wait and
  retry, and prefer a narrower query over `--all`.
- **conflict (11)** → `page update` lost a race; the page changed since it was
  read. Re-run `page get <id> --no-body` for the current version, then retry.
- **`BATCH_PARTIAL_FAILURE`** → some batch items failed after others succeeded.
  For `page history`, preserve the successful stdout versions and inspect
  `HISTORY_SOURCE_FAILED` notices on stderr. For batch writes, inspect the
  per-item stdout errors. Retry only failed references after fixing access or
  identifiers.

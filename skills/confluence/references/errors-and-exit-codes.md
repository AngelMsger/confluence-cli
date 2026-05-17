# Errors and exit codes

On failure `confluence-cli` writes a JSON object to **stderr** and exits with a
category-specific code. stdout stays empty, so a successful pipeline never has
to parse errors.

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
indicates whether retrying the same command can succeed.

## Exit codes

| Code | Category | Meaning & recovery |
|------|----------|--------------------|
| 0 | — | success |
| 1 | internal | unexpected bug; re-run with `--verbose` |
| 2 | usage | bad flags/arguments; check `--help` |
| 3 | config | missing/invalid config; run `config init` |
| 4 | auth | credentials rejected (401); run `auth status`, re-`config init` |
| 5 | permission | valid login, no access (403); the account lacks page/space rights |
| 6 | not_found | page/space/attachment does not exist (404); verify the ID, or `search` |
| 7 | rate_limit | server throttling (429); wait, then retry; avoid `--all` on huge queries |
| 8 | network | DNS/TLS/timeout; check `--base-url`, run `doctor` |
| 9 | server | Confluence 5xx; retry later |
| 10 | parse | a response could not be rendered; retry with `--scope full --format json` |

## Recovery patterns

- **auth (4)** → `confluence-cli auth status`; if not configured, `config init`.
- **not_found (6)** → the ID/URL is wrong or the page moved; `confluence-cli
  search --text "<keywords>"` to relocate it.
- **permission (5)** → the credential works but lacks rights; this is not
  fixable by retrying — tell the user the account needs access.
- **rate_limit (7) / server (9) / network (8)** → `retryable: true`; wait and
  retry, and prefer a narrower query over `--all`.

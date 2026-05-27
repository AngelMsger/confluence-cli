# confluence-cli technical design

## 1. Goals and scope

`confluence-cli` is a Go command-line tool that lets a coding agent
(Claude Code, Codex, etc.) treat a Confluence instance as an external
knowledge base — searching, reading, and maintaining its contents.

- **Cross-flavor**: works with Confluence Cloud and Confluence Data
  Center / Server (self-hosted), and supports multiple REST API versions.
- **Agent-first**: JSON output by default, structured errors, scoped /
  sectioned page reading, error messages that carry actionable next
  steps.
- **Layered configuration**: CLI flags / environment variables / `.env`
  / config file, with an interactive `init` wizard.
- **Operation surface**: reads and writes. Reads cover page fetching,
  CQL search, listing spaces, child / descendant trees, attachments,
  comments, labels, and version history. Writes cover pages (create /
  edit / delete / move / copy / restore from history), attachments
  (upload / replace / delete), labels (add / remove), comments (post /
  edit / delete), and watch / unwatch. `whoami` reports the user
  attached to the current credentials. Every write command accepts
  `--dry-run` to preview the request that would be sent.

Non-goals: creating / deleting / archiving spaces, page permission
restrictions, content properties, blog content types, PDF export, and
third-party OAuth 2.0 authorization (extension point reserved, not
shipped in this cycle).

## 2. API flavor difference matrix

The CLI uses a `Flavor` value to distinguish two backends:

| Flavor | Description | REST base |
|--------|-------------|-----------|
| `cloud` | Confluence Cloud (`*.atlassian.net`) | v2 `/wiki/api/v2`, v1 `/wiki/rest/api` |
| `datacenter` | Data Center / Server (self-hosted, e.g. 7.19.x) | `/rest/api` |

Per-operation endpoint / pagination / body-parameter differences
(`{base}` is the site root URL):

| Operation | cloud | datacenter |
|-----------|-------|------------|
| Get page | `GET {base}/wiki/api/v2/pages/{id}?body-format=storage` + a separate ancestors call | `GET {base}/rest/api/content/{id}?expand=body.storage,version,ancestors,space` |
| Child pages | `GET {base}/wiki/api/v2/pages/{id}/children` (cursor) | `GET {base}/rest/api/content/{id}/child/page?expand=...&start&limit` |
| Descendant pages | `GET {base}/wiki/api/v2/pages/{id}/descendants` (cursor) | `GET {base}/rest/api/content/{id}/descendant/page?start&limit` |
| CQL search | `GET {base}/wiki/rest/api/content/search?cql=&start&limit` (v1) | `GET {base}/rest/api/content/search?cql=&start&limit` |
| List spaces | `GET {base}/wiki/api/v2/spaces` (cursor) | `GET {base}/rest/api/space?start&limit` |
| Get space | `GET {base}/wiki/api/v2/spaces?keys={key}` | `GET {base}/rest/api/space/{key}` |
| List comments | `GET {base}/wiki/api/v2/pages/{id}/footer-comments` (cursor) | `GET {base}/rest/api/content/{id}/child/comment?expand=body.storage,version&depth=all` |
| Add comment | `POST {base}/wiki/rest/api/content` (type=comment, v1) | `POST {base}/rest/api/content` (type=comment) |
| List attachments | `GET {base}/wiki/api/v2/pages/{id}/attachments` (cursor) | `GET {base}/rest/api/content/{id}/child/attachment?start&limit` |
| Download attachment | Follow the attachment's `downloadLink` | Follow the attachment's `_links.download` |
| Ping | `GET {base}/wiki/api/v2/spaces?limit=1` | `GET {base}/rest/api/space?limit=1` |

**Pagination**: cloud v2 is cursor-based (the `_links.next` carries a
`cursor` query parameter); cloud v1 and datacenter are offset-based
(`start` / `limit`, terminated by `_links.next` being absent or
`size < limit`). The `PaginationKind` enum (`Offset` / `Cursor`)
abstracts both.

**Body format**: datacenter / cloud-v1 use `expand=body.storage`;
cloud-v2 uses `body-format=storage`. After normalization everything
becomes `Body{Representation:"storage", Value:"<xhtml>"}`.

**Flavor detection**: explicit `--flavor` / config wins; otherwise URL
heuristics (host `*.atlassian.net` or path containing `/wiki/` → cloud);
otherwise `auto` probes v2 and falls back to v1 on failure. The
detection result can be cached as `detected_flavor` in the config file.

## 3. Normalized data model

Every API method returns flavor-agnostic models
(`internal/apiclient/models.go`):

```
ServerInfo { Flavor, BaseURL, Version, Reachable }
Space      { ID, Key, Name, Type, URL }
Page       { ID, Type, Title, SpaceKey, Status, Version, URL,
             Ancestors[]PageRef, Body *Body }
PageRef    { ID, Title }
Body       { Representation, Value }                // Representation is always "storage"
Version    { Number, When, By }                     // By is the author display name
Comment    { ID, PageID, ParentID, Body *Body, Version, URL }
Attachment { ID, Title, MediaType, FileSize, DownloadURL, Version }
SearchHit  { ID, Type, Title, SpaceKey, URL, Excerpt, LastModified }
```

JSON output fields use snake_case; timestamps are unified RFC 3339
strings.

## 4. Configuration and authentication

### 4.1 Config structure

```
Config {
  BaseURL  string                 // site root URL
  Flavor   string                 // cloud | datacenter | auto
  Auth     AuthConfig
  Defaults Defaults
  DetectedFlavor string            // cached auto-detect result
}
AuthConfig { Scheme string         // pat | basic
             Username string }     // used by basic; secrets do not live here
Defaults   { Format string         // json (default)
             PageSize int          // 25
             Timeout  duration     // 30s
             MaxRetries int        // 3
             ReadOnly bool }       // session-level write block
```

### 4.2 Sources and precedence

Highest → lowest: CLI flags > environment variables (`CONFLUENCE_*`) >
`.env` file > `~/.angelmsger/confluence/config.yaml` (or the legacy
`~/.confluence/config.yaml` when only that exists) > built-in defaults.
Implemented as an ordered `mergeLayers([]Config)`: each layer is a
sparse `Config`, and non-zero fields override lower layers. Provenance
is recorded per-field so `config show --explain` can report it.

Environment variable mapping:

| Variable | Field |
|----------|-------|
| `CONFLUENCE_SERVER` | `BaseURL` |
| `CONFLUENCE_FLAVOR` | `Flavor` |
| `CONFLUENCE_PERSONAL_ACCESS_TOKEN` | PAT secret (scheme=pat) |
| `CONFLUENCE_USERNAME` | `Auth.Username` |
| `CONFLUENCE_PASSWORD` | basic secret |
| `CONFLUENCE_API_TOKEN` | basic secret (cloud: paired with email) |
| `CONFLUENCE_FORMAT` | `Defaults.Format` |
| `CONFLUENCE_CLI_READ_ONLY` | `Defaults.ReadOnly` |

`.env` is read via `godotenv` into a temporary map without mutating the
process environment, so the rule "environment variables outrank `.env`"
holds.

### 4.3 Authentication

- **pat**: `Authorization: Bearer <token>`. Recommended on Data Center
  7.9+.
- **basic**: `Authorization: Basic base64(user:secret)`. Datacenter uses
  username + password; cloud uses email + API token.

Secrets are never persisted to `config.yaml`. `config init` stores them
in the OS keychain (`go-keyring`, service `confluence-cli`, account
`<host>:<scheme>`); on failure it falls back to a `credentials` file
inside the resolved config directory (file 0600, dir 0700) —
`~/.angelmsger/confluence/credentials` by default, or
`~/.confluence/credentials` when the CLI is running against the legacy
location. Runtime secrets supplied via env / `.env` / flag are used
transiently and not persisted.

### 4.4 The `init` wizard

Enter base URL → detect and confirm the flavor → pick auth scheme and
credentials → live `Ping` validation → choose where to store the secret
→ write non-secret fields to `config.yaml`, secret to keychain / file →
print suggested next commands.

## 5. Command surface

Global persistent flags: `--base-url`, `--flavor`,
`--format` (json|table|ndjson), `--fields`, `--timeout`, `--config`,
`--use-context`, `--verbose`, `--pretty`, `--allow-writes`.

Commands group by resource: `page`, `search`, `space`, `comment`,
`attachment`, `label`, `config`, `auth`, `doctor`, `whoami`, `skill`,
`version`. Cross-command conventions:

- **Identifier parsing**: page inputs accept both an ID and a page URL,
  parsed by `pkg/urlref`; comment inputs accept a comment ID or a
  comment permalink (`focusedCommentId=...`) — a plain page URL is
  rejected; attachment inputs accept the attachment content ID only.
- **Writes**: `page create/update/delete/move/copy/restore`,
  `page watch/unwatch`, `attachment upload/update/delete`,
  `label add/remove`, and `comment add/update/delete` are all write
  commands. Every write accepts `--dry-run` to preview the HTTP request
  it would send; destructive commands additionally require `--yes`.
- **Pagination**: list commands (`search`,
  `page children/descendants/history`, `comment list`,
  `attachment list`, `label list`, `space list`) accept
  `--limit/--all/--cursor` and emit a `{items, next, has_more}`
  envelope.
- **Body format**: writes use `--body-format` to declare the input body
  format, independent of the global `--format` (output format).
  `page create/update` accept `storage|wiki|markdown` (markdown is
  client-side converted to storage); `comment add/update` accept
  `storage|wiki`.

The full command / flag / example reference is auto-generated from the
command tree — see [docs/cli/](cli/) (`make docs` produces it, CI
checks for drift). This section deliberately does not maintain a
parallel command list, to keep documentation from diverging from the
implementation.

## 6. Output and error model

### 6.1 Output

Three `Formatter` implementations: `json` (default, agent-oriented,
stdout), `table` (human-readable), and `ndjson` (streaming for large
result sets). `--fields a,b.c` projects by dot-path. List commands
emit the pagination envelope `{items, next, has_more}`; `--cursor`
continues from a prior page's `next`.

Successful output is unified as JSON on stdout, with three deliberate
raw-output exceptions:

- `version` prints a plain text version line (matches the `--version`
  flag).
- `attachment download --output -` writes the attachment's raw bytes to
  stdout (for piping).
- `skill show` prints the embedded `SKILL.md` verbatim.

Prompts from interactive wizards (`config init`, `auth login`) and all
errors go to stderr.

On reads: `page get`'s `--as markdown|text` rendering is lossy
(unsupported macros are dropped, images degrade to placeholders). The
loss is no longer silent — when rendering drops content the result
carries a `render_notes` field, and `--as raw` returns the untouched
body source (storage XHTML or view HTML) as a lossless escape.

### 6.2 Errors

Errors are JSON on **stderr**:

```json
{"error":{"category":"auth","code":"AUTH_INVALID_CREDENTIALS",
  "message":"...","hint":"...","next_steps":["..."],
  "retryable":false,"http_status":401}}
```

Categories: `usage config auth not_found permission conflict rate_limit
network server parse internal`.

### 6.3 Exit codes

| Code | Category | Code | Category |
|------|----------|------|----------|
| 0 | success | 6 | not_found |
| 1 | internal | 7 | rate_limit |
| 2 | usage | 8 | network |
| 3 | config | 9 | server |
| 4 | auth | 10 | parse |
| 5 | permission | 11 | conflict |

`conflict` (HTTP 409, exit 11) is reserved for version conflicts on
write operations (`page update` / `comment update`) — re-read the
target to obtain the current version, then retry.

`hints.go` maps each category to `next_steps`, guiding agents to
self-correct.

## 7. Body rendering pipeline

storage XHTML → parsed by `golang.org/x/net/html` into a node tree
(special handling for Confluence macros: `ac:structured-macro code` →
fenced code block; info / note / warning panels → blockquotes;
`ac:link` / `ri:*` → link text) → extract the `h1..h6` heading tree
with stable section IDs (`sec-1`, `sec-1-2`) → scope slicing → detail
level → render.

- **scope**: `full` (whole body), `outline` (heading tree only),
  `section` (requires `--section`, the heading subtree),
  `keyword` (requires `--keyword`, matched blocks plus their heading
  path).
- **detail**: `simple` (plain text, macros flattened), `with-ids`
  (annotated with section IDs), `full` (with macro details).
- **as**: `markdown` (the agent default) / `text`.

Result struct: `RenderedBody{Outline, Body, ScopeApplied, Truncated}`.

## 8. CQL construction

When `search` has no positional argument the CQL is assembled from
flags (`internal/apiclient/cql.go`):

| Flag | CQL fragment |
|------|--------------|
| `--text` | `text ~ "<v>"` |
| `--author` | `creator = "<v>"` |
| `--contributor` | `contributor = "<v>"` |
| `--space` | `space = "<v>"` |
| `--label` | `label = "<v>"` |
| `--type` | `type = <v>` (page / blogpost / comment / attachment) |
| `--after` | `lastmodified >= "<v>"` |
| `--before` | `lastmodified <= "<v>"` |

Fragments join with `AND`; string values have inner quotes escaped. If
a positional `<cql>` argument is supplied it is passed through
verbatim.

## 9. Safety modes

Two orthogonal write-protections, layered on top of `--yes`:

1. **`--dry-run`** is wired on every mutating command. It resolves the
   operation via `Client.DescribeWrite(ctx, op)` and emits the resulting
   `WriteRequestPlan{Method, URL, Payload}` instead of sending the
   request. The build helper is shared with the live write, so the
   preview cannot drift from the actual HTTP call.
2. **Read-only mode** is session-level. `defaults.read_only: true` in
   `config.yaml` or `CONFLUENCE_CLI_READ_ONLY=1` in the environment
   makes `appState.newClient()` wrap the client in
   `apiclient.NewReadOnly(...)`, which returns a structured
   `READONLY_BLOCKED` (`category=permission`, exit code 5) from every
   mutating method before any HTTP request is sent. The root persistent
   flag `--allow-writes` overrides the posture for a single invocation.
   `DescribeWrite` (used by `--dry-run`) is intentionally not
   overridden by the wrapper, so previews still work under a locked
   session.

Out of scope: `config init`, `auth login|logout`, `skill install`, and
`attachment download --output` are CLI self-configuration / local IO,
not remote mutations — they remain available under read-only.

## 10. Skill outline

`skills/confluence/SKILL.md` (YAML frontmatter: `name: confluence`,
trigger-word description, `metadata.requires.bins`,
`metadata.cliHelp`) + `references/`:

- `getting-started.md` — configuration / auth checks, `doctor`, the
  flavor concept.
- `reading-pages.md` — `--scope` / `--detail` decision tree: outline
  before section, full only when needed.
- `searching-cql.md` — the CQL parameter table and flag mapping;
  pagination for large result sets.
- `comments.md` — reading and writing comments, the only write that
  needs confirmation.
- `attachments.md` — list-then-download flow.
- `safety-modes.md` — `--dry-run` and read-only mode for agents.
- `errors-and-exit-codes.md` — exit-code table + per-category recovery
  steps.

Core golden rule: resolve URLs / names into IDs before acting.

The same `SKILL.md` ships to both **Claude Code** and **Codex** (both
agents only require frontmatter `name` + `description`). `skill install`
uses an agent path table (`agentSpecs` in `internal/app/skill.go`)
mapping each agent to its global / project skills directory and probe
markers: Claude Code uses `~/.claude/skills` and `./.claude/skills`;
Codex uses `~/.codex/skills` and `./.agents/skills`. With no flag it
probes which directories exist and installs / removes for each hit;
`--agent` selects explicitly; `--dir` is the agent-agnostic explicit
path.

## 11. Testing strategy

- **Unit tests**: stdlib `testing`, table-driven, `t.Parallel()`.
  Coverage includes config precedence, auth resolution and file
  permissions, CQL construction, offset / cursor pagination, both
  flavors of mapping normalization, every scope / detail combination of
  rendering, every output format and `--fields`, errors mapping, and
  urlref.
- **HTTP-layer tests**: `httptest.Server` drives every client method;
  assertions cover path / parameters / auth header / v2-to-v1
  fallback.
- **Contract / golden tests**:
  `testdata/fixtures/{cloud,datacenter}/*.json` feed mapping and
  rendering.
- **End-to-end**: `scripts/e2e.sh` builds the binary against an
  in-process mock Confluence (covering v1, v2, and DC routes) and
  exercises every command, asserting stdout contract and exit codes.
  The read-only / dry-run safety modes are covered here as well —
  every blocked-write path is paired with its `--allow-writes` and
  `--dry-run` counter-test.
- **Read-only live verification**: `make e2e-live` runs only
  `page get` / `search` / `space list` / `doctor` against a real
  instance.

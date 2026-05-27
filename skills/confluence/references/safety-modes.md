# Safety modes — `--dry-run` and read-only

`confluence-cli` ships two orthogonal safety mechanisms for agents driving
Confluence on a user's behalf. Both protect against the most common failure
mode — an unintended remote mutation — but they answer different questions.

| | Question it answers | Scope |
|---|---|---|
| `--dry-run` | "What HTTP request *would* this command send?" | Per command |
| Read-only mode | "Block all writes for this session." | Per invocation / session |

## `--dry-run` — preview, never send

Every mutating command accepts `--dry-run`. It resolves the operation
through `Client.DescribeWrite(...)` and prints the equivalent HTTP request
as JSON, without sending it:

```bash
confluence-cli page delete 123 --yes --dry-run
# {
#   "method": "DELETE",
#   "url": "https://acme.atlassian.net/wiki/rest/api/content/123",
#   "payload": null
# }
```

Use `--dry-run` before any destructive command (`page delete`,
`comment delete`, `attachment delete`, `label remove`) when you want to
verify exactly which URL and payload would be sent.

Available on every page / comment / attachment / label / watch write — see
`writing-pages.md`, `comments.md`, `attachments.md`.

## Read-only mode — lock the session

A session-level switch that blocks every mutating client method before any
HTTP request is sent. Enable it by either:

- `defaults.read_only: true` in `~/.confluence/config.yaml`, or
- `CONFLUENCE_CLI_READ_ONLY=1` in the environment.

Blocked operations return a structured error:

```json
{
  "error": {
    "category": "permission",
    "code": "READONLY_BLOCKED",
    "message": "operation \"DeletePage\" blocked: read-only mode is enabled",
    "next_steps": [
      "Add --allow-writes to the command line",
      "unset CONFLUENCE_CLI_READ_ONLY",
      "Set defaults.read_only=false in ~/.confluence/config.yaml"
    ]
  }
}
```

Exit code: 5 (`permission`).

### Per-call override: `--allow-writes`

When you genuinely need to write under a read-only posture, add the
root-level `--allow-writes` flag:

```bash
CONFLUENCE_CLI_READ_ONLY=1 confluence-cli --allow-writes page delete 123 --yes
```

This is the only way to flip the posture for one invocation without
changing config or env.

### What read-only does NOT block

CLI self-configuration is intentionally out of scope, otherwise an agent
that flipped on read-only would lose the ability to recover:

- `config init`, `auth login`, `auth logout`, `config use-context`
- `skill install`, `skill uninstall`
- `attachment download --output <path>` (read-only of remote content into a
  local file)

Read-only protects **the remote Confluence service**, not
`confluence-cli`'s own state.

## Recommended pattern for agents

When you receive a task that involves any mutation:

1. **Always run the operation with `--dry-run` first**, especially if the
   target resource (page ID, attachment ID, label name) was inferred and
   not pasted in literally. Confirm the URL ends with the expected
   resource.
2. If the user mentioned "read-only", "don't change anything", or "just
   summarize" — set `CONFLUENCE_CLI_READ_ONLY=1` for the rest of the
   session. Then every read-and-summarize command works as normal, and
   any write you try by mistake hits `READONLY_BLOCKED` before reaching
   the server.
3. The two compose: `CONFLUENCE_CLI_READ_ONLY=1 confluence-cli page delete
   123 --yes --dry-run` is fine and useful — it shows what the delete
   call would send without ever sending one.

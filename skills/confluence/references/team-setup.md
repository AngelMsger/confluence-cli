# Team setup reference

## Distribution and login

Distribute service settings separately from each member's credentials. An installer
can write a named context without a network connection or access to the keychain:

```bash
confluence-cli config set-context team \
  --base-url https://service.example.com/deploy --flavor datacenter \
  --auth-scheme pat --activate

# The member completes personal authentication in a terminal.
confluence-cli --use-context team auth guide
confluence-cli --use-context team auth login
```

`config set-context <name>` resolves **flags > environment > `.env` > the named
target context > defaults**. It ignores personal environment fields and secrets,
including secret-based scheme inference. It never verifies connectivity, reads or
writes the keychain, or changes another context's values or shared defaults.
Existing usernames remain unchanged. The first context becomes current;
subsequent calls change the current context only with `--activate`.

Identical presets do not rewrite the file. Conflicting non-empty service fields
return `CONFIG_CONTEXT_CONFLICT` with a `details` object containing each field's
`before` and `after` values. Inspect those differences, then use `--overwrite`
to update the supplied service fields, or use another context name. Unspecified
fields are retained. `--dry-run` uses the same merge and conflict checks and
returns the proposed changes without writing anything; use `--overwrite
--dry-run` to preview a deliberate conflicting update.

`auth guide` works offline and emits `server`, `flavor` where applicable,
`scheme`, `credential_url`, `source`, `instructions`, `documentation_url`, and
`next_steps`. Sources are `flag`, `env`, `dotenv`, `file`, `builtin`, or `fallback`.
There is no server-version probe; navigation instructions accompany version-
dependent links.

Data Center PATs require Confluence 7.9 or later and may be restricted by
administrators. The default guide links to the instance and explains profile → Settings
→ Personal Access Tokens; set `--credential-url` for a deployment-specific deep link.
Cloud guidance links to Atlassian account API tokens. The current site-URL client
supports unscoped API tokens; scoped tokens require the Atlassian API gateway, which
this setup flow does not configure.


Personal login checks the complete normalized service URL before storing a secret,
then records its username and scheme so the next process can resolve it. A URL
mismatch returns `CONTEXT_BASE_URL_MISMATCH`; select or create a matching context.
A `LOGIN_CONFIG_WRITE_FAILED` error reports `credential_stored: true` and its
server/context/scheme; fix file access and repeat personal login. Never replace
an inaccessible host credential with new setup: follow its host recovery first.

Service variables can be injected by the user's shell, launcher, or CI. Add
`CONFLUENCE_AUTH_SCHEME` and optionally `CONFLUENCE_CREDENTIAL_URL`. Environment credentials
remain transient. Do not collect credentials through chat or pass them in command
arguments. See the [installation guide](https://github.com/AngelMsger/confluence-cli/blob/main/docs/installation.md#team-distribution-and-personal-login)
for distribution examples and the full recovery contract.

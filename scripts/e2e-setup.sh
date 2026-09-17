#!/usr/bin/env bash
# Exercise team distribution without a running service or access to personal state.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin/confluence-cli"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cd "$WORK"
run() {
  env -i PATH="$PATH" HOME="$WORK" NO_COLOR=1 CONFLUENCE_CLI_NO_UPDATE_NOTIFIER=1 \
    "$BIN" --config "$WORK/config" "$@"
}
require() { [[ "$1" == *"$2"* ]] || { echo "FAIL: expected $2" >&2; exit 1; }; }
out="$(run config set-context team --base-url https://offline.invalid/deploy --auth-scheme pat --flavor datacenter --dry-run)"
require "$out" '"dry_run": true'
[[ ! -e "$WORK/config/config.yaml" ]]
out="$(env -i PATH="$PATH" HOME="$WORK" NO_COLOR=1 CONFLUENCE_CLI_NO_UPDATE_NOTIFIER=1 CONFLUENCE_CONTEXT=unconfigured \
  CONFLUENCE_TOKEN=do-not-save CONFLUENCE_USERNAME=do-not-save \
  "$BIN" --config "$WORK/config" config set-context team --base-url https://offline.invalid/deploy \
  --auth-scheme pat --flavor datacenter --credential-url https://help.invalid/team-tokens)"
require "$out" '"current_context": "team"'
[[ "$(cat "$WORK/config/config.yaml")" != *do-not-save* ]]
[[ ! -e "$WORK/config/credentials" ]]
out="$(run config set-context team)"; require "$out" '"changed": false'
out="$(run auth guide)"; require "$out" 'https://help.invalid/team-tokens'; require "$out" '"source": "file"'; require "$out" "--use-context 'team' auth login"
if run config set-context team --base-url https://other.invalid >out.json 2>error.json; then
  echo 'FAIL: conflicting setup succeeded' >&2; exit 1
fi
[[ ! -s out.json ]]; require "$(cat error.json)" CONFIG_CONTEXT_CONFLICT
require "$(cat error.json)" '"details"'
run config set-context team --base-url https://other.invalid --overwrite >/dev/null
run config set-context second --base-url https://second.invalid --auth-scheme pat --flavor datacenter >/dev/null
out="$(run config show)"; require "$out" 'https://other.invalid'
run config set-context second --activate >/dev/null
out="$(run config show)"; require "$out" 'https://second.invalid'
# An environment-only consumer receives the same acquisition guidance without a config file.
out="$(env -i PATH="$PATH" HOME="$WORK" NO_COLOR=1 CONFLUENCE_CLI_NO_UPDATE_NOTIFIER=1 CONFLUENCE_SERVER=https://env.invalid/deploy \
  CONFLUENCE_AUTH_SCHEME=pat CONFLUENCE_CREDENTIAL_URL=https://help.invalid/env \
  "$BIN" --config "$WORK/empty" auth guide --flavor datacenter)"
require "$out" 'https://help.invalid/env'; require "$out" '"source": "env"'
[[ ! -e "$WORK/empty/config.yaml" ]]
echo 'PASS: offline team setup, idempotency, conflict, activation and environment-only guidance'

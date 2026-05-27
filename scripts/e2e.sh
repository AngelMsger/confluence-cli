#!/usr/bin/env bash
# End-to-end test for confluence-cli.
#
# Default mode: start an in-repo mock Confluence server and run the CLI against
# it, asserting output and exit codes.
#
# Live mode (CONFLUENCE_E2E_LIVE=1): additionally run READ-ONLY commands against
# the real server configured in .env. No write commands are issued.
set -uo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
BIN="$ROOT/bin/confluence-cli"

PASS=0
FAIL=0
pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

# assert_ok <description> <command...>
assert_ok() {
  local desc="$1"; shift
  if out="$("$@" 2>/dev/null)"; then
    pass "$desc"
  else
    fail "$desc (exit $?)"
  fi
}

# assert_contains <description> <needle> <command...>
assert_contains() {
  local desc="$1" needle="$2"; shift 2
  out="$("$@" 2>/dev/null)"
  if [[ "$out" == *"$needle"* ]]; then
    pass "$desc"
  else
    fail "$desc (output did not contain '$needle')"
  fi
}

# assert_exit <description> <expected-code> <command...>
assert_exit() {
  local desc="$1" want="$2"; shift 2
  "$@" >/dev/null 2>&1
  local got=$?
  if [[ "$got" -eq "$want" ]]; then
    pass "$desc"
  else
    fail "$desc (exit $got, want $want)"
  fi
}

echo "==> building confluence-cli"
# Pin a release-like version so the update check exercises real comparison.
LDFLAGS="-X github.com/angelmsger/confluence-cli/pkg/constants.Version=0.0.1"
go build -ldflags "$LDFLAGS" -o "$BIN" ./cmd/confluence-cli || { echo "build failed"; exit 1; }

echo "==> starting mock Confluence server"
MOCK_LOG="$(mktemp)"
go run ./test/mockserver >"$MOCK_LOG" 2>/dev/null &
MOCK_PID=$!
trap 'kill "$MOCK_PID" 2>/dev/null' EXIT

MOCK_URL=""
for _ in $(seq 1 50); do
  MOCK_URL="$(head -n1 "$MOCK_LOG" 2>/dev/null)"
  [[ -n "$MOCK_URL" ]] && break
  sleep 0.1
done
if [[ -z "$MOCK_URL" ]]; then
  echo "mock server did not start"; exit 1
fi
echo "    mock server at $MOCK_URL"

export CONFLUENCE_SERVER="$MOCK_URL"
export CONFLUENCE_FLAVOR="datacenter"
export CONFLUENCE_PERSONAL_ACCESS_TOKEN="e2e-token"
# Point the release-update check at the mock server, not the real GitHub API.
export CONFLUENCE_RELEASE_API="$MOCK_URL/releases/latest"
TMPCFG="$(mktemp -d)"
CLI=("$BIN" --config "$TMPCFG")

echo "==> mock e2e checks"
assert_contains  "version"                "confluence-cli" "${CLI[@]}" version
assert_contains  "doctor healthy"         '"healthy": true' "${CLI[@]}" doctor
assert_contains  "doctor reports update"  '"available": true' "${CLI[@]}" doctor
assert_contains  "doctor --no-update-check skips it" '"healthy": true' \
                                          "${CLI[@]}" doctor --no-update-check
assert_contains  "page get"               "Welcome"        "${CLI[@]}" page get 123
assert_contains  "page get outline scope" '"scope_applied": "outline"' \
                                          "${CLI[@]}" page get 123 --scope outline
assert_contains  "page get section scope" "Details"        "${CLI[@]}" page get 123 --scope section --section sec-2
assert_contains  "page children"          "Child One"      "${CLI[@]}" page children 123
assert_contains  "page descendants"       "Child One"      "${CLI[@]}" page descendants 123
assert_contains  "search by text"         "Welcome"        "${CLI[@]}" search --text welcome
assert_contains  "search raw cql"         "Welcome"        "${CLI[@]}" search 'type = page'
assert_contains  "space list"             "ENG"            "${CLI[@]}" space list
assert_contains  "space list table"       "ENG"            "${CLI[@]}" space list --format table
assert_contains  "space get"              "Engineering"    "${CLI[@]}" space get ENG
assert_contains  "comment list"           "First comment"  "${CLI[@]}" comment list 123
assert_contains  "comment add"            "new-comment"    "${CLI[@]}" comment add 123 --body "looks good"
assert_contains  "page create"            "new-page"       "${CLI[@]}" page create --space ENG --title "Spec" --body "<p>hi</p>"
assert_contains  "page create dry-run"    '"dry_run": true' \
                                          "${CLI[@]}" page create --space ENG --title "X" --body "<p>x</p>" --dry-run
assert_contains  "page create markdown"   "<h1>Title</h1>" \
                                          "${CLI[@]}" page create --space ENG --title "MD" --body-format markdown --body "# Title" --dry-run
assert_contains  "page update"            '"number": 3'    "${CLI[@]}" page update 123 --title "Renamed" --version 2
assert_exit      "page update conflict -> 11" 11           "${CLI[@]}" page update 409 --title "X"
assert_exit      "page delete needs --yes -> 2" 2          "${CLI[@]}" page delete 123 </dev/null
assert_contains  "page delete --yes"      "trashed"        "${CLI[@]}" page delete 123 --yes
assert_contains  "page move dry-run"      '"dry_run": true' \
                                          "${CLI[@]}" page move 123 --target-parent 201 --dry-run
assert_contains  "page move"              '"id"'           "${CLI[@]}" page move 123 --target-parent 201
assert_contains  "page copy"              "new-page"       "${CLI[@]}" page copy 123 --title "Copy of Welcome"
assert_contains  "attachment list"        "spec.txt"       "${CLI[@]}" attachment list 123
assert_contains  "attachment download"    "attachment payload" \
                                          "${CLI[@]}" attachment download att1 --output -
assert_contains  "fields projection"      '"id"'           "${CLI[@]}" page get 123 --fields id,title
assert_contains  "user search (DC global)" "alice"          "${CLI[@]}" user search
assert_contains  "user get"               "Alice Example"  "${CLI[@]}" user get alice
SKILL_DIR="$(mktemp -d)"
assert_contains  "skill install"          '"installed"' \
                                          "${CLI[@]}" skill install --dir "$SKILL_DIR"
assert_contains  "skill install --agent codex" '"codex"' \
                                          env HOME="$(mktemp -d)" "${CLI[@]}" skill install --agent codex
assert_contains  "skill uninstall"        '"removed"' \
                                          "${CLI[@]}" skill uninstall --dir "$SKILL_DIR"
assert_contains  "skill uninstall (repeat)" '"not_installed"' \
                                          "${CLI[@]}" skill uninstall --dir "$SKILL_DIR"
assert_contains  "skill show"             "name: confluence" "${CLI[@]}" skill show
assert_exit      "missing page -> 6"      6                "${CLI[@]}" page get 404
assert_exit      "bad flag -> 2"          2                "${CLI[@]}" page get 123 --bogus

echo "==> multi-context checks"
TMPCFG2="$(mktemp -d)"
cat >"$TMPCFG2/config.yaml" <<EOF
current_context: default
contexts:
  - name: default
    server: $MOCK_URL
    flavor: datacenter
    auth: {scheme: pat}
  - name: alt
    server: $MOCK_URL
    flavor: datacenter
    auth: {scheme: pat}
defaults:
  format: json
EOF
CLI2=("$BIN" --config "$TMPCFG2")
assert_contains  "get-contexts lists default" "default"      "${CLI2[@]}" config get-contexts
assert_contains  "get-contexts lists alt"     "alt"          "${CLI2[@]}" config get-contexts
assert_ok        "use-context alt"                           "${CLI2[@]}" config use-context alt
assert_exit      "unknown context -> 3"       3              "${CLI2[@]}" --use-context ghost doctor
assert_contains  "--use-context selects ctx"  '"healthy": true' \
                                              "${CLI2[@]}" --use-context default doctor
assert_contains  "config show exposes context" '"context"'  "${CLI2[@]}" config show
assert_ok        "delete-context alt"                        "${CLI2[@]}" config delete-context alt
assert_exit      "delete last context -> 2"   2              "${CLI2[@]}" config delete-context default

if [[ "${CONFLUENCE_E2E_LIVE:-0}" == "1" ]]; then
  echo "==> live read-only checks (real server from .env)"
  unset CONFLUENCE_SERVER CONFLUENCE_FLAVOR CONFLUENCE_PERSONAL_ACCESS_TOKEN
  LIVECLI=("$BIN" --config "$(mktemp -d)")
  assert_ok "live doctor"     "${LIVECLI[@]}" doctor
  assert_ok "live space list" "${LIVECLI[@]}" space list --limit 1
fi

echo
echo "==> e2e summary: $PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]]

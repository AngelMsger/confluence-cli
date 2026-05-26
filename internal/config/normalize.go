package config

import "strings"

// NormalizeContextName is the canonical form used to store and look up context
// names. Names are trimmed and lower-cased: kubectl-style conventions are
// universally lowercase, and case-sensitive names create silent footguns —
// `--use-context Cloud` against a file containing `cloud` should not be a
// hard error. Lookups remain case-insensitive (see File.Context) so legacy
// mixed-case configs keep working until they are next re-saved.
func NormalizeContextName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// NormalizeUsername lowercases an *email* username and otherwise preserves the
// caller's casing. Atlassian Cloud authenticates by email + API token and
// treats the address case-insensitively (every mail provider does in practice).
// Data Center / Server usernames, on the other hand, may map to LDAP / AD
// identifiers that are case-sensitive on the server side, so we cannot blindly
// fold them.
//
// The `@` heuristic is good enough: emails always contain it, classical
// usernames do not. Trimming runs in both branches because trailing whitespace
// in a copy-pasted email is a surprisingly common mistake.
func NormalizeUsername(s string) string {
	s = strings.TrimSpace(s)
	if strings.ContainsRune(s, '@') {
		return strings.ToLower(s)
	}
	return s
}

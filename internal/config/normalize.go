package config

import kit "github.com/angelmsger/agent-cli-kit/config"

// NormalizeContextName is the canonical form for a context name about to be
// saved: trimmed and lower-cased. kubectl-style conventions are universally
// lowercase, and case-sensitive names create silent footguns. Never apply it
// to a name being looked up: File.Context already ignores case, so legacy
// mixed-case configs keep working until they are next re-saved.
func NormalizeContextName(s string) string { return kit.NormalizeContextName(s) }

// NormalizeUsername lowercases an *email* username and otherwise preserves the
// caller's casing, after trimming. Atlassian Cloud authenticates by email +
// API token and treats the address case-insensitively; Data Center / Server
// usernames may map to LDAP / AD identifiers that are case-sensitive on the
// server side, so they are not folded.
func NormalizeUsername(s string) string { return kit.NormalizeUsername(s) }

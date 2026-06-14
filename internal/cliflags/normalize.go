// Package cliflags rewrites argv before cobra parses it, absorbing two common
// slips an LLM makes when generating a command line:
//
//   - camelCase / snake_case flag names — `--userId` / `--user_name` instead of
//     the canonical kebab-case `--user-id` / `--user-name`;
//   - a flag stuck to its value — `--limit100` instead of `--limit 100`.
//
// A rewrite happens ONLY when the result is a flag the command tree actually
// defines (and, for the sticky split, an integer-typed one), so unknown tokens
// pass through untouched and still surface cobra's normal "unknown flag" error.
// Every rewrite is reported as a Correction so the caller can echo it back —
// fixing the call and teaching the canonical form in one go.
package cliflags

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Correction records one argv rewrite, for echoing back to the user/agent.
type Correction struct {
	Original  string `json:"original"`
	Corrected string `json:"corrected"`
	Kind      string `json:"kind"` // "flag-name" | "sticky-value"
}

// FlagInfo is the set of flags known to the command tree. Known holds every
// kebab-case flag name; Numeric is the subset whose value is integer/float
// typed (the only flags eligible for the sticky-value split).
type FlagInfo struct {
	Known   map[string]bool
	Numeric map[string]bool
}

// Collect walks the whole command tree and records every flag name (and which
// are numeric). It is intentionally tree-wide rather than per-command: argv has
// not been parsed yet, so the target subcommand is not known, and a flag that
// exists somewhere is a safe normalization target.
func Collect(root *cobra.Command) FlagInfo {
	info := FlagInfo{Known: map[string]bool{}, Numeric: map[string]bool{}}
	add := func(f *pflag.Flag) {
		info.Known[f.Name] = true
		if isNumericType(f.Value.Type()) {
			info.Numeric[f.Name] = true
		}
	}
	var visit func(c *cobra.Command)
	visit = func(c *cobra.Command) {
		c.Flags().VisitAll(add)
		c.PersistentFlags().VisitAll(add)
		for _, sub := range c.Commands() {
			visit(sub)
		}
	}
	visit(root)
	return info
}

// isNumericType reports whether a pflag value type is numeric, so a trailing
// digit run can be safely split off as the value.
func isNumericType(t string) bool {
	return strings.HasPrefix(t, "int") || strings.HasPrefix(t, "uint") ||
		strings.HasPrefix(t, "float") || t == "count"
}

// Normalize rewrites args (argv without the program name) per the package doc.
// It returns the rewritten args and the list of corrections applied. Tokens
// after the `--` end-of-flags marker, short flags, and unknown long flags are
// left untouched.
func Normalize(args []string, info FlagInfo) (out []string, corrections []Correction) {
	// Leave shell-completion invocations entirely alone: cobra drives those
	// with its own synthetic argv and rewriting would corrupt it.
	if len(args) > 0 && strings.HasPrefix(args[0], "__complete") {
		return args, nil
	}
	out = make([]string, 0, len(args))
	endFlags := false
	for _, tok := range args {
		if endFlags || !strings.HasPrefix(tok, "--") || tok == "--" {
			if tok == "--" {
				endFlags = true
			}
			out = append(out, tok)
			continue
		}
		name, val, hasEq := splitEq(tok[2:])

		// Already canonical: nothing to do.
		if info.Known[name] {
			out = append(out, tok)
			continue
		}
		// 1. Flag-name normalization (--userId -> --user-id).
		if norm := kebab(name); norm != name && info.Known[norm] {
			corrected := "--" + norm
			if hasEq {
				corrected += "=" + val
			}
			corrections = append(corrections, Correction{Original: tok, Corrected: corrected, Kind: "flag-name"})
			out = append(out, corrected)
			continue
		}
		// 2. Sticky-value split (--limit100 -> --limit 100), integer flags only.
		if !hasEq {
			if base, num, ok := splitSticky(name); ok {
				kb := kebab(base)
				if info.Numeric[kb] {
					corrections = append(corrections, Correction{
						Original: tok, Corrected: "--" + kb + " " + num, Kind: "sticky-value"})
					out = append(out, "--"+kb, num)
					continue
				}
			}
		}
		// Unknown: leave it for cobra to reject with its usual error.
		out = append(out, tok)
	}
	return out, corrections
}

// splitEq divides a flag body into name and value at the first '='.
func splitEq(body string) (name, val string, hasEq bool) {
	if i := strings.IndexByte(body, '='); i >= 0 {
		return body[:i], body[i+1:], true
	}
	return body, "", false
}

// splitSticky peels a trailing run of digits off a flag name. ok is false when
// there are no trailing digits or the token is all digits (no flag name left).
func splitSticky(name string) (base, num string, ok bool) {
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	if i == len(name) || i == 0 {
		return "", "", false
	}
	return name[:i], name[i:], true
}

// kebab converts a camelCase / snake_case flag name to kebab-case. It only acts
// on ASCII letters, so a non-ASCII token is returned unchanged and cannot be
// mistaken for a known flag.
func kebab(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_':
			b.WriteByte('-')
		case c >= 'A' && c <= 'Z':
			if i > 0 {
				p := s[i-1]
				if (p >= 'a' && p <= 'z') || (p >= '0' && p <= '9') {
					b.WriteByte('-')
				}
			}
			b.WriteByte(c - 'A' + 'a')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// FlagValues carries the global CLI flags that override configuration. Empty
// fields are ignored (not treated as overrides).
type FlagValues struct {
	BaseURL string
	Flavor  string
	Format  string
	Timeout string
}

func (f FlagValues) layer() map[string]string {
	m := map[string]string{}
	put(m, fieldServer, f.BaseURL)
	put(m, fieldFlavor, f.Flavor)
	put(m, fieldFormat, f.Format)
	put(m, fieldTimeout, f.Timeout)
	return m
}

// LoadOptions controls where configuration is read from. All fields are
// optional; sensible defaults are used when empty.
type LoadOptions struct {
	// ConfigDir overrides the directory containing config.yaml.
	ConfigDir string
	// DotenvPath overrides the .env file path. Empty means ".env".
	DotenvPath string
	// Flags carries global flag overrides (highest precedence).
	Flags FlagValues
	// Context selects a named context (from the --use-context flag). It wins
	// over CONFLUENCE_CONTEXT and the file's current_context.
	Context string
}

type namedLayer struct {
	name string
	data map[string]string
}

// selectContext picks the active context name for f. Precedence: the flag, the
// CONFLUENCE_CONTEXT env var, the file's current_context, the sole context,
// then a context literally named "default". It returns "" (no error) when no
// context can or need be selected — a missing file, or an ambiguous
// multi-context file with no current_context; the missing-server error is then
// raised later, at the point a server is actually needed. An override naming a
// context that does not exist is an error.
func selectContext(f File, flagCtx, envCtx string) (string, error) {
	pick := func(name, src string) (string, error) {
		if c, ok := f.Context(name); ok {
			// Return the canonical (as-stored) name so downstream lookups,
			// keychain account keys, and any user-facing labels all agree on
			// one spelling regardless of how the user wrote the override.
			return c.Name, nil
		}
		return "", cerrors.Newf(cerrors.CategoryConfig, "UNKNOWN_CONTEXT",
			"context %q (from %s) is not defined in the config file", name, src).
			WithHint(unknownContextHint(name, f.ContextNames()))
	}
	switch {
	case flagCtx != "":
		return pick(flagCtx, "--use-context")
	case envCtx != "":
		return pick(envCtx, "CONFLUENCE_CONTEXT")
	case f.CurrentContext != "":
		return pick(f.CurrentContext, "current_context")
	case len(f.Contexts) == 1:
		return f.Contexts[0].Name, nil
	default:
		if _, ok := f.Context(DefaultContextName); ok {
			return DefaultContextName, nil
		}
		return "", nil
	}
}

// UnknownContextHint builds the hint shown when a context override (flag,
// env, current_context, or a positional argument to a config subcommand)
// names a context that does not exist. It prefers a case-insensitive "did
// you mean" suggestion; otherwise it lists every available name; otherwise
// it falls back to a generic pointer to get-contexts.
//
// File.Context already does CI lookup, so the "did you mean" branch is now
// mostly a defensive belt — it can still fire for legacy configs that have
// two contexts differing only in case (CI lookup hits the first one in
// iteration order; a user typing the second one's casing still misses).
func UnknownContextHint(name string, available []string) string {
	return unknownContextHint(name, available)
}

func unknownContextHint(name string, available []string) string {
	for _, a := range available {
		if strings.EqualFold(a, name) && a != name {
			return fmt.Sprintf("Did you mean %q? Context names are case-sensitive.", a)
		}
	}
	if len(available) > 0 {
		return fmt.Sprintf("Available contexts: %s.", strings.Join(available, ", "))
	}
	return "Run `confluence-cli config get-contexts` to list defined contexts."
}

// buildFileLayer flattens the active context's fields plus the shared runtime
// defaults into a layer map. An empty ctxName yields just the defaults.
func buildFileLayer(f File, ctxName string) map[string]string {
	m := map[string]string{}
	if ctxName != "" {
		if c, ok := f.Context(ctxName); ok {
			put(m, fieldServer, c.BaseURL)
			put(m, fieldFlavor, c.Flavor)
			put(m, fieldDetectedFlavor, c.DetectedFlavor)
			put(m, fieldAuthScheme, c.Auth.Scheme)
			put(m, fieldAuthUsername, c.Auth.Username)
		}
	}
	put(m, fieldFormat, f.Defaults.Format)
	if f.Defaults.PageSize > 0 {
		m[fieldPageSize] = strconv.Itoa(f.Defaults.PageSize)
	}
	if f.Defaults.Timeout > 0 {
		m[fieldTimeout] = f.Defaults.Timeout.String()
	}
	if f.Defaults.MaxRetries > 0 {
		m[fieldMaxRetries] = strconv.Itoa(f.Defaults.MaxRetries)
	}
	if f.Defaults.ReadOnly {
		m[fieldReadOnly] = "true"
	}
	return m
}

// Load resolves configuration from all sources and returns the merged result
// with per-field provenance.
func Load(opt LoadOptions) (*Resolved, error) {
	dir := opt.ConfigDir
	if dir == "" {
		d, err := ResolveConfigDir()
		if err != nil {
			return nil, err
		}
		dir = d
	}
	file, _, err := ReadFile(dir)
	if err != nil {
		return nil, err
	}
	ctxName, err := selectContext(file, opt.Context, os.Getenv("CONFLUENCE_CONTEXT"))
	if err != nil {
		return nil, err
	}
	fileLayer := buildFileLayer(file, ctxName)

	dotenvPath := opt.DotenvPath
	if dotenvPath == "" {
		dotenvPath = ".env"
	}
	dotLayer, err := dotenvLayer(dotenvPath)
	if err != nil {
		return nil, err
	}

	// Lowest precedence first.
	layers := []namedLayer{
		{"default", defaultLayer()},
		{"file", fileLayer},
		{"dotenv", dotLayer},
		{"env", envLayer()},
		{"flag", opt.Flags.layer()},
	}

	merged := map[string]string{}
	sources := map[string]string{}
	for _, l := range layers {
		for k, v := range l.data {
			merged[k] = v
			sources[k] = l.name
		}
	}

	return &Resolved{
		Config: configFromMap(merged),
		Secrets: Secrets{
			PAT:      merged[fieldPAT],
			Password: merged[fieldPassword],
			APIToken: merged[fieldAPIToken],
		},
		Sources:       sources,
		ActiveContext: ctxName,
		ContextNames:  file.ContextNames(),
	}, nil
}

// ExplainField returns a human-readable provenance label for a field key,
// e.g. ExplainField(sources, "server") -> "env". Unknown fields report "default".
func ExplainField(sources map[string]string, field string) string {
	if s, ok := sources[field]; ok {
		return s
	}
	return "default"
}

// Field key accessors for callers outside this package (e.g. config show).
const (
	FieldServer     = fieldServer
	FieldFlavor     = fieldFlavor
	FieldAuthScheme = fieldAuthScheme
	FieldAuthUser   = fieldAuthUsername
	FieldFormat     = fieldFormat
	FieldTimeout    = fieldTimeout
	FieldPageSize   = fieldPageSize
	FieldMaxRetries = fieldMaxRetries
	FieldReadOnly   = fieldReadOnly
)

package config

import (
	kit "github.com/angelmsger/agent-cli-kit/config"
)

// FlagValues carries the global CLI flags that override configuration. Empty
// fields are ignored (not treated as overrides).
type FlagValues struct {
	AuthScheme    string `config:"auth.scheme"`
	CredentialURL string `config:"auth.credential_url"`
	BaseURL       string `config:"server"`
	Flavor        string `config:"flavor"`
	Format        string `config:"defaults.format"`
	Timeout       string `config:"defaults.timeout"`
}

// LoadOptions controls where configuration is read from. All fields are
// optional; sensible defaults are used when empty.
type LoadOptions struct {
	// Setup resolves Context even when new, ignores runtime context selection,
	// and excludes personal environment fields before auth-scheme inference.
	Setup bool
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

// Load resolves configuration from all sources and returns the merged result
// with per-field provenance. Context selection is the flag,
// CONFLUENCE_CONTEXT, the file's current_context, the sole context, then a
// context literally named "default"; naming a context that does not exist is
// UNKNOWN_CONTEXT. A detected flavor is dropped when the server in effect is
// not the stored one, and Cloud defaults to basic auth.
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
	r, err := kit.Load(file, kit.LoadOptions{
		Setup:      opt.Setup,
		DotenvPath: opt.DotenvPath,
		Flags:      kit.Encode(opt.Flags),
		Context:    opt.Context,
	})
	if err != nil {
		return nil, err
	}
	out := &Resolved{
		Config:        configOf(r.Values),
		Sources:       r.Sources,
		ActiveContext: r.ActiveContext,
		ContextSource: r.ContextSource,
		ContextNames:  r.ContextNames,
	}
	kit.Decode(r.Values, &out.Secrets)
	out.Config.CredentialBaseURL = r.StoredServer
	return out, nil
}

// StoredContext resolves only persisted context data and built-in defaults.
// Credential reuse must not inherit another context's environment or .env identity.
func StoredContext(context NamedContext, defaults Defaults) Config {
	return configOf(kit.StoredContext(context, defaults))
}

// ExplainField returns a human-readable provenance label for a field key,
// e.g. ExplainField(sources, "server") -> "env". Unknown fields report "default".
func ExplainField(sources map[string]string, field string) string {
	return kit.ExplainField(sources, field)
}

// UnknownContextHint builds the hint shown when a context override (flag,
// env, current_context, or a positional argument to a config subcommand)
// names a context that does not exist: every available name, otherwise a
// pointer to get-contexts.
func UnknownContextHint(name string, available []string) string {
	return kit.UnknownContextHint(name, available)
}

// ReminderBlock is the --help block shown when several contexts exist.
func ReminderBlock(active, source string, names []string) string {
	return kit.ReminderBlock(active, source, names)
}

// ContextNotice is the _notice payload for a command that runs against one of
// several contexts chosen implicitly.
func ContextNotice(active, source string, names []string) map[string]any {
	return kit.ContextNotice(active, source, names)
}

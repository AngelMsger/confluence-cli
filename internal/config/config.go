// Package config resolves CLI configuration from layered sources: CLI flags,
// environment variables, a .env file, a YAML config file and built-in
// defaults, in that precedence order (highest first).
//
// The engine — layering, provenance, context selection, the atomic file
// write and team presets — comes from agent-cli-kit/config. This package
// registers confluence-cli's fields, environment variables, file location,
// command wording and flavor rules with it (schema.go), keeps the YAML shape
// of the file (file.go), and keeps the names the rest of the repo uses.
//
// Secrets (tokens, passwords) are never stored in the YAML config file. They
// are surfaced through Resolved.Secrets when supplied via flags/env/.env, or
// loaded from the OS keychain by the auth package.
package config

import (
	"time"

	kit "github.com/angelmsger/agent-cli-kit/config"
)

// Flavor values for the Confluence backend.
const (
	FlavorAuto       = "auto"
	FlavorCloud      = "cloud"
	FlavorDataCenter = "datacenter"
)

// Auth scheme values.
const (
	SchemePAT   = "pat"
	SchemeBasic = "basic"
)

// DefaultContextName is the name given to an unnamed context, and to the
// single context synthesized from a legacy flat config file.
const DefaultContextName = kit.DefaultContextName

// Context selection sources, reported on Resolved.ContextSource. The first two
// are explicit (the caller named a context for this invocation); the rest are
// implicit (the CLI fell back to a stored or sole context).
const (
	ContextSourceFlag    = kit.ContextSourceFlag    // --use-context
	ContextSourceEnv     = kit.ContextSourceEnv     // CONFLUENCE_CONTEXT
	ContextSourceCurrent = kit.ContextSourceCurrent // the file's current_context
	ContextSourceSingle  = kit.ContextSourceSingle  // the sole defined context
	ContextSourceDefault = kit.ContextSourceDefault // a context literally named "default"
	ContextSourceNone    = kit.ContextSourceNone    // nothing selected (no/empty config)
)

// NamedContext is one named Confluence server profile inside the config file.
// Runtime defaults are shared across contexts and live in File.Defaults.
type NamedContext struct {
	Name           string `config:"name"`
	BaseURL        string `config:"server"`
	Flavor         string `config:"flavor"`
	DetectedFlavor string `config:"detected_flavor"`
	Auth           AuthConfig
}

// Config holds the resolved, non-secret configuration.
type Config struct {
	// CredentialBaseURL preserves an equivalent stored URL's native lookup key.
	// It is runtime-only and never changes the request destination.
	CredentialBaseURL string `yaml:"-" json:"-"`
	// ReuseStep is the `auth reuse` command that goes before any sign-in step
	// for this context, or "" when the file offers nothing to reuse. It is
	// runtime-only, computed offline from the configuration file.
	ReuseStep      string     `yaml:"-" json:"-"`
	BaseURL        string     `yaml:"server" config:"server"`
	Flavor         string     `yaml:"flavor" config:"flavor"`
	DetectedFlavor string     `yaml:"detected_flavor,omitempty" config:"detected_flavor"`
	Auth           AuthConfig `yaml:"auth"`
	Defaults       Defaults   `yaml:"defaults"`
}

// AuthConfig holds non-secret auth settings.
type AuthConfig struct {
	CredentialURL string `yaml:"credential_url,omitempty" config:"auth.credential_url"`
	Scheme        string `yaml:"scheme" config:"auth.scheme"`
	Username      string `yaml:"username,omitempty" config:"auth.username"`
}

// Defaults holds tunable runtime defaults.
type Defaults struct {
	Format     string        `yaml:"format" config:"defaults.format"`
	PageSize   int           `yaml:"page_size" config:"defaults.page_size"`
	Timeout    time.Duration `yaml:"timeout" config:"defaults.timeout"`
	MaxRetries int           `yaml:"max_retries" config:"defaults.max_retries"`
	// ReadOnly blocks every mutating client method. Settable from the config
	// file, from CONFLUENCE_CLI_READ_ONLY, or temporarily overridden via
	// --allow-writes.
	ReadOnly bool `yaml:"read_only,omitempty" config:"defaults.read_only"`
}

// Secrets holds credentials observed in non-file layers. Empty fields mean the
// secret was not supplied via flags/env/.env and must come from the keychain.
type Secrets struct {
	PAT      string `config:"secret.pat"`
	Password string `config:"secret.password"`
	APIToken string `config:"secret.api_token"`
}

// Resolved is the outcome of Load: the merged Config plus provenance and any
// transient secrets.
type Resolved struct {
	Config  Config
	Secrets Secrets
	// Sources maps a field key (see the field* constants) to the layer name
	// that supplied its final value: "flag", "env", "dotenv", "file", "default".
	Sources map[string]string
	// ActiveContext is the name of the context whose fields were applied.
	// Empty when no config file (or no contexts) exists — pure-env usage.
	ActiveContext string
	// ContextSource records which precedence rule chose ActiveContext (one of
	// the ContextSource* constants), so callers can tell an explicit choice
	// (flag/env) from an implicit fallback (current_context, sole, default).
	ContextSource string
	// ContextNames lists every context defined in the file, in file order.
	ContextNames []string
}

// ContextSelectedExplicitly reports whether the active context was chosen for
// this invocation (via --use-context or CONFLUENCE_CONTEXT) rather than fallen
// back to from the file's current_context, the sole context, or a "default"
// context. It is the signal used to decide whether to nudge an agent that may
// not realise which of several contexts it is hitting.
func (r *Resolved) ContextSelectedExplicitly() bool {
	return r.ContextSource == ContextSourceFlag || r.ContextSource == ContextSourceEnv
}

// Field keys used for layer maps and provenance tracking.
const (
	fieldCredentialURL  = kit.KeyCredentialURL
	fieldServer         = kit.KeyServer
	fieldFlavor         = "flavor"
	fieldDetectedFlavor = "detected_flavor"
	fieldAuthScheme     = kit.KeyAuthScheme
	fieldAuthUsername   = "auth.username"
	fieldFormat         = "defaults.format"
	fieldPageSize       = "defaults.page_size"
	fieldTimeout        = "defaults.timeout"
	fieldMaxRetries     = "defaults.max_retries"
	fieldReadOnly       = "defaults.read_only"
	// Secret field keys (never persisted to the YAML file).
	fieldPAT      = "secret.pat"
	fieldPassword = "secret.password"
	fieldAPIToken = "secret.api_token"
)

// Field key accessors for callers outside this package (e.g. config show).
const (
	FieldServer        = fieldServer
	FieldCredentialURL = fieldCredentialURL
	FieldFlavor        = fieldFlavor
	FieldAuthScheme    = fieldAuthScheme
	FieldAuthUser      = fieldAuthUsername
	FieldFormat        = fieldFormat
	FieldTimeout       = fieldTimeout
	FieldPageSize      = fieldPageSize
	FieldMaxRetries    = fieldMaxRetries
	FieldReadOnly      = fieldReadOnly
)

// configOf decodes merged values into a Config.
func configOf(v kit.Values) Config {
	var c Config
	kit.Decode(v, &c)
	return c
}

// defaultConfig is the configuration the built-in defaults alone resolve to.
func defaultConfig() Config { return configOf(kit.DefaultValues()) }

// contextConfig builds a Config from a NamedContext plus the built-in runtime
// defaults. It is used to feed the wizard's detect/validate hooks, which
// operate on a whole Config.
func contextConfig(nc NamedContext) Config {
	c := defaultConfig()
	c.BaseURL = nc.BaseURL
	c.Flavor = nc.Flavor
	c.DetectedFlavor = nc.DetectedFlavor
	c.Auth = nc.Auth
	return c
}

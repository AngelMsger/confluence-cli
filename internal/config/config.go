// Package config resolves CLI configuration from layered sources: CLI flags,
// environment variables, a .env file, a YAML config file and built-in
// defaults, in that precedence order (highest first).
//
// Secrets (tokens, passwords) are never stored in the YAML config file. They
// are surfaced through Resolved.Secrets when supplied via flags/env/.env, or
// loaded from the OS keychain by the auth package.
package config

import (
	"strconv"
	"strings"
	"time"

	"github.com/angelmsger/confluence-cli/pkg/constants"
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
const DefaultContextName = "default"

// NamedContext is one named Confluence server profile inside the config file.
// Runtime defaults are shared across contexts and live in File.Defaults.
type NamedContext struct {
	Name           string
	BaseURL        string
	Flavor         string
	DetectedFlavor string
	Auth           AuthConfig
}

// Config holds the resolved, non-secret configuration.
type Config struct {
	BaseURL        string     `yaml:"server"`
	Flavor         string     `yaml:"flavor"`
	DetectedFlavor string     `yaml:"detected_flavor,omitempty"`
	Auth           AuthConfig `yaml:"auth"`
	Defaults       Defaults   `yaml:"defaults"`
}

// AuthConfig holds non-secret auth settings.
type AuthConfig struct {
	Scheme   string `yaml:"scheme"`
	Username string `yaml:"username,omitempty"`
}

// Defaults holds tunable runtime defaults.
type Defaults struct {
	Format     string        `yaml:"format"`
	PageSize   int           `yaml:"page_size"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxRetries int           `yaml:"max_retries"`
	// ReadOnly blocks every mutating client method. Settable from the config
	// file, from CONFLUENCE_CLI_READ_ONLY, or temporarily overridden via
	// --allow-writes.
	ReadOnly bool `yaml:"read_only,omitempty"`
}

// Secrets holds credentials observed in non-file layers. Empty fields mean the
// secret was not supplied via flags/env/.env and must come from the keychain.
type Secrets struct {
	PAT      string
	Password string
	APIToken string
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
	// ContextNames lists every context defined in the file, in file order.
	ContextNames []string
}

// Field keys used for layer maps and provenance tracking.
const (
	fieldServer         = "server"
	fieldFlavor         = "flavor"
	fieldDetectedFlavor = "detected_flavor"
	fieldAuthScheme     = "auth.scheme"
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

// defaultLayer returns the built-in defaults as a layer map.
func defaultLayer() map[string]string {
	return map[string]string{
		fieldFlavor:     FlavorAuto,
		fieldAuthScheme: SchemePAT,
		fieldFormat:     constants.DefaultFormat,
		fieldPageSize:   strconv.Itoa(constants.DefaultPageSize),
		fieldTimeout:    constants.DefaultTimeout.String(),
		fieldMaxRetries: strconv.Itoa(constants.DefaultMaxRetries),
	}
}

// configFromMap builds a Config from a fully merged layer map.
func configFromMap(m map[string]string) Config {
	c := Config{
		BaseURL:        m[fieldServer],
		Flavor:         m[fieldFlavor],
		DetectedFlavor: m[fieldDetectedFlavor],
		Auth: AuthConfig{
			Scheme:   m[fieldAuthScheme],
			Username: m[fieldAuthUsername],
		},
		Defaults: Defaults{
			Format:     m[fieldFormat],
			PageSize:   atoiOr(m[fieldPageSize], constants.DefaultPageSize),
			Timeout:    durationOr(m[fieldTimeout], constants.DefaultTimeout),
			MaxRetries: atoiOr(m[fieldMaxRetries], constants.DefaultMaxRetries),
			ReadOnly:   boolOr(m[fieldReadOnly], false),
		},
	}
	return c
}

// boolOr parses a flag-style truthy string. "1", "true", "yes", "on" count as
// true; everything else (including empty) yields the fallback.
func boolOr(s string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

// contextConfig builds a Config from a NamedContext plus the built-in runtime
// defaults. It is used to feed the wizard's detect/validate hooks, which
// operate on a whole Config.
func contextConfig(nc NamedContext) Config {
	c := configFromMap(defaultLayer())
	c.BaseURL = nc.BaseURL
	c.Flavor = nc.Flavor
	c.DetectedFlavor = nc.DetectedFlavor
	c.Auth = nc.Auth
	return c
}

func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}

func durationOr(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return fallback
}

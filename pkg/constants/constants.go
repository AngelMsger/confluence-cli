// Package constants holds project-wide constants and build-time metadata.
package constants

import "time"

// Build-time metadata, injected via -ldflags. See Makefile.
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

const (
	// AppName is the binary / command name.
	AppName = "confluence-cli"

	// EnvPrefix is the environment variable prefix for all settings.
	EnvPrefix = "CONFLUENCE_"

	// ConfigParentDirName groups every angelmsger CLI's per-user config under
	// one shared $HOME-relative directory (~/.angelmsger).
	ConfigParentDirName = ".angelmsger"

	// ConfigDirName is the per-CLI config directory under ConfigParentDirName,
	// i.e. ~/.angelmsger/confluence.
	ConfigDirName = "confluence"

	// LegacyConfigDirName is the pre-0.7 per-user config directory at
	// ~/.confluence. It is consulted as a read-write fallback when the new
	// location does not yet exist, so existing users keep working until they
	// move the directory manually.
	LegacyConfigDirName = ".confluence"

	// ConfigFileName is the YAML config file within ConfigDirName.
	ConfigFileName = "config.yaml"

	// CredentialsFileName is the fallback secret store when no keychain is available.
	CredentialsFileName = "credentials"

	// KeychainService is the service name used for OS keychain entries.
	KeychainService = "confluence-cli"
)

// Defaults for runtime behaviour.
const (
	DefaultFormat     = "json"
	DefaultPageSize   = 25
	DefaultTimeout    = 30 * time.Second
	DefaultMaxRetries = 3
	// MaxPageSize caps a single API page request.
	MaxPageSize = 250
)

// UserAgent identifies the CLI to the Confluence server.
func UserAgent() string {
	return AppName + "/" + Version
}

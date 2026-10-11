package config

import (
	"path/filepath"
	"strconv"

	kit "github.com/angelmsger/agent-cli-kit/config"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors" // also installs Confluence's error guidance
)

// EnvNoContextHint opts out of the runtime multi-context notice.
const EnvNoContextHint = "CONFLUENCE_CLI_NO_CONTEXT_HINT"

func init() {
	kit.Register(&kit.Schema{
		// Each row names a field's environment variable and built-in
		// default. Service fields are the team presets a setup load keeps;
		// the user name and secrets are personal.
		Fields: []kit.Field{
			{Key: fieldServer, Env: []string{"CONFLUENCE_SERVER"}, Service: true},
			{Key: fieldFlavor, Env: []string{"CONFLUENCE_FLAVOR"}, Default: FlavorAuto, Service: true},
			// The flavor a probe detected is a cache: it no longer holds once
			// the server or the configured flavor changes.
			{Key: fieldDetectedFlavor, ResetBy: []string{fieldServer, fieldFlavor}},
			{Key: fieldCredentialURL, Env: []string{"CONFLUENCE_CREDENTIAL_URL"}, Service: true},
			{Key: fieldAuthScheme, Env: []string{"CONFLUENCE_AUTH_SCHEME"}, Default: SchemePAT, Service: true},
			{Key: fieldAuthUsername, Env: []string{"CONFLUENCE_USERNAME"}},
			{Key: fieldFormat, Env: []string{"CONFLUENCE_FORMAT"}, Default: constants.DefaultFormat, Service: true},
			{Key: fieldPageSize, Default: strconv.Itoa(constants.DefaultPageSize)},
			{Key: fieldTimeout, Default: constants.DefaultTimeout.String()},
			{Key: fieldMaxRetries, Default: strconv.Itoa(constants.DefaultMaxRetries)},
			{Key: fieldReadOnly, Env: []string{"CONFLUENCE_CLI_READ_ONLY"}, Service: true},
			// A secret in the environment selects its scheme; a personal
			// access token wins over a password, and a password over an API
			// token.
			{Key: fieldPAT, Env: []string{"CONFLUENCE_PERSONAL_ACCESS_TOKEN"}, Implies: SchemePAT},
			{Key: fieldPassword, Env: []string{"CONFLUENCE_PASSWORD"}, Implies: SchemeBasic},
			{Key: fieldAPIToken, Env: []string{"CONFLUENCE_API_TOKEN"}, Implies: SchemeBasic},
		},
		Paths: kit.Paths{
			Dir:    filepath.Join(constants.ConfigParentDirName, constants.ConfigDirName),
			File:   constants.ConfigFileName,
			Legacy: constants.LegacyConfigDirName,
		},
		Words: kit.Words{
			App:          constants.AppName,
			ContextFlag:  "--use-context",
			ContextEnv:   "CONFLUENCE_CONTEXT",
			NoHintEnv:    EnvNoContextHint,
			ListContexts: "config get-contexts",
			UseContext:   "config use-context",
			SetContext:   "config set-context",
			AuthGuide:    "auth guide",
			AuthLogin:    "auth login",
		},
		Schemes:    []string{SchemePAT, SchemeBasic},
		AfterMerge: resolveAuthDefaults,
		Validate:   validateFlavor,
	})
}

// resolveAuthDefaults picks basic auth when no layer chose a scheme and the
// service is known to be Cloud, where personal access tokens do not work.
func resolveAuthDefaults(values kit.Values, sources map[string]string) {
	if sources[fieldAuthScheme] == "default" && setupFlavor(configOf(values)) == FlavorCloud {
		values[fieldAuthScheme] = SchemeBasic
	}
}

// validateFlavor checks the flavor and that Cloud is paired with basic auth.
func validateFlavor(v kit.Values) error {
	cfg := configOf(v)
	if cfg.Flavor != "" && cfg.Flavor != FlavorAuto && cfg.Flavor != FlavorCloud && cfg.Flavor != FlavorDataCenter {
		return cerrors.New(cerrors.CategoryConfig, "BAD_FLAVOR", "flavor must be auto, cloud or datacenter")
	}
	if setupFlavor(cfg) == FlavorCloud && cfg.Auth.Scheme == SchemePAT {
		return cerrors.New(cerrors.CategoryConfig, "AUTH_CLOUD_NEEDS_BASIC", "Cloud requires email and an API token using basic authentication")
	}
	return nil
}

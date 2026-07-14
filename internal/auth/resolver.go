package auth

import (
	"errors"

	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// Resolve produces a Credential from configuration. A secret supplied via
// flags/env/.env (carried in secrets) takes precedence; otherwise the secret
// is loaded from the Store. The returned credential is validated.
func Resolve(cfg config.Config, secrets config.Secrets, store *Store) (Credential, error) {
	scheme := cfg.Auth.Scheme
	if scheme == "" {
		scheme = SchemePAT
	}
	cred := Credential{Scheme: scheme, Username: cfg.Auth.Username}

	switch scheme {
	case SchemePAT:
		cred.Secret = secrets.PAT
	case SchemeBasic:
		if secrets.Password != "" {
			cred.Secret = secrets.Password
		} else {
			cred.Secret = secrets.APIToken
		}
	}

	if cred.Secret == "" && store != nil && cfg.BaseURL != "" {
		loaded, err := store.Load(AccountKey(cfg.BaseURL, scheme))
		switch {
		case err == nil:
			cred.Secret = loaded
		case errors.Is(err, ErrSecretNotFound):
			return Credential{}, credentialNotVisibleOrMissingError()
		default:
			return Credential{}, credentialStoreInaccessibleError(err)
		}
	}

	if err := cred.Validate(); err != nil {
		return Credential{}, err
	}
	return cred, nil
}

func credentialNotVisibleOrMissingError() error {
	return cerrors.New(cerrors.CategoryConfig, "CREDENTIAL_NOT_VISIBLE_OR_MISSING",
		"stored Confluence credentials are missing or not visible in this execution environment").
		WithHint("An agent sandbox may have a different home or keychain view even when the host is already configured.").
		WithNextSteps(
			"Retry the same command with access to the host user environment (home directory and OS keychain).",
			"confluence-cli doctor",
			"Only if the host retry also reports missing credentials, run `confluence-cli config init` in the user's terminal or set CONFLUENCE_* environment variables.").
		WithRecovery(hostCredentialRecovery())
}

func credentialStoreInaccessibleError(err error) error {
	return cerrors.Wrap(err, cerrors.CategoryConfig, "CREDENTIAL_STORE_INACCESSIBLE",
		"stored Confluence credentials cannot be read in this execution environment").
		WithHint("The configured credential store is inaccessible; this commonly happens when an agent sandbox cannot access the host keychain or credential file.").
		WithNextSteps(
			"Retry the same command with access to the host user environment (home directory and OS keychain).",
			"confluence-cli doctor",
			"Do not run `config init` unless the same check also fails in the host environment.").
		WithRecovery(hostCredentialRecovery())
}

func hostCredentialRecovery() cerrors.Recovery {
	return cerrors.Recovery{
		Action:   "retry_current_command",
		Scope:    "host",
		Requires: []string{"user_home", "os_keychain"},
	}
}

// Save stores a credential's secret for later resolution and returns the
// backend ("keychain" or "file") that accepted it.
func Save(baseURL string, cred Credential, store *Store) (string, error) {
	if err := cred.Validate(); err != nil {
		return "", err
	}
	return store.Save(AccountKey(baseURL, cred.Scheme), cred.Secret)
}

// Forget removes any stored secret for the base URL and scheme.
func Forget(baseURL, scheme string, store *Store) error {
	return store.Delete(AccountKey(baseURL, scheme))
}

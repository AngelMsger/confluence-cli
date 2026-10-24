package auth

import (
	kit "github.com/angelmsger/agent-cli-kit/auth"
	"github.com/angelmsger/agent-cli-kit/credstore"
	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// Resolve produces a Credential from configuration. A secret supplied via
// flags/env/.env (carried in secrets) takes precedence; otherwise the secret
// is loaded from the Store. The returned credential is validated.
func Resolve(cfg config.Config, secrets config.Secrets, store *Store) (Credential, error) {
	cred, _, err := ResolveLookup(cfg, secrets, store)
	return cred, err
}

// ResolveLookup is Resolve that also reports what the store answered, so
// credential reuse can judge how far the answer can be trusted.
func ResolveLookup(cfg config.Config, secrets config.Secrets, store *Store) (Credential, credstore.Lookup, error) {
	var missingSteps []string
	if cfg.ReuseStep != "" {
		// The host retry stays first; reuse goes before any step that signs in.
		missingSteps = []string{cfg.ReuseStep, missingStep}
	}
	cred, lookup, err := kit.ResolveLookup(kit.Request{
		Scheme:       cfg.Auth.Scheme,
		Username:     cfg.Auth.Username,
		Secrets:      map[string]string{SchemePAT: secrets.PAT, SchemeBasic: basicSecret(secrets)},
		BaseURL:      cfg.BaseURL,
		LookupURL:    func() (string, error) { return CredentialLookupURL(cfg) },
		MissingSteps: missingSteps,
	}, store)
	if err != nil {
		return Credential{}, lookup, config.WithCredentialGuide(withReuseStep(err, cfg), cfg)
	}
	return cred, lookup, nil
}

// basicSecret is the basic scheme's secret: the password when one is set,
// otherwise the API token (Cloud pairs an email with an API token).
func basicSecret(secrets config.Secrets) string {
	if secrets.Password != "" {
		return secrets.Password
	}
	return secrets.APIToken
}

// Save stores a credential's secret for later resolution and returns the
// backend ("keychain" or "file") that accepted it.
func Save(baseURL string, cred Credential, store *Store) (string, error) {
	return kit.Save(baseURL, cred, store)
}

// Forget removes any stored secret for the base URL and scheme.
func Forget(baseURL, scheme string, store *Store) error {
	return kit.Forget(baseURL, scheme, store)
}

// CredentialLookupURL retains legacy store addressing only for the same complete service.
func CredentialLookupURL(cfg config.Config) (string, error) {
	if cfg.CredentialBaseURL == "" {
		return cfg.BaseURL, nil
	}
	target, e1 := config.NormalizeServiceURL(cfg.BaseURL)
	stored, e2 := config.NormalizeServiceURL(cfg.CredentialBaseURL)
	if e1 != nil || e2 != nil || target != stored {
		return "", kit.ServiceMismatchError()
	}
	return cfg.CredentialBaseURL, nil
}

// ForgetForConfig removes the same entry that configured requests resolve.
func ForgetForConfig(cfg config.Config, scheme string, store *Store) error {
	base, err := CredentialLookupURL(cfg)
	if err != nil {
		return err
	}
	return Forget(base, scheme, store)
}

// withReuseStep puts `auth reuse` first when a context lacks a secret or an
// identity that a stored login could supply.
func withReuseStep(err error, cfg config.Config) error {
	ce := cerrors.AsCLIError(err)
	switch ce.Code {
	case "AUTH_NO_BASIC", "AUTH_NO_TOKEN":
		if cfg.ReuseStep != "" {
			return ce.WithNextSteps(append([]string{cfg.ReuseStep}, ce.NextSteps...)...)
		}
	}
	return err
}

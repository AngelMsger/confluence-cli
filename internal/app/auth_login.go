package app

import (
	"context"
	"reflect"
	"strings"

	"github.com/angelmsger/agent-cli-kit/reuse"
	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// loginServices keeps verification and persistence independently testable.
type loginServices struct {
	Verify func(config.Config, auth.Credential) error
	Save   func(string, auth.Credential) (string, error)
	Write  func(string, config.File) error
}

func (s *appState) loginServices() loginServices {
	return loginServices{Verify: func(cfg config.Config, cred auth.Credential) error { return verifyCredential(s, cfg, cred) },
		Save: func(base string, cred auth.Credential) (string, error) { return auth.Save(base, cred, s.store) }, Write: config.WriteFile}
}

// loginFile checks the complete service identity before any verification or secret write.
func loginFile(s *appState, cfg config.Config, cred auth.Credential) (config.File, string, error) {
	base, err := config.NormalizeServiceURL(cfg.BaseURL)
	if err != nil {
		return config.File{}, "", err
	}
	file, _, err := config.ReadFile(s.cfgDir)
	if err != nil {
		return config.File{}, "", cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_READ", "failed to read login target")
	}
	name := s.resolved.ActiveContext
	if name == "" {
		name = config.DefaultContextName
	}
	nc, found := file.Context(name)
	if found {
		name = nc.Name
	}
	if found && nc.BaseURL != "" {
		previous, e := config.NormalizeServiceURL(nc.BaseURL)
		if e != nil || previous != base {
			return config.File{}, "", cerrors.New(cerrors.CategoryConfig, "CONTEXT_BASE_URL_MISMATCH", "login target differs from the configured context; no credential was stored").WithDetails(map[string]string{"context": name, "server": base}).WithHint("Select or create a context for the intended service before logging in.").WithNextSteps(constants.AppName+" config set-context <name> --base-url <url>", constants.AppName+" --use-context <name> auth login")
		}
	}
	if !found {
		nc = config.NamedContext{Name: name, Flavor: cfg.Flavor, DetectedFlavor: cfg.DetectedFlavor}
	}
	if nc.BaseURL == "" {
		// Preserve legacy keychain lookup for bare-host input whose normalized key differs.
		nc.BaseURL = base
		if auth.AccountKey(base, cred.Scheme) != auth.AccountKey(cfg.BaseURL, cred.Scheme) {
			nc.BaseURL = cfg.BaseURL
		}
	}
	// Equivalent URL spellings can still have different legacy keychain keys.
	// Record the spelling used by resolution so login also works after env overrides disappear.
	if auth.AccountKey(nc.BaseURL, cred.Scheme) != auth.AccountKey(cfg.BaseURL, cred.Scheme) {
		nc.BaseURL = cfg.BaseURL
	}
	nc.Auth.Scheme = cred.Scheme
	nc.Auth.Username = cred.Username
	if !found {
		nc.Auth.CredentialURL = cfg.Auth.CredentialURL
	}
	updated := false
	for i, c := range file.Contexts {
		if c.Name == name {
			file.Contexts[i] = nc
			updated = true
			break
		}
	}
	if !updated {
		file.Contexts = append(file.Contexts, nc)
	}
	if file.CurrentContext == "" {
		file.CurrentContext = name
	}
	return file, name, nil
}

// completeLogin reports partial persistence honestly; it never reports a stored
// secret as a completed login until the associated identity can be reloaded.
func completeLogin(s *appState, cfg config.Config, cred auth.Credential, services loginServices) (string, error) {
	if err := cred.Validate(); err != nil {
		return "", err
	}
	file, name, err := loginFile(s, cfg, cred)
	if err != nil {
		return "", err
	}
	original, _, err := config.ReadFile(s.cfgDir)
	if err != nil {
		return "", cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_READ", "failed to read login configuration")
	}
	if err := services.Verify(cfg, cred); err != nil {
		return "", err
	}
	backend, err := services.Save(cfg.BaseURL, cred)
	if err != nil {
		message := "credentials verified, but credential storage did not complete; login identity was not changed"
		if backend != "" {
			message = "credential stored in " + backend + ", but stale credential cleanup failed; login identity was not changed"
		}
		return "", credentialStoreRecovery(err, "CREDENTIAL_SAVE_FAILED", message).
			WithDetails(map[string]any{"credential_stored": backend != "", "stored_in": backend})
	}
	if !reflect.DeepEqual(original, file) {
		if err := services.Write(s.cfgDir, file); err != nil {
			return "", cerrors.Wrap(err, cerrors.CategoryConfig, "LOGIN_CONFIG_WRITE_FAILED", "credential stored, but login identity could not be saved").WithDetails(map[string]any{"context": name, "server": cfg.BaseURL, "scheme": cred.Scheme, "credential_stored": true}).WithHint("Fix config-directory access, then run auth login to save a matching credential and identity.").WithNextSteps(constants.AppName+" auth status", constants.AppName+" auth login")
		}
	}
	return backend, nil
}

func verifyCredential(s *appState, cfg config.Config, cred auth.Credential) error {
	_, err := verifyCredentialWithin(context.Background(), s, cfg, cred)
	return err
}

// verifyCredentialWithin verifies cred under parent, which may carry an earlier
// deadline than the request timeout, and reports the account it authenticated
// as from the same current-user response.
func verifyCredentialWithin(parent context.Context, s *appState, cfg config.Config, cred auth.Credential) (reuse.Account, error) {
	if err := cred.Validate(); err != nil {
		return reuse.Account{}, err
	}
	ctx, cancel := context.WithTimeout(parent, s.timeout())
	defer cancel()
	client, flavor, err := apiclient.Build(ctx, apiclient.BuildParams{BaseURL: cfg.BaseURL, Flavor: cfg.Flavor, AuthDecorator: cred.Decorator(), Timeout: s.timeout(), MaxRetries: cfg.Defaults.MaxRetries})
	if err != nil {
		return reuse.Account{}, err
	}
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return reuse.Account{}, err
	}
	if user == nil || (user.AccountID == "" && user.Username == "" && user.UserKey == "") || strings.EqualFold(user.Type, "anonymous") {
		return reuse.Account{}, cerrors.New(cerrors.CategoryAuth, "AUTH_IDENTITY_UNAVAILABLE", "the service did not return an authenticated user")
	}
	return canonicalAccount(flavor, user), nil
}

// canonicalAccount is the account's stable identifier on its deployment:
// accountId on Cloud, userKey on Data Center. A username is not substituted
// for a missing userKey: the two name different things, and a match between
// them would merge distinct users. An empty ID means the account is unknown.
func canonicalAccount(flavor apiclient.Flavor, user *apiclient.User) reuse.Account {
	id := user.AccountID
	if flavor != apiclient.FlavorCloud {
		id = user.UserKey
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	return reuse.Account{ID: id, Name: name}
}

// Store recovery requires access to the existing credentials, not a new token.
func credentialStoreRecovery(err error, code, message string) *cerrors.CLIError {
	return cerrors.Wrap(err, cerrors.CategoryConfig, code, message).
		WithHint("Restore access to the host user home and OS credential store, then retry the same command once. Do not request a new token or reinitialize the service configuration for a storage failure.").
		WithNextSteps("Retry the same command in the host environment with access to the existing credential store.").
		WithRecovery(cerrors.Recovery{Action: "retry_current_command", Scope: "host", Requires: []string{"user_home", "os_keychain"}})
}

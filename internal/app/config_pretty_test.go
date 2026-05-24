package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/angelmsger/confluence-cli/pkg/constants"
)

// TestRunWizardPrettyNoTTY verifies that --pretty refuses to start the huh
// TUI when stdin is not a terminal. The Go test binary runs with non-TTY
// stdin (the harness pipes to it), so this exercises exactly the path a
// user hits when running `config init --pretty < /dev/null`.
func TestRunWizardPrettyNoTTY(t *testing.T) {
	t.Parallel()
	if stdinIsTTY() {
		t.Skip("test harness reports a TTY stdin; this guard only fires off-TTY")
	}
	s := &appState{}
	s.gflags.pretty = true
	_, err := runWizard(s, config.WizardHooks{}, config.WizardInputs{})
	if err == nil {
		t.Fatal("expected --pretty without a TTY to be refused")
	}
	ce := cerrors.AsCLIError(err)
	if ce.Code != "PRETTY_NEEDS_TTY" {
		t.Errorf("error code = %q, want PRETTY_NEEDS_TTY", ce.Code)
	}
	if ce.Category != cerrors.CategoryUsage {
		t.Errorf("error category = %q, want %q", ce.Category, cerrors.CategoryUsage)
	}
}

// TestPersistInitResultPreservesOldCredentialOnWriteFailure proves that when
// the new config.yaml cannot be written (URL/scheme change scenario), the
// previously stored credential for the OLD account key is NOT deleted. This
// guards against an earlier ordering bug where Forget ran before WriteFile,
// so a write failure could leave the user with a still-active old config
// pointing at a now-vanished credential.
func TestPersistInitResultPreservesOldCredentialOnWriteFailure(t *testing.T) {
	t.Parallel()
	cfgDir := t.TempDir()

	// Pre-create config.yaml as a directory so os.WriteFile (used by
	// config.WriteFile) fails with "is a directory" — a synthetic WriteFile
	// failure without needing a mock filesystem.
	if err := os.MkdirAll(filepath.Join(cfgDir, constants.ConfigFileName), 0o755); err != nil {
		t.Fatal(err)
	}

	store := auth.NewStore(cfgDir)
	const (
		oldURL = "https://old.atlassian.net/wiki"
		newURL = "https://new.atlassian.net/wiki"
	)
	// Seed the OLD credential — this is the state we want to preserve.
	if _, err := store.Save(auth.AccountKey(oldURL, auth.SchemeBasic), "old-token"); err != nil {
		t.Fatalf("seed old cred: %v", err)
	}

	s := &appState{cfgDir: cfgDir, store: store}

	// Existing config still references the OLD URL.
	existing := config.File{
		CurrentContext: "default",
		Contexts: []config.NamedContext{{
			Name: "default", BaseURL: oldURL, Flavor: config.FlavorCloud,
			Auth: config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
		}},
	}
	// Wizard result rewrites the same context with a NEW URL.
	result := &config.WizardResult{
		File: config.File{
			CurrentContext: "default",
			Contexts: []config.NamedContext{{
				Name: "default", BaseURL: newURL, Flavor: config.FlavorCloud,
				Auth: config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
			}},
		},
		Creds: []config.ContextResult{{
			Context: config.NamedContext{
				Name: "default", BaseURL: newURL, Flavor: config.FlavorCloud,
				Auth: config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
			},
			Secrets: config.Secrets{APIToken: "new-token"},
		}},
	}

	_, err := persistInitResult(s, result, existing)
	if err == nil {
		t.Fatal("expected WriteFile to fail (config.yaml is a directory), got nil")
	}

	// The old credential MUST still be in the store — its removal was
	// deferred to after WriteFile succeeded, which it didn't.
	got, gerr := store.Load(auth.AccountKey(oldURL, auth.SchemeBasic))
	if gerr != nil {
		t.Fatalf("old credential should still exist; load err = %v", gerr)
	}
	if got != "old-token" {
		t.Errorf("old credential value = %q, want %q", got, "old-token")
	}
}

// TestPersistInitResultCleansUpOrphanAfterSuccess confirms the happy path:
// when persistence fully succeeds, the orphaned old credential is forgotten.
func TestPersistInitResultCleansUpOrphanAfterSuccess(t *testing.T) {
	t.Parallel()
	cfgDir := t.TempDir()
	store := auth.NewStore(cfgDir)
	const (
		oldURL = "https://old.atlassian.net/wiki"
		newURL = "https://new.atlassian.net/wiki"
	)
	if _, err := store.Save(auth.AccountKey(oldURL, auth.SchemeBasic), "old-token"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	s := &appState{cfgDir: cfgDir, store: store}
	existing := config.File{
		CurrentContext: "default",
		Contexts: []config.NamedContext{{
			Name: "default", BaseURL: oldURL, Flavor: config.FlavorCloud,
			Auth: config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
		}},
	}
	result := &config.WizardResult{
		File: config.File{
			CurrentContext: "default",
			Contexts: []config.NamedContext{{
				Name: "default", BaseURL: newURL, Flavor: config.FlavorCloud,
				Auth: config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
			}},
		},
		Creds: []config.ContextResult{{
			Context: config.NamedContext{
				Name: "default", BaseURL: newURL, Flavor: config.FlavorCloud,
				Auth: config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
			},
			Secrets: config.Secrets{APIToken: "new-token"},
		}},
	}

	if _, err := persistInitResult(s, result, existing); err != nil {
		t.Fatalf("persistInitResult: %v", err)
	}

	// New credential is stored.
	if v, err := store.Load(auth.AccountKey(newURL, auth.SchemeBasic)); err != nil || v != "new-token" {
		t.Errorf("new credential not stored: v=%q err=%v", v, err)
	}
	// Old credential is gone — cleaned up after WriteFile succeeded.
	if _, err := store.Load(auth.AccountKey(oldURL, auth.SchemeBasic)); err == nil {
		t.Error("old credential should have been cleaned up after successful persist")
	}
}

// TestPersistInitResultRejectsEmptyContextName guards against a UI bug that
// produces a context with an empty Name (e.g. an edit flow that failed to
// resolve the target context). The defensive check must reject it before any
// disk or keychain write.
func TestPersistInitResultRejectsEmptyContextName(t *testing.T) {
	t.Parallel()
	cfgDir := t.TempDir()
	s := &appState{cfgDir: cfgDir, store: auth.NewStore(cfgDir)}
	result := &config.WizardResult{
		File: config.File{
			CurrentContext: "",
			Contexts: []config.NamedContext{{
				Name: "", BaseURL: "https://acme.atlassian.net/wiki",
				Flavor: config.FlavorCloud,
				Auth:   config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
			}},
		},
		Creds: []config.ContextResult{{
			Context: config.NamedContext{
				Name: "", BaseURL: "https://acme.atlassian.net/wiki",
				Flavor: config.FlavorCloud,
				Auth:   config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
			},
			Secrets: config.Secrets{APIToken: "tok"},
		}},
	}

	_, err := persistInitResult(s, result, config.File{})
	if err == nil {
		t.Fatal("expected empty context name to be rejected")
	}
	ce := cerrors.AsCLIError(err)
	if ce.Code != "CTX_NAME_EMPTY" {
		t.Errorf("error code = %q, want CTX_NAME_EMPTY", ce.Code)
	}

	// Config file must not exist — the pipeline aborted before WriteFile.
	configPath := filepath.Join(cfgDir, constants.ConfigFileName)
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Errorf("config file should not have been written; stat err = %v", statErr)
	}
}

// TestPersistInitResultEmptySecretLeavesConfigUntouched proves the post-wizard
// pipeline rejects a context whose secret is empty BEFORE the config file is
// written, so a UI bug that lets an empty secret through cannot leave a
// half-written config.yaml pointing at a non-existent credential.
func TestPersistInitResultEmptySecretLeavesConfigUntouched(t *testing.T) {
	t.Parallel()
	cfgDir := t.TempDir()
	s := &appState{
		cfgDir: cfgDir,
		store:  auth.NewStore(cfgDir),
	}
	result := &config.WizardResult{
		File: config.File{
			CurrentContext: "default",
			Contexts: []config.NamedContext{{
				Name:    "default",
				BaseURL: "https://acme.atlassian.net/wiki",
				Flavor:  config.FlavorCloud,
				Auth:    config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
			}},
		},
		Creds: []config.ContextResult{{
			Context: config.NamedContext{
				Name:    "default",
				BaseURL: "https://acme.atlassian.net/wiki",
				Flavor:  config.FlavorCloud,
				Auth:    config.AuthConfig{Scheme: config.SchemeBasic, Username: "u@acme.com"},
			},
			Secrets: config.Secrets{}, // empty — the exact UI-bug shape
		}},
	}

	_, err := persistInitResult(s, result, config.File{})
	if err == nil {
		t.Fatal("expected an error for empty credential, got nil")
	}
	ce := cerrors.AsCLIError(err)
	if ce.Code != "CRED_INVALID" {
		t.Errorf("error code = %q, want CRED_INVALID", ce.Code)
	}

	// Config file must NOT exist — the pipeline aborted before WriteFile.
	configPath := filepath.Join(cfgDir, constants.ConfigFileName)
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Errorf("config file should not have been written; stat err = %v", statErr)
	}
}

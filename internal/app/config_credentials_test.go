package app

import (
	"errors"
	"os"
	"testing"

	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/zalando/go-keyring"
)

// A stored secret is keyed by the server's host and the scheme, so a team
// preset and a personal context on one Confluence share it — which is what
// `auth reuse` relies on. Losing it forces a new login on a context the user
// did not touch.
//
// The go-keyring mock is process-wide, so every test starts from a fresh one.

func storedSecret(t *testing.T, cfgDir, baseURL, scheme string) (string, error) {
	t.Helper()
	return auth.NewStore(cfgDir).Load(auth.AccountKey(baseURL, scheme))
}

func personalContext(name, baseURL, scheme string) config.NamedContext {
	nc := config.NamedContext{Name: name, BaseURL: baseURL, Flavor: config.FlavorDataCenter, Auth: config.AuthConfig{Scheme: scheme}}
	if scheme == config.SchemeBasic {
		nc.Auth.Username = "PersonalUser"
	}
	return nc
}

func secretsFor(scheme, secret string) config.Secrets {
	if scheme == config.SchemeBasic {
		return config.Secrets{Password: secret}
	}
	return config.Secrets{PAT: secret}
}

func saveSecret(t *testing.T, s *appState, nc config.NamedContext, secret string) {
	t.Helper()
	if _, err := auth.Save(nc.BaseURL, credentialFromContext(nc, secretsFor(nc.Auth.Scheme, secret)), s.store); err != nil {
		t.Fatal(err)
	}
}

// A preset stores the normalized URL, without a trailing slash; a URL typed
// into the wizard keeps the user's spelling. Re-running the wizard across that
// difference, or correcting the deployment path of a server, changes the URL
// but not the account, and must not delete the credential it has just saved.
func TestConfigInitKeepsTheCredentialWhenTheURLChangesOnTheSameHost(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"spelling", "https://confluence.example.test/wiki", "https://confluence.example.test/wiki/"},
		{"deployment path", "https://confluence.example.test", "https://confluence.example.test/wiki"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keyring.MockInit()
			s, _, _ := loginStateForTest(t)
			before := personalContext("default", tc.before, config.SchemePAT)
			saveSecret(t, s, before, "previous-token")
			existing := config.File{CurrentContext: "default", Contexts: []config.NamedContext{before}}
			edited := personalContext("default", tc.after, config.SchemePAT)
			result := &config.WizardResult{
				File:  config.File{CurrentContext: "default", Contexts: []config.NamedContext{edited}},
				Creds: []config.ContextResult{{Context: edited, Secrets: config.Secrets{PAT: "access-token"}}},
			}
			if _, err := persistInitResult(s, result, existing); err != nil {
				t.Fatal(err)
			}
			if got, err := storedSecret(t, s.cfgDir, edited.BaseURL, config.SchemePAT); err != nil || got != "access-token" {
				t.Fatalf("the wizard deleted the credential it had just saved: %q %v", got, err)
			}
		})
	}
}

// Moving a context to another server, or to another scheme, still clears the
// secret nothing uses any more, and leaves one that a second context resolves.
func TestConfigInitForgetsOnlyCredentialsNoContextUses(t *testing.T) {
	old := personalContext("default", "https://old.example.test/wiki", config.SchemePAT)
	for _, tc := range []struct {
		name   string
		edited config.NamedContext
		other  *config.NamedContext
		kept   bool
	}{
		{name: "moved server", edited: personalContext("default", "https://new.example.test/wiki", config.SchemePAT)},
		{name: "changed scheme", edited: personalContext("default", old.BaseURL, config.SchemeBasic)},
		{
			name:   "another context on the old server",
			edited: personalContext("default", "https://new.example.test/wiki", config.SchemePAT),
			other:  &config.NamedContext{Name: "team", BaseURL: "https://old.example.test/wiki/", Auth: config.AuthConfig{Scheme: config.SchemePAT}},
			kept:   true,
		},
		{
			name:   "another context with another scheme",
			edited: personalContext("default", "https://new.example.test/wiki", config.SchemePAT),
			other:  &config.NamedContext{Name: "team", BaseURL: old.BaseURL, Auth: config.AuthConfig{Scheme: config.SchemeBasic}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keyring.MockInit()
			s, _, _ := loginStateForTest(t)
			saveSecret(t, s, old, "old-secret")
			existing := config.File{CurrentContext: "default", Contexts: []config.NamedContext{old}}
			file := config.File{CurrentContext: "default", Contexts: []config.NamedContext{tc.edited}}
			if tc.other != nil {
				existing.Contexts = append(existing.Contexts, *tc.other)
				file.Contexts = append([]config.NamedContext{*tc.other}, tc.edited)
			}
			result := &config.WizardResult{File: file, Creds: []config.ContextResult{{Context: tc.edited, Secrets: secretsFor(tc.edited.Auth.Scheme, "new-secret")}}}
			if _, err := persistInitResult(s, result, existing); err != nil {
				t.Fatal(err)
			}
			if got, err := storedSecret(t, s.cfgDir, tc.edited.BaseURL, tc.edited.Auth.Scheme); err != nil || got != "new-secret" {
				t.Fatalf("new credential missing: %q %v", got, err)
			}
			got, err := storedSecret(t, s.cfgDir, old.BaseURL, old.Auth.Scheme)
			if tc.kept && (err != nil || got != "old-secret") {
				t.Fatalf("a credential another context still uses was deleted: %q %v", got, err)
			}
			if !tc.kept && !errors.Is(err, auth.ErrSecretNotFound) {
				t.Fatalf("an unused credential was left behind: %q %v", got, err)
			}
		})
	}
}

func TestDeleteContextKeepsACredentialAnotherContextUses(t *testing.T) {
	keyring.MockInit()
	s, _, _ := loginStateForTest(t)
	file := config.File{CurrentContext: "default", Contexts: []config.NamedContext{
		personalContext("default", "https://confluence.example.test/wiki/", config.SchemePAT),
		// A preset on the personal context's server shares its secret. Its scheme
		// is left to the default, as a context written by an older release may be.
		{Name: "team", BaseURL: "https://confluence.example.test/wiki", Flavor: config.FlavorDataCenter},
		personalContext("basic", "https://confluence.example.test/wiki", config.SchemeBasic),
		personalContext("elsewhere", "https://elsewhere.example.test/wiki", config.SchemePAT),
	}}
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	for _, c := range []config.NamedContext{file.Contexts[0], file.Contexts[2], file.Contexts[3]} {
		saveSecret(t, s, c, "secret-for-"+c.Name)
	}
	if _, err := runCLIFile(t, s.cfgDir, "config", "delete-context", "team"); err != nil {
		t.Fatal(err)
	}
	if got, err := storedSecret(t, s.cfgDir, file.Contexts[0].BaseURL, config.SchemePAT); err != nil || got != "secret-for-default" {
		t.Fatalf("deleting the preset removed the personal context's credential: %q %v", got, err)
	}
	// A context that is the only user of its account takes its secret with it:
	// another server, or another scheme on a shared one.
	for _, c := range []config.NamedContext{file.Contexts[3], file.Contexts[2]} {
		if _, err := runCLIFile(t, s.cfgDir, "config", "delete-context", c.Name); err != nil {
			t.Fatal(err)
		}
		if got, err := storedSecret(t, s.cfgDir, c.BaseURL, c.Auth.Scheme); !errors.Is(err, auth.ErrSecretNotFound) {
			t.Fatalf("%s: an unused credential was left behind: %q %v", c.Name, got, err)
		}
	}
	if got, err := storedSecret(t, s.cfgDir, file.Contexts[0].BaseURL, config.SchemePAT); err != nil || got != "secret-for-default" {
		t.Fatalf("an unrelated delete removed the remaining credential: %q %v", got, err)
	}
	after, _, err := config.ReadFile(s.cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Contexts) != 1 || after.Contexts[0].Name != "default" {
		t.Fatalf("unexpected contexts: %+v", after)
	}
}

// A context written without a scheme resolves basic on Cloud and pat elsewhere.
// Cleanup has to address the same account: a fixed default would delete the
// token of another context and leave the real secret behind.
func TestDeleteContextResolvesTheSchemeOfAContextWithoutOne(t *testing.T) {
	keyring.MockInit()
	s, _, _ := loginStateForTest(t)
	const cloud = "https://cloud.example.test/wiki"
	file := config.File{CurrentContext: "token", Contexts: []config.NamedContext{
		{Name: "token", BaseURL: cloud, Flavor: config.FlavorCloud, Auth: config.AuthConfig{Scheme: config.SchemePAT}},
		{Name: "personal", BaseURL: cloud, Flavor: config.FlavorCloud, Auth: config.AuthConfig{Scheme: config.SchemeBasic, Username: "member@example.test"}},
		{Name: "legacy", BaseURL: cloud, Flavor: config.FlavorCloud, Auth: config.AuthConfig{Username: "member@example.test"}},
	}}
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	saveSecret(t, s, file.Contexts[0], "bearer-token")
	saveSecret(t, s, file.Contexts[1], "api-token")
	want := func(step, scheme, secret string) {
		t.Helper()
		got, err := storedSecret(t, s.cfgDir, cloud, scheme)
		if secret == "" && !errors.Is(err, auth.ErrSecretNotFound) {
			t.Fatalf("%s: the %s credential was left behind: %q %v", step, scheme, got, err)
		}
		if secret != "" && (err != nil || got != secret) {
			t.Fatalf("%s: the %s credential was deleted: %q %v", step, scheme, got, err)
		}
	}
	// The legacy context still resolves the API token, so it survives.
	if _, err := runCLIFile(t, s.cfgDir, "config", "delete-context", "personal"); err != nil {
		t.Fatal(err)
	}
	want("deleted personal", config.SchemeBasic, "api-token")
	// Its own removal takes the API token, not the other context's bearer token.
	if _, err := runCLIFile(t, s.cfgDir, "config", "delete-context", "legacy"); err != nil {
		t.Fatal(err)
	}
	want("deleted legacy", config.SchemeBasic, "")
	want("deleted legacy", config.SchemePAT, "bearer-token")
}

// A failed config write leaves the context in the file, so its credential must
// stay resolvable as well.
func TestDeleteContextKeepsTheCredentialWhenTheConfigWriteFails(t *testing.T) {
	keyring.MockInit()
	s, _, _ := loginStateForTest(t)
	file := config.File{CurrentContext: "default", Contexts: []config.NamedContext{
		personalContext("default", "https://confluence.example.test/wiki", config.SchemePAT),
		personalContext("doomed", "https://doomed.example.test/wiki", config.SchemePAT),
	}}
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	saveSecret(t, s, file.Contexts[1], "doomed-token")
	// A read-only directory refuses the temporary file of the atomic replace.
	if err := os.Chmod(s.cfgDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(s.cfgDir, 0o700) })
	if _, err := runCLIFile(t, s.cfgDir, "config", "delete-context", "doomed"); err == nil {
		t.Skip("the config directory stayed writable (Windows or a privileged user)")
	}
	if got, err := storedSecret(t, s.cfgDir, file.Contexts[1].BaseURL, config.SchemePAT); err != nil || got != "doomed-token" {
		t.Fatalf("a failed delete-context removed the credential the context still names: %q %v", got, err)
	}
}

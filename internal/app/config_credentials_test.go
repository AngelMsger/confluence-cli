package app

import (
	"errors"
	"os"
	"testing"

	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
)

// A stored secret is keyed by the server's host and the scheme, so a team
// preset and a personal context on one server share it, as do two spellings of
// one URL. Losing it forces the user to issue a new token, so cleanup may only
// remove a secret no remaining context resolves.
//
// The go-keyring mock is process-wide: every test here uses its own hosts.

func storedSecret(t *testing.T, cfgDir, baseURL string) (string, error) {
	t.Helper()
	return auth.NewStore(cfgDir).Load(auth.AccountKey(baseURL, config.SchemePAT))
}

func patContext(name, baseURL string) config.NamedContext {
	return config.NamedContext{Name: name, BaseURL: baseURL, Flavor: config.FlavorDataCenter,
		Auth: config.AuthConfig{Scheme: config.SchemePAT}}
}

// A preset stores the URL without a trailing slash; a wizard answer may spell
// it with one, or move the context to another path on the same host. Neither
// edit changes the account key, so re-running the wizard must not delete the
// credential it has just saved.
func TestConfigInitKeepsTheCredentialWhenTheAccountKeyIsUnchanged(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"trailing slash", "https://spelling.example.test/confluence", "https://spelling.example.test/confluence/"},
		{"another path on the same host", "https://moved-path.example.test/wiki", "https://moved-path.example.test/confluence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := loginStateForTest(t)
			existing := config.File{CurrentContext: "default", Contexts: []config.NamedContext{patContext("default", tc.before)}}
			edited := patContext("default", tc.after)
			result := &config.WizardResult{
				File:  config.File{CurrentContext: "default", Contexts: []config.NamedContext{edited}},
				Creds: []config.ContextResult{{Context: edited, Secrets: config.Secrets{PAT: "fresh-token"}}},
			}
			if _, err := persistInitResult(s, result, existing); err != nil {
				t.Fatal(err)
			}
			if got, err := storedSecret(t, s.cfgDir, edited.BaseURL); err != nil || got != "fresh-token" {
				t.Fatalf("the wizard deleted the credential it had just saved: %q %v", got, err)
			}
		})
	}
}

// Moving a context to another server still clears the secret nothing uses any
// more, and leaves one that a second context on the old server resolves.
func TestConfigInitForgetsOnlyCredentialsNoContextUses(t *testing.T) {
	for _, shared := range []bool{false, true} {
		s, _, _ := loginStateForTest(t)
		old := patContext("default", "https://old-server.example.test/confluence")
		if _, err := auth.Save(old.BaseURL, auth.Credential{Scheme: config.SchemePAT, Secret: "old-token"}, s.store); err != nil {
			t.Fatal(err)
		}
		existing := config.File{CurrentContext: "default", Contexts: []config.NamedContext{old}}
		moved := patContext("default", "https://new-server.example.test/confluence")
		file := config.File{CurrentContext: "default", Contexts: []config.NamedContext{moved}}
		if shared {
			other := patContext("other", "https://old-server.example.test/")
			existing.Contexts = append(existing.Contexts, other)
			file.Contexts = append([]config.NamedContext{other}, moved)
		}
		result := &config.WizardResult{File: file, Creds: []config.ContextResult{{Context: moved, Secrets: config.Secrets{PAT: "new-token"}}}}
		if _, err := persistInitResult(s, result, existing); err != nil {
			t.Fatal(err)
		}
		if got, err := storedSecret(t, s.cfgDir, moved.BaseURL); err != nil || got != "new-token" {
			t.Fatalf("shared=%v: new credential missing: %q %v", shared, got, err)
		}
		got, err := storedSecret(t, s.cfgDir, old.BaseURL)
		if shared && (err != nil || got != "old-token") {
			t.Fatalf("a credential another context still uses was deleted: %q %v", got, err)
		}
		if !shared && !errors.Is(err, auth.ErrSecretNotFound) {
			t.Fatalf("an unused credential was left behind: %q %v", got, err)
		}
	}
}

func TestDeleteContextKeepsACredentialAnotherContextUses(t *testing.T) {
	s, _, _ := loginStateForTest(t)
	file := config.File{CurrentContext: "default", Contexts: []config.NamedContext{
		patContext("default", "https://shared.example.test/confluence/"),
		// A preset written before login carries no scheme; it defaults to pat.
		{Name: "team", BaseURL: "https://shared.example.test/confluence", Flavor: config.FlavorDataCenter},
		patContext("elsewhere", "https://elsewhere.example.test/confluence"),
	}}
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	for _, c := range []config.NamedContext{file.Contexts[0], file.Contexts[2]} {
		if _, err := auth.Save(c.BaseURL, auth.Credential{Scheme: config.SchemePAT, Secret: "token-for-" + c.Name}, s.store); err != nil {
			t.Fatal(err)
		}
	}
	// The preset shares the personal context's server, and therefore its secret.
	if _, err := runCLIFile(t, s.cfgDir, "config", "delete-context", "team"); err != nil {
		t.Fatal(err)
	}
	if got, err := storedSecret(t, s.cfgDir, "https://shared.example.test/confluence/"); err != nil || got != "token-for-default" {
		t.Fatalf("deleting the preset removed the personal context's credential: %q %v", got, err)
	}
	// A context that is the only user of its server takes its secret with it.
	if _, err := runCLIFile(t, s.cfgDir, "config", "delete-context", "elsewhere"); err != nil {
		t.Fatal(err)
	}
	if got, err := storedSecret(t, s.cfgDir, "https://elsewhere.example.test/confluence"); !errors.Is(err, auth.ErrSecretNotFound) {
		t.Fatalf("an unused credential was left behind: %q %v", got, err)
	}
	after, _, err := config.ReadFile(s.cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Contexts) != 1 || after.Contexts[0].Name != "default" {
		t.Fatalf("unexpected contexts: %+v", after)
	}
}

// A failed config write leaves the context in the file, so its credential must
// stay resolvable as well.
func TestDeleteContextKeepsTheCredentialWhenTheConfigWriteFails(t *testing.T) {
	s, _, _ := loginStateForTest(t)
	file := config.File{CurrentContext: "default", Contexts: []config.NamedContext{
		patContext("default", "https://kept-on-failure.example.test/confluence"),
		patContext("doomed", "https://write-failure.example.test/confluence"),
	}}
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Save(file.Contexts[1].BaseURL, auth.Credential{Scheme: config.SchemePAT, Secret: "doomed-token"}, s.store); err != nil {
		t.Fatal(err)
	}
	// A read-only directory refuses the temporary file of the atomic replace.
	if err := os.Chmod(s.cfgDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(s.cfgDir, 0o700) })
	if _, err := runCLIFile(t, s.cfgDir, "config", "delete-context", "doomed"); err == nil {
		t.Skip("the config directory stayed writable (Windows or a privileged user)")
	}
	if got, err := storedSecret(t, s.cfgDir, file.Contexts[1].BaseURL); err != nil || got != "doomed-token" {
		t.Fatalf("a failed delete-context removed the credential the context still names: %q %v", got, err)
	}
}

// A context saved without a scheme resolves pat on Data Center and basic on
// Cloud, so cleanup has to compare the key each context really resolves.
func TestCredentialKeyFollowsTheSchemeAContextResolves(t *testing.T) {
	for _, tc := range []struct {
		name string
		nc   config.NamedContext
		want string
	}{
		{"explicit scheme", config.NamedContext{Flavor: config.FlavorCloud, Auth: config.AuthConfig{Scheme: config.SchemePAT}}, "key.example.test:pat"},
		{"schemeless Data Center", config.NamedContext{Flavor: config.FlavorDataCenter}, "key.example.test:pat"},
		{"schemeless Cloud", config.NamedContext{Flavor: config.FlavorCloud}, "key.example.test:basic"},
		{"schemeless detected Cloud", config.NamedContext{Flavor: config.FlavorAuto, DetectedFlavor: config.FlavorCloud}, "key.example.test:basic"},
	} {
		tc.nc.Name, tc.nc.BaseURL = "stored", "https://key.example.test/wiki"
		if got := credentialKey(tc.nc); got != tc.want {
			t.Errorf("%s: credentialKey = %q, want %q", tc.name, got, tc.want)
		}
	}
}

package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/zalando/go-keyring"
)

func loginStateForTest(t *testing.T) (*appState, config.Config, auth.Credential) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{BaseURL: "https://service.example.test/deploy", Auth: config.AuthConfig{Scheme: "basic", CredentialURL: "https://help.example.test/credentials"}, Flavor: "datacenter"}
	return &appState{cfgDir: dir, resolved: &config.Resolved{Config: cfg}, store: auth.NewStore(dir)}, cfg, auth.Credential{Scheme: "basic", Username: "PersonalUser", Secret: "personal-secret"}
}

func TestPersonalLoginSurvivesFreshConfigLoad(t *testing.T) {
	keyring.MockInit()
	s, cfg, cred := loginStateForTest(t)
	services := s.loginServices()
	services.Verify = func(config.Config, auth.Credential) error { return nil }
	if _, err := completeLogin(s, cfg, cred, services); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.ReadFile(s.cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	nc, ok := file.Context("default")
	if !ok || nc.Auth.Username != cred.Username || nc.Auth.CredentialURL != cfg.Auth.CredentialURL {
		t.Fatal(file)
	}
	// Reload the persisted identity and resolve the secret from a new store instance.
	fresh := cfg
	fresh.BaseURL = nc.BaseURL
	fresh.Auth = nc.Auth
	got, err := auth.Resolve(fresh, config.Secrets{}, auth.NewStore(s.cfgDir))
	if err != nil {
		t.Fatal(err)
	}
	if got != cred {
		t.Fatalf("fresh process cannot resolve login: %+v", got)
	}
}

func TestLoginRejectsChangedDeploymentPathBeforeAnyIO(t *testing.T) {
	s, cfg, cred := loginStateForTest(t)
	original := config.File{CurrentContext: "team", Contexts: []config.NamedContext{{Name: "team", BaseURL: "https://service.example.test/other", Auth: config.AuthConfig{Scheme: "basic", Username: "OldUser"}}}}
	if err := config.WriteFile(s.cfgDir, original); err != nil {
		t.Fatal(err)
	}
	s.resolved.ActiveContext = "team"
	called := false
	services := loginServices{Verify: func(config.Config, auth.Credential) error { called = true; return nil }, Save: func(string, auth.Credential) (string, error) { called = true; return "", nil }, Write: func(string, config.File) error { called = true; return nil }}
	_, err := completeLogin(s, cfg, cred, services)
	if err == nil || cerrors.AsCLIError(err).Code != "CONTEXT_BASE_URL_MISMATCH" || called {
		t.Fatalf("unsafe mismatch handling: %v called=%v", err, called)
	}
	after, _, _ := config.ReadFile(s.cfgDir)
	if !reflect.DeepEqual(original, after) {
		t.Fatal("changed old configuration")
	}
}

func TestLoginPersistenceFailuresAreDistinguishable(t *testing.T) {
	for _, stage := range []string{"verify", "save", "write"} {
		t.Run(stage, func(t *testing.T) {
			s, cfg, cred := loginStateForTest(t)
			cause := errors.New("injected failure")
			saved, written := false, false
			services := loginServices{
				Verify: func(config.Config, auth.Credential) error {
					if stage == "verify" {
						return cause
					}
					return nil
				},
				Save: func(string, auth.Credential) (string, error) {
					saved = true
					if stage == "save" {
						return "", cause
					}
					return "test", nil
				},
				Write: func(string, config.File) error { written = true; return cause },
			}
			_, err := completeLogin(s, cfg, cred, services)
			if !errors.Is(err, cause) {
				t.Fatalf("lost cause: %v", err)
			}
			if stage == "verify" && (saved || written) {
				t.Fatal("persisted after failed verification")
			}
			if stage == "save" && written {
				t.Fatal("wrote config after failed credential storage")
			}
			if stage == "write" {
				ce := cerrors.AsCLIError(err)
				if ce.Code != "LOGIN_CONFIG_WRITE_FAILED" || ce.Retryable || ce.Details == nil {
					t.Fatalf("lost partial outcome: %+v", ce)
				}
			}
			if _, err := os.Stat(config.ConfigFilePath(s.cfgDir)); !os.IsNotExist(err) {
				t.Fatal("failed login created config")
			}
		})
	}
}

func TestSetContextDryRunAndRepeatAvoidWrites(t *testing.T) {
	s, cfg, _ := loginStateForTest(t)
	s.store = nil // The setup path must not need a credential store.
	s.resolved = &config.Resolved{Config: cfg, Sources: map[string]string{config.FieldServer: "flag", config.FieldAuthScheme: "flag", config.FieldCredentialURL: "flag"}}
	s.resolved.Config.Defaults.Format = "json"
	cmd := newConfigSetContextCmd(s)
	_ = cmd.Flags().Set("dry-run", "true")
	if err := cmd.RunE(cmd, []string{"team"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(config.ConfigFilePath(s.cfgDir)); !os.IsNotExist(err) {
		t.Fatal("dry run wrote config")
	}
	_ = cmd.Flags().Set("dry-run", "false")
	if err := cmd.RunE(cmd, []string{"team"}); err != nil {
		t.Fatal(err)
	}
	path := config.ConfigFilePath(s.cfgDir)
	old := time.Unix(123456789, 0)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, []string{"team"}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if !info.ModTime().Equal(old) {
		t.Fatal("identical setup rewrote config")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("unexpected setup files: %v", entries)
	}
}

func TestVerifyLoginRejectsAnonymousResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"anonymous"}`))
	}))
	defer server.Close()
	s, cfg, cred := loginStateForTest(t)
	cfg.BaseURL = server.URL
	cfg.Defaults.Timeout = time.Second
	s.resolved.Config = cfg
	if err := verifyCredential(s, cfg, cred); err == nil {
		t.Fatal("anonymous response was accepted as an authenticated login")
	}
}

func TestEquivalentURLLoginKeepsCredentialKeyAfterReload(t *testing.T) {
	keyring.MockInit()
	s, cfg, cred := loginStateForTest(t)
	before := config.NamedContext{Name: "team", BaseURL: cfg.BaseURL, Auth: config.AuthConfig{Scheme: "basic"}}
	if err := config.WriteFile(s.cfgDir, config.File{CurrentContext: "team", Contexts: []config.NamedContext{before}}); err != nil {
		t.Fatal(err)
	}
	s.resolved.ActiveContext = "team"
	cfg.BaseURL = "https://SERVICE.example.test:443/deploy/"
	s.resolved.Config = cfg
	services := s.loginServices()
	services.Verify = func(config.Config, auth.Credential) error { return nil }
	if _, err := completeLogin(s, cfg, cred, services); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.ReadFile(s.cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	nc, _ := file.Context("team")
	fresh := cfg
	fresh.BaseURL = nc.BaseURL
	fresh.Auth = nc.Auth
	got, err := auth.Resolve(fresh, config.Secrets{}, auth.NewStore(s.cfgDir))
	if err != nil || got != cred {
		t.Fatalf("equivalent URL orphaned credential: %v", err)
	}
}

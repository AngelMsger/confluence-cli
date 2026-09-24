package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/zalando/go-keyring"
)

func reuseFixture(t *testing.T) (*appState, config.File, reuseServices) {
	t.Helper()
	dir := t.TempDir()
	source := config.NamedContext{Name: "personal", BaseURL: "https://service.example.test/deploy", Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}}
	target := source
	target.Name = "team"
	target.Auth.Username = ""
	file := config.File{CurrentContext: "personal", Contexts: []config.NamedContext{source, target}}
	if err := config.WriteFile(dir, file); err != nil {
		t.Fatal(err)
	}
	s := &appState{cfgDir: dir, resolved: &config.Resolved{Config: reuseContextConfig(target, file.Defaults), ActiveContext: "team"}}
	services := reuseServices{Read: config.ReadFile, Write: config.WriteFile,
		Resolve: func(cfg config.Config) (auth.Credential, error) {
			return auth.Credential{Scheme: cfg.Auth.Scheme, Username: cfg.Auth.Username, Secret: "test-secret"}, nil
		},
		Verify: func(config.Config, auth.Credential) error { return nil }}
	return s, file, services
}

func TestAuthReusePreviewApplyAndIdempotence(t *testing.T) {
	s, before, services := reuseFixture(t)
	path := config.ConfigFilePath(s.cfgDir)
	bytesBefore, _ := os.ReadFile(path)
	preview, err := reuseAuthentication(s, "", true, services)
	if err != nil || !preview.Changed || preview.State != "available" || !preview.Verified {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	afterPreview, _ := os.ReadFile(path)
	if string(afterPreview) != string(bytesBefore) {
		t.Fatal("preview changed config")
	}
	applied, err := reuseAuthentication(s, "", false, services)
	if err != nil || applied.State != "reused" || applied.SourceContext != "personal" {
		t.Fatalf("apply: %+v %v", applied, err)
	}
	after, _, _ := config.ReadFile(s.cfgDir)
	target, _ := after.Context("team")
	if target.Auth.Username != "existing-user" || after.CurrentContext != before.CurrentContext || !reflect.DeepEqual(after.Contexts[0], before.Contexts[0]) {
		t.Fatal("did not preserve personal source/current context")
	}
	services.Write = func(string, config.File) error { t.Fatal("repeated write"); return nil }
	services.Resolve = func(config.Config) (auth.Credential, error) {
		t.Fatal("populated identity must remain unchanged")
		return auth.Credential{}, nil
	}
	repeated, err := reuseAuthentication(s, "", false, services)
	if err != nil || repeated.Changed || repeated.State != "unchanged" {
		t.Fatalf("repeat: %+v %v", repeated, err)
	}
	raw, _ := json.Marshal(applied)
	if strings.Contains(string(raw), "test-secret") {
		t.Fatal("secret reached output")
	}
}

func TestAuthReuseNativeCredentialSurvivesFreshLoad(t *testing.T) {
	keyring.MockInit()
	s, _, services := reuseFixture(t)
	store := auth.NewStore(s.cfgDir)
	credential := auth.Credential{Scheme: "basic", Username: "existing-user", Secret: "retained-secret"}
	if _, err := auth.Save(s.cfg().BaseURL, credential, store); err != nil {
		t.Fatal(err)
	}
	services.Resolve = func(cfg config.Config) (auth.Credential, error) { return auth.Resolve(cfg, config.Secrets{}, store) }
	if _, err := reuseAuthentication(s, "", false, services); err != nil {
		t.Fatal(err)
	}
	file, _, _ := config.ReadFile(s.cfgDir)
	target, _ := file.Context("team")
	got, err := auth.Resolve(reuseContextConfig(target, file.Defaults), config.Secrets{}, auth.NewStore(s.cfgDir))
	if err != nil || got.Secret != credential.Secret || got.Username != credential.Username {
		t.Fatalf("fresh identity could not resolve existing credential: %v", err)
	}
}

func TestAuthReuseMatchesCompleteServiceBeforeCredentialAccess(t *testing.T) {
	for _, kind := range []string{"host", "path", "protocol", "port", "scheme", "flavor"} {
		t.Run(kind, func(t *testing.T) {
			s, file, services := reuseFixture(t)
			switch kind {
			case "host":
				file.Contexts[0].BaseURL = "https://other.example.test/deploy"
			case "path":
				file.Contexts[0].BaseURL = "https://service.example.test/other"
			case "protocol":
				file.Contexts[0].BaseURL = "http://service.example.test/deploy"
			case "port":
				file.Contexts[0].BaseURL = "https://service.example.test:8443/deploy"
			case "scheme":
				file.Contexts[0].Auth.Scheme = "token"
			case "flavor":
				file.Contexts[0].Flavor = "cloud"
			}
			if err := config.WriteFile(s.cfgDir, file); err != nil {
				t.Fatal(err)
			}
			services.Resolve = func(config.Config) (auth.Credential, error) {
				t.Fatal("read credential before matching service")
				return auth.Credential{}, nil
			}
			result, err := reuseAuthentication(s, "", true, services)
			if err != nil || result.Changed || result.State != "unavailable" {
				t.Fatalf("mismatch: %+v %v", result, err)
			}
			_, err = reuseAuthentication(s, "personal", true, services)
			if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_SOURCE_MISMATCH" {
				t.Fatalf("explicit mismatch: %v", err)
			}
		})
	}
}

func TestAuthReusePreservesDestinationIdentity(t *testing.T) {
	s, file, services := reuseFixture(t)
	file.Contexts[1].Auth.Username = "other-user"
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	services.Resolve = func(config.Config) (auth.Credential, error) {
		t.Fatal("must not replace an existing identity")
		return auth.Credential{}, nil
	}
	r, err := reuseAuthentication(s, "", false, services)
	if err != nil || r.State != "unchanged" || r.Changed {
		t.Fatalf("existing identity: %+v %v", r, err)
	}
}

func TestAuthReuseAmbiguityAndExplicitSelection(t *testing.T) {
	s, file, services := reuseFixture(t)
	other := file.Contexts[0]
	other.Name = "second"
	other.Auth.Username = "second-user"
	file.Contexts = append(file.Contexts, other)
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	_, err := reuseAuthentication(s, "", true, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_AMBIGUOUS" {
		t.Fatalf("ambiguity: %v", err)
	}
	r, err := reuseAuthentication(s, "SECOND", false, services)
	if err != nil || r.SourceContext != "second" || r.State != "reused" {
		t.Fatalf("explicit choice: %+v %v", r, err)
	}
}

func TestAuthReuseDuplicateAliasesDoNotRequireChoice(t *testing.T) {
	s, file, services := reuseFixture(t)
	alias := file.Contexts[0]
	alias.Name = "alias"
	file.Contexts = append(file.Contexts, alias)
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	r, err := reuseAuthentication(s, "", true, services)
	if err != nil || !r.Changed {
		t.Fatalf("same native credential: %+v %v", r, err)
	}
}

func TestAuthReuseRetainsOperationalFailures(t *testing.T) {
	for _, kind := range []string{"store", "network", "permission"} {
		t.Run(kind, func(t *testing.T) {
			s, before, services := reuseFixture(t)
			failure := cerrors.New(cerrors.CategoryNetwork, "NETWORK_ERROR", "network unavailable")
			if kind == "store" {
				failure = cerrors.New(cerrors.CategoryConfig, "CREDENTIAL_STORE_INACCESSIBLE", "host keychain unavailable")
				services.Resolve = func(config.Config) (auth.Credential, error) { return auth.Credential{}, failure }
			}
			if kind == "permission" {
				failure = cerrors.New(cerrors.CategoryPermission, "FORBIDDEN", "access denied")
			}
			if kind != "store" {
				services.Verify = func(config.Config, auth.Credential) error { return failure }
			}
			_, err := reuseAuthentication(s, "", false, services)
			if !errors.Is(err, failure) {
				t.Fatalf("lost diagnostic: %v", err)
			}
			after, _, _ := config.ReadFile(s.cfgDir)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("failure mutated config")
			}
		})
	}
}

func TestAuthReuseExpiredIdentityIsUnavailable(t *testing.T) {
	s, _, services := reuseFixture(t)
	services.Verify = func(config.Config, auth.Credential) error {
		return cerrors.New(cerrors.CategoryAuth, "HTTP_UNAUTHORIZED", "expired").WithHTTPStatus(401)
	}
	r, err := reuseAuthentication(s, "", true, services)
	if err != nil || r.State != "unavailable" || r.Changed {
		t.Fatalf("expired credential: %+v %v", r, err)
	}
}

func TestAuthReuseSelfConfigurationAndConcurrentChange(t *testing.T) {
	s, _, services := reuseFixture(t)
	s.resolved.Config.Defaults.ReadOnly = true
	r, err := reuseAuthentication(s, "", true, services)
	if err != nil || !r.Changed {
		t.Fatal("read-only preview blocked", err)
	}
	r, err = reuseAuthentication(s, "", false, services)
	if err != nil || r.State != "reused" {
		t.Fatalf("native self-configuration should remain available: %+v %v", r, err)
	}
	s, _, services = reuseFixture(t)
	services.Verify = func(config.Config, auth.Credential) error {
		f, _, e := config.ReadFile(s.cfgDir)
		if e != nil {
			return e
		}
		f.CurrentContext = "changed-by-user"
		return config.WriteFile(s.cfgDir, f)
	}
	_, err = reuseAuthentication(s, "", false, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_CONFIG_CHANGED" {
		t.Fatalf("concurrent change: %v", err)
	}
}

func TestAuthReuseRejectsTargetOverridesAndUnknownSource(t *testing.T) {
	s, _, services := reuseFixture(t)
	s.resolved.Config.BaseURL = "https://service.example.test/other"
	_, err := reuseAuthentication(s, "", true, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_TARGET_MISMATCH" {
		t.Fatal(err)
	}
	s.resolved.Config.BaseURL = "https://service.example.test/deploy"
	_, err = reuseAuthentication(s, "missing", true, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_SOURCE_NOT_FOUND" {
		t.Fatal(err)
	}
}

func TestAuthReuseRejectsCredentialRotationBeforeAssociation(t *testing.T) {
	s, before, services := reuseFixture(t)
	loads := 0
	services.Resolve = func(cfg config.Config) (auth.Credential, error) {
		if cfg.Auth.Username != "" {
			loads++
		}
		secret := "verified-secret"
		if loads > 1 {
			secret = "rotated-secret"
		}
		return auth.Credential{Scheme: cfg.Auth.Scheme, Username: cfg.Auth.Username, Secret: secret}, nil
	}
	_, err := reuseAuthentication(s, "", false, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_CREDENTIAL_CHANGED" {
		t.Fatalf("credential rotation: %v", err)
	}
	after, _, _ := config.ReadFile(s.cfgDir)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("rotation changed destination")
	}
}

func TestAuthReuseUsesPersistedDefaultsWithoutEnvironmentIdentity(t *testing.T) {
	s, file, services := reuseFixture(t)
	file.Contexts[0].Auth.Scheme = ""
	cfg := config.StoredContext(file.Contexts[0], file.Defaults)
	file.Contexts[1].Auth.Scheme = cfg.Auth.Scheme
	s.resolved.Config = config.StoredContext(file.Contexts[1], file.Defaults)
	s.resolved.Secrets = config.Secrets{Password: "environment-secret"}
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	sourceResolver := services.Resolve
	services.Resolve = func(cfg config.Config) (auth.Credential, error) {
		if cfg.Auth.Username == "" {
			return auth.Credential{}, cerrors.New(cerrors.CategoryConfig, "CREDENTIAL_NOT_VISIBLE_OR_MISSING", "no destination credential")
		}
		return sourceResolver(cfg)
	}
	r, err := reuseAuthentication(s, "", false, services)
	if err != nil || r.State != "reused" {
		t.Fatalf("stored defaults: %+v %v", r, err)
	}
	after, _, _ := config.ReadFile(s.cfgDir)
	target, _ := after.Context("team")
	if target.Auth.Username != "existing-user" {
		t.Fatal("identity did not come from stored source")
	}
}

func TestAuthReuseCommandVerifiesNativeStoredCredential(t *testing.T) {
	for _, flavor := range []string{"cloud", "datacenter"} {
		t.Run(flavor, func(t *testing.T) {
			keyring.MockInit()
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				user, password, ok := r.BasicAuth()
				if r.Method != http.MethodGet || !ok || user != "existing-user" || password != "retained-secret" {
					t.Error("unexpected request authentication or mutation")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-AUSERNAME", "existing-user")
				_, _ = w.Write([]byte(`{"id":"existing-user","name":"existing-user","username":"existing-user","slug":"existing-user","uuid":"test-user","accountId":"test-user","userKey":"test-user","type":"known","authenticated":true,"anonymous":false,"status":"success","data":{"version":"1.0.0"}}`))
			}))
			defer server.Close()
			s, file, _ := reuseFixture(t)
			for i := range file.Contexts {
				file.Contexts[i].BaseURL = server.URL + "/deploy"
			}

			for i := range file.Contexts {
				file.Contexts[i].Flavor = flavor
			}
			if err := config.WriteFile(s.cfgDir, file); err != nil {
				t.Fatal(err)
			}
			s.resolved.Config = config.StoredContext(file.Contexts[1], file.Defaults)
			s.store = auth.NewStore(s.cfgDir)
			credential := auth.Credential{Scheme: "basic", Username: "existing-user", Secret: "retained-secret"}
			if _, err := auth.Save(file.Contexts[0].BaseURL, credential, s.store); err != nil {
				t.Fatal(err)
			}
			command := newAuthReuseCmd(s)
			if err := command.RunE(command, nil); err != nil {
				t.Fatal(err)
			}
			if requests == 0 {
				t.Fatal("did not verify with native API client")
			}
			after, _, _ := config.ReadFile(s.cfgDir)
			target, _ := after.Context("team")
			actual, err := auth.Resolve(config.StoredContext(target, after.Defaults), config.Secrets{}, auth.NewStore(s.cfgDir))
			if err != nil || actual.Username != "existing-user" || actual.Secret != credential.Secret {
				t.Fatalf("native reuse unavailable after reload: %v", err)
			}

		})
	}
}

func TestAuthReuseDoesNotHideUnavailableIdentityProof(t *testing.T) {
	s, _, services := reuseFixture(t)
	failure := cerrors.New(cerrors.CategoryAuth, "AUTH_IDENTITY_UNAVAILABLE", "authenticated identity could not be established")
	services.Verify = func(config.Config, auth.Credential) error { return failure }
	_, err := reuseAuthentication(s, "", true, services)
	if !errors.Is(err, failure) {
		t.Fatalf("lost identity diagnostic: %v", err)
	}
}

func TestAuthReusePreservesLegacyStoreKeyUnderEquivalentOverride(t *testing.T) {
	keyring.MockInit()
	s, file, services := reuseFixture(t)
	file.Contexts[0].BaseURL = "https://SERVICE.example.test:443/deploy/"
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	store := auth.NewStore(s.cfgDir)
	credential := auth.Credential{Scheme: "basic", Username: "existing-user", Secret: "retained-secret"}
	if _, err := auth.Save(file.Contexts[0].BaseURL, credential, store); err != nil {
		t.Fatal(err)
	}
	services.Resolve = func(cfg config.Config) (auth.Credential, error) { return auth.Resolve(cfg, config.Secrets{}, store) }
	if _, err := reuseAuthentication(s, "", false, services); err != nil {
		t.Fatal(err)
	}
	fresh, err := config.Load(config.LoadOptions{ConfigDir: s.cfgDir, Context: "team", DotenvPath: filepath.Join(t.TempDir(), "absent"), Flags: config.FlagValues{BaseURL: "https://service.example.test/deploy"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := auth.Resolve(fresh.Config, config.Secrets{}, auth.NewStore(s.cfgDir))
	if err != nil || got.Secret != credential.Secret || got.Username != credential.Username {
		t.Fatalf("equivalent override lost native credential: %v", err)
	}
	if fresh.Config.BaseURL != "https://service.example.test/deploy" {
		t.Fatal("changed request destination")
	}
	if err := auth.ForgetForConfig(fresh.Config, fresh.Config.Auth.Scheme, store); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(auth.AccountKey(file.Contexts[0].BaseURL, credential.Scheme)); err == nil {
		t.Fatal("logout left the original credential active")
	}
	fresh.Config.BaseURL = "https://service.example.test/other"
	_, err = auth.Resolve(fresh.Config, config.Secrets{}, store)
	if err == nil || cerrors.AsCLIError(err).Code != "CREDENTIAL_SERVICE_MISMATCH" {
		t.Fatalf("foreign deployment reused stored key: %v", err)
	}
}

func TestAuthReusePreservesWorkingTokenWithoutUsername(t *testing.T) {
	keyring.MockInit()
	s, file, services := reuseFixture(t)
	file.Contexts[0].BaseURL = "https://SERVICE.example.test:443/deploy/"
	for i := range file.Contexts {
		file.Contexts[i].Auth.Scheme = "pat"
	}
	file.Contexts[1].Auth.Username = ""
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	s.resolved.Config = config.StoredContext(file.Contexts[1], file.Defaults)
	store := auth.NewStore(s.cfgDir)
	existing := auth.Credential{Scheme: "pat", Secret: "destination-secret"}
	source := auth.Credential{Scheme: "pat", Secret: "other-source-secret", Username: "existing-user"}
	if _, err := auth.Save(file.Contexts[1].BaseURL, existing, store); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Save(file.Contexts[0].BaseURL, source, store); err != nil {
		t.Fatal(err)
	}
	services.Resolve = func(cfg config.Config) (auth.Credential, error) { return auth.Resolve(cfg, config.Secrets{}, store) }
	services.Verify = func(_ config.Config, credential auth.Credential) error {
		if credential.Secret != existing.Secret {
			t.Fatal("attempted to switch an already valid destination login")
		}
		return nil
	}
	r, err := reuseAuthentication(s, "", false, services)
	if err != nil || r.Changed || !r.Verified || r.State != "unchanged" {
		t.Fatalf("existing token: %+v %v", r, err)
	}
	after, _, _ := config.ReadFile(s.cfgDir)
	if !reflect.DeepEqual(file, after) {
		t.Fatal("modified a valid token-only context")
	}
}

func TestAuthReuseMatchesPreviouslyDetectedFlavor(t *testing.T) {
	s, file, services := reuseFixture(t)
	file.Contexts[0].Flavor = "auto"
	file.Contexts[0].DetectedFlavor = "datacenter"
	file.Contexts[1].Flavor = "datacenter"
	s.resolved.Config = config.StoredContext(file.Contexts[1], file.Defaults)
	if err := config.WriteFile(s.cfgDir, file); err != nil {
		t.Fatal(err)
	}
	services.Verify = func(cfg config.Config, _ auth.Credential) error {
		if cfg.Flavor != "datacenter" {
			t.Fatal("ignored verified deployment flavor")
		}
		return nil
	}
	r, err := reuseAuthentication(s, "", false, services)
	if err != nil || r.State != "reused" {
		t.Fatalf("detected flavor: %+v %v", r, err)
	}
}

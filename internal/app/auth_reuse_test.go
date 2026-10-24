package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angelmsger/agent-cli-kit/credstore"
	"github.com/angelmsger/agent-cli-kit/reuse"
	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

// The decision itself is agent-cli-kit/reuse's and tested there. These tests
// cover confluence-cli's wiring: how contexts are described (URL plus
// effective flavor), how the destination is rewritten, the Cloud limit to
// basic, the canonical account, and the native store, verification and command.

const reuseBase = "https://service.example.test/deploy"

// fakeStore answers lookups from keychain entries keyed like the real store.
type fakeStore map[string]string

func (f fakeStore) lookup(key string) credstore.Lookup {
	if v, ok := f[key]; ok {
		return credstore.Lookup{Secret: v, Keychain: credstore.Hit}
	}
	return credstore.Lookup{Keychain: credstore.Miss, File: credstore.Miss}
}

func (f fakeStore) resolve(cfg config.Config) (auth.Credential, credstore.Lookup, error) {
	l := f.lookup(auth.AccountKey(cfg.BaseURL, cfg.Auth.Scheme))
	if !l.Found() {
		return auth.Credential{}, l, cerrors.New(cerrors.CategoryConfig, "CREDENTIAL_NOT_VISIBLE_OR_MISSING", "missing")
	}
	cred := auth.Credential{Scheme: cfg.Auth.Scheme, Username: cfg.Auth.Username, Secret: l.Secret}
	return cred, l, cred.Validate()
}

func rejected() *cerrors.CLIError {
	return cerrors.New(cerrors.CategoryAuth, "HTTP_UNAUTHORIZED", "rejected").WithHTTPStatus(401)
}

// reuseFixture writes contexts (the last one is the destination) and returns
// services over a fake keychain holding the shared basic secret. Its
// verification accepts that secret with any username and reports an account
// per username.
func reuseFixture(t *testing.T, contexts ...config.NamedContext) (*appState, fakeStore, reuseServices) {
	t.Helper()
	dir := t.TempDir()
	if len(contexts) == 0 {
		contexts = []config.NamedContext{
			{Name: "personal", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
			{Name: "team", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic"}},
		}
	}
	file := config.File{CurrentContext: contexts[0].Name, Contexts: contexts}
	if err := config.WriteFile(dir, file); err != nil {
		t.Fatal(err)
	}
	target := contexts[len(contexts)-1]
	s := &appState{cfgDir: dir, resolved: &config.Resolved{Config: config.StoredContext(target, file.Defaults), ActiveContext: target.Name}}
	store := fakeStore{auth.AccountKey(reuseBase, "basic"): "test-secret"}
	services := reuseServices{Read: config.ReadFile, Write: config.WriteFile, Resolve: store.resolve, Lookup: store.lookup,
		Verify: func(_ context.Context, _ config.Config, cred auth.Credential) (reuse.Account, error) {
			if cred.Scheme == "basic" && cred.Secret == "test-secret" && cred.Username != "" {
				return reuse.Account{ID: "key-" + cred.Username, Name: cred.Username}, nil
			}
			return reuse.Account{}, rejected()
		}}
	return s, store, services
}

func targetOf(t *testing.T, s *appState) config.NamedContext {
	t.Helper()
	file, _, err := config.ReadFile(s.cfgDir)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := file.Context(s.resolved.ActiveContext)
	return c
}

func TestAuthReusePreviewApplyAndIdempotence(t *testing.T) {
	s, _, services := reuseFixture(t)
	path := config.ConfigFilePath(s.cfgDir)
	bytesBefore, _ := os.ReadFile(path)
	before, _, _ := config.ReadFile(s.cfgDir)
	preview, err := reuseAuthentication(s, reuseOptions{DryRun: true}, services)
	if err != nil || !preview.Changed || preview.State != "available" || !preview.Verified || preview.Scheme != "basic" || preview.Account != "key-existing-user" {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if afterPreview, _ := os.ReadFile(path); string(afterPreview) != string(bytesBefore) {
		t.Fatal("preview changed config")
	}
	applied, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || applied.State != "reused" || applied.SourceContext != "personal" || applied.Username != "existing-user" {
		t.Fatalf("apply: %+v %v", applied, err)
	}
	after, _, _ := config.ReadFile(s.cfgDir)
	if got := targetOf(t, s); got.Auth.Username != "existing-user" || after.CurrentContext != before.CurrentContext || !reflect.DeepEqual(after.Contexts[0], before.Contexts[0]) {
		t.Fatalf("did not preserve the source or the current context: %+v", after)
	}
	services.Write = func(string, config.File) error { t.Fatal("repeated write"); return nil }
	services.Resolve = func(config.Config) (auth.Credential, credstore.Lookup, error) {
		t.Fatal("a basic username is an identity and stays unchanged")
		return auth.Credential{}, credstore.Lookup{}, nil
	}
	repeated, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || repeated.Changed || repeated.State != "unchanged" {
		t.Fatalf("repeat: %+v %v", repeated, err)
	}
	raw, _ := json.Marshal(applied)
	if strings.Contains(string(raw), "test-secret") {
		t.Fatal("secret reached output")
	}
	if len(applied.NextSteps) != 1 || !strings.HasSuffix(applied.NextSteps[0], "whoami") {
		t.Fatalf("next steps: %v", applied.NextSteps)
	}
}

// On Data Center a pat context whose token is gone adopts the personal basic
// login on the same service.
func TestAuthReuseAdoptsAnotherSchemesContext(t *testing.T) {
	s, _, services := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat"}},
	)
	r, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || r.State != "reused" || r.SourceContext != "personal" || r.Scheme != "basic" || r.Account != "key-existing-user" {
		t.Fatalf("got %+v %v", r, err)
	}
	if got := targetOf(t, s); got.Auth.Scheme != "basic" || got.Auth.Username != "existing-user" || got.Flavor != "datacenter" {
		t.Fatalf("destination %+v", got)
	}
}

// Cloud accepts only basic: a pat source is never adopted automatically, and
// selecting one explicitly stops before any request.
func TestAuthReuseCloudNeverAdoptsPAT(t *testing.T) {
	s, store, services := reuseFixture(t,
		config.NamedContext{Name: "token", BaseURL: reuseBase, Flavor: "cloud", Auth: config.AuthConfig{Scheme: "pat"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Flavor: "cloud", Auth: config.AuthConfig{Scheme: "basic"}},
	)
	store[auth.AccountKey(reuseBase, "pat")] = "pat-secret"
	resolve := services.Resolve
	services.Resolve = func(cfg config.Config) (auth.Credential, credstore.Lookup, error) {
		if cfg.Auth.Scheme == "pat" {
			t.Fatal("resolved a pat source for a Cloud destination")
		}
		return resolve(cfg)
	}
	services.Verify = func(context.Context, config.Config, auth.Credential) (reuse.Account, error) {
		t.Fatal("verified a pat source for a Cloud destination")
		return reuse.Account{}, nil
	}
	r, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || r.Changed || r.State != "unavailable" {
		t.Fatalf("automatic: %+v %v", r, err)
	}
	services.Resolve = resolve
	for _, opts := range []reuseOptions{{FromScheme: "pat"}, {From: "token"}} {
		_, err = reuseAuthentication(s, opts, services)
		if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_SOURCE_MISMATCH" {
			t.Fatalf("explicit %+v: %v", opts, err)
		}
	}
	if got := targetOf(t, s); got.Auth.Scheme != "basic" {
		t.Fatalf("destination %+v", got)
	}
}

// A pat context whose username is only a label adopts the basic secret stored
// for its service, which no context points at, paired with that label.
func TestAuthReuseLabelledPATAdoptsStoredSecret(t *testing.T) {
	s, _, services := reuseFixture(t,
		config.NamedContext{Name: "team", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat", Username: "existing-user"}},
	)
	r, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || r.State != "reused" || r.SourceContext != "" || r.Username != "existing-user" || !strings.Contains(r.Reason, "stored") {
		t.Fatalf("got %+v %v", r, err)
	}
	if got := targetOf(t, s); got.Auth.Scheme != "basic" || got.Auth.Username != "existing-user" {
		t.Fatalf("destination %+v", got)
	}
}

// Labelled pat contexts on the destination's own key used to fail the
// command when that token was gone. Now the stored basic secret pairs with
// each recorded label; two accounts are a choice, made with --from-scheme.
func TestAuthReuseStoredSecretPairsWithRecordedUsernames(t *testing.T) {
	s, _, services := reuseFixture(t,
		config.NamedContext{Name: "a", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat", Username: "alice"}},
		config.NamedContext{Name: "b", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat", Username: "bob"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat"}},
	)
	_, err := reuseAuthentication(s, reuseOptions{DryRun: true}, services)
	ce := cerrors.AsCLIError(err)
	if err == nil || ce.Code != "AUTH_REUSE_AMBIGUOUS" {
		t.Fatalf("ambiguity: %v", err)
	}
	if details, _ := ce.Details.(map[string]any); len(details["candidates"].([]reuse.Candidate)) != 2 {
		t.Fatalf("candidates: %+v", ce.Details)
	}
	if !strings.Contains(strings.Join(ce.NextSteps, "\n"), "auth reuse --from-scheme basic") {
		t.Fatalf("next steps: %v", ce.NextSteps)
	}
	r, err := reuseAuthentication(s, reuseOptions{FromScheme: "basic", From: "b"}, services)
	if err != nil || r.State != "reused" || r.Username != "bob" || r.Account != "key-bob" {
		t.Fatalf("explicit: %+v %v", r, err)
	}
	if got := targetOf(t, s); got.Auth.Scheme != "basic" || got.Auth.Username != "bob" {
		t.Fatalf("destination %+v", got)
	}
}

// Two tokens of one account are one login; tokens of two accounts are a
// choice.
func TestAuthReuseCountsOneAccountAsOneLogin(t *testing.T) {
	const respelled = "https://SERVICE.example.test:443/deploy"
	for _, sameAccount := range []bool{true, false} {
		s, store, services := reuseFixture(t,
			config.NamedContext{Name: "token-a", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat"}},
			config.NamedContext{Name: "token-b", BaseURL: respelled, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat"}},
			config.NamedContext{Name: "team", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "basic"}},
		)
		store[auth.AccountKey(reuseBase, "pat")] = "pat-a"
		store[auth.AccountKey(respelled, "pat")] = "pat-b"
		services.Verify = func(_ context.Context, _ config.Config, cred auth.Credential) (reuse.Account, error) {
			if sameAccount {
				return reuse.Account{ID: "key-1"}, nil
			}
			return reuse.Account{ID: "key-" + cred.Secret}, nil
		}
		r, err := reuseAuthentication(s, reuseOptions{}, services)
		if sameAccount {
			if err != nil || r.State != "reused" || r.SourceContext != "token-a" || r.Scheme != "pat" || r.Account != "key-1" {
				t.Fatalf("one account: %+v %v", r, err)
			}
			continue
		}
		if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_AMBIGUOUS" {
			t.Fatalf("two accounts: %v", err)
		}
		r, err = reuseAuthentication(s, reuseOptions{From: "token-b"}, services)
		if err != nil || r.State != "reused" || r.Account != "key-pat-b" {
			t.Fatalf("explicit: %+v %v", r, err)
		}
		if got := targetOf(t, s); got.BaseURL != respelled || got.Auth.Scheme != "pat" {
			t.Fatalf("destination does not resolve the chosen key: %+v", got)
		}
	}
}

func TestAuthReuseMatchesCompleteServiceBeforeCredentialAccess(t *testing.T) {
	for _, kind := range []string{"host", "path", "protocol", "port", "flavor"} {
		t.Run(kind, func(t *testing.T) {
			personal := config.NamedContext{Name: "personal", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}}
			switch kind {
			case "host":
				personal.BaseURL = "https://other.example.test/deploy"
			case "path":
				personal.BaseURL = "https://service.example.test/other"
			case "protocol":
				personal.BaseURL = "http://service.example.test/deploy"
			case "port":
				personal.BaseURL = "https://service.example.test:8443/deploy"
			case "flavor":
				personal.Flavor = "cloud"
			}
			s, _, services := reuseFixture(t, personal, config.NamedContext{Name: "team", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic"}})
			resolve := services.Resolve
			services.Resolve = func(cfg config.Config) (auth.Credential, credstore.Lookup, error) {
				if cfg.Auth.Username == "existing-user" {
					t.Fatal("read another service's credential")
				}
				return resolve(cfg)
			}
			result, err := reuseAuthentication(s, reuseOptions{DryRun: true}, services)
			if err != nil || result.Changed || result.State != "unavailable" {
				t.Fatalf("mismatch: %+v %v", result, err)
			}
			_, err = reuseAuthentication(s, reuseOptions{From: "personal", DryRun: true}, services)
			if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_SOURCE_MISMATCH" {
				t.Fatalf("explicit mismatch: %v", err)
			}
		})
	}
}

// A scheme the invocation supplies keeps reuse within that scheme.
func TestAuthReuseSuppliedSchemeStaysWithinIt(t *testing.T) {
	s, _, services := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat"}},
	)
	s.resolved.Sources = map[string]string{config.FieldAuthScheme: "flag"}
	r, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || r.State != "unavailable" || r.Changed {
		t.Fatalf("pinned: %+v %v", r, err)
	}
	for _, opts := range []reuseOptions{{From: "personal"}, {FromScheme: "basic"}} {
		_, err = reuseAuthentication(s, opts, services)
		if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_SOURCE_MISMATCH" {
			t.Fatalf("pinned explicit %+v: %v", opts, err)
		}
	}
}

func TestAuthReusePreservesDestinationIdentity(t *testing.T) {
	s, _, services := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "other-user"}},
	)
	services.Resolve = func(config.Config) (auth.Credential, credstore.Lookup, error) {
		t.Fatal("must not replace an existing identity")
		return auth.Credential{}, credstore.Lookup{}, nil
	}
	r, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || r.State != "unchanged" || r.Changed {
		t.Fatalf("existing identity: %+v %v", r, err)
	}
}

func TestAuthReuseAmbiguityAndExplicitSelection(t *testing.T) {
	s, _, services := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "second", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "second-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic"}},
	)
	_, err := reuseAuthentication(s, reuseOptions{DryRun: true}, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_AMBIGUOUS" {
		t.Fatalf("ambiguity: %v", err)
	}
	r, err := reuseAuthentication(s, reuseOptions{From: "SECOND"}, services)
	if err != nil || r.SourceContext != "second" || r.State != "reused" || r.Account != "key-second-user" {
		t.Fatalf("explicit choice: %+v %v", r, err)
	}
}

func TestAuthReuseDuplicateAliasesDoNotRequireChoice(t *testing.T) {
	s, _, services := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "alias", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic"}},
	)
	r, err := reuseAuthentication(s, reuseOptions{DryRun: true}, services)
	if err != nil || !r.Changed {
		t.Fatalf("same native credential: %+v %v", r, err)
	}
}

func TestAuthReuseRetainsOperationalFailures(t *testing.T) {
	for _, kind := range []string{"store", "network", "permission", "identity"} {
		t.Run(kind, func(t *testing.T) {
			s, _, services := reuseFixture(t)
			before, _, _ := config.ReadFile(s.cfgDir)
			failure := cerrors.New(cerrors.CategoryNetwork, "NETWORK_ERROR", "network unavailable")
			switch kind {
			case "store":
				failure = cerrors.New(cerrors.CategoryConfig, "CREDENTIAL_STORE_INACCESSIBLE", "host keychain unavailable")
				services.Resolve = func(config.Config) (auth.Credential, credstore.Lookup, error) {
					return auth.Credential{}, credstore.Lookup{Keychain: credstore.Failed}, failure
				}
			case "permission":
				failure = cerrors.New(cerrors.CategoryPermission, "FORBIDDEN", "access denied")
			case "identity":
				// The service answered without a usable user: not a rejection.
				failure = cerrors.New(cerrors.CategoryAuth, "AUTH_IDENTITY_UNAVAILABLE", "authenticated identity could not be established")
			}
			if kind != "store" {
				services.Verify = func(context.Context, config.Config, auth.Credential) (reuse.Account, error) {
					return reuse.Account{}, failure
				}
			}
			_, err := reuseAuthentication(s, reuseOptions{}, services)
			if !errors.Is(err, failure) {
				t.Fatalf("lost diagnostic: %v", err)
			}
			if after, _, _ := config.ReadFile(s.cfgDir); !reflect.DeepEqual(before, after) {
				t.Fatal("failure mutated config")
			}
		})
	}
}

func TestAuthReuseExpiredIdentityIsUnavailable(t *testing.T) {
	s, _, services := reuseFixture(t)
	services.Verify = func(context.Context, config.Config, auth.Credential) (reuse.Account, error) {
		return reuse.Account{}, rejected()
	}
	r, err := reuseAuthentication(s, reuseOptions{DryRun: true}, services)
	if err != nil || r.State != "unavailable" || r.Changed {
		t.Fatalf("expired credential: %+v %v", r, err)
	}
}

func TestAuthReuseSelfConfigurationAndConcurrentChange(t *testing.T) {
	s, _, services := reuseFixture(t)
	s.resolved.Config.Defaults.ReadOnly = true
	r, err := reuseAuthentication(s, reuseOptions{DryRun: true}, services)
	if err != nil || !r.Changed {
		t.Fatal("read-only preview blocked", err)
	}
	r, err = reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || r.State != "reused" {
		t.Fatalf("native self-configuration should remain available: %+v %v", r, err)
	}

	s, _, services = reuseFixture(t)
	verify := services.Verify
	services.Verify = func(ctx context.Context, cfg config.Config, cred auth.Credential) (reuse.Account, error) {
		f, _, _ := config.ReadFile(s.cfgDir)
		f.CurrentContext = "changed-by-user"
		if err := config.WriteFile(s.cfgDir, f); err != nil {
			return reuse.Account{}, err
		}
		return verify(ctx, cfg, cred)
	}
	_, err = reuseAuthentication(s, reuseOptions{}, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_CONFIG_CHANGED" {
		t.Fatalf("concurrent change: %v", err)
	}
}

func TestAuthReuseRejectsCredentialRotationBeforeAssociation(t *testing.T) {
	s, store, services := reuseFixture(t)
	before, _, _ := config.ReadFile(s.cfgDir)
	resolve := services.Resolve
	calls := 0
	services.Resolve = func(cfg config.Config) (auth.Credential, credstore.Lookup, error) {
		calls++
		if calls > 1 {
			store[auth.AccountKey(reuseBase, "basic")] = "rotated-secret"
		}
		return resolve(cfg)
	}
	_, err := reuseAuthentication(s, reuseOptions{}, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_CREDENTIAL_CHANGED" {
		t.Fatalf("credential rotation: %v", err)
	}
	if after, _, _ := config.ReadFile(s.cfgDir); !reflect.DeepEqual(before, after) {
		t.Fatal("rotation changed destination")
	}
}

func TestAuthReuseRejectsTargetOverridesAndUnknownSource(t *testing.T) {
	s, _, services := reuseFixture(t)
	s.resolved.Config.BaseURL = "https://service.example.test/other"
	_, err := reuseAuthentication(s, reuseOptions{DryRun: true}, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_TARGET_MISMATCH" {
		t.Fatal(err)
	}
	s.resolved.Config.BaseURL = reuseBase
	_, err = reuseAuthentication(s, reuseOptions{From: "missing", DryRun: true}, services)
	if err == nil || cerrors.AsCLIError(err).Code != "AUTH_REUSE_SOURCE_NOT_FOUND" {
		t.Fatal(err)
	}
}

// A source's scheme comes from its stored fields and built-in defaults (pat
// on Data Center), never from this invocation's environment, and its label is
// not copied to a destination that adopts a pat.
func TestAuthReuseUsesPersistedDefaultsWithoutEnvironmentIdentity(t *testing.T) {
	s, store, services := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "basic"}},
	)
	store[auth.AccountKey(reuseBase, "pat")] = "pat-secret"
	s.resolved.Secrets = config.Secrets{Password: "environment-secret", PAT: "environment-token"}
	services.Verify = func(_ context.Context, _ config.Config, cred auth.Credential) (reuse.Account, error) {
		if cred.Scheme == "pat" && cred.Secret == "pat-secret" {
			return reuse.Account{ID: "key-1"}, nil
		}
		return reuse.Account{}, rejected()
	}
	r, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || r.State != "reused" || r.SourceContext != "personal" || r.Scheme != "pat" || r.Username != "" {
		t.Fatalf("stored defaults: %+v %v", r, err)
	}
	if got := targetOf(t, s); got.Auth.Scheme != "pat" || got.Auth.Username != "" {
		t.Fatalf("destination %+v", got)
	}
}

func TestAuthReuseMatchesPreviouslyDetectedFlavor(t *testing.T) {
	s, _, services := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: reuseBase, Flavor: "auto", DetectedFlavor: "datacenter", Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "basic"}},
	)
	verify := services.Verify
	services.Verify = func(ctx context.Context, cfg config.Config, cred auth.Credential) (reuse.Account, error) {
		if cfg.Flavor != "datacenter" {
			t.Fatal("ignored verified deployment flavor")
		}
		return verify(ctx, cfg, cred)
	}
	r, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || r.State != "reused" {
		t.Fatalf("detected flavor: %+v %v", r, err)
	}
}

func TestAuthReusePreservesWorkingTokenWithoutUsername(t *testing.T) {
	const respelled = "https://SERVICE.example.test:443/deploy/"
	s, store, services := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: respelled, Auth: config.AuthConfig{Scheme: "pat", Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "pat"}},
	)
	store[auth.AccountKey(reuseBase, "pat")] = "destination-secret"
	store[auth.AccountKey(respelled, "pat")] = "other-source-secret"
	before, _, _ := config.ReadFile(s.cfgDir)
	services.Verify = func(_ context.Context, _ config.Config, cred auth.Credential) (reuse.Account, error) {
		if cred.Secret != "destination-secret" {
			t.Fatal("attempted to switch an already valid destination login")
		}
		return reuse.Account{ID: "key-1"}, nil
	}
	r, err := reuseAuthentication(s, reuseOptions{}, services)
	if err != nil || r.Changed || !r.Verified || r.State != "unchanged" {
		t.Fatalf("existing token: %+v %v", r, err)
	}
	if after, _, _ := config.ReadFile(s.cfgDir); !reflect.DeepEqual(before, after) {
		t.Fatal("modified a valid token-only context")
	}
}

// The command resolves through the native store, verifies with the native API
// client on both flavors and reports the canonical account: accountId on
// Cloud, userKey on Data Center. Two usernames of one account are one login.
func TestAuthReuseCommandVerifiesNativeStoredCredential(t *testing.T) {
	for _, flavor := range []string{"cloud", "datacenter"} {
		t.Run(flavor, func(t *testing.T) {
			keyring.MockInit()
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				user, password, ok := r.BasicAuth()
				if r.Method != http.MethodGet || !ok || !strings.HasPrefix(user, "existing-user") || password != "retained-secret" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"type":"known","username":"` + user + `","accountId":"cloud-account","userKey":"dc-key","displayName":"Existing User"}`))
			}))
			defer server.Close()
			base := server.URL + "/deploy"
			s, _, _ := reuseFixture(t,
				config.NamedContext{Name: "personal", BaseURL: base, Flavor: flavor, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
				config.NamedContext{Name: "personal-email", BaseURL: base, Flavor: flavor, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user@example.test"}},
				config.NamedContext{Name: "team", BaseURL: base, Flavor: flavor, Auth: config.AuthConfig{Scheme: "basic"}},
			)
			s.store = auth.NewStore(s.cfgDir)
			credential := auth.Credential{Scheme: "basic", Username: "existing-user", Secret: "retained-secret"}
			if _, err := auth.Save(base, credential, s.store); err != nil {
				t.Fatal(err)
			}
			r, err := reuseAuthentication(s, reuseOptions{}, s.reuseServices())
			want := map[string]string{"cloud": "cloud-account", "datacenter": "dc-key"}[flavor]
			if err != nil || r.State != "reused" || r.SourceContext != "personal" || r.Account != want || r.AccountName != "Existing User" {
				t.Fatalf("got %+v %v", r, err)
			}
			if requests == 0 {
				t.Fatal("did not verify with the native API client")
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

func TestAuthReusePreservesLegacyStoreKeyUnderEquivalentOverride(t *testing.T) {
	keyring.MockInit()
	legacy := "https://SERVICE.example.test:443/deploy/"
	s, _, services := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: legacy, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic"}},
	)
	store := auth.NewStore(s.cfgDir)
	credential := auth.Credential{Scheme: "basic", Username: "existing-user", Secret: "retained-secret"}
	if _, err := auth.Save(legacy, credential, store); err != nil {
		t.Fatal(err)
	}
	services.Resolve = func(cfg config.Config) (auth.Credential, credstore.Lookup, error) {
		return auth.ResolveLookup(cfg, config.Secrets{}, store)
	}
	services.Lookup = store.Lookup
	services.Verify = func(context.Context, config.Config, auth.Credential) (reuse.Account, error) {
		return reuse.Account{}, nil
	}
	if _, err := reuseAuthentication(s, reuseOptions{}, services); err != nil {
		t.Fatal(err)
	}
	fresh, err := config.Load(config.LoadOptions{ConfigDir: s.cfgDir, Context: "team", DotenvPath: filepath.Join(t.TempDir(), "absent"), Flags: config.FlagValues{BaseURL: reuseBase}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := auth.Resolve(fresh.Config, config.Secrets{}, auth.NewStore(s.cfgDir))
	if err != nil || got.Secret != credential.Secret || got.Username != credential.Username {
		t.Fatalf("equivalent override lost native credential: %v", err)
	}
	if fresh.Config.BaseURL != reuseBase {
		t.Fatal("changed request destination")
	}
	if err := auth.ForgetForConfig(fresh.Config, fresh.Config.Auth.Scheme, store); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(auth.AccountKey(legacy, credential.Scheme)); err == nil {
		t.Fatal("logout left the original credential active")
	}
	fresh.Config.BaseURL = "https://service.example.test/other"
	_, err = auth.Resolve(fresh.Config, config.Secrets{}, store)
	if err == nil || cerrors.AsCLIError(err).Code != "CREDENTIAL_SERVICE_MISMATCH" {
		t.Fatalf("foreign deployment reused stored key: %v", err)
	}
}

// A source-selection failure recovers through the context listing. That
// command's name differs across the CLI family, so resolve what each error
// advertises against this command tree instead of trusting its spelling.
func TestAuthReuseSourceRecoveryNamesARealCommand(t *testing.T) {
	failures := map[string]error{}

	s, _, services := reuseFixture(t)
	_, failures["AUTH_REUSE_SOURCE_NOT_FOUND"] = reuseAuthentication(s, reuseOptions{From: "missing", DryRun: true}, services)

	s, _, services = reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "second", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic", Username: "second-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic"}},
	)
	_, failures["AUTH_REUSE_AMBIGUOUS"] = reuseAuthentication(s, reuseOptions{DryRun: true}, services)

	s, _, services = reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: "https://other.example.test/deploy", Auth: config.AuthConfig{Scheme: "basic", Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: reuseBase, Auth: config.AuthConfig{Scheme: "basic"}},
	)
	_, failures["AUTH_REUSE_SOURCE_MISMATCH"] = reuseAuthentication(s, reuseOptions{From: "personal", DryRun: true}, services)

	s, store, services := reuseFixture(t,
		config.NamedContext{Name: "team", BaseURL: reuseBase, Flavor: "cloud", Auth: config.AuthConfig{Scheme: "basic"}},
	)
	store[auth.AccountKey(reuseBase, "pat")] = "pat-secret"
	_, failures["AUTH_REUSE_SOURCE_MISMATCH Cloud pat"] = reuseAuthentication(s, reuseOptions{FromScheme: "pat", DryRun: true}, services)

	for name, err := range failures {
		code, _, _ := strings.Cut(name, " ")
		if err == nil || cerrors.AsCLIError(err).Code != code {
			t.Fatalf("%s: got %v", name, err)
		}
		steps := cerrors.AsCLIError(err).NextSteps
		if len(steps) == 0 {
			t.Fatalf("%s: no recovery command", name)
		}
		words := strings.Fields(steps[0])
		if words[0] != constants.AppName {
			t.Fatalf("%s: recovery %q is not a %s command", name, steps[0], constants.AppName)
		}
		cmd, rest, findErr := NewRootCmd().Find(words[1:])
		if findErr != nil || len(rest) != 0 || !cmd.Runnable() {
			t.Errorf("%s: recovery %q does not resolve to a command (unresolved %v, %v)", name, steps[0], rest, findErr)
		}
	}
}

// A rejected credential leads with reuse when a stored login could replace
// it; other failures and the reuse and login commands are left alone.
func TestRejectedCredentialRecoveryLeadsWithReuse(t *testing.T) {
	s := &appState{resolved: &config.Resolved{Config: config.Config{BaseURL: reuseBase,
		Auth: config.AuthConfig{Scheme: "pat"}, ReuseStep: "confluence-cli --use-context 'team' auth reuse"}}}
	ce := rejected()
	withReuseRecovery(ce, s, &cobra.Command{Use: "search"})
	if len(ce.NextSteps) < 2 || ce.NextSteps[0] != s.cfg().ReuseStep {
		t.Fatalf("next steps %v", ce.NextSteps)
	}
	for _, tc := range []struct {
		err *cerrors.CLIError
		cmd string
	}{
		{cerrors.New(cerrors.CategoryPermission, "FORBIDDEN", "denied").WithHTTPStatus(403), "search"},
		{rejected(), "reuse"},
		{rejected(), "login"},
	} {
		steps := append([]string(nil), tc.err.NextSteps...)
		withReuseRecovery(tc.err, s, &cobra.Command{Use: tc.cmd})
		if !reflect.DeepEqual(steps, tc.err.NextSteps) {
			t.Fatalf("%s from %s gained a reuse step: %v", tc.err.Code, tc.cmd, tc.err.NextSteps)
		}
	}
}

// doctor offers reuse for a missing or rejected credential, not for an
// unreadable store, a permission or a network failure.
func TestDoctorOffersReuseOnlyForAuthentication(t *testing.T) {
	missing := cerrors.New(cerrors.CategoryConfig, "CREDENTIAL_NOT_VISIBLE_OR_MISSING", "missing")
	inaccessible := cerrors.New(cerrors.CategoryConfig, "CREDENTIAL_STORE_INACCESSIBLE", "locked")
	forbidden := cerrors.New(cerrors.CategoryPermission, "FORBIDDEN", "denied").WithHTTPStatus(403)
	network := cerrors.New(cerrors.CategoryNetwork, "NETWORK_ERROR", "down")
	for _, tc := range []struct {
		cred, conn error
		want       bool
	}{
		{missing, nil, true}, {nil, rejected(), true},
		{inaccessible, nil, false}, {nil, forbidden, false}, {nil, network, false}, {nil, nil, false},
	} {
		if got := authUnhealthy(tc.cred, tc.conn); got != tc.want {
			t.Errorf("authUnhealthy(%v, %v) = %v", tc.cred, tc.conn, got)
		}
	}
}

// A Data Center user without a userKey has no canonical account: its username
// must not stand in for one, or it could equal another user's key.
func TestCanonicalAccountDoesNotMixIdentifierKinds(t *testing.T) {
	keyed := canonicalAccount(apiclient.FlavorDataCenter, &apiclient.User{UserKey: "k", Username: "alice"})
	named := canonicalAccount(apiclient.FlavorDataCenter, &apiclient.User{Username: "k"})
	if keyed.ID != "k" || named.ID != "" || named.Name != "k" {
		t.Fatalf("keyed %+v, named %+v", keyed, named)
	}
}

// doctor offers reuse when only the current-user probe is rejected.
func TestDoctorOffersReuseWhenOnlyCurrentUserIsRejected(t *testing.T) {
	keyring.MockInit()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "user/current") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"size":0,"limit":1,"start":0,"_links":{}}`))
	}))
	defer server.Close()
	base := server.URL + "/deploy"
	s, _, _ := reuseFixture(t,
		config.NamedContext{Name: "personal", BaseURL: base, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat", Username: "existing-user"}},
		config.NamedContext{Name: "team", BaseURL: base, Flavor: "datacenter", Auth: config.AuthConfig{Scheme: "pat"}},
	)
	s.store = auth.NewStore(s.cfgDir)
	if _, err := auth.Save(base, auth.Credential{Scheme: "pat", Secret: "stale"}, s.store); err != nil {
		t.Fatal(err)
	}
	s.resolved.Config.ReuseStep = "confluence-cli --use-context 'team' auth reuse"
	report := runDoctor(s, true)
	if len(report.NextSteps) != 1 || report.NextSteps[0] != s.resolved.Config.ReuseStep {
		t.Fatalf("next steps %v, checks %+v", report.NextSteps, report.Checks)
	}
}

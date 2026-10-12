package auth

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/angelmsger/agent-cli-kit/credstore"
	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/zalando/go-keyring"
)

// failingKeyring is a keychain backend whose every call fails with err.
type failingKeyring struct{ err error }

func (f failingKeyring) Get(string, string) (string, error) { return "", f.err }
func (f failingKeyring) Set(string, string, string) error   { return f.err }
func (f failingKeyring) Delete(string, string) error        { return f.err }

// newStoreWithKeyring returns confluence-cli's Store over a substitute keychain
// backend. The store's own behavior is tested in agent-cli-kit/credstore.
func newStoreWithKeyring(dir string, backend credstore.Backend) *Store {
	return credstore.NewStoreWithBackend(dir, constants.KeychainService, backend)
}

func TestMain(m *testing.M) {
	// Use an in-memory keychain for all tests in this package.
	keyring.MockInit()
	os.Exit(m.Run())
}

// TestCredentialValidate pins each scheme's error code and message; header
// encoding and masking are tested in agent-cli-kit/auth.
func TestCredentialValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cred    Credential
		code    string
		message string
	}{
		{"pat ok", Credential{Scheme: SchemePAT, Secret: "tok"}, "", ""},
		{"pat missing secret", Credential{Scheme: SchemePAT}, "AUTH_NO_TOKEN", "no Personal Access Token configured"},
		{"basic ok", Credential{Scheme: SchemeBasic, Username: "u", Secret: "p"}, "", ""},
		{"basic missing user", Credential{Scheme: SchemeBasic, Secret: "p"}, "AUTH_NO_BASIC", "basic auth requires both a username and a password/token"},
		{"basic missing secret", Credential{Scheme: SchemeBasic, Username: "u"}, "AUTH_NO_BASIC", "basic auth requires both a username and a password/token"},
		{"unknown scheme", Credential{Scheme: "oauth", Secret: "x"}, "AUTH_BAD_SCHEME", `unknown auth scheme "oauth"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cred.Validate()
			if tc.code == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			ce := cerrors.AsCLIError(err)
			if err == nil || ce.Code != tc.code || ce.Message != tc.message {
				t.Fatalf("Validate() = %+v, want %s %q", ce, tc.code, tc.message)
			}
		})
	}
}

// TestCredentialHeader pins each scheme's wire form: pat is a Bearer token,
// basic is HTTP Basic.
func TestCredentialHeader(t *testing.T) {
	t.Parallel()
	pat := Credential{Scheme: SchemePAT, Secret: "abc"}
	if got := pat.Header(); got != "Bearer abc" {
		t.Errorf("pat header = %q", got)
	}
	basic := Credential{Scheme: SchemeBasic, Username: "alice", Secret: "pw"}
	// base64("alice:pw") == YWxpY2U6cHc=
	if got := basic.Header(); got != "Basic YWxpY2U6cHc=" {
		t.Errorf("basic header = %q", got)
	}
}

// TestAccountKey pins the keychain account: host and scheme, without the
// deployment path, so existing stored secrets stay reachable.
func TestAccountKey(t *testing.T) {
	t.Parallel()
	got := AccountKey("https://kms.example.com/wiki", SchemePAT)
	if got != "kms.example.com:pat" {
		t.Errorf("AccountKey = %q", got)
	}
}

func TestResolveDefaultsToPAT(t *testing.T) {
	t.Parallel()
	cfg := config.Config{BaseURL: "https://kms.example.com"}
	cred, err := Resolve(cfg, config.Secrets{PAT: "flag-token"}, NewStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if cred.Scheme != SchemePAT || cred.Secret != "flag-token" {
		t.Errorf("Resolve() = %+v, want pat with flag-token", cred)
	}
}

func TestResolveBasicSecret(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		BaseURL: "https://kms.example.com",
		Auth:    config.AuthConfig{Scheme: SchemeBasic, Username: "alice"},
	}
	tests := []struct {
		name    string
		secrets config.Secrets
		want    string
	}{
		{"password", config.Secrets{Password: "pw"}, "pw"},
		{"api token", config.Secrets{APIToken: "api"}, "api"},
		{"password wins over api token", config.Secrets{Password: "pw", APIToken: "api"}, "pw"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cred, err := Resolve(cfg, tc.secrets, NewStore(t.TempDir()))
			if err != nil {
				t.Fatal(err)
			}
			if cred.Secret != tc.want {
				t.Errorf("Secret = %q, want %q", cred.Secret, tc.want)
			}
		})
	}
}

func TestResolvePrefersTransientSecret(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir())
	if _, err := s.Save("kms.example.com:pat", "stored-token"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		BaseURL: "https://kms.example.com",
		Auth:    config.AuthConfig{Scheme: SchemePAT},
	}
	cred, err := Resolve(cfg, config.Secrets{PAT: "flag-token"}, s)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Secret != "flag-token" {
		t.Errorf("Secret = %q, want transient flag-token", cred.Secret)
	}
}

func TestResolveFallsBackToStore(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir())
	if _, err := s.Save("kms2.example.com:pat", "stored-token"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		BaseURL: "https://kms2.example.com",
		Auth:    config.AuthConfig{Scheme: SchemePAT},
	}
	cred, err := Resolve(cfg, config.Secrets{}, s)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Secret != "stored-token" {
		t.Errorf("Secret = %q, want stored-token", cred.Secret)
	}
}

func TestResolveNoCredentialErrors(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		BaseURL: "https://nocreds.example.com",
		Auth:    config.AuthConfig{Scheme: SchemePAT},
	}
	hostRetry := "Retry the same command with access to the host user environment (home directory and OS keychain)."
	tests := []struct {
		name    string
		err     error
		code    string
		message string
		steps   []string
	}{
		{"missing", keyring.ErrNotFound, "CREDENTIAL_NOT_VISIBLE_OR_MISSING",
			"stored Confluence credentials are missing or not visible in this execution environment",
			[]string{hostRetry, "confluence-cli doctor", "Only if the host retry also reports missing credentials, run `confluence-cli config init` in the user's terminal or set CONFLUENCE_* environment variables."}},
		{"inaccessible", errors.New("sandbox denied keychain"), "CREDENTIAL_STORE_INACCESSIBLE",
			"stored Confluence credentials cannot be read in this execution environment",
			[]string{hostRetry, "confluence-cli doctor", "Do not run `config init` unless the same check also fails in the host environment."}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newStoreWithKeyring(t.TempDir(), failingKeyring{err: tc.err})
			_, err := Resolve(cfg, config.Secrets{}, s)
			ce := cerrors.AsCLIError(err)
			if ce == nil || ce.Code != tc.code || ce.Message != tc.message {
				t.Fatalf("Resolve() error = %+v, want %s %q", ce, tc.code, tc.message)
			}
			if len(ce.NextSteps) < len(tc.steps) || !slices.Equal(ce.NextSteps[:len(tc.steps)], tc.steps) {
				t.Fatalf("Resolve() next steps = %q, want them to start with %q", ce.NextSteps, tc.steps)
			}
			if ce.Recovery == nil || ce.Recovery.Scope != "host" || ce.Retryable {
				t.Fatalf("Resolve() recovery = %+v retryable=%v, want host and false", ce.Recovery, ce.Retryable)
			}
		})
	}
}

package auth

import (
	"errors"
	"net/http"
	"os"
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

func TestCredentialValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cred Credential
		ok   bool
	}{
		{"pat ok", Credential{Scheme: SchemePAT, Secret: "tok"}, true},
		{"pat missing secret", Credential{Scheme: SchemePAT}, false},
		{"basic ok", Credential{Scheme: SchemeBasic, Username: "u", Secret: "p"}, true},
		{"basic missing user", Credential{Scheme: SchemeBasic, Secret: "p"}, false},
		{"unknown scheme", Credential{Scheme: "oauth", Secret: "x"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cred.Validate()
			if (err == nil) != tc.ok {
				t.Errorf("Validate() err=%v, want ok=%v", err, tc.ok)
			}
		})
	}
}

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

func TestDecoratorSetsAuthorization(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	Credential{Scheme: SchemePAT, Secret: "tok"}.Decorator()(req)
	if req.Header.Get("Authorization") != "Bearer tok" {
		t.Errorf("Authorization = %q", req.Header.Get("Authorization"))
	}
}

func TestAccountKey(t *testing.T) {
	t.Parallel()
	got := AccountKey("https://kms.example.com/wiki", SchemePAT)
	if got != "kms.example.com:pat" {
		t.Errorf("AccountKey = %q", got)
	}
}

func TestRedacted(t *testing.T) {
	t.Parallel()
	r := Credential{Scheme: SchemePAT, Secret: "supersecret"}.Redacted()
	if r.Secret == "supersecret" || r.Secret[len(r.Secret)-4:] != "cret" {
		t.Errorf("Redacted secret = %q", r.Secret)
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
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"missing", keyring.ErrNotFound, "CREDENTIAL_NOT_VISIBLE_OR_MISSING"},
		{"inaccessible", errors.New("sandbox denied keychain"), "CREDENTIAL_STORE_INACCESSIBLE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newStoreWithKeyring(t.TempDir(), failingKeyring{err: tc.err})
			_, err := Resolve(cfg, config.Secrets{}, s)
			ce := cerrors.AsCLIError(err)
			if ce == nil || ce.Code != tc.code {
				t.Fatalf("Resolve() error = %+v, want code %s", ce, tc.code)
			}
			if ce.Recovery == nil || ce.Recovery.Scope != "host" || ce.Retryable {
				t.Fatalf("Resolve() recovery = %+v retryable=%v, want host and false", ce.Recovery, ce.Retryable)
			}
		})
	}
}

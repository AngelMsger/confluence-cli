package auth

import (
	"net/http"
	"os"
	"testing"

	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/zalando/go-keyring"
)

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

func TestStoreKeychainRoundTrip(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir())
	backend, err := s.Save("host:pat", "tok-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if backend != BackendKeychain {
		t.Errorf("backend = %q, want keychain", backend)
	}
	got, err := s.Load("host:pat")
	if err != nil {
		t.Fatal(err)
	}
	if got != "tok-xyz" {
		t.Errorf("Load = %q", got)
	}
	if err := s.Delete("host:pat"); err != nil {
		t.Fatal(err)
	}
}

func TestStoreFileFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.fileSave("acct", "filesecret"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.credentialsPath())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials file perm = %o, want 600", perm)
	}
	got, err := s.fileLoad("acct")
	if err != nil || got != "filesecret" {
		t.Errorf("fileLoad = %q, %v", got, err)
	}
	if _, err := s.fileLoad("missing"); err != ErrSecretNotFound {
		t.Errorf("fileLoad(missing) err = %v, want ErrSecretNotFound", err)
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
	s := NewStore(t.TempDir())
	cfg := config.Config{
		BaseURL: "https://nocreds.example.com",
		Auth:    config.AuthConfig{Scheme: SchemePAT},
	}
	if _, err := Resolve(cfg, config.Secrets{}, s); err == nil {
		t.Error("expected error when no credential is available")
	}
}

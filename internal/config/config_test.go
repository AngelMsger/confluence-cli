package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestFileFormat pins the bytes of config.yaml. The engine is shared; the
// shape is confluence-cli's, and existing files must keep reading and writing
// the same way.
func TestFileFormat(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cfg")
	in := File{
		CurrentContext: "prod",
		Contexts: []NamedContext{
			{Name: "default", BaseURL: "https://dc.example.com", Flavor: FlavorDataCenter,
				Auth: AuthConfig{Scheme: SchemePAT}},
			{Name: "prod", BaseURL: "https://acme.atlassian.net/wiki", Flavor: FlavorAuto,
				DetectedFlavor: FlavorCloud, Auth: AuthConfig{Scheme: SchemeBasic, Username: "alice@acme.com",
					CredentialURL: "https://id.atlassian.com/manage-profile/security/api-tokens"}},
		},
		Defaults: Defaults{Format: "table", PageSize: 50, Timeout: 90 * time.Second, MaxRetries: 5, ReadOnly: true},
	}
	if err := WriteFile(dir, in); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(ConfigFilePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	const want = `current_context: prod
contexts:
    - name: default
      server: https://dc.example.com
      flavor: datacenter
      auth:
        scheme: pat
    - name: prod
      server: https://acme.atlassian.net/wiki
      flavor: auto
      detected_flavor: cloud
      auth:
        credential_url: https://id.atlassian.com/manage-profile/security/api-tokens
        scheme: basic
        username: alice@acme.com
defaults:
    format: table
    page_size: 50
    timeout: 1m30s
    max_retries: 5
    read_only: true
`
	if string(raw) != want {
		t.Errorf("config.yaml changed:\n%s", raw)
	}
	got, ok, err := ReadFile(dir)
	if err != nil || !ok || !reflect.DeepEqual(got, in) {
		t.Errorf("read back %+v, %v, %v", got, ok, err)
	}
}

// TestReadFileLegacyFlatFormat covers the pre-contexts file: its root fields
// become one context named "default", which is current, and the next write
// uses the multi-context form.
func TestReadFileLegacyFlatFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, ConfigFilePath(dir),
		"server: https://legacy.example.com\nflavor: datacenter\ndetected_flavor: datacenter\nauth:\n  scheme: pat\n  username: ops\ndefaults:\n  page_size: 10\n")
	got, exists, err := ReadFile(dir)
	if err != nil || !exists {
		t.Fatalf("ReadFile: exists=%v err=%v", exists, err)
	}
	want := File{
		CurrentContext: DefaultContextName,
		Contexts: []NamedContext{{Name: DefaultContextName, BaseURL: "https://legacy.example.com", Flavor: FlavorDataCenter,
			DetectedFlavor: FlavorDataCenter, Auth: AuthConfig{Scheme: SchemePAT, Username: "ops"}}},
		Defaults: Defaults{PageSize: 10},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy file = %+v, want %+v", got, want)
	}
	if err := WriteFile(dir, got); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(ConfigFilePath(dir))
	const rewritten = `current_context: default
contexts:
    - name: default
      server: https://legacy.example.com
      flavor: datacenter
      detected_flavor: datacenter
      auth:
        scheme: pat
        username: ops
defaults:
    page_size: 10
`
	if string(raw) != rewritten {
		t.Errorf("legacy file rewritten as:\n%s", raw)
	}
}

func TestLoadLegacyFlatUnchanged(t *testing.T) {
	cleanSetupEnv(t)
	dir := t.TempDir()
	writeFile(t, ConfigFilePath(dir), "server: https://file.example.com\nflavor: datacenter\n")
	got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: filepath.Join(dir, "absent.env")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.BaseURL != "https://file.example.com" || got.Config.Flavor != FlavorDataCenter {
		t.Errorf("legacy load mismatch: %+v", got.Config)
	}
	if got.Sources[fieldServer] != "file" {
		t.Errorf("server source = %q, want file", got.Sources[fieldServer])
	}
	if got.ActiveContext != DefaultContextName || got.ContextSource != ContextSourceCurrent {
		t.Errorf("ActiveContext = %q (%s), want default", got.ActiveContext, got.ContextSource)
	}
}

// TestSchema pins confluence-cli's environment variables, defaults and scheme
// inference.
func TestSchema(t *testing.T) {
	cleanSetupEnv(t)
	dir := t.TempDir()
	load := func() *Resolved {
		t.Helper()
		r, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: filepath.Join(dir, "absent.env")})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r := load()
	defaults := Defaults{Format: "json", PageSize: 25, Timeout: 30 * time.Second, MaxRetries: 3}
	want := Config{Flavor: FlavorAuto, Auth: AuthConfig{Scheme: SchemePAT}, Defaults: defaults}
	if !reflect.DeepEqual(r.Config, want) || r.Sources[FieldAuthScheme] != "default" || r.ContextSource != ContextSourceNone {
		t.Fatalf("defaults = %+v (%v)", r.Config, r.Sources)
	}
	// The wizard writes the built-in defaults into a fresh file.
	if got := defaultConfig().Defaults; got != defaults {
		t.Errorf("wizard defaults = %+v", got)
	}

	t.Setenv("CONFLUENCE_SERVER", "https://wiki.example.com")
	t.Setenv("CONFLUENCE_FLAVOR", FlavorDataCenter)
	t.Setenv("CONFLUENCE_USERNAME", "alice")
	t.Setenv("CONFLUENCE_CREDENTIAL_URL", "https://help.example.com/token")
	t.Setenv("CONFLUENCE_FORMAT", "table")
	t.Setenv("CONFLUENCE_CLI_READ_ONLY", "yes")
	t.Setenv("CONFLUENCE_API_TOKEN", "api")
	r = load()
	if r.Config.BaseURL != "https://wiki.example.com" || r.Config.Flavor != FlavorDataCenter || r.Config.Auth.Username != "alice" ||
		r.Config.Auth.Scheme != SchemeBasic || r.Config.Auth.CredentialURL != "https://help.example.com/token" ||
		r.Config.Defaults.Format != "table" || !r.Config.Defaults.ReadOnly || r.Secrets != (Secrets{APIToken: "api"}) {
		t.Fatalf("environment = %+v %+v", r.Config, r.Secrets)
	}
	t.Setenv("CONFLUENCE_PASSWORD", "pw")
	if r = load(); r.Config.Auth.Scheme != SchemeBasic || r.Secrets.Password != "pw" {
		t.Errorf("a password selects basic: %+v %+v", r.Config.Auth, r.Secrets)
	}
	t.Setenv("CONFLUENCE_PERSONAL_ACCESS_TOKEN", "tok")
	if r = load(); r.Config.Auth.Scheme != SchemePAT || r.Secrets.PAT != "tok" {
		t.Errorf("a personal access token wins over a password: %q", r.Config.Auth.Scheme)
	}
	t.Setenv("CONFLUENCE_AUTH_SCHEME", SchemeBasic)
	if r = load(); r.Config.Auth.Scheme != SchemeBasic {
		t.Errorf("an explicit scheme wins: %q", r.Config.Auth.Scheme)
	}
	if _, err := NormalizeServiceURL("wiki.example.com"); cerrors.AsCLIError(err).Code != "BAD_BASE_URL" {
		t.Errorf("a bare host must be BAD_BASE_URL, got %v", err)
	}

	writeFile(t, ConfigFilePath(dir), "current_context: a\ncontexts:\n  - name: a\n  - name: b\n")
	t.Setenv("CONFLUENCE_CONTEXT", "B")
	if r = load(); r.ActiveContext != "b" || r.ContextSource != ContextSourceEnv {
		t.Errorf("CONFLUENCE_CONTEXT selects the context: %q (%s)", r.ActiveContext, r.ContextSource)
	}
}

// TestFlavorRules pins the Atlassian rules registered with the engine: Cloud
// defaults to basic auth, Cloud rejects a personal access token, the flavor
// is checked, and a detected flavor is dropped once the server or flavor
// changes.
func TestFlavorRules(t *testing.T) {
	cleanSetupEnv(t)
	dir := t.TempDir()
	absent := filepath.Join(dir, "absent.env")
	writeFile(t, ConfigFilePath(dir), `current_context: dc
contexts:
  - name: dc
    server: HTTPS://Wiki.Example.com:443/
    flavor: auto
    detected_flavor: datacenter
    auth: {scheme: pat}
  - name: cloud
    server: https://acme.atlassian.net/wiki
  - name: probed
    server: https://wiki.acme.corp
    detected_flavor: cloud
`)
	load := func(opt LoadOptions) *Resolved {
		t.Helper()
		opt.ConfigDir, opt.DotenvPath = dir, absent
		r, err := Load(opt)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	if r := load(LoadOptions{Context: "cloud"}); r.Config.Auth.Scheme != SchemeBasic || r.Sources[fieldAuthScheme] != "default" {
		t.Errorf("Cloud host defaults to basic: %q (%s)", r.Config.Auth.Scheme, r.Sources[fieldAuthScheme])
	}
	if r := load(LoadOptions{Context: "probed"}); r.Config.Auth.Scheme != SchemeBasic {
		t.Errorf("a detected Cloud flavor defaults to basic: %q", r.Config.Auth.Scheme)
	}
	if r := load(LoadOptions{Context: "cloud", Flags: FlagValues{AuthScheme: SchemePAT}}); r.Config.Auth.Scheme != SchemePAT {
		t.Errorf("an explicit scheme is kept: %q", r.Config.Auth.Scheme)
	}
	f, _, _ := ReadFile(dir)
	cloud, _ := f.Context("cloud")
	if got := StoredContext(cloud, f.Defaults); got.Auth.Scheme != SchemeBasic {
		t.Errorf("StoredContext defaults Cloud to basic: %q", got.Auth.Scheme)
	}

	r := load(LoadOptions{Flags: FlagValues{BaseURL: "https://wiki.example.com"}})
	if r.Config.DetectedFlavor != FlavorDataCenter || r.Config.CredentialBaseURL != "HTTPS://Wiki.Example.com:443/" {
		t.Errorf("an equivalent spelling keeps the detected flavor and the stored key: %+v", r.Config)
	}
	r = load(LoadOptions{Flags: FlagValues{BaseURL: "https://other.example.com"}})
	if r.Config.DetectedFlavor != "" || r.Config.CredentialBaseURL != "" {
		t.Errorf("another server drops the detected flavor: %+v", r.Config)
	}
	if _, ok := r.Sources[fieldDetectedFlavor]; ok {
		t.Errorf("a dropped detected flavor has no source: %v", r.Sources)
	}

	for _, tc := range []struct {
		cfg  Config
		code string
	}{
		{Config{BaseURL: "https://wiki.example.com", Flavor: "server", Auth: AuthConfig{Scheme: SchemePAT}}, "BAD_FLAVOR"},
		{Config{BaseURL: "https://acme.atlassian.net/wiki", Auth: AuthConfig{Scheme: SchemePAT}}, "AUTH_CLOUD_NEEDS_BASIC"},
		{Config{BaseURL: "https://wiki.example.com", Flavor: FlavorCloud, Auth: AuthConfig{Scheme: SchemePAT}}, "AUTH_CLOUD_NEEDS_BASIC"},
		{Config{BaseURL: "https://wiki.example.com", DetectedFlavor: FlavorCloud, Auth: AuthConfig{Scheme: SchemePAT}}, "AUTH_CLOUD_NEEDS_BASIC"},
		{Config{BaseURL: "https://wiki.example.com", Auth: AuthConfig{Scheme: "bearer"}}, "AUTH_BAD_SCHEME"},
		{Config{BaseURL: "https://acme.atlassian.net/wiki", Auth: AuthConfig{Scheme: SchemeBasic}}, ""},
	} {
		err := ValidateService(tc.cfg)
		if code := cerrors.AsCLIError(err); (tc.code == "" && err != nil) || (tc.code != "" && (code == nil || code.Code != tc.code)) {
			t.Errorf("ValidateService(%+v) = %v, want %q", tc.cfg, err, tc.code)
		}
	}
	if ce := cerrors.AsCLIError(ValidateService(Config{BaseURL: "https://x.example.com", Auth: AuthConfig{Scheme: "x"}})); ce.Hint != "Use one of: pat, basic." {
		t.Errorf("AUTH_BAD_SCHEME hint = %q", ce.Hint)
	}

	// set-context clears the cached flavor when the flavor changes.
	setup := load(LoadOptions{Setup: true, Context: "dc", Flags: FlagValues{Flavor: FlavorDataCenter}})
	plan, err := PlanServiceContext(f, "dc", setup, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if dc, _ := plan.File.Context("dc"); dc.Flavor != FlavorDataCenter || dc.DetectedFlavor != "" {
		t.Errorf("a flavor change clears the detected flavor: %+v", dc)
	}
}

// TestUnknownContextWording pins the UNKNOWN_CONTEXT envelope with
// confluence-cli's names for the flag, the variable and the listing command.
func TestUnknownContextWording(t *testing.T) {
	cleanSetupEnv(t)
	dir := t.TempDir()
	absent := filepath.Join(dir, "absent.env")
	writeFile(t, ConfigFilePath(dir), "current_context: ghost\ncontexts:\n  - name: alpha\n  - name: beta\n")
	check := func(err error, message, hint string) {
		t.Helper()
		ce := cerrors.AsCLIError(err)
		if ce == nil || ce.Code != "UNKNOWN_CONTEXT" || ce.Message != message || ce.Hint != hint ||
			!reflect.DeepEqual(ce.NextSteps, []string{"confluence-cli config get-contexts"}) {
			t.Errorf("got %#v", err)
		}
	}
	const listed = "Available contexts: alpha, beta."
	_, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: absent, Context: "Gamma"})
	check(err, `context "Gamma" (from --use-context) is not defined in the config file`, listed)
	t.Setenv("CONFLUENCE_CONTEXT", "gamma")
	_, err = Load(LoadOptions{ConfigDir: dir, DotenvPath: absent})
	check(err, `context "gamma" (from CONFLUENCE_CONTEXT) is not defined in the config file`, listed)
	t.Setenv("CONFLUENCE_CONTEXT", "")
	_, err = Load(LoadOptions{ConfigDir: dir, DotenvPath: absent})
	check(err, `context "ghost" (from current_context) is not defined in the config file`, listed)

	f := File{}
	_, err = UseContext(&f, "ghost")
	check(err, `context "ghost" is not defined`, "Run `confluence-cli config get-contexts` to list defined contexts.")
}

// TestResolveConfigDir covers the "prefer new, fall back to legacy" rule used
// when --config was not supplied. It overrides $HOME with a temp directory so
// the real user's config is never touched.
func TestResolveConfigDir(t *testing.T) {
	withFakeHome := func(t *testing.T) string {
		t.Helper()
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("USERPROFILE", home)
		return home
	}

	t.Run("neither exists → new path", func(t *testing.T) {
		home := withFakeHome(t)
		dir, err := ResolveConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(home, ".angelmsger", "confluence")
		if dir != want {
			t.Errorf("dir = %q, want %q", dir, want)
		}
		if legacy, _ := LegacyConfigDir(); legacy != filepath.Join(home, ".confluence") {
			t.Errorf("legacy dir = %q", legacy)
		}
	})

	t.Run("only legacy has config.yaml → legacy path", func(t *testing.T) {
		home := withFakeHome(t)
		legacy := filepath.Join(home, ".confluence")
		if err := os.MkdirAll(legacy, 0o700); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(legacy, "config.yaml"), "server: https://x")
		dir, err := ResolveConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		if dir != legacy {
			t.Errorf("dir = %q, want %q (legacy fallback)", dir, legacy)
		}
	})

	t.Run("new has config.yaml → new path wins", func(t *testing.T) {
		home := withFakeHome(t)
		newDir := filepath.Join(home, ".angelmsger", "confluence")
		if err := os.MkdirAll(newDir, 0o700); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(newDir, "config.yaml"), "server: https://y")
		// Also create legacy to ensure new still wins when both exist.
		legacy := filepath.Join(home, ".confluence")
		if err := os.MkdirAll(legacy, 0o700); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(legacy, "config.yaml"), "server: https://legacy")
		dir, err := ResolveConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		if dir != newDir {
			t.Errorf("dir = %q, want %q (new path)", dir, newDir)
		}
	})
}

package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: filepath.Join(dir, "absent.env")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.Flavor != FlavorAuto {
		t.Errorf("Flavor = %q, want auto", got.Config.Flavor)
	}
	if got.Config.Defaults.Format != "json" {
		t.Errorf("Format = %q, want json", got.Config.Defaults.Format)
	}
	if got.Config.Defaults.PageSize != 25 {
		t.Errorf("PageSize = %d, want 25", got.Config.Defaults.PageSize)
	}
}

func TestLoadFileLayer(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, ConfigFilePath(dir), "server: https://file.example.com\nflavor: datacenter\n")
	got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: filepath.Join(dir, "absent.env")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.BaseURL != "https://file.example.com" {
		t.Errorf("BaseURL = %q", got.Config.BaseURL)
	}
	if got.Sources[fieldServer] != "file" {
		t.Errorf("server source = %q, want file", got.Sources[fieldServer])
	}
}

func TestLoadPrecedenceEnvOverridesDotenvAndFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, ConfigFilePath(dir), "server: https://file.example.com\n")
	dotenv := filepath.Join(dir, ".env")
	writeFile(t, dotenv, "CONFLUENCE_SERVER=https://dotenv.example.com\n")
	t.Setenv("CONFLUENCE_SERVER", "https://env.example.com")

	got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: dotenv})
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.BaseURL != "https://env.example.com" {
		t.Errorf("BaseURL = %q, want env value", got.Config.BaseURL)
	}
	if got.Sources[fieldServer] != "env" {
		t.Errorf("source = %q, want env", got.Sources[fieldServer])
	}
}

func TestLoadFlagWinsOverEverything(t *testing.T) {
	dir := t.TempDir()
	dotenv := filepath.Join(dir, ".env")
	writeFile(t, dotenv, "CONFLUENCE_SERVER=https://dotenv.example.com\n")
	t.Setenv("CONFLUENCE_SERVER", "https://env.example.com")

	got, err := Load(LoadOptions{
		ConfigDir:  dir,
		DotenvPath: dotenv,
		Flags:      FlagValues{BaseURL: "https://flag.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.BaseURL != "https://flag.example.com" {
		t.Errorf("BaseURL = %q, want flag value", got.Config.BaseURL)
	}
	if got.Sources[fieldServer] != "flag" {
		t.Errorf("source = %q, want flag", got.Sources[fieldServer])
	}
}

func TestLoadDotenvOverridesFileButNotEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, ConfigFilePath(dir), "server: https://file.example.com\n")
	dotenv := filepath.Join(dir, ".env")
	writeFile(t, dotenv, "CONFLUENCE_SERVER=https://dotenv.example.com\n")

	got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: dotenv})
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.BaseURL != "https://dotenv.example.com" {
		t.Errorf("BaseURL = %q, want dotenv value", got.Config.BaseURL)
	}
}

func TestLoadSecretFromEnvInfersScheme(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CONFLUENCE_PERSONAL_ACCESS_TOKEN", "tok-123")
	got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: filepath.Join(dir, "absent.env")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Secrets.PAT != "tok-123" {
		t.Errorf("PAT = %q", got.Secrets.PAT)
	}
	if got.Config.Auth.Scheme != SchemePAT {
		t.Errorf("scheme = %q, want pat", got.Config.Auth.Scheme)
	}
}

func TestDotenvDoesNotMutateProcessEnv(t *testing.T) {
	dir := t.TempDir()
	dotenv := filepath.Join(dir, ".env")
	writeFile(t, dotenv, "CONFLUENCE_FORMAT=table\n")
	if _, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: dotenv}); err != nil {
		t.Fatal(err)
	}
	if _, ok := os.LookupEnv("CONFLUENCE_FORMAT"); ok {
		t.Error(".env load must not set process environment variables")
	}
}

func TestWriteFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := File{
		CurrentContext: "prod",
		Contexts: []NamedContext{
			{Name: "default", BaseURL: "https://dc.example.com", Flavor: FlavorDataCenter,
				Auth: AuthConfig{Scheme: SchemePAT}},
			{Name: "prod", BaseURL: "https://cloud.example.com", Flavor: FlavorCloud,
				DetectedFlavor: FlavorCloud, Auth: AuthConfig{Scheme: SchemeBasic, Username: "alice"}},
		},
		Defaults: Defaults{Format: "table", PageSize: 50, Timeout: 15 * time.Second, MaxRetries: 5},
	}
	if err := WriteFile(dir, in); err != nil {
		t.Fatal(err)
	}
	got, exists, err := ReadFile(dir)
	if err != nil || !exists {
		t.Fatalf("ReadFile: exists=%v err=%v", exists, err)
	}
	if got.CurrentContext != "prod" || len(got.Contexts) != 2 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	prod, ok := got.Context("prod")
	if !ok || prod.BaseURL != "https://cloud.example.com" || prod.Auth.Username != "alice" {
		t.Errorf("prod context = %+v", prod)
	}
	if got.Defaults.Timeout != 15*time.Second || got.Defaults.PageSize != 50 {
		t.Errorf("defaults = %+v", got.Defaults)
	}
}

func TestWriteFilePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested")
	if err := WriteFile(dir, File{Contexts: []NamedContext{{Name: "default", BaseURL: "https://x"}}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(ConfigFilePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file perm = %o, want 600", perm)
	}
}

func TestReadFileLegacyFlatFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, ConfigFilePath(dir),
		"server: https://legacy.example.com\nflavor: datacenter\nauth:\n  scheme: pat\n")
	got, exists, err := ReadFile(dir)
	if err != nil || !exists {
		t.Fatalf("ReadFile: exists=%v err=%v", exists, err)
	}
	if len(got.Contexts) != 1 || got.Contexts[0].Name != DefaultContextName {
		t.Fatalf("legacy file should yield one default context: %+v", got.Contexts)
	}
	if got.CurrentContext != DefaultContextName {
		t.Errorf("CurrentContext = %q, want default", got.CurrentContext)
	}
	if got.Contexts[0].BaseURL != "https://legacy.example.com" {
		t.Errorf("BaseURL = %q", got.Contexts[0].BaseURL)
	}
}

func TestLoadLegacyFlatUnchanged(t *testing.T) {
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
	if got.ActiveContext != DefaultContextName {
		t.Errorf("ActiveContext = %q, want default", got.ActiveContext)
	}
}

func writeMultiContextFile(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, ConfigFilePath(dir), `current_context: alpha
contexts:
  - name: alpha
    server: https://alpha.example.com
    flavor: datacenter
    auth: {scheme: pat}
  - name: beta
    server: https://beta.example.com
    flavor: cloud
    auth: {scheme: basic, username: bob}
defaults:
  format: json
`)
}

func TestLoadMultiContextSelection(t *testing.T) {
	absent := func(dir string) string { return filepath.Join(dir, "absent.env") }

	t.Run("current_context", func(t *testing.T) {
		dir := t.TempDir()
		writeMultiContextFile(t, dir)
		got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: absent(dir)})
		if err != nil {
			t.Fatal(err)
		}
		if got.ActiveContext != "alpha" || got.Config.BaseURL != "https://alpha.example.com" {
			t.Errorf("active=%q url=%q", got.ActiveContext, got.Config.BaseURL)
		}
		if len(got.ContextNames) != 2 {
			t.Errorf("ContextNames = %v", got.ContextNames)
		}
	})

	t.Run("flag overrides current_context", func(t *testing.T) {
		dir := t.TempDir()
		writeMultiContextFile(t, dir)
		got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: absent(dir), Context: "beta"})
		if err != nil {
			t.Fatal(err)
		}
		if got.ActiveContext != "beta" || got.Config.BaseURL != "https://beta.example.com" {
			t.Errorf("active=%q url=%q", got.ActiveContext, got.Config.BaseURL)
		}
	})

	t.Run("env overrides current_context", func(t *testing.T) {
		dir := t.TempDir()
		writeMultiContextFile(t, dir)
		t.Setenv("CONFLUENCE_CONTEXT", "beta")
		got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: absent(dir)})
		if err != nil {
			t.Fatal(err)
		}
		if got.ActiveContext != "beta" {
			t.Errorf("active = %q, want beta", got.ActiveContext)
		}
	})

	t.Run("CONFLUENCE_SERVER still overrides selected context", func(t *testing.T) {
		dir := t.TempDir()
		writeMultiContextFile(t, dir)
		t.Setenv("CONFLUENCE_SERVER", "https://override.example.com")
		got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: absent(dir)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Config.BaseURL != "https://override.example.com" {
			t.Errorf("BaseURL = %q, want env override", got.Config.BaseURL)
		}
	})

	t.Run("unknown context errors", func(t *testing.T) {
		dir := t.TempDir()
		writeMultiContextFile(t, dir)
		if _, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: absent(dir), Context: "ghost"}); err == nil {
			t.Error("expected error for unknown context")
		}
	})
}

func TestSelectContext(t *testing.T) {
	f := File{
		CurrentContext: "alpha",
		Contexts:       []NamedContext{{Name: "alpha"}, {Name: "beta"}},
	}
	cases := []struct {
		name, flag, env, want, wantSource string
		wantErr                           bool
	}{
		{name: "flag wins", flag: "beta", want: "beta", wantSource: ContextSourceFlag},
		{name: "env over current", env: "beta", want: "beta", wantSource: ContextSourceEnv},
		{name: "current_context", want: "alpha", wantSource: ContextSourceCurrent},
		{name: "unknown flag errors", flag: "ghost", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, source, err := selectContext(f, tc.flag, tc.env)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if source != tc.wantSource {
				t.Errorf("source = %q, want %q", source, tc.wantSource)
			}
		})
	}

	t.Run("sole context", func(t *testing.T) {
		got, source, err := selectContext(File{Contexts: []NamedContext{{Name: "only"}}}, "", "")
		if err != nil || got != "only" || source != ContextSourceSingle {
			t.Errorf("got %q source %q err %v", got, source, err)
		}
	})
	t.Run("default-named context", func(t *testing.T) {
		got, source, err := selectContext(File{Contexts: []NamedContext{{Name: "a"}, {Name: "default"}}}, "", "")
		if err != nil || got != "default" || source != ContextSourceDefault {
			t.Errorf("got %q source %q err %v", got, source, err)
		}
	})
	t.Run("ambiguous yields empty no error", func(t *testing.T) {
		got, source, err := selectContext(File{Contexts: []NamedContext{{Name: "a"}, {Name: "b"}}}, "", "")
		if err != nil || got != "" || source != ContextSourceNone {
			t.Errorf("got %q source %q err %v, want empty/none/no-error", got, source, err)
		}
	})
	t.Run("no file yields empty", func(t *testing.T) {
		got, source, err := selectContext(File{}, "", "")
		if err != nil || got != "" || source != ContextSourceNone {
			t.Errorf("got %q source %q err %v", got, source, err)
		}
	})

	// UNKNOWN_CONTEXT must surface a structured CLIError (not a generic load
	// failure) — the surrounding app layer is supposed to pass it through
	// untouched so the caller sees the code/hint.
	t.Run("unknown returns UNKNOWN_CONTEXT cli error", func(t *testing.T) {
		_, _, err := selectContext(f, "ghost", "")
		ce, ok := err.(*cerrors.CLIError)
		if !ok || ce.Code != "UNKNOWN_CONTEXT" {
			t.Fatalf("got %T %v, want *CLIError UNKNOWN_CONTEXT", err, err)
		}
		if ce.Hint == "" {
			t.Error("hint should be set")
		}
	})

	// Case-insensitive lookup: `--use-context cloud` against a legacy
	// `Cloud` config must succeed and return the canonical name. The
	// `current_context` we persist must agree with what is in the contexts
	// list, otherwise CI lookup against the rewritten file would orphan it.
	t.Run("case-insensitive lookup returns canonical name", func(t *testing.T) {
		ff := File{Contexts: []NamedContext{{Name: "Cloud"}, {Name: "default"}}}
		got, _, err := selectContext(ff, "cloud", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "Cloud" {
			t.Errorf("got %q, want canonical %q", got, "Cloud")
		}
	})

	// Without a CI match, the hint should list available names so the user
	// does not have to run a second command to recover.
	t.Run("hint lists available contexts", func(t *testing.T) {
		ff := File{Contexts: []NamedContext{{Name: "alpha"}, {Name: "beta"}}}
		_, _, err := selectContext(ff, "gamma", "")
		ce := err.(*cerrors.CLIError)
		if !strings.Contains(ce.Hint, "alpha") || !strings.Contains(ce.Hint, "beta") {
			t.Errorf("hint = %q, want both context names listed", ce.Hint)
		}
	})
}

// TestResolveConfigDir covers the "prefer new, fall back to legacy" rule used
// when --config was not supplied. It overrides $HOME with a temp directory so
// the real user's config is never touched.
func TestResolveConfigDir(t *testing.T) {
	withFakeHome := func(t *testing.T) string {
		t.Helper()
		home := t.TempDir()
		t.Setenv("HOME", home)
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

	t.Run("legacy dir without config.yaml is ignored", func(t *testing.T) {
		home := withFakeHome(t)
		legacy := filepath.Join(home, ".confluence")
		if err := os.MkdirAll(legacy, 0o700); err != nil {
			t.Fatal(err)
		}
		// No config.yaml inside legacy. Expect resolver to return the new
		// path, not the empty legacy dir.
		dir, err := ResolveConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(home, ".angelmsger", "confluence")
		if dir != want {
			t.Errorf("dir = %q, want %q", dir, want)
		}
	})
}

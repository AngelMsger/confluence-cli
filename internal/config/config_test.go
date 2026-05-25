package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
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
		name, flag, env, want string
		wantErr               bool
	}{
		{name: "flag wins", flag: "beta", want: "beta"},
		{name: "env over current", env: "beta", want: "beta"},
		{name: "current_context", want: "alpha"},
		{name: "unknown flag errors", flag: "ghost", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectContext(f, tc.flag, tc.env)
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
		})
	}

	t.Run("sole context", func(t *testing.T) {
		got, err := selectContext(File{Contexts: []NamedContext{{Name: "only"}}}, "", "")
		if err != nil || got != "only" {
			t.Errorf("got %q err %v", got, err)
		}
	})
	t.Run("ambiguous yields empty no error", func(t *testing.T) {
		got, err := selectContext(File{Contexts: []NamedContext{{Name: "a"}, {Name: "b"}}}, "", "")
		if err != nil || got != "" {
			t.Errorf("got %q err %v, want empty/no-error", got, err)
		}
	})
	t.Run("no file yields empty", func(t *testing.T) {
		got, err := selectContext(File{}, "", "")
		if err != nil || got != "" {
			t.Errorf("got %q err %v", got, err)
		}
	})

	// UNKNOWN_CONTEXT must surface a structured CLIError (not a generic load
	// failure) — the surrounding app layer is supposed to pass it through
	// untouched so the caller sees the code/hint.
	t.Run("unknown returns UNKNOWN_CONTEXT cli error", func(t *testing.T) {
		_, err := selectContext(f, "ghost", "")
		ce, ok := err.(*cerrors.CLIError)
		if !ok || ce.Code != "UNKNOWN_CONTEXT" {
			t.Fatalf("got %T %v, want *CLIError UNKNOWN_CONTEXT", err, err)
		}
		if ce.Hint == "" {
			t.Error("hint should be set")
		}
	})

	// Case-insensitive matches should be called out — "Did you mean X?" is far
	// more actionable than a context list when the user just got the case wrong.
	t.Run("hint suggests case-different match", func(t *testing.T) {
		ff := File{Contexts: []NamedContext{{Name: "Cloud"}, {Name: "default"}}}
		_, err := selectContext(ff, "cloud", "")
		ce := err.(*cerrors.CLIError)
		if !strings.Contains(ce.Hint, `Did you mean "Cloud"`) {
			t.Errorf("hint = %q, want did-you-mean suggestion", ce.Hint)
		}
	})

	// Without a CI match, the hint should list available names so the user
	// does not have to run a second command to recover.
	t.Run("hint lists available contexts", func(t *testing.T) {
		ff := File{Contexts: []NamedContext{{Name: "alpha"}, {Name: "beta"}}}
		_, err := selectContext(ff, "gamma", "")
		ce := err.(*cerrors.CLIError)
		if !strings.Contains(ce.Hint, "alpha") || !strings.Contains(ce.Hint, "beta") {
			t.Errorf("hint = %q, want both context names listed", ce.Hint)
		}
	})
}

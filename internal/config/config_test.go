package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestWriteConfigFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		BaseURL: "https://rt.example.com",
		Flavor:  FlavorDataCenter,
		Auth:    AuthConfig{Scheme: SchemeBasic, Username: "alice"},
		Defaults: Defaults{
			Format: "table", PageSize: 50,
			Timeout: 15 * time.Second, MaxRetries: 5,
		},
	}
	if err := WriteConfigFile(dir, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(LoadOptions{ConfigDir: dir, DotenvPath: filepath.Join(dir, "absent.env")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.BaseURL != cfg.BaseURL || got.Config.Auth.Username != "alice" {
		t.Errorf("round trip mismatch: %+v", got.Config)
	}
	if got.Config.Defaults.Timeout != 15*time.Second {
		t.Errorf("Timeout = %v, want 15s", got.Config.Defaults.Timeout)
	}
	if got.Config.Defaults.PageSize != 50 {
		t.Errorf("PageSize = %d, want 50", got.Config.Defaults.PageSize)
	}
}

func TestWriteConfigFilePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested")
	if err := WriteConfigFile(dir, Config{BaseURL: "https://x"}); err != nil {
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

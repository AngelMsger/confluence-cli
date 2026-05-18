package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
)

// writeContextFile writes a two-context config file into dir.
func writeContextFile(t *testing.T, dir, alphaURL, betaURL string) {
	t.Helper()
	content := fmt.Sprintf(`current_context: alpha
contexts:
  - name: alpha
    server: %s
    flavor: datacenter
    auth: {scheme: pat}
  - name: beta
    server: %s
    flavor: datacenter
    auth: {scheme: pat}
defaults:
  format: json
`, alphaURL, betaURL)
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// runCLIFile runs the command tree against a config directory with no
// CONFLUENCE_* environment overrides, so the config file alone drives behaviour.
func runCLIFile(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	for _, k := range []string{"CONFLUENCE_SERVER", "CONFLUENCE_FLAVOR",
		"CONFLUENCE_PERSONAL_ACCESS_TOKEN", "CONFLUENCE_CONTEXT", "CONFLUENCE_FORMAT"} {
		t.Setenv(k, "")
	}
	full := append([]string{"--config", dir}, args...)
	root := newRootCmd()
	root.SetArgs(full)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	outCh := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		outCh <- string(b)
	}()
	err := root.Execute()
	w.Close()
	os.Stdout = old
	return <-outCh, err
}

func TestCmdGetContexts(t *testing.T) {
	dir := t.TempDir()
	writeContextFile(t, dir, "https://alpha.example.com", "https://beta.example.com")
	out, err := runCLIFile(t, dir, "config", "get-contexts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Errorf("get-contexts output missing a context: %s", out)
	}
	if !strings.Contains(out, "https://alpha.example.com") {
		t.Errorf("get-contexts output missing server: %s", out)
	}
}

func TestCmdUseContext(t *testing.T) {
	dir := t.TempDir()
	writeContextFile(t, dir, "https://alpha.example.com", "https://beta.example.com")
	if _, err := runCLIFile(t, dir, "config", "use-context", "beta"); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.ReadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if file.CurrentContext != "beta" {
		t.Errorf("CurrentContext = %q, want beta", file.CurrentContext)
	}
}

func TestCmdUseContextUnknown(t *testing.T) {
	dir := t.TempDir()
	writeContextFile(t, dir, "https://alpha.example.com", "https://beta.example.com")
	if _, err := runCLIFile(t, dir, "config", "use-context", "ghost"); err == nil {
		t.Error("expected error switching to an unknown context")
	}
}

func TestCmdUseContextOverrideFlag(t *testing.T) {
	dir := t.TempDir()
	writeContextFile(t, dir, "https://alpha.example.com", "https://beta.example.com")
	out, err := runCLIFile(t, dir, "--use-context", "beta", "config", "show")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "https://beta.example.com") {
		t.Errorf("--use-context beta did not select beta's server: %s", out)
	}
	if !strings.Contains(out, "\"context\"") {
		t.Errorf("multi-context `config show` should include a context field: %s", out)
	}
}

func TestCmdDeleteContext(t *testing.T) {
	dir := t.TempDir()
	writeContextFile(t, dir, "https://alpha.example.com", "https://beta.example.com")
	store := auth.NewStore(dir)
	if _, err := auth.Save("https://beta.example.com",
		auth.Credential{Scheme: auth.SchemePAT, Secret: "beta-secret"}, store); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLIFile(t, dir, "config", "delete-context", "beta"); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.ReadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := file.Context("beta"); ok {
		t.Error("beta context should be gone after delete-context")
	}
	if file.CurrentContext != "alpha" {
		t.Errorf("CurrentContext = %q, want alpha", file.CurrentContext)
	}
	// The deleted context's credential must be gone too.
	if _, err := store.Load(auth.AccountKey("https://beta.example.com", auth.SchemePAT)); err == nil {
		t.Error("beta credential should have been removed")
	}
}

func TestCmdDeleteLastContextRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("server: https://only.example.com\nflavor: datacenter\nauth: {scheme: pat}\n"),
		0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLIFile(t, dir, "config", "delete-context", "default"); err == nil {
		t.Error("deleting the only context should be refused")
	}
}

func TestCmdConfigShowSingleContextHasNoContextField(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("server: https://only.example.com\nflavor: datacenter\nauth: {scheme: pat}\n"),
		0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runCLIFile(t, dir, "config", "show")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\"context\"") {
		t.Errorf("single-context `config show` must not expose a context field: %s", out)
	}
}

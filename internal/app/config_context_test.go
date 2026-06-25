package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
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

// --use-context with an unknown name must surface the underlying
// UNKNOWN_CONTEXT error (with its available-contexts hint) instead of being
// swallowed by the appState.load() wrapper that turned every error into a
// generic CONFIG_LOAD "failed to load configuration".
func TestCmdUseContextUnknownPreservesStructuredError(t *testing.T) {
	dir := t.TempDir()
	writeContextFile(t, dir, "https://alpha.example.com", "https://beta.example.com")
	_, err := runCLIFile(t, dir, "--use-context", "ghost", "config", "show")
	if err == nil {
		t.Fatal("expected error")
	}
	var ce *cerrors.CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %T %v, want *CLIError", err, err)
	}
	if ce.Code != "UNKNOWN_CONTEXT" {
		t.Errorf("code = %q, want UNKNOWN_CONTEXT (raw err: %v)", ce.Code, err)
	}
	if !strings.Contains(ce.Hint, "alpha") || !strings.Contains(ce.Hint, "beta") {
		t.Errorf("hint should list available contexts, got %q", ce.Hint)
	}
}

// Case-only mismatch should now succeed via case-insensitive lookup — the
// previous "Did you mean X?" UX is unnecessary when the lookup just works.
// The output must show the *canonical* server, not whatever URL might have
// been associated with a hypothetical lower-cased duplicate.
func TestCmdUseContextLowercaseMatchesMixedCaseLegacyContext(t *testing.T) {
	dir := t.TempDir()
	content := `current_context: alpha
contexts:
  - name: Cloud
    server: https://acme.atlassian.net/wiki
    flavor: cloud
    auth: {scheme: basic, username: u@acme.com}
  - name: alpha
    server: https://alpha.example.com
    flavor: datacenter
    auth: {scheme: pat}
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runCLIFile(t, dir, "--use-context", "cloud", "config", "show")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "https://acme.atlassian.net/wiki") {
		t.Errorf("--use-context cloud should resolve to the Cloud context's server: %s", out)
	}
}

// `config use-context` persists the *canonical* name, not whatever casing
// the user typed — otherwise a CI lookup against the rewritten file would
// leave current_context pointing at a spelling that is not in the
// contexts list.
func TestCmdUseContextPersistsCanonicalName(t *testing.T) {
	dir := t.TempDir()
	content := `current_context: alpha
contexts:
  - name: Cloud
    server: https://acme.atlassian.net/wiki
    flavor: cloud
    auth: {scheme: basic, username: u@acme.com}
  - name: alpha
    server: https://alpha.example.com
    flavor: datacenter
    auth: {scheme: pat}
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLIFile(t, dir, "config", "use-context", "cloud"); err != nil {
		t.Fatalf("use-context cloud: %v", err)
	}
	file, _, err := config.ReadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if file.CurrentContext != "Cloud" {
		t.Errorf("CurrentContext = %q, want canonical %q", file.CurrentContext, "Cloud")
	}
}

// `config show --explain` must annotate auth.scheme and auth.user with their
// source — without that, env-var inference (e.g. CONFLUENCE_PERSONAL_ACCESS_TOKEN
// silently forcing scheme=pat over a Cloud context's basic) is invisible to
// the user trying to diagnose why their credentials are wrong.
func TestCmdConfigShowExplainAnnotatesAuthFields(t *testing.T) {
	dir := t.TempDir()
	content := `current_context: only
contexts:
  - name: only
    server: https://acme.atlassian.net/wiki
    flavor: cloud
    auth: {scheme: basic, username: u@acme.com}
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runCLIFile(t, dir, "config", "show", "--explain")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"auth.scheme": "basic (from `) {
		t.Errorf("explain should annotate auth.scheme with its source: %s", out)
	}
	if !strings.Contains(out, `"auth.user": "u@acme.com (from `) {
		t.Errorf("explain should annotate auth.user with its source: %s", out)
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

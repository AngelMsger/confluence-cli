package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

func cleanSetupEnv(t *testing.T) {
	t.Helper()
	for key := range envBindings {
		t.Setenv(key, "")
	}
	t.Setenv("CONFLUENCE_CONTEXT", "")
}
func setupFixture() NamedContext {
	return NamedContext{Name: "team", BaseURL: "https://service.example.test/deploy", Auth: AuthConfig{Scheme: "pat", Username: "PersonalUser", CredentialURL: "https://help.example.test/tokens"}, Flavor: "datacenter"}
}

func TestSetupTargetsNamedContextAndExcludesPersonalEnvironment(t *testing.T) {
	cleanSetupEnv(t)
	dir := t.TempDir()
	target := setupFixture()
	file := File{CurrentContext: "other", Contexts: []NamedContext{{Name: "other", BaseURL: "https://other.example.test", Auth: AuthConfig{Scheme: "pat"}}, target}}
	if err := WriteFile(dir, file); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFLUENCE_CONTEXT", "not-defined")
	t.Setenv("CONFLUENCE_FORMAT", "ndjson")
	t.Setenv("CONFLUENCE_USERNAME", "InjectedUser")
	t.Setenv("CONFLUENCE_API_TOKEN", "must-not-be-copied")
	dotenv := filepath.Join(dir, "presets.env")
	if err := os.WriteFile(dotenv, []byte("CONFLUENCE_SERVER=https://dotenv.example.test/deploy\nCONFLUENCE_CREDENTIAL_URL=https://help.example.test/new\nCONFLUENCE_API_TOKEN=dotenv-secret\nCONFLUENCE_USERNAME=dotenv-user\n"), 0600); err != nil {
		t.Fatal(err)
	}
	opt := LoadOptions{ConfigDir: dir, DotenvPath: dotenv, Context: "TEAM", Setup: true}
	got, err := Load(opt)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.Defaults.Format != "ndjson" {
		t.Fatal("setup ignored the output format environment variable")
	}
	if got.ActiveContext != "team" || got.Config.BaseURL != "https://dotenv.example.test/deploy" || got.Config.Auth.Username != "PersonalUser" || got.Config.Auth.Scheme != "pat" {
		t.Fatalf("wrong target or personal override: %+v", got)
	}
	if !reflect.DeepEqual(got.Secrets, Secrets{}) {
		t.Fatalf("setup retained secrets: %+v", got.Secrets)
	}
	if got.Sources[fieldServer] != "dotenv" || got.Sources[fieldCredentialURL] != "dotenv" {
		t.Fatal(got.Sources)
	}
	t.Setenv("CONFLUENCE_SERVER", "https://env.example.test")
	t.Setenv("CONFLUENCE_AUTH_SCHEME", "basic")
	got, err = Load(opt)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.BaseURL != "https://env.example.test" || got.Config.Auth.Scheme != "basic" || got.Sources[fieldServer] != "env" {
		t.Fatal(got)
	}
	opt.Flags = FlagValues{BaseURL: "https://flag.example.test", CredentialURL: "https://help.example.test/flag", AuthScheme: "pat"}
	got, err = Load(opt)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.BaseURL != "https://flag.example.test" || got.Sources[fieldServer] != "flag" || got.Sources[fieldCredentialURL] != "flag" {
		t.Fatal(got)
	}
	opt.Context = "new"
	opt.Flags = FlagValues{}
	t.Setenv("CONFLUENCE_SERVER", "")
	opt.DotenvPath = filepath.Join(dir, "absent")
	got, err = Load(opt)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config.BaseURL != "" || got.Config.Auth.Username != "" {
		t.Fatalf("new target inherited active context: %+v", got)
	}
}

func TestPlanServiceContextConflictIdempotencyAndPreservation(t *testing.T) {
	cleanSetupEnv(t)
	dir := t.TempDir()
	target := setupFixture()
	file := File{CurrentContext: "other", Contexts: []NamedContext{{Name: "other", BaseURL: "https://other.example.test"}, target}, Defaults: Defaults{ReadOnly: true}}
	if err := WriteFile(dir, file); err != nil {
		t.Fatal(err)
	}
	opt := LoadOptions{ConfigDir: dir, Context: "TEAM", Setup: true, DotenvPath: filepath.Join(dir, "absent")}
	resolved, err := Load(opt)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanServiceContext(file, "TEAM", resolved, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Changed {
		t.Fatalf("repeat is not idempotent: %+v", plan.Changes)
	}
	opt.Flags.BaseURL = "https://new.example.test/deploy"
	resolved, err = Load(opt)
	if err != nil {
		t.Fatal(err)
	}
	_, err = PlanServiceContext(file, "team", resolved, false, false)
	ce := cerrors.AsCLIError(err)
	if ce == nil || ce.Code != "CONFIG_CONTEXT_CONFLICT" || ce.Details == nil {
		t.Fatalf("missing structured differences: %v", err)
	}
	plan, err = PlanServiceContext(file, "team", resolved, true, false)
	if err != nil {
		t.Fatal(err)
	}
	nc, _ := plan.File.Context("team")
	if nc.Auth != target.Auth || plan.File.CurrentContext != "other" || !reflect.DeepEqual(plan.File.Defaults, file.Defaults) || plan.File.Contexts[0] != file.Contexts[0] {
		t.Fatalf("clobbered unrelated values: %+v", plan.File)
	}
	if err := WriteFile(dir, plan.File); err != nil {
		t.Fatal(err)
	}
	roundtrip, _, err := ReadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundtrip, plan.File) {
		t.Fatalf("roundtrip lost fields: got %+v want %+v", roundtrip, plan.File)
	}
	plan, err = PlanServiceContext(file, "team", resolved, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentContext != "team" {
		t.Fatal(plan)
	}
	initial := &Resolved{Config: resolved.Config, Sources: resolved.Sources}
	plan, err = PlanServiceContext(File{}, "team", initial, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentContext != "team" {
		t.Fatal("first context must activate")
	}
}

func TestCredentialGuideIsDisplayOnlyAndKeepsDeploymentPath(t *testing.T) {
	nc := setupFixture()
	cfg := Config{BaseURL: nc.BaseURL, Auth: nc.Auth, Flavor: "datacenter"}
	cfg.Auth.CredentialURL = ""
	g, err := Guide(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if g.CredentialURL != nc.BaseURL {
		t.Fatalf("wrong default URL: %+v", g)
	}
	if len(g.Instructions) == 0 || g.DocumentationURL == "" || len(g.NextSteps) == 0 {
		t.Fatal(g)
	}
	cfg.Auth.CredentialURL = "https://login.example.test/custom?view=credentials"
	g, err = Guide(cfg, map[string]string{fieldCredentialURL: "env"})
	if err != nil {
		t.Fatal(err)
	}
	if g.CredentialURL != cfg.Auth.CredentialURL || g.Server != nc.BaseURL || g.Source != "env" {
		t.Fatal(g)
	}
	raw, _ := json.Marshal(g)
	if strings.Contains(string(raw), "PersonalUser") {
		t.Fatal("guide should not expose personal identity")
	}
	for _, bad := range []string{"javascript:alert(1)", "https://user:secret@example.test", "/relative", "https://example.test/\npath"} {
		cfg.Auth.CredentialURL = bad
		if _, err := Guide(cfg, nil); err == nil {
			t.Fatalf("accepted unsafe link %q", bad)
		}
	}
}

func TestNormalizeServiceIdentityAndRejectInvalidInput(t *testing.T) {
	normal, err := NormalizeServiceURL("https://SERVICE.example.test:443/deploy/")
	if err != nil || normal != "https://service.example.test/deploy" {
		t.Fatalf("%q %v", normal, err)
	}
	for _, bad := range []string{"", "file:///tmp/token", "https://user:secret@example.test", "https://example.test/?token=secret", "https://example.test/#fragment"} {
		if _, err := NormalizeServiceURL(bad); err == nil {
			t.Fatalf("accepted invalid service URL %q", bad)
		}
	}
}

func TestSetupRecoveryQuotesContextNames(t *testing.T) {
	nc := setupFixture()
	resolved := &Resolved{Config: Config{BaseURL: nc.BaseURL, Auth: nc.Auth, Flavor: "datacenter"}}
	plan, err := PlanServiceContext(File{}, "ops team's", resolved, false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range plan.NextSteps {
		if !strings.Contains(step, `--use-context 'ops team'"'"'s'`) {
			t.Fatalf("context was not safely quoted: %s", step)
		}
	}
}

package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/angelmsger/confluence-cli/internal/auth" // registers the auth schemes
	"github.com/angelmsger/confluence-cli/internal/config"
)

// An external test package: internal/auth, which registers the schemes reuse
// needs, imports this package.

func cleanReuseEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"CONFLUENCE_SERVER", "CONFLUENCE_FLAVOR", "CONFLUENCE_CREDENTIAL_URL", "CONFLUENCE_AUTH_SCHEME",
		"CONFLUENCE_USERNAME", "CONFLUENCE_FORMAT", "CONFLUENCE_CLI_READ_ONLY", "CONFLUENCE_PERSONAL_ACCESS_TOKEN",
		"CONFLUENCE_PASSWORD", "CONFLUENCE_API_TOKEN", "CONFLUENCE_CONTEXT"} {
		t.Setenv(key, "")
	}
}

func TestLoadOffersReuseBeforeSignIn(t *testing.T) {
	cleanReuseEnv(t)
	dir := t.TempDir()
	file := config.File{CurrentContext: "team", Contexts: []config.NamedContext{
		{Name: "personal", BaseURL: "https://wiki.example.test", Flavor: config.FlavorDataCenter, Auth: config.AuthConfig{Scheme: config.SchemeBasic, Username: "me"}},
		{Name: "team", BaseURL: "https://wiki.example.test", Flavor: config.FlavorDataCenter, Auth: config.AuthConfig{Scheme: config.SchemePAT}},
		{Name: "cloud", BaseURL: "https://example.atlassian.net/wiki", Auth: config.AuthConfig{Scheme: config.SchemeBasic}},
	}}
	if err := config.WriteFile(dir, file); err != nil {
		t.Fatal(err)
	}
	load := func(opt config.LoadOptions) *config.Resolved {
		t.Helper()
		opt.ConfigDir, opt.DotenvPath = dir, filepath.Join(t.TempDir(), "absent.env")
		r, err := config.Load(opt)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	r := load(config.LoadOptions{})
	if r.Config.ReuseStep != "confluence-cli --use-context 'team' auth reuse" {
		t.Fatalf("reuse step %q", r.Config.ReuseStep)
	}
	g, err := config.Guide(r.Config, r.Sources)
	if err != nil || g.NextSteps[0] != r.Config.ReuseStep || !strings.Contains(g.Instructions[0], "auth reuse") {
		t.Fatalf("guide %+v %v", g, err)
	}
	if r = load(config.LoadOptions{Context: "personal"}); r.Config.ReuseStep != "" {
		t.Fatalf("a context with an identity was offered reuse: %q", r.Config.ReuseStep)
	}
	if r = load(config.LoadOptions{Flags: config.FlagValues{AuthScheme: config.SchemePAT}}); r.Config.ReuseStep != "" {
		t.Fatalf("a supplied scheme without same-scheme sources was offered reuse: %q", r.Config.ReuseStep)
	}
	if r = load(config.LoadOptions{Flags: config.FlagValues{BaseURL: "https://other.example.test"}}); r.Config.ReuseStep != "" {
		t.Fatalf("an override was offered reuse: %q", r.Config.ReuseStep)
	}
	// Cloud accepts only basic, and no other Cloud context holds a login.
	if r = load(config.LoadOptions{Context: "cloud"}); r.Config.ReuseStep != "" {
		t.Fatalf("a Cloud context without a basic source was offered reuse: %q", r.Config.ReuseStep)
	}
}

func TestSetupPlanLeadsWithReuse(t *testing.T) {
	cleanReuseEnv(t)
	file := config.File{Contexts: []config.NamedContext{
		{Name: "personal", BaseURL: "https://wiki.example.test", Flavor: config.FlavorDataCenter, Auth: config.AuthConfig{Scheme: config.SchemeBasic, Username: "me"}},
	}}
	// A preset's scheme is the context's own, not a limit on what reuse adopts.
	resolved := &config.Resolved{Config: config.Config{BaseURL: "https://wiki.example.test", Flavor: config.FlavorDataCenter, Auth: config.AuthConfig{Scheme: config.SchemePAT}},
		Sources: map[string]string{config.FieldServer: "flag", config.FieldFlavor: "flag", config.FieldAuthScheme: "flag"}}
	plan, err := config.PlanServiceContext(file, "team", resolved, false, false)
	if err != nil || len(plan.NextSteps) == 0 || plan.NextSteps[0] != "confluence-cli --use-context 'team' auth reuse" {
		t.Fatalf("plan %+v %v", plan.NextSteps, err)
	}
}

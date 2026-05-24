package config

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// newPlainTest builds a PlainDriver wired to scripted stdin and a captured
// output buffer.
func newPlainTest(script string) (*PlainDriver, *bytes.Buffer) {
	out := &bytes.Buffer{}
	in := io.NopCloser(strings.NewReader(script))
	return NewPlainDriver(in, out), out
}

func TestAssembleContextResult(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		picks       contextPicks
		kept        Secrets
		wantSecrets Secrets
		wantUser    string
	}{
		{
			name: "cloud_basic_new",
			picks: contextPicks{
				Name: "default", BaseURL: "https://x.atlassian.net/wiki",
				Flavor: FlavorCloud, Scheme: SchemeBasic,
				Username: "u@x.com", Secret: "tok",
			},
			wantSecrets: Secrets{APIToken: "tok"},
			wantUser:    "u@x.com",
		},
		{
			name: "cloud_basic_keep",
			picks: contextPicks{
				Name: "default", Flavor: FlavorCloud, Scheme: SchemeBasic,
				Username: "u@x.com", KeepSecret: true,
			},
			kept:        Secrets{APIToken: "stored"},
			wantSecrets: Secrets{APIToken: "stored"},
			wantUser:    "u@x.com",
		},
		{
			name: "dc_basic_new",
			picks: contextPicks{
				Name: "default", Flavor: FlavorDataCenter, Scheme: SchemeBasic,
				Username: "ops", Secret: "pw",
			},
			wantSecrets: Secrets{Password: "pw"},
			wantUser:    "ops",
		},
		{
			name: "dc_basic_keep_falls_back_when_token_empty",
			picks: contextPicks{
				Name: "default", Flavor: FlavorDataCenter, Scheme: SchemeBasic,
				Username: "ops", KeepSecret: true,
			},
			kept:        Secrets{Password: "stored-pw"},
			wantSecrets: Secrets{Password: "stored-pw"},
			wantUser:    "ops",
		},
		{
			name: "pat_new",
			picks: contextPicks{
				Name: "default", Flavor: FlavorDataCenter,
				Scheme: SchemePAT, Secret: "pat-tok",
			},
			wantSecrets: Secrets{PAT: "pat-tok"},
		},
		{
			name: "pat_keep",
			picks: contextPicks{
				Name: "default", Flavor: FlavorDataCenter,
				Scheme: SchemePAT, KeepSecret: true,
			},
			kept:        Secrets{PAT: "stored-pat"},
			wantSecrets: Secrets{PAT: "stored-pat"},
		},
		{
			name: "detected_cloud_routes_to_api_token",
			picks: contextPicks{
				Name: "default", Flavor: FlavorAuto, DetectedFlavor: FlavorCloud,
				Scheme: SchemeBasic, Username: "auto", Secret: "tok",
			},
			wantSecrets: Secrets{APIToken: "tok"},
			wantUser:    "auto",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := assembleContextResult(tc.picks, tc.kept)
			if got.Secrets != tc.wantSecrets {
				t.Errorf("secrets = %+v, want %+v", got.Secrets, tc.wantSecrets)
			}
			if got.Context.Auth.Username != tc.wantUser {
				t.Errorf("username = %q, want %q", got.Context.Auth.Username, tc.wantUser)
			}
			if got.Context.Name != tc.picks.Name {
				t.Errorf("name lost: %q", got.Context.Name)
			}
		})
	}
}

func TestRunWizardFresh(t *testing.T) {
	t.Parallel()
	d, out := newPlainTest(strings.Join([]string{
		"https://acme.atlassian.net/wiki",
		"cloud",
		"basic",
		"alice@acme.com",
		"sekret",
		"n",
	}, "\n") + "\n")

	result, err := RunWizard(d, WizardHooks{}, WizardInputs{})
	if err != nil {
		t.Fatalf("RunWizard: %v", err)
	}
	if len(result.Creds) != 1 {
		t.Fatalf("want 1 creds entry, got %d", len(result.Creds))
	}
	got := result.Creds[0]
	if got.Context.Name != DefaultContextName {
		t.Errorf("context name = %q", got.Context.Name)
	}
	if got.Context.BaseURL != "https://acme.atlassian.net/wiki" {
		t.Errorf("base URL = %q", got.Context.BaseURL)
	}
	if got.Context.Flavor != FlavorCloud {
		t.Errorf("flavor = %q", got.Context.Flavor)
	}
	if got.Context.Auth.Scheme != SchemeBasic || got.Context.Auth.Username != "alice@acme.com" {
		t.Errorf("auth = %+v", got.Context.Auth)
	}
	// Cloud + basic → secret must land in APIToken, not Password.
	if got.Secrets.APIToken != "sekret" || got.Secrets.Password != "" {
		t.Errorf("secrets = %+v", got.Secrets)
	}

	text := out.String()
	// Fresh setup must surface the example placeholders so the user knows the
	// expected shape; no "Existing configuration" preamble.
	if !strings.Contains(text, "https://your-site.atlassian.net/wiki") {
		t.Errorf("missing base URL example in prompts:\n%s", text)
	}
	if !strings.Contains(text, "you@example.com") {
		t.Errorf("missing username example in prompts:\n%s", text)
	}
	if strings.Contains(text, "Existing configuration") {
		t.Errorf("fresh wizard should not show existing-config preamble:\n%s", text)
	}
}

func TestRunWizardEditKeepsSecret(t *testing.T) {
	t.Parallel()
	existing := &File{
		CurrentContext: "default",
		Contexts: []NamedContext{{
			Name:    "default",
			BaseURL: "https://acme.atlassian.net/wiki",
			Flavor:  FlavorCloud,
			Auth:    AuthConfig{Scheme: SchemeBasic, Username: "alice@acme.com"},
		}},
	}
	inputs := WizardInputs{
		Existing: existing,
		LoadSecret: func(nc NamedContext) (Secrets, bool) {
			return Secrets{APIToken: "stored-token"}, true
		},
	}
	// Action: edit; press Enter on every field to accept the stored value;
	// final "n" to skip adding more.
	d, out := newPlainTest(strings.Join([]string{
		"edit",
		"",
		"",
		"",
		"",
		"",
		"n",
	}, "\n") + "\n")

	result, err := RunWizard(d, WizardHooks{}, inputs)
	if err != nil {
		t.Fatalf("RunWizard: %v", err)
	}
	if len(result.Creds) != 1 {
		t.Fatalf("want 1 creds entry, got %d", len(result.Creds))
	}
	got := result.Creds[0]
	if got.Context.BaseURL != "https://acme.atlassian.net/wiki" {
		t.Errorf("kept URL lost: %q", got.Context.BaseURL)
	}
	if got.Secrets.APIToken != "stored-token" {
		t.Errorf("stored token not preserved: %+v", got.Secrets)
	}
	text := out.String()
	if !strings.Contains(text, "Existing configuration") {
		t.Errorf("edit flow should list existing contexts:\n%s", text)
	}
	if !strings.Contains(text, "[press Enter to keep current]") {
		t.Errorf("secret prompt should offer keep-current:\n%s", text)
	}
}

func TestRunWizardAddPreservesOthers(t *testing.T) {
	t.Parallel()
	existing := &File{
		CurrentContext: "prod",
		Contexts: []NamedContext{{
			Name:    "prod",
			BaseURL: "https://prod.atlassian.net/wiki",
			Flavor:  FlavorCloud,
			Auth:    AuthConfig{Scheme: SchemeBasic, Username: "ops@acme.com"},
		}},
	}
	d, _ := newPlainTest(strings.Join([]string{
		"add",                                // action
		"staging",                            // new context name
		"https://staging.atlassian.net/wiki", // base URL
		"cloud",                              // flavor
		"basic",                              // scheme
		"qa@acme.com",                        // username
		"staging-token",                      // token
		"n",                                  // add another
	}, "\n") + "\n")

	result, err := RunWizard(d, WizardHooks{}, WizardInputs{Existing: existing})
	if err != nil {
		t.Fatalf("RunWizard: %v", err)
	}
	if len(result.File.Contexts) != 2 {
		t.Fatalf("want 2 contexts in file, got %d", len(result.File.Contexts))
	}
	// Only the new context should appear in Creds — the existing one carries
	// forward untouched and the caller must not re-save its credential.
	if len(result.Creds) != 1 || result.Creds[0].Context.Name != "staging" {
		t.Errorf("creds = %+v", result.Creds)
	}
	if result.File.CurrentContext != "prod" {
		t.Errorf("current_context should stay 'prod', got %q", result.File.CurrentContext)
	}
}

func TestDefaultSchemeForFlavor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		flavor, detected, want string
	}{
		{FlavorCloud, "", SchemeBasic},
		{FlavorAuto, FlavorCloud, SchemeBasic},
		{FlavorDataCenter, "", SchemePAT},
		{FlavorAuto, FlavorDataCenter, SchemePAT},
		{FlavorAuto, "", SchemePAT},
		{"", "", SchemePAT},
	}
	for _, tc := range cases {
		if got := defaultSchemeForFlavor(tc.flavor, tc.detected); got != tc.want {
			t.Errorf("defaultSchemeForFlavor(%q, %q) = %q, want %q",
				tc.flavor, tc.detected, got, tc.want)
		}
	}
}

// When flavor detection resolves to Cloud, pressing Enter on the auth scheme
// prompt must default to basic — Cloud's id.atlassian.com API tokens 403 with
// Bearer/PAT, so the historical SchemePAT default trapped users (see the bug
// where a valid token kept "Validating credentials… 403 FORBIDDEN").
func TestRunWizardAutoDetectedCloudDefaultsToBasic(t *testing.T) {
	t.Parallel()
	d, out := newPlainTest(strings.Join([]string{
		"https://acme.atlassian.net/wiki", // base URL
		"",                                // flavor — accept default (auto)
		"",                                // scheme — accept default (should be basic post-detection)
		"alice@acme.com",                  // username (only asked when basic)
		"sekret",                          // API token
		"n",                               // add another?
	}, "\n") + "\n")

	hooks := WizardHooks{
		DetectFlavor: func(string) (string, error) { return FlavorCloud, nil },
	}
	result, err := RunWizard(d, hooks, WizardInputs{})
	if err != nil {
		t.Fatalf("RunWizard: %v", err)
	}
	got := result.Creds[0]
	if got.Context.Auth.Scheme != SchemeBasic {
		t.Errorf("auth.scheme = %q, want basic\n%s", got.Context.Auth.Scheme, out.String())
	}
	if got.Secrets.APIToken != "sekret" {
		t.Errorf("secret = %+v", got.Secrets)
	}
}

// Explicit Cloud flavor (no detection hook) should also default scheme to
// basic on a fresh wizard run.
func TestRunWizardExplicitCloudDefaultsToBasic(t *testing.T) {
	t.Parallel()
	d, out := newPlainTest(strings.Join([]string{
		"https://acme.atlassian.net/wiki", // base URL
		"cloud",                           // flavor
		"",                                // scheme — accept default
		"alice@acme.com",                  // username
		"sekret",                          // token
		"n",
	}, "\n") + "\n")

	result, err := RunWizard(d, WizardHooks{}, WizardInputs{})
	if err != nil {
		t.Fatalf("RunWizard: %v", err)
	}
	if got := result.Creds[0].Context.Auth.Scheme; got != SchemeBasic {
		t.Errorf("auth.scheme = %q, want basic\n%s", got, out.String())
	}
}

// Data Center flavor keeps the historical PAT default — most DC users come
// from 7.9+ environments and expect Bearer auth.
func TestRunWizardDataCenterDefaultsToPAT(t *testing.T) {
	t.Parallel()
	d, out := newPlainTest(strings.Join([]string{
		"https://wiki.acme.corp", // base URL
		"datacenter",             // flavor
		"",                       // scheme — accept default (PAT)
		"pat-tok",                // PAT
		"n",
	}, "\n") + "\n")

	result, err := RunWizard(d, WizardHooks{}, WizardInputs{})
	if err != nil {
		t.Fatalf("RunWizard: %v", err)
	}
	got := result.Creds[0]
	if got.Context.Auth.Scheme != SchemePAT {
		t.Errorf("auth.scheme = %q, want pat\n%s", got.Context.Auth.Scheme, out.String())
	}
	if got.Secrets.PAT != "pat-tok" {
		t.Errorf("secret = %+v", got.Secrets)
	}
}

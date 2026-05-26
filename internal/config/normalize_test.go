package config

import "testing"

func TestNormalizeContextName(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Cloud":       "cloud",
		"  Cloud  ":   "cloud",
		"PROD":        "prod",
		"already-low": "already-low",
		"":            "",
		"\t\n":        "",
		"MixedCase-1": "mixedcase-1",
		" two words ": "two words",
	}
	for in, want := range cases {
		if got := NormalizeContextName(in); got != want {
			t.Errorf("NormalizeContextName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeUsername(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		// Email-shaped inputs are lowercased — Atlassian Cloud (and every
		// real-world mail provider) treats them case-insensitively, and
		// keeping mixed case strands users when an SSO email differs from
		// what they typed.
		{"Alice@Acme.COM", "alice@acme.com"},
		{"  bob@Example.com\n", "bob@example.com"},
		{"alice@acme.com", "alice@acme.com"},
		// Non-email usernames — could be LDAP / AD identifiers that the
		// DC server treats case-sensitively. Preserve casing; only trim.
		{"AliceJ", "AliceJ"},
		{"  CaseSensitive  ", "CaseSensitive"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeUsername(tc.in); got != tc.want {
			t.Errorf("NormalizeUsername(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// File.Context must do case-insensitive lookup and return the canonical
// (as-stored) name, so callers persisting a reference to it don't write
// back a spelling that isn't in the contexts list.
func TestFileContextCaseInsensitive(t *testing.T) {
	t.Parallel()
	f := File{Contexts: []NamedContext{
		{Name: "Cloud", BaseURL: "https://acme.atlassian.net/wiki"},
		{Name: "default", BaseURL: "https://kms.example.com"},
	}}
	for _, lookup := range []string{"cloud", "Cloud", "CLOUD", "cLoUd"} {
		got, ok := f.Context(lookup)
		if !ok {
			t.Errorf("Context(%q) not found", lookup)
			continue
		}
		if got.Name != "Cloud" {
			t.Errorf("Context(%q).Name = %q, want canonical %q", lookup, got.Name, "Cloud")
		}
	}
	if _, ok := f.Context("ghost"); ok {
		t.Error("Context(ghost) should not match")
	}
}

// assembleContextResult is the single seam where every wizard path (plain,
// huh, hand-built results in tests) lands. Normalization there means we do
// not have to chase every entry point individually.
func TestAssembleContextResultNormalizesNameAndEmailUsername(t *testing.T) {
	t.Parallel()
	got := assembleContextResult(contextPicks{
		Name:     "  Cloud  ",
		BaseURL:  "https://acme.atlassian.net/wiki",
		Flavor:   FlavorCloud,
		Scheme:   SchemeBasic,
		Username: "Alice@Acme.COM",
		Secret:   "tok",
	}, Secrets{})

	if got.Context.Name != "cloud" {
		t.Errorf("Name = %q, want %q (trimmed + lowercased)", got.Context.Name, "cloud")
	}
	if got.Context.Auth.Username != "alice@acme.com" {
		t.Errorf("Username = %q, want %q (lowercased)", got.Context.Auth.Username, "alice@acme.com")
	}
}

// DC-style non-email usernames must be left alone — they may map to an LDAP
// / AD identifier that the DC server treats case-sensitively.
func TestAssembleContextResultPreservesNonEmailUsernameCasing(t *testing.T) {
	t.Parallel()
	got := assembleContextResult(contextPicks{
		Name:     "DC",
		BaseURL:  "https://wiki.acme.corp",
		Flavor:   FlavorDataCenter,
		Scheme:   SchemeBasic,
		Username: "AliceJ",
		Secret:   "pw",
	}, Secrets{})

	if got.Context.Name != "dc" {
		t.Errorf("Name = %q, want %q", got.Context.Name, "dc")
	}
	if got.Context.Auth.Username != "AliceJ" {
		t.Errorf("Username = %q, want %q (preserved)", got.Context.Auth.Username, "AliceJ")
	}
}

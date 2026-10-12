package config

import "testing"

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

package config

import (
	"maps"
	"net/url"
	"strings"

	kit "github.com/angelmsger/agent-cli-kit/config"
	"github.com/angelmsger/confluence-cli/pkg/constants"
)

// NormalizeServiceURL retains the deployment path and rejects credential-bearing URLs.
// It is used at setup and persistence boundaries, never to derive an API route.
func NormalizeServiceURL(raw string) (string, error) { return kit.NormalizeServiceURL(raw) }

// ValidateService checks only public setup fields and never probes a server.
func ValidateService(cfg Config) error { return kit.ValidateService(kit.Encode(cfg)) }

// FieldChange describes a public configuration change; credentials are never included.
type FieldChange = kit.FieldChange

// ContextPlan is shared by preview and execution. File is the exact proposed file.
type ContextPlan = kit.ContextPlan[NamedContext, Defaults]

// PlanServiceContext builds a non-secret, target-specific merge without side
// effects: the service URL, flavor, auth scheme and credential page from
// resolved. A changed server or flavor clears the detected flavor. When reuse
// could adopt an existing login, the next steps lead with it.
func PlanServiceContext(file File, name string, resolved *Resolved, overwrite, activate bool) (ContextPlan, error) {
	plan, err := kit.PlanServiceContext(file, name, kit.Encode(resolved.Config), resolved.Sources, overwrite, activate)
	if err != nil {
		return ContextPlan{}, err
	}
	// The scheme a preset supplies is the context's own from now on, not a
	// limit on what reuse may adopt.
	sources := maps.Clone(resolved.Sources)
	delete(sources, fieldAuthScheme)
	planned, _ := plan.File.Context(plan.Context)
	if step := reuseStep(StoredContext(planned, plan.File.Defaults), sources, plan.File, plan.Context); step != "" {
		plan.NextSteps = append([]string{step}, plan.NextSteps...)
	}
	return plan, nil
}

// AuthGuide is an offline acquisition guide, not a capability or authentication probe.
type AuthGuide = kit.AuthGuide

// Guide derives display-only links. No request is ever sent to CredentialURL.
func Guide(cfg Config, sources map[string]string) (AuthGuide, error) {
	if err := ValidateService(cfg); err != nil {
		return AuthGuide{}, err
	}
	base, _ := NormalizeServiceURL(cfg.BaseURL)
	g := AuthGuide{Server: base, Scheme: cfg.Auth.Scheme, CredentialURL: base, Source: "fallback",
		NextSteps: []string{constants.AppName + " auth login"}}
	g.Flavor = setupFlavor(cfg)
	g.DocumentationURL = "https://confluence.atlassian.com/enterprise/using-personal-access-tokens-1026032365.html"
	if g.Flavor == FlavorCloud {
		g.Source = "builtin"
		g.CredentialURL = "https://id.atlassian.com/manage-profile/security/api-tokens"
		g.DocumentationURL = "https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account"
		g.Instructions = []string{"Create an API token for this product and use your Atlassian account email as the username.", "The current site-URL client supports unscoped API tokens; scoped tokens require the Atlassian API gateway and are not configured by this guide."}
	} else {
		g.Instructions = []string{"Open your profile, then Settings, then Personal Access Tokens to create a token.", "PAT requires Confluence 7.9+ and may be restricted by the administrator. Use --credential-url for the exact page on your deployment."}
	}
	if cfg.Auth.Scheme == SchemeBasic && g.Flavor != FlavorCloud {
		g.CredentialURL = base
		g.Source = "fallback"
		g.Instructions = []string{"Use your instance username and password if password authentication is enabled. For PAT authentication, select --auth-scheme pat instead."}
	}
	if cfg.ReuseStep != "" {
		g.Instructions = append([]string{"This machine may already hold a login for this service, under this or another scheme. Before signing in, run `" + cfg.ReuseStep + "`: it verifies a stored login and points this context at it. No secret is copied."}, g.Instructions...)
		g.NextSteps = append([]string{cfg.ReuseStep}, g.NextSteps...)
	}

	if cfg.Auth.CredentialURL != "" {
		g.CredentialURL = cfg.Auth.CredentialURL
		g.Source = sources[fieldCredentialURL]
		if g.Source == "" {
			g.Source = "config"
		}
	}
	return g, nil
}

// WithCredentialGuide preserves host-store recovery and appends acquisition guidance
// only to absence errors, never to inaccessible-store errors.
func WithCredentialGuide(err error, cfg Config) error {
	return kit.WithCredentialGuide(err, func() (AuthGuide, error) { return Guide(cfg, nil) })
}

// setupFlavor uses explicit/cached flavor and known Cloud hosts without a probe.
func setupFlavor(cfg Config) string {
	if cfg.Flavor != "" && cfg.Flavor != FlavorAuto {
		return cfg.Flavor
	}
	if cfg.DetectedFlavor != "" {
		return cfg.DetectedFlavor
	}
	u, err := url.Parse(cfg.BaseURL)
	if err == nil {
		host := strings.ToLower(u.Hostname())
		if strings.HasSuffix(host, ".atlassian.net") || host == "api.bitbucket.org" || host == "bitbucket.org" {
			return FlavorCloud
		}
	}
	return FlavorAuto
}

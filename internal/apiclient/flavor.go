package apiclient

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/angelmsger/confluence-cli/internal/transport"
)

// NormalizeBaseURL trims a trailing slash and a trailing "/wiki" segment so the
// dialect can append the correct REST prefix for either flavor.
func NormalizeBaseURL(raw string) string {
	s := strings.TrimRight(raw, "/")
	if strings.HasSuffix(s, "/wiki") {
		s = strings.TrimSuffix(s, "/wiki")
	}
	return s
}

// Detect probes baseURL to determine the Confluence flavor. The order of
// attempts is chosen to be fast and resilient:
//
//  1. Hostname shortcut. `*.atlassian.net` is the only host suffix Atlassian
//     uses for Cloud tenants, so we can answer Cloud with zero network calls.
//     Custom-domain tenants are rare for Confluence and still covered by the
//     network probes below.
//  2. `<base>/_edge/tenant_info` — Atlassian Cloud's tenant sentinel. Returns
//     `{cloudId, baseUrl}` 200 JSON without authentication, and unlike the
//     authenticated REST endpoints does not 302-redirect anonymous traffic to
//     the SSO login page (which previously caused probes to come back as
//     `200 text/html` and be rejected).
//  3. `<base>/wiki/api/v2/spaces?limit=1` — Cloud Confluence v2 API. Kept as
//     a fallback for non-standard tenant URLs.
//  4. `<base>/rest/api/space?limit=1` — Data Center / Server.
func Detect(ctx context.Context, http *transport.Client, baseURL string) (Flavor, error) {
	if isAtlassianCloudHost(baseURL) {
		return FlavorCloud, nil
	}
	base := NormalizeBaseURL(baseURL)
	if probeOK(ctx, http, base+"/_edge/tenant_info") {
		return FlavorCloud, nil
	}
	if probeOK(ctx, http, base+"/wiki/api/v2/spaces?limit=1") {
		return FlavorCloud, nil
	}
	if probeOK(ctx, http, base+"/rest/api/space?limit=1") {
		return FlavorDataCenter, nil
	}
	return FlavorAuto, cerrors.New(cerrors.CategoryNetwork, "DETECT_FAILED",
		"could not determine the Confluence flavor; neither the Cloud nor the Data Center API responded").
		WithNextSteps("Set the flavor explicitly with --flavor cloud|datacenter.",
			"confluence-cli doctor")
}

// isAtlassianCloudHost reports whether rawURL points at an `*.atlassian.net`
// tenant — the host suffix Atlassian reserves for Cloud instances. Inputs
// without a scheme are tolerated so a user typing `acme.atlassian.net/wiki`
// in the wizard is still recognized.
func isAtlassianCloudHost(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	host := ""
	if err == nil && parsed.Host != "" {
		host = parsed.Hostname()
	} else {
		// Likely missing a scheme; pull the host portion out manually so the
		// fast path still fires for `acme.atlassian.net` and friends.
		s := strings.TrimSpace(rawURL)
		s = strings.TrimPrefix(s, "//")
		if i := strings.IndexAny(s, "/?#"); i >= 0 {
			s = s[:i]
		}
		host = s
	}
	host = strings.ToLower(strings.TrimSpace(host))
	return strings.HasSuffix(host, ".atlassian.net")
}

// probeOK reports whether endpoint is a live REST API of the expected shape.
// A real API answers 200 with JSON, or 401/403 when authentication is missing.
// A redirect to a login page (followed to an HTML response) is rejected, so a
// Data Center server is not mistaken for Cloud when probed for the v2 API.
func probeOK(ctx context.Context, client *transport.Client, endpoint string) bool {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(ctx, req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return true
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return strings.Contains(resp.Header.Get("Content-Type"), "json")
}

// Ping verifies connectivity and credentials against the configured flavor.
func (c *apiClient) Ping(ctx context.Context) (ServerInfo, error) {
	info := ServerInfo{Flavor: c.flavor, BaseURL: c.baseURL}
	if err := c.getJSON(ctx, c.v1Base()+"/space", offsetQuery("", 1), &struct{}{}); err != nil {
		return info, err
	}
	info.Reachable = true
	return info, nil
}

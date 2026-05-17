package apiclient

import (
	"context"
	"net/http"
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

// Detect probes baseURL to determine the Confluence flavor. It tries the Cloud
// v2 API first, then the Data Center REST API.
func Detect(ctx context.Context, http *transport.Client, baseURL string) (Flavor, error) {
	base := NormalizeBaseURL(baseURL)
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

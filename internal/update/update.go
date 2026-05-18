// Package update reports whether a newer confluence-cli release is available.
//
// It is a passive, opt-in check: nothing here runs unless a command (today
// only `doctor`) explicitly calls Check. A failed check never returns an
// error — it degrades into an informational Status — so an offline or
// rate-limited environment never turns a diagnostic into a failure.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/angelmsger/confluence-cli/internal/transport"
)

// DefaultEndpoint is the GitHub API URL for the latest published release.
const DefaultEndpoint = "https://api.github.com/repos/angelmsger/confluence-cli/releases/latest"

// EndpointEnv overrides DefaultEndpoint. It exists so tests (and the e2e
// harness) can point the check at a local server instead of GitHub.
const EndpointEnv = "CONFLUENCE_RELEASE_API"

// Status is the outcome of an update check. It is always safe to render:
// a failed check sets Available=false and explains itself in Detail.
type Status struct {
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	Available bool   `json:"available"`
	Detail    string `json:"detail"`
}

// endpoint returns the release-metadata URL, honouring the env override.
func endpoint() string {
	if v := strings.TrimSpace(os.Getenv(EndpointEnv)); v != "" {
		return v
	}
	return DefaultEndpoint
}

// Check fetches the latest release and compares it with the running version.
// It never returns an error: a failed lookup is reported in Status.Detail.
func Check(ctx context.Context, doer transport.Doer, current string) Status {
	st := Status{Current: current}

	latest, err := fetchLatest(ctx, doer)
	if err != nil {
		st.Detail = "could not check for updates: " + err.Error()
		return st
	}
	st.Latest = latest

	cur, curOK := parse(current)
	lat, latOK := parse(latest)
	if !curOK || !latOK {
		st.Detail = "version comparison skipped (non-release build)"
		return st
	}
	if less(cur, lat) {
		st.Available = true
		st.Detail = fmt.Sprintf(
			"a newer release is available: %s -> %s; see %s",
			current, latest, "https://github.com/angelmsger/confluence-cli/releases/latest")
		return st
	}
	st.Detail = "up to date"
	return st
}

// fetchLatest queries the release endpoint and returns the latest version with
// any leading "v" stripped.
func fetchLatest(ctx context.Context, doer transport.Doer) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := doer.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release API returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("malformed release response: %w", err)
	}
	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", fmt.Errorf("release response carried no tag name")
	}
	return strings.TrimPrefix(tag, "v"), nil
}

// parse splits a "MAJOR.MINOR.PATCH" version into numeric components, ignoring
// any pre-release or build suffix. ok is false for non-release versions such
// as "dev", so callers skip the comparison rather than guess.
func parse(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// less reports whether version a precedes version b.
func less(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

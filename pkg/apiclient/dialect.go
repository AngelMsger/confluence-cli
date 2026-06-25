package apiclient

import (
	"net/url"
	"strconv"
)

// dialect.go centralises the per-flavor REST differences. The client targets
// REST API v1; Cloud and Data Center differ only in the base path prefix.
// A future v2 code path would add its own base + pagination helpers here.

// v1Base returns the REST v1 base path for the flavor.
//   - Cloud:       /wiki/rest/api
//   - Data Center: /rest/api
func (c *apiClient) v1Base() string {
	if c.flavor == FlavorCloud {
		return "/wiki/rest/api"
	}
	return "/rest/api"
}

// offsetQuery builds start/limit query parameters for offset pagination.
// The cursor, when present, carries the numeric start index.
func offsetQuery(cursor string, limit int) url.Values {
	q := url.Values{}
	start := 0
	if cursor != "" {
		if n, err := strconv.Atoi(cursor); err == nil {
			start = n
		}
	}
	q.Set("start", strconv.Itoa(start))
	q.Set("limit", strconv.Itoa(limit))
	return q
}

// nextOffsetToken returns the cursor for the following offset page, or "" when
// the current page was the last one.
func nextOffsetToken(cursor string, limit, size int) string {
	if limit <= 0 || size < limit {
		return ""
	}
	start := 0
	if cursor != "" {
		if n, err := strconv.Atoi(cursor); err == nil {
			start = n
		}
	}
	return strconv.Itoa(start + limit)
}

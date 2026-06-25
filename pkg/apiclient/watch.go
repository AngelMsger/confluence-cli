package apiclient

import (
	"context"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// watch.go holds the page-watch operations for the authenticated user. The
// v1 user-watch endpoints act on the credentialed user when no user is passed,
// so no current-user resolution is needed.

// watchPath returns the user-watch endpoint for a content ID.
func (c *apiClient) watchPath(pageID string) string {
	return c.v1Base() + "/user/watch/content/" + url.PathEscape(pageID)
}

// WatchStatus reports whether the authenticated user watches a page.
func (c *apiClient) WatchStatus(ctx context.Context, pageID string) (bool, error) {
	if pageID == "" {
		return false, cerrors.New(cerrors.CategoryUsage, "WATCH_NO_PAGE",
			"a page ID is required to check watch status")
	}
	var raw rawWatch
	if err := c.getJSON(ctx, c.watchPath(pageID), nil, &raw); err != nil {
		return false, err
	}
	return raw.Watching, nil
}

// buildSetWatch assembles the request that starts or stops watching a page.
func (c *apiClient) buildSetWatch(req WatchReq) (method, path string, err error) {
	if req.PageID == "" {
		return "", "", cerrors.New(cerrors.CategoryUsage, "WATCH_NO_PAGE",
			"a page ID is required to change watch status")
	}
	method = "DELETE"
	if req.Watching {
		method = "POST"
	}
	return method, c.watchPath(req.PageID), nil
}

// SetWatch starts (Watching=true) or stops (Watching=false) the authenticated
// user watching a page.
func (c *apiClient) SetWatch(ctx context.Context, req WatchReq) error {
	method, path, err := c.buildSetWatch(req)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, method, path, nil, nil, nil)
}

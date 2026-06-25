package apiclient

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// versions.go holds the page version-history operations: listing a page's
// versions and restoring it to an earlier one. Restore has no native v1
// endpoint, so it republishes a historical version's body as a new version,
// reusing buildUpdatePage — the history is left intact.

// ListPageVersions lists a page's version history, newest first.
func (c *apiClient) ListPageVersions(ctx context.Context, id string, opt ListOpts) (ListResult[PageVersion], error) {
	limit := c.limitOf(opt)
	q := offsetQuery(opt.Cursor, limit)
	path := c.v1Base() + "/content/" + url.PathEscape(id) + "/version"
	var raw rawVersionList
	if err := c.getJSON(ctx, path, q, &raw); err != nil {
		return ListResult[PageVersion]{}, err
	}
	res := ListResult[PageVersion]{Next: nextOffsetToken(opt.Cursor, limit, len(raw.Results))}
	for _, r := range raw.Results {
		res.Items = append(res.Items, pageVersionOf(r))
	}
	return res, nil
}

// getPageVersionBody fetches the storage-format body of a historical version.
func (c *apiClient) getPageVersionBody(ctx context.Context, id string, version int) (PageBody, error) {
	q := url.Values{}
	q.Set("expand", "content.body.storage")
	path := c.v1Base() + "/content/" + url.PathEscape(id) + "/version/" + strconv.Itoa(version)
	var raw rawContentVersion
	if err := c.getJSON(ctx, path, q, &raw); err != nil {
		return PageBody{}, err
	}
	b := bodyOf(raw.Content.Body)
	if b == nil {
		return PageBody{}, cerrors.Newf(cerrors.CategoryNotFound, "VERSION_NO_BODY",
			"version %d of page %s has no retrievable body", version, id)
	}
	return PageBody{Value: b.Value, Format: "storage"}, nil
}

// buildRestorePage assembles the PUT request that restores a page to an earlier
// version: it fetches that version's body and delegates to buildUpdatePage,
// which carries over the current title and bumps the version number.
func (c *apiClient) buildRestorePage(ctx context.Context, req RestorePageReq) (method, path string, payload any, err error) {
	if req.ID == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "PAGE_NO_ID",
			"a page ID is required to restore a page")
	}
	if req.Version <= 0 {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "RESTORE_NO_VERSION",
			"a positive --version is required to restore a page")
	}
	body, err := c.getPageVersionBody(ctx, req.ID, req.Version)
	if err != nil {
		return "", "", nil, err
	}
	message := req.Message
	if message == "" {
		message = fmt.Sprintf("Restored to version %d", req.Version)
	}
	return c.buildUpdatePage(ctx, UpdatePageReq{ID: req.ID, Body: &body, Message: message})
}

// RestorePage restores a page to an earlier version by republishing that
// version's body as a new version.
func (c *apiClient) RestorePage(ctx context.Context, req RestorePageReq) (*Page, error) {
	method, path, payload, err := c.buildRestorePage(ctx, req)
	if err != nil {
		return nil, err
	}
	var raw rawContent
	if err := c.doJSON(ctx, method, path, nil, payload, &raw); err != nil {
		return nil, err
	}
	return c.mapPage(raw), nil
}

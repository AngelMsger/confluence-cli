package apiclient

import (
	"context"
	"net/url"
	"strings"
)

// GetPage fetches a single page, optionally including its body.
func (c *apiClient) GetPage(ctx context.Context, id string, opt GetPageOpts) (*Page, error) {
	expand := []string{"version", "space", "ancestors"}
	if opt.WithBody {
		if strings.EqualFold(opt.BodyFormat, "view") {
			expand = append(expand, "body.view")
		} else {
			expand = append(expand, "body.storage")
		}
	}
	q := url.Values{}
	q.Set("expand", strings.Join(expand, ","))

	var raw rawContent
	if err := c.getJSON(ctx, c.v1Base()+"/content/"+url.PathEscape(id), q, &raw); err != nil {
		return nil, err
	}
	return c.mapPage(raw), nil
}

// ListChildren lists the direct child pages of a page.
func (c *apiClient) ListChildren(ctx context.Context, id string, opt ListOpts) (ListResult[Page], error) {
	return c.listPages(ctx, c.v1Base()+"/content/"+url.PathEscape(id)+"/child/page", opt)
}

// ListDescendants lists all descendant pages of a page.
func (c *apiClient) ListDescendants(ctx context.Context, id string, opt ListOpts) (ListResult[Page], error) {
	return c.listPages(ctx, c.v1Base()+"/content/"+url.PathEscape(id)+"/descendant/page", opt)
}

// listPages runs an offset-paginated content listing and normalizes the result.
func (c *apiClient) listPages(ctx context.Context, path string, opt ListOpts) (ListResult[Page], error) {
	limit := c.limitOf(opt)
	q := offsetQuery(opt.Cursor, limit)
	q.Set("expand", "version,space")

	var raw rawContentList
	if err := c.getJSON(ctx, path, q, &raw); err != nil {
		return ListResult[Page]{}, err
	}
	res := ListResult[Page]{Next: nextOffsetToken(opt.Cursor, limit, len(raw.Results))}
	for _, r := range raw.Results {
		res.Items = append(res.Items, *c.mapPage(r))
	}
	return res, nil
}

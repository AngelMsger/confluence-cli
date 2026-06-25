package apiclient

import (
	"context"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// ListSpaces lists spaces, optionally filtered by type.
func (c *apiClient) ListSpaces(ctx context.Context, opt SpaceListOpts) (ListResult[Space], error) {
	limit := c.limitOf(opt.ListOpts)
	q := offsetQuery(opt.Cursor, limit)
	if opt.Type != "" {
		q.Set("type", opt.Type)
	}

	var raw rawSpaceList
	if err := c.getJSON(ctx, c.v1Base()+"/space", q, &raw); err != nil {
		return ListResult[Space]{}, err
	}
	res := ListResult[Space]{Next: nextOffsetToken(opt.Cursor, limit, len(raw.Results))}
	for _, r := range raw.Results {
		res.Items = append(res.Items, *c.mapSpace(r))
	}
	return res, nil
}

// GetSpace fetches a single space by key.
func (c *apiClient) GetSpace(ctx context.Context, key string) (*Space, error) {
	if key == "" {
		return nil, cerrors.New(cerrors.CategoryUsage, "SPACE_KEY_REQUIRED",
			"a space key is required")
	}
	var raw rawSpace
	if err := c.getJSON(ctx, c.v1Base()+"/space/"+url.PathEscape(key), nil, &raw); err != nil {
		return nil, err
	}
	return c.mapSpace(raw), nil
}

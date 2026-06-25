package apiclient

import (
	"context"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// labels.go holds the content-label operations: list, add and remove. Labels
// are simple JSON writes; add and remove expose build* helpers shared with
// DescribeWrite for the --dry-run path.

type rawLabel struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	Label  string `json:"label"`
}

type rawLabelList struct {
	Results []rawLabel `json:"results"`
	Size    int        `json:"size"`
	Limit   int        `json:"limit"`
}

func mapLabel(r rawLabel) Label {
	name := r.Name
	if name == "" {
		name = r.Label
	}
	return Label{ID: r.ID, Name: name, Prefix: r.Prefix}
}

// ListLabels lists the labels on a page.
func (c *apiClient) ListLabels(ctx context.Context, pageID string, opt ListOpts) (ListResult[Label], error) {
	limit := c.limitOf(opt)
	q := offsetQuery(opt.Cursor, limit)
	path := c.v1Base() + "/content/" + url.PathEscape(pageID) + "/label"
	var raw rawLabelList
	if err := c.getJSON(ctx, path, q, &raw); err != nil {
		return ListResult[Label]{}, err
	}
	res := ListResult[Label]{Next: nextOffsetToken(opt.Cursor, limit, len(raw.Results))}
	for _, r := range raw.Results {
		res.Items = append(res.Items, mapLabel(r))
	}
	return res, nil
}

// labelWrite is one entry of the add-labels payload.
type labelWrite struct {
	Prefix string `json:"prefix"`
	Name   string `json:"name"`
}

// buildAddLabels assembles the POST request for adding labels to a page.
func (c *apiClient) buildAddLabels(req AddLabelsReq) (method, path string, payload any, err error) {
	if req.PageID == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "LABEL_NO_PAGE",
			"a page ID is required to add labels")
	}
	if len(req.Names) == 0 {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "LABEL_NO_NAME",
			"at least one label is required")
	}
	entries := make([]labelWrite, 0, len(req.Names))
	for _, n := range req.Names {
		if n == "" {
			return "", "", nil, cerrors.New(cerrors.CategoryUsage, "LABEL_NO_NAME",
				"a label name must not be empty")
		}
		entries = append(entries, labelWrite{Prefix: "global", Name: n})
	}
	return "POST", c.v1Base() + "/content/" + url.PathEscape(req.PageID) + "/label", entries, nil
}

// AddLabels adds one or more labels to a page and returns the resulting set.
func (c *apiClient) AddLabels(ctx context.Context, req AddLabelsReq) ([]Label, error) {
	method, path, payload, err := c.buildAddLabels(req)
	if err != nil {
		return nil, err
	}
	var raw rawLabelList
	if err := c.doJSON(ctx, method, path, nil, payload, &raw); err != nil {
		return nil, err
	}
	labels := make([]Label, 0, len(raw.Results))
	for _, r := range raw.Results {
		labels = append(labels, mapLabel(r))
	}
	return labels, nil
}

// buildRemoveLabel assembles the DELETE request for removing a label.
func (c *apiClient) buildRemoveLabel(req RemoveLabelReq) (method, path string, err error) {
	if req.PageID == "" {
		return "", "", cerrors.New(cerrors.CategoryUsage, "LABEL_NO_PAGE",
			"a page ID is required to remove a label")
	}
	if req.Name == "" {
		return "", "", cerrors.New(cerrors.CategoryUsage, "LABEL_NO_NAME",
			"a label name is required")
	}
	path = c.v1Base() + "/content/" + url.PathEscape(req.PageID) +
		"/label?name=" + url.QueryEscape(req.Name)
	return "DELETE", path, nil
}

// RemoveLabel removes a label from a page.
func (c *apiClient) RemoveLabel(ctx context.Context, req RemoveLabelReq) error {
	method, path, err := c.buildRemoveLabel(req)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, method, path, nil, nil, nil)
}

package apiclient

import (
	"context"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

// ListComments lists the footer comments of a page.
func (c *apiClient) ListComments(ctx context.Context, pageID string, opt ListOpts) (ListResult[Comment], error) {
	limit := c.limitOf(opt)
	q := offsetQuery(opt.Cursor, limit)
	q.Set("expand", "body.storage,version,ancestors")
	q.Set("depth", "all")
	q.Set("location", "footer")

	path := c.v1Base() + "/content/" + url.PathEscape(pageID) + "/child/comment"
	var raw rawContentList
	if err := c.getJSON(ctx, path, q, &raw); err != nil {
		return ListResult[Comment]{}, err
	}
	res := ListResult[Comment]{Next: nextOffsetToken(opt.Cursor, limit, len(raw.Results))}
	for _, r := range raw.Results {
		res.Items = append(res.Items, *c.mapComment(r, pageID))
	}
	return res, nil
}

// commentRequest is the v1 payload for creating a comment.
type commentRequest struct {
	Type      string                  `json:"type"`
	Container commentContainer        `json:"container"`
	Ancestors []commentAncestor       `json:"ancestors,omitempty"`
	Body      map[string]rawValueBody `json:"body"`
}

type commentContainer struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type commentAncestor struct {
	ID string `json:"id"`
}

type rawValueBody struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

// AddComment creates a footer comment on a page (the only write operation).
func (c *apiClient) AddComment(ctx context.Context, req AddCommentReq) (*Comment, error) {
	if req.PageID == "" {
		return nil, cerrors.New(cerrors.CategoryUsage, "COMMENT_NO_PAGE",
			"a page ID is required to add a comment")
	}
	if req.Body == "" {
		return nil, cerrors.New(cerrors.CategoryUsage, "COMMENT_NO_BODY",
			"comment body must not be empty")
	}
	repr := "storage"
	if req.Format == "wiki" {
		repr = "wiki"
	}
	payload := commentRequest{
		Type:      "comment",
		Container: commentContainer{ID: req.PageID, Type: "page"},
		Body: map[string]rawValueBody{
			repr: {Value: req.Body, Representation: repr},
		},
	}
	if req.ParentID != "" {
		payload.Ancestors = []commentAncestor{{ID: req.ParentID}}
	}

	var raw rawContent
	err := c.doJSON(ctx, "POST", c.v1Base()+"/content", nil, payload, &raw)
	if err != nil {
		return nil, err
	}
	return c.mapComment(raw, req.PageID), nil
}

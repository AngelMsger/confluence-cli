package apiclient

import (
	"context"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// comments_write.go holds the comment write operations: editing and deleting a
// footer comment. A comment is a content object, so these mirror the page
// update/delete shapes; each has a build* helper shared with DescribeWrite.

// commentUpdateRequest is the v1 payload for editing a comment.
type commentUpdateRequest struct {
	Type    string                  `json:"type"`
	Body    map[string]rawValueBody `json:"body"`
	Version *pageVersionWrite       `json:"version"`
}

// getComment fetches a single comment as a content object, used to learn the
// current version before an edit.
func (c *apiClient) getComment(ctx context.Context, id string) (*Comment, error) {
	q := url.Values{}
	q.Set("expand", "version,body.storage")
	var raw rawContent
	if err := c.getJSON(ctx, c.v1Base()+"/content/"+url.PathEscape(id), q, &raw); err != nil {
		return nil, err
	}
	return c.mapComment(raw, ""), nil
}

// buildUpdateComment assembles the PUT request for editing a comment. It GETs
// the comment to learn the current version when one was not asserted (the GET
// is safe under --dry-run).
func (c *apiClient) buildUpdateComment(ctx context.Context, req UpdateCommentReq) (method, path string, payload any, err error) {
	if req.ID == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "COMMENT_NO_ID",
			"a comment ID is required to update a comment")
	}
	if req.Body == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "COMMENT_NO_BODY",
			"comment body must not be empty")
	}
	version := req.ExpectVersion
	if version == 0 {
		cur, err := c.getComment(ctx, req.ID)
		if err != nil {
			return "", "", nil, err
		}
		if cur.Version != nil {
			version = cur.Version.Number
		}
	}
	repr := "storage"
	if req.Format == "wiki" {
		repr = "wiki"
	}
	p := commentUpdateRequest{
		Type:    "comment",
		Body:    map[string]rawValueBody{repr: {Value: req.Body, Representation: repr}},
		Version: &pageVersionWrite{Number: version + 1},
	}
	return "PUT", c.v1Base() + "/content/" + url.PathEscape(req.ID), p, nil
}

// UpdateComment edits a footer comment's body. The new version is the current
// version + 1; a 409 surfaces as a version-conflict error.
func (c *apiClient) UpdateComment(ctx context.Context, req UpdateCommentReq) (*Comment, error) {
	method, path, payload, err := c.buildUpdateComment(ctx, req)
	if err != nil {
		return nil, err
	}
	var raw rawContent
	if err := c.doJSON(ctx, method, path, nil, payload, &raw); err != nil {
		return nil, err
	}
	return c.mapComment(raw, ""), nil
}

// buildDeleteComment assembles the DELETE request for a comment.
func (c *apiClient) buildDeleteComment(req DeleteCommentReq) (method, path string, err error) {
	if req.ID == "" {
		return "", "", cerrors.New(cerrors.CategoryUsage, "COMMENT_NO_ID",
			"a comment ID is required to delete a comment")
	}
	return "DELETE", c.v1Base() + "/content/" + url.PathEscape(req.ID), nil
}

// DeleteComment deletes a comment by its content ID.
func (c *apiClient) DeleteComment(ctx context.Context, req DeleteCommentReq) error {
	method, path, err := c.buildDeleteComment(req)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, method, path, nil, nil, nil)
}

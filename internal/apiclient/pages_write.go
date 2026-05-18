package apiclient

import (
	"context"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

// pages_write.go holds the page write operations: create, update, delete,
// move and copy. Each operation's payload is built by an unexported build*
// helper shared between the real call and DescribeWrite (the --dry-run path),
// so the previewed request can never diverge from the sent one.

// pageWriteRequest is the v1 payload for creating or updating a page.
type pageWriteRequest struct {
	Type      string                  `json:"type"`
	Title     string                  `json:"title"`
	Space     *pageSpaceRef           `json:"space,omitempty"`
	Ancestors []pageAncestor          `json:"ancestors,omitempty"`
	Version   *pageVersionWrite       `json:"version,omitempty"`
	Body      map[string]rawValueBody `json:"body,omitempty"`
}

type pageSpaceRef struct {
	Key string `json:"key"`
}

type pageAncestor struct {
	ID string `json:"id"`
}

type pageVersionWrite struct {
	Number    int    `json:"number"`
	MinorEdit bool   `json:"minorEdit,omitempty"`
	Message   string `json:"message,omitempty"`
}

// bodyMap renders a PageBody into the v1 body object keyed by representation.
func bodyMap(b PageBody) map[string]rawValueBody {
	repr := "storage"
	if b.Format == "wiki" {
		repr = "wiki"
	}
	return map[string]rawValueBody{
		repr: {Value: b.Value, Representation: repr},
	}
}

// buildCreatePage assembles the POST request for creating a page.
func (c *apiClient) buildCreatePage(req CreatePageReq) (method, path string, payload any, err error) {
	if req.SpaceKey == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "PAGE_NO_SPACE",
			"a space key is required to create a page")
	}
	if req.Title == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "PAGE_NO_TITLE",
			"a title is required to create a page")
	}
	p := pageWriteRequest{
		Type:  "page",
		Title: req.Title,
		Space: &pageSpaceRef{Key: req.SpaceKey},
		Body:  bodyMap(req.Body),
	}
	if req.ParentID != "" {
		p.Ancestors = []pageAncestor{{ID: req.ParentID}}
	}
	return "POST", c.v1Base() + "/content", p, nil
}

// CreatePage creates a new page.
func (c *apiClient) CreatePage(ctx context.Context, req CreatePageReq) (*Page, error) {
	method, path, payload, err := c.buildCreatePage(req)
	if err != nil {
		return nil, err
	}
	var raw rawContent
	if err := c.doJSON(ctx, method, path, nil, payload, &raw); err != nil {
		return nil, err
	}
	return c.mapPage(raw), nil
}

// buildUpdatePage assembles the PUT request for updating a page. It performs a
// read-only GET when the version, title or body must be carried over from the
// current page (the GET is safe under --dry-run).
func (c *apiClient) buildUpdatePage(ctx context.Context, req UpdatePageReq) (method, path string, payload any, err error) {
	if req.ID == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "PAGE_NO_ID",
			"a page ID is required to update a page")
	}
	title := req.Title
	version := req.ExpectVersion
	hasBody := req.Body != nil
	var body PageBody
	if hasBody {
		body = *req.Body
	}
	if version == 0 || title == "" || !hasBody {
		cur, err := c.GetPage(ctx, req.ID, GetPageOpts{WithBody: !hasBody, BodyFormat: "storage"})
		if err != nil {
			return "", "", nil, err
		}
		if version == 0 && cur.Version != nil {
			version = cur.Version.Number
		}
		if title == "" {
			title = cur.Title
		}
		if !hasBody {
			body = PageBody{Format: "storage"}
			if cur.Body != nil {
				body.Value = cur.Body.Value
			}
		}
	}
	p := pageWriteRequest{
		Type:  "page",
		Title: title,
		Version: &pageVersionWrite{
			Number:    version + 1,
			MinorEdit: req.Minor,
			Message:   req.Message,
		},
		Body: bodyMap(body),
	}
	return "PUT", c.v1Base() + "/content/" + url.PathEscape(req.ID), p, nil
}

// UpdatePage updates a page's title and/or body. The new version number is the
// current version + 1; a 409 response surfaces as a PAGE_VERSION_CONFLICT error.
func (c *apiClient) UpdatePage(ctx context.Context, req UpdatePageReq) (*Page, error) {
	method, path, payload, err := c.buildUpdatePage(ctx, req)
	if err != nil {
		return nil, err
	}
	var raw rawContent
	if err := c.doJSON(ctx, method, path, nil, payload, &raw); err != nil {
		return nil, err
	}
	return c.mapPage(raw), nil
}

// buildDeletePage assembles the DELETE request for a page. When Purge is set
// the path targets the trashed copy for permanent removal.
func (c *apiClient) buildDeletePage(req DeletePageReq) (method, path string, payload any, err error) {
	if req.ID == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "PAGE_NO_ID",
			"a page ID is required to delete a page")
	}
	path = c.v1Base() + "/content/" + url.PathEscape(req.ID)
	if req.Purge {
		path += "?status=trashed"
	}
	return "DELETE", path, nil, nil
}

// DeletePage moves a page to the trash. With Purge it then permanently removes
// the trashed page (trashing first when the page is still current).
func (c *apiClient) DeletePage(ctx context.Context, req DeletePageReq) error {
	if req.ID == "" {
		return cerrors.New(cerrors.CategoryUsage, "PAGE_NO_ID",
			"a page ID is required to delete a page")
	}
	base := c.v1Base() + "/content/" + url.PathEscape(req.ID)
	if !req.Purge {
		return c.doJSON(ctx, "DELETE", base, nil, nil, nil)
	}
	// Trash first; tolerate not-found in case the page is already trashed.
	if err := c.doJSON(ctx, "DELETE", base, nil, nil, nil); err != nil {
		if ce := cerrors.AsCLIError(err); ce.Category != cerrors.CategoryNotFound {
			return err
		}
	}
	return c.doJSON(ctx, "DELETE", base+"?status=trashed", nil, nil, nil)
}

// buildMovePage assembles the PUT request for moving a page. It GETs the
// current page to carry over its title, version and body unchanged.
func (c *apiClient) buildMovePage(ctx context.Context, req MovePageReq) (method, path string, payload any, err error) {
	if req.ID == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "PAGE_NO_ID",
			"a page ID is required to move a page")
	}
	if req.TargetParent == "" && req.TargetSpace == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "PAGE_MOVE_NO_TARGET",
			"specify --target-parent and/or --target-space")
	}
	cur, err := c.GetPage(ctx, req.ID, GetPageOpts{WithBody: true, BodyFormat: "storage"})
	if err != nil {
		return "", "", nil, err
	}
	version := 0
	if cur.Version != nil {
		version = cur.Version.Number
	}
	body := PageBody{Format: "storage"}
	if cur.Body != nil {
		body.Value = cur.Body.Value
	}
	p := pageWriteRequest{
		Type:    "page",
		Title:   cur.Title,
		Version: &pageVersionWrite{Number: version + 1},
		Body:    bodyMap(body),
	}
	if req.TargetSpace != "" {
		p.Space = &pageSpaceRef{Key: req.TargetSpace}
	}
	if req.TargetParent != "" {
		p.Ancestors = []pageAncestor{{ID: req.TargetParent}}
	}
	return "PUT", c.v1Base() + "/content/" + url.PathEscape(req.ID), p, nil
}

// MovePage moves a page under a new parent and/or into a new space.
func (c *apiClient) MovePage(ctx context.Context, req MovePageReq) (*Page, error) {
	method, path, payload, err := c.buildMovePage(ctx, req)
	if err != nil {
		return nil, err
	}
	var raw rawContent
	if err := c.doJSON(ctx, method, path, nil, payload, &raw); err != nil {
		return nil, err
	}
	return c.mapPage(raw), nil
}

// buildCopyPage GETs the source page and assembles a create request for the
// copy. v1 has no native copy endpoint, so a copy is a shallow re-create:
// title and body only, no child pages or attachments.
func (c *apiClient) buildCopyPage(ctx context.Context, req CopyPageReq) (method, path string, payload any, err error) {
	if req.SourceID == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "PAGE_NO_ID",
			"a source page ID is required to copy a page")
	}
	if req.Title == "" {
		return "", "", nil, cerrors.New(cerrors.CategoryUsage, "PAGE_NO_TITLE",
			"a title is required for the copied page")
	}
	src, err := c.GetPage(ctx, req.SourceID, GetPageOpts{WithBody: true, BodyFormat: "storage"})
	if err != nil {
		return "", "", nil, err
	}
	create := CreatePageReq{
		SpaceKey: req.SpaceKey,
		Title:    req.Title,
		ParentID: req.ParentID,
		Body:     PageBody{Format: "storage"},
	}
	if create.SpaceKey == "" {
		create.SpaceKey = src.SpaceKey
	}
	if create.ParentID == "" && len(src.Ancestors) > 0 {
		create.ParentID = src.Ancestors[len(src.Ancestors)-1].ID
	}
	if src.Body != nil {
		create.Body.Value = src.Body.Value
	}
	return c.buildCreatePage(create)
}

// CopyPage creates a shallow copy of a page (title and body only).
func (c *apiClient) CopyPage(ctx context.Context, req CopyPageReq) (*Page, error) {
	method, path, payload, err := c.buildCopyPage(ctx, req)
	if err != nil {
		return nil, err
	}
	var raw rawContent
	if err := c.doJSON(ctx, method, path, nil, payload, &raw); err != nil {
		return nil, err
	}
	return c.mapPage(raw), nil
}

// DescribeWrite returns the HTTP request a write operation would send, without
// sending it. The op must be one of the *Req types. Read-only GETs needed to
// compute a payload (version, title, body) are still performed.
func (c *apiClient) DescribeWrite(ctx context.Context, op any) (WriteRequestPlan, error) {
	var (
		method, path string
		payload      any
		err          error
	)
	switch v := op.(type) {
	case CreatePageReq:
		method, path, payload, err = c.buildCreatePage(v)
	case UpdatePageReq:
		method, path, payload, err = c.buildUpdatePage(ctx, v)
	case DeletePageReq:
		method, path, payload, err = c.buildDeletePage(v)
	case MovePageReq:
		method, path, payload, err = c.buildMovePage(ctx, v)
	case CopyPageReq:
		method, path, payload, err = c.buildCopyPage(ctx, v)
	case UploadAttachmentReq:
		var file multipartFile
		var fields map[string]string
		method, path, file, fields, err = c.buildUploadAttachment(v)
		if err == nil {
			payload = multipartPlanOf(file, fields)
		}
	case UpdateAttachmentReq:
		var file multipartFile
		var fields map[string]string
		method, path, file, fields, err = c.buildUpdateAttachment(v)
		if err == nil {
			payload = multipartPlanOf(file, fields)
		}
	case DeleteAttachmentReq:
		method, path, err = c.buildDeleteAttachment(v)
	case AddLabelsReq:
		method, path, payload, err = c.buildAddLabels(v)
	case RemoveLabelReq:
		method, path, err = c.buildRemoveLabel(v)
	default:
		return WriteRequestPlan{}, cerrors.New(cerrors.CategoryInternal, "DRYRUN_BAD_OP",
			"unsupported write operation for dry-run")
	}
	if err != nil {
		return WriteRequestPlan{}, err
	}
	return WriteRequestPlan{Method: method, URL: c.baseURL + path, Payload: payload}, nil
}

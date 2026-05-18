package apiclient

import (
	"context"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

// attachments_write.go holds the attachment write operations: upload (attach a
// new file), update (replace a file with a new version) and delete. Uploads
// use multipart/form-data; as with the page writes, each operation's request
// is built by an unexported build* helper shared with DescribeWrite (the
// --dry-run path), so the previewed request can never diverge from the sent one.

// attachmentFileField is the multipart field name Confluence expects for the
// uploaded file.
const attachmentFileField = "file"

// uploadFields renders the optional comment/minorEdit form fields shared by
// the upload and update endpoints.
func uploadFields(comment string, minorEdit bool) map[string]string {
	f := map[string]string{}
	if comment != "" {
		f["comment"] = comment
	}
	if minorEdit {
		f["minorEdit"] = "true"
	}
	return f
}

// multipartPlanOf describes a multipart upload for a --dry-run preview.
func multipartPlanOf(file multipartFile, fields map[string]string) MultipartPlan {
	return MultipartPlan{FileName: file.FileName, FileBytes: len(file.Data), Fields: fields}
}

// buildUploadAttachment assembles the multipart POST for attaching a new file.
func (c *apiClient) buildUploadAttachment(req UploadAttachmentReq) (method, path string, file multipartFile, fields map[string]string, err error) {
	if req.PageID == "" {
		return "", "", multipartFile{}, nil, cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_PAGE",
			"a page ID is required to upload an attachment")
	}
	if req.FileName == "" {
		return "", "", multipartFile{}, nil, cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_NAME",
			"an attachment file name is required")
	}
	path = c.v1Base() + "/content/" + url.PathEscape(req.PageID) + "/child/attachment"
	file = multipartFile{FieldName: attachmentFileField, FileName: req.FileName, Data: req.Data}
	return "POST", path, file, uploadFields(req.Comment, req.MinorEdit), nil
}

// UploadAttachment attaches a new file to a page.
func (c *apiClient) UploadAttachment(ctx context.Context, req UploadAttachmentReq) (*Attachment, error) {
	method, path, file, fields, err := c.buildUploadAttachment(req)
	if err != nil {
		return nil, err
	}
	var raw rawContentList
	if err := c.doMultipart(ctx, method, path, file, fields, &raw); err != nil {
		return nil, err
	}
	if len(raw.Results) == 0 {
		return nil, cerrors.New(cerrors.CategoryParse, "ATTACH_NO_RESULT",
			"Confluence returned no attachment in the upload response")
	}
	return c.mapAttachment(raw.Results[0], req.PageID), nil
}

// buildUpdateAttachment assembles the multipart POST for replacing an
// attachment's content with a new version.
func (c *apiClient) buildUpdateAttachment(req UpdateAttachmentReq) (method, path string, file multipartFile, fields map[string]string, err error) {
	if req.PageID == "" {
		return "", "", multipartFile{}, nil, cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_PAGE",
			"a page ID is required to update an attachment")
	}
	if req.AttachmentID == "" {
		return "", "", multipartFile{}, nil, cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_ID",
			"an attachment ID is required to update an attachment")
	}
	if req.FileName == "" {
		return "", "", multipartFile{}, nil, cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_NAME",
			"an attachment file name is required")
	}
	path = c.v1Base() + "/content/" + url.PathEscape(req.PageID) +
		"/child/attachment/" + url.PathEscape(req.AttachmentID) + "/data"
	file = multipartFile{FieldName: attachmentFileField, FileName: req.FileName, Data: req.Data}
	return "POST", path, file, uploadFields(req.Comment, req.MinorEdit), nil
}

// UpdateAttachment replaces an attachment's content, creating a new version.
func (c *apiClient) UpdateAttachment(ctx context.Context, req UpdateAttachmentReq) (*Attachment, error) {
	method, path, file, fields, err := c.buildUpdateAttachment(req)
	if err != nil {
		return nil, err
	}
	var raw rawContent
	if err := c.doMultipart(ctx, method, path, file, fields, &raw); err != nil {
		return nil, err
	}
	return c.mapAttachment(raw, req.PageID), nil
}

// buildDeleteAttachment assembles the DELETE request for an attachment.
func (c *apiClient) buildDeleteAttachment(req DeleteAttachmentReq) (method, path string, err error) {
	if req.AttachmentID == "" {
		return "", "", cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_ID",
			"an attachment ID is required to delete an attachment")
	}
	return "DELETE", c.v1Base() + "/content/" + url.PathEscape(req.AttachmentID), nil
}

// DeleteAttachment deletes an attachment by its content ID.
func (c *apiClient) DeleteAttachment(ctx context.Context, req DeleteAttachmentReq) error {
	method, path, err := c.buildDeleteAttachment(req)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, method, path, nil, nil, nil)
}

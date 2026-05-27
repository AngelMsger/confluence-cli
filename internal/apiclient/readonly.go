package apiclient

import (
	"context"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

// NewReadOnly wraps inner so that every mutating method returns a
// READONLY_BLOCKED error before any HTTP request is sent. Reads (every other
// method on Client) and DescribeWrite (the --dry-run preview path) pass
// straight through, so safe inspection still works.
func NewReadOnly(inner Client) Client { return &readOnlyClient{Client: inner} }

// readOnlyClient is the read-only enforcement layer. It embeds Client so every
// non-mutating method is inherited unchanged; mutating methods are overridden
// to return READONLY_BLOCKED.
type readOnlyClient struct{ Client }

// blocked returns the structured error for a blocked write. op names the
// operation (e.g. "DeletePage") so the error message is precise about which
// call was refused.
func blocked(op string) *cerrors.CLIError {
	return cerrors.Newf(cerrors.CategoryPermission, "READONLY_BLOCKED",
		"operation %q blocked: read-only mode is enabled", op).
		WithHint("Re-run with --allow-writes to permit writes for this invocation, "+
			"or unset CONFLUENCE_CLI_READ_ONLY / defaults.read_only.").
		WithNextSteps(
			"Add --allow-writes to the command line",
			"unset CONFLUENCE_CLI_READ_ONLY",
			"Set defaults.read_only=false in ~/.confluence/config.yaml",
		)
}

// Page writes.
func (r *readOnlyClient) CreatePage(_ context.Context, _ CreatePageReq) (*Page, error) {
	return nil, blocked("CreatePage")
}
func (r *readOnlyClient) UpdatePage(_ context.Context, _ UpdatePageReq) (*Page, error) {
	return nil, blocked("UpdatePage")
}
func (r *readOnlyClient) DeletePage(_ context.Context, _ DeletePageReq) error {
	return blocked("DeletePage")
}
func (r *readOnlyClient) MovePage(_ context.Context, _ MovePageReq) (*Page, error) {
	return nil, blocked("MovePage")
}
func (r *readOnlyClient) CopyPage(_ context.Context, _ CopyPageReq) (*Page, error) {
	return nil, blocked("CopyPage")
}
func (r *readOnlyClient) RestorePage(_ context.Context, _ RestorePageReq) (*Page, error) {
	return nil, blocked("RestorePage")
}

// Watch writes.
func (r *readOnlyClient) SetWatch(_ context.Context, _ WatchReq) error {
	return blocked("SetWatch")
}

// Comment writes.
func (r *readOnlyClient) AddComment(_ context.Context, _ AddCommentReq) (*Comment, error) {
	return nil, blocked("AddComment")
}
func (r *readOnlyClient) UpdateComment(_ context.Context, _ UpdateCommentReq) (*Comment, error) {
	return nil, blocked("UpdateComment")
}
func (r *readOnlyClient) DeleteComment(_ context.Context, _ DeleteCommentReq) error {
	return blocked("DeleteComment")
}

// Attachment writes.
func (r *readOnlyClient) UploadAttachment(_ context.Context, _ UploadAttachmentReq) (*Attachment, error) {
	return nil, blocked("UploadAttachment")
}
func (r *readOnlyClient) UpdateAttachment(_ context.Context, _ UpdateAttachmentReq) (*Attachment, error) {
	return nil, blocked("UpdateAttachment")
}
func (r *readOnlyClient) DeleteAttachment(_ context.Context, _ DeleteAttachmentReq) error {
	return blocked("DeleteAttachment")
}

// Label writes.
func (r *readOnlyClient) AddLabels(_ context.Context, _ AddLabelsReq) ([]Label, error) {
	return nil, blocked("AddLabels")
}
func (r *readOnlyClient) RemoveLabel(_ context.Context, _ RemoveLabelReq) error {
	return blocked("RemoveLabel")
}

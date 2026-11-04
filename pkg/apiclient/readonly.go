package apiclient

import (
	"context"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// NewReadOnly wraps inner so that every mutating method returns a
// READONLY_BLOCKED error before any HTTP request is sent. Reads (the Reader
// half of Client, including DescribeWrite, the --dry-run preview path) pass
// straight through, so safe inspection still works.
func NewReadOnly(inner Client) Client { return &readOnlyClient{Reader: inner} }

// readOnlyClient is the read-only enforcement layer. It keeps inner only as a
// Reader, so reads are delegated unchanged and no write can reach inner. Every
// Writer method is overridden below to return READONLY_BLOCKED; the assertion
// that follows stops compilation when a new Writer method lacks its override.
type readOnlyClient struct{ Reader }

var _ Client = (*readOnlyClient)(nil)

// blocked returns the family's READONLY_BLOCKED error for a refused write. op
// names the operation (e.g. "DeletePage") so the message says which call was
// refused. Turning read-only mode off is the user's decision, so the error
// names the user's switches without offering to change them.
func blocked(op string) *cerrors.CLIError {
	return cerrors.ReadOnlyBlocked(op, cerrors.ReadOnlyMode{
		Env:        "CONFLUENCE_CLI_READ_ONLY",
		ConfigFile: "~/.angelmsger/confluence/config.yaml",
		DryRun:     true,
	})
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

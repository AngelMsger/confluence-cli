// Package apiclient is a flavor-agnostic Confluence REST client. It supports
// Confluence Cloud (REST v1 + v2) and Data Center / Server (REST v1) behind a
// single Client interface returning normalized models.
package apiclient

// Flavor identifies the Confluence backend variant.
type Flavor string

const (
	FlavorCloud      Flavor = "cloud"
	FlavorDataCenter Flavor = "datacenter"
	FlavorAuto       Flavor = "auto"
)

// ServerInfo is the result of a connectivity probe.
type ServerInfo struct {
	Flavor    Flavor `json:"flavor"`
	BaseURL   string `json:"base_url"`
	Reachable bool   `json:"reachable"`
}

// Space is a normalized Confluence space.
type Space struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
}

// PageRef is a lightweight reference to a page (used for ancestors).
type PageRef struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}

// Version describes a content version.
type Version struct {
	Number int    `json:"number"`
	When   string `json:"when,omitempty"`
	By     string `json:"by,omitempty"`
}

// Body is page or comment content in a single representation.
type Body struct {
	Representation string `json:"representation"`
	Value          string `json:"value"`
}

// Page is a normalized Confluence page.
type Page struct {
	ID        string    `json:"id"`
	Type      string    `json:"type,omitempty"`
	Title     string    `json:"title"`
	SpaceKey  string    `json:"space_key,omitempty"`
	Status    string    `json:"status,omitempty"`
	Version   *Version  `json:"version,omitempty"`
	URL       string    `json:"url,omitempty"`
	Ancestors []PageRef `json:"ancestors,omitempty"`
	Body      *Body     `json:"body,omitempty"`
}

// Comment is a normalized Confluence footer comment.
type Comment struct {
	ID       string   `json:"id"`
	PageID   string   `json:"page_id,omitempty"`
	ParentID string   `json:"parent_id,omitempty"`
	Body     *Body    `json:"body,omitempty"`
	Version  *Version `json:"version,omitempty"`
	URL      string   `json:"url,omitempty"`
}

// Attachment is a normalized Confluence attachment.
type Attachment struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	MediaType   string `json:"media_type,omitempty"`
	FileSize    int64  `json:"file_size,omitempty"`
	PageID      string `json:"page_id,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

// Label is a normalized Confluence content label.
type Label struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	Prefix string `json:"prefix,omitempty"`
}

// SearchHit is a normalized CQL search result.
type SearchHit struct {
	ID           string `json:"id"`
	Type         string `json:"type,omitempty"`
	Title        string `json:"title"`
	SpaceKey     string `json:"space_key,omitempty"`
	URL          string `json:"url,omitempty"`
	Excerpt      string `json:"excerpt,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
}

// ListResult is one page of a paginated listing. Next is an opaque cursor for
// the following page, empty when the listing is exhausted.
type ListResult[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next,omitempty"`
}

// GetPageOpts controls a page fetch.
type GetPageOpts struct {
	// BodyFormat is "storage" (default) or "view".
	BodyFormat string
	// WithBody requests the page body; false fetches metadata only.
	WithBody bool
}

// ListOpts controls a paginated listing.
type ListOpts struct {
	// Limit is the page size; 0 uses the client default.
	Limit int
	// Cursor continues a previous listing; empty starts from the beginning.
	Cursor string
}

// SpaceListOpts controls a space listing.
type SpaceListOpts struct {
	ListOpts
	// Type filters by "global" or "personal"; empty returns all.
	Type string
}

// AddCommentReq is a request to create a footer comment.
type AddCommentReq struct {
	PageID   string
	ParentID string // optional: reply to this comment
	Body     string
	// Format is "storage" (XHTML, default) or "wiki" (wiki markup).
	Format string
}

// DownloadMeta describes a completed attachment download.
type DownloadMeta struct {
	ContentType string `json:"content_type,omitempty"`
	Bytes       int64  `json:"bytes"`
}

// PageBody is body content in a single representation for a write request.
// Format is "storage" (XHTML, default) or "wiki" (wiki markup, converted
// server-side). Markdown input is lowered to storage by the caller.
type PageBody struct {
	Value  string
	Format string
}

// CreatePageReq is a request to create a page.
type CreatePageReq struct {
	SpaceKey string
	Title    string
	ParentID string // optional: place the page under this parent
	Body     PageBody
}

// UpdatePageReq is a request to update a page. An empty Title keeps the
// current title; a nil Body keeps the current body. ExpectVersion 0 makes the
// client fetch the current version before updating; a non-zero value asserts
// the version the caller last read.
type UpdatePageReq struct {
	ID            string
	Title         string
	Body          *PageBody
	ExpectVersion int
	Minor         bool
	Message       string
}

// DeletePageReq is a request to delete a page. Purge permanently removes the
// page (it is trashed first when not already trashed).
type DeletePageReq struct {
	ID    string
	Purge bool
}

// MovePageReq is a request to move a page under a new parent and/or space.
type MovePageReq struct {
	ID           string
	TargetParent string // optional
	TargetSpace  string // optional
}

// CopyPageReq is a request to copy a page's title and body into a new page.
type CopyPageReq struct {
	SourceID string
	Title    string
	SpaceKey string // optional, default = source page's space
	ParentID string // optional, default = source page's parent
}

// UploadAttachmentReq is a request to attach a new file to a page. When an
// attachment of the same FileName already exists, Confluence stores the upload
// as a new version of it.
type UploadAttachmentReq struct {
	PageID    string
	FileName  string
	Data      []byte
	Comment   string // optional version comment
	MinorEdit bool
}

// UpdateAttachmentReq replaces an existing attachment's content with a new
// version. PageID is the page the attachment belongs to.
type UpdateAttachmentReq struct {
	PageID       string
	AttachmentID string
	FileName     string
	Data         []byte
	Comment      string // optional version comment
	MinorEdit    bool
}

// DeleteAttachmentReq is a request to delete an attachment by its content ID.
type DeleteAttachmentReq struct {
	AttachmentID string
}

// AddLabelsReq is a request to add one or more labels to a page.
type AddLabelsReq struct {
	PageID string
	Names  []string
}

// RemoveLabelReq is a request to remove a label from a page.
type RemoveLabelReq struct {
	PageID string
	Name   string
}

// WriteRequestPlan describes the HTTP request a write operation would send,
// without sending it. It is used to render --dry-run previews.
type WriteRequestPlan struct {
	Method  string `json:"method"`
	URL     string `json:"url"`
	Payload any    `json:"payload,omitempty"`
}

// MultipartPlan describes a multipart/form-data upload for a --dry-run preview.
// It reports the file and form fields rather than the raw bytes.
type MultipartPlan struct {
	FileName  string            `json:"file_name"`
	FileBytes int               `json:"file_bytes"`
	Fields    map[string]string `json:"fields,omitempty"`
}

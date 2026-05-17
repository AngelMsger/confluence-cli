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

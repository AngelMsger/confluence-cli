package apiclient

import (
	"strconv"
	"strings"
)

// spaceIDString renders a space ID that may arrive as a JSON number (Data
// Center) or string (Cloud).
func spaceIDString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case int:
		return strconv.Itoa(t)
	case nil:
		return ""
	default:
		return ""
	}
}

// mapping.go holds the raw JSON shapes returned by the Confluence REST API and
// the functions that normalize them into the package's public models.
//
// The client targets REST API v1, which is served by both Confluence Cloud
// (under /wiki/rest/api) and Data Center / Server (under /rest/api). The two
// flavors differ only in the base path; the JSON payload shapes are identical,
// so a single set of mappers covers both. The dialect (see dialect.go) is the
// seam where a future v2 code path would plug in.

type rawValue struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

type rawBody struct {
	Storage *rawValue `json:"storage,omitempty"`
	View    *rawValue `json:"view,omitempty"`
	Wiki    *rawValue `json:"wiki,omitempty"`
}

type rawVersion struct {
	Number int    `json:"number"`
	When   string `json:"when"`
	By     struct {
		DisplayName string `json:"displayName"`
	} `json:"by"`
}

type rawSpace struct {
	ID    any      `json:"id"` // server returns a number, Cloud a string
	Key   string   `json:"key"`
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Links rawLinks `json:"_links"`
}

type rawLinks struct {
	WebUI    string `json:"webui"`
	Base     string `json:"base"`
	Next     string `json:"next"`
	Download string `json:"download"`
}

// rawContent is the v1 "content" object: page, comment or attachment.
type rawContent struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	Status    string       `json:"status"`
	Title     string       `json:"title"`
	Space     *rawSpace    `json:"space"`
	Version   *rawVersion  `json:"version"`
	Ancestors []rawContent `json:"ancestors"`
	Body      *rawBody     `json:"body"`
	Metadata  *struct {
		MediaType string `json:"mediaType"`
	} `json:"metadata"`
	Extensions *struct {
		FileSize  int64  `json:"fileSize"`
		MediaType string `json:"mediaType"`
	} `json:"extensions"`
	Container *struct {
		ID string `json:"id"`
	} `json:"container"`
	Links rawLinks `json:"_links"`
}

type rawContentList struct {
	Results []rawContent `json:"results"`
	Size    int          `json:"size"`
	Limit   int          `json:"limit"`
	Start   int          `json:"start"`
	Links   rawLinks     `json:"_links"`
}

type rawSpaceList struct {
	Results []rawSpace `json:"results"`
	Size    int        `json:"size"`
	Limit   int        `json:"limit"`
	Links   rawLinks   `json:"_links"`
}

type rawSearchList struct {
	Results []struct {
		Content      rawContent `json:"content"`
		Title        string     `json:"title"`
		Excerpt      string     `json:"excerpt"`
		LastModified string     `json:"lastModified"`
		EntityType   string     `json:"entityType"`
	} `json:"results"`
	Size  int      `json:"size"`
	Limit int      `json:"limit"`
	Links rawLinks `json:"_links"`
}

// --- normalizers ---

func bodyOf(b *rawBody) *Body {
	if b == nil {
		return nil
	}
	switch {
	case b.Storage != nil && b.Storage.Value != "":
		return &Body{Representation: "storage", Value: b.Storage.Value}
	case b.View != nil && b.View.Value != "":
		return &Body{Representation: "view", Value: b.View.Value}
	case b.Wiki != nil && b.Wiki.Value != "":
		return &Body{Representation: "wiki", Value: b.Wiki.Value}
	}
	return nil
}

func versionOf(v *rawVersion) *Version {
	if v == nil {
		return nil
	}
	return &Version{Number: v.Number, When: v.When, By: v.By.DisplayName}
}

// mapPage normalizes a v1 content object into a Page.
func (c *apiClient) mapPage(r rawContent) *Page {
	p := &Page{
		ID:      r.ID,
		Type:    r.Type,
		Status:  r.Status,
		Title:   r.Title,
		Version: versionOf(r.Version),
		Body:    bodyOf(r.Body),
		URL:     c.absURL(r.Links.WebUI),
	}
	if r.Space != nil {
		p.SpaceKey = r.Space.Key
	}
	for _, a := range r.Ancestors {
		p.Ancestors = append(p.Ancestors, PageRef{ID: a.ID, Title: a.Title})
	}
	return p
}

// mapComment normalizes a v1 comment content object into a Comment.
func (c *apiClient) mapComment(r rawContent, pageID string) *Comment {
	cm := &Comment{
		ID:      r.ID,
		PageID:  pageID,
		Body:    bodyOf(r.Body),
		Version: versionOf(r.Version),
		URL:     c.absURL(r.Links.WebUI),
	}
	if cm.PageID == "" && r.Container != nil {
		cm.PageID = r.Container.ID
	}
	if len(r.Ancestors) > 0 {
		cm.ParentID = r.Ancestors[len(r.Ancestors)-1].ID
	}
	return cm
}

// mapAttachment normalizes a v1 attachment content object into an Attachment.
func (c *apiClient) mapAttachment(r rawContent, pageID string) *Attachment {
	a := &Attachment{
		ID:          r.ID,
		Title:       r.Title,
		PageID:      pageID,
		DownloadURL: c.absURL(r.Links.Download),
	}
	if r.Metadata != nil {
		a.MediaType = r.Metadata.MediaType
	}
	if r.Extensions != nil {
		if a.MediaType == "" {
			a.MediaType = r.Extensions.MediaType
		}
		a.FileSize = r.Extensions.FileSize
	}
	if a.PageID == "" && r.Container != nil {
		a.PageID = r.Container.ID
	}
	return a
}

// mapSpace normalizes a v1 space object into a Space.
func (c *apiClient) mapSpace(r rawSpace) *Space {
	return &Space{
		ID:   spaceIDString(r.ID),
		Key:  r.Key,
		Name: r.Name,
		Type: r.Type,
		URL:  c.absURL(r.Links.WebUI),
	}
}

// absURL resolves a relative link against the configured base URL.
func (c *apiClient) absURL(link string) string {
	if link == "" || strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}
	return c.baseURL + link
}

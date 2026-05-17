// Package urlref parses Confluence page references. A reference may be a bare
// page ID or a full Confluence URL (Cloud or Data Center, in several layouts).
package urlref

import (
	"net/url"
	"regexp"
	"strings"
)

// FlavorHint is a best-effort guess of the backend flavor derived from a URL.
type FlavorHint string

const (
	// FlavorUnknown means the reference carried no flavor signal (e.g. a bare ID).
	FlavorUnknown FlavorHint = ""
	// FlavorCloud indicates a Confluence Cloud URL.
	FlavorCloud FlavorHint = "cloud"
	// FlavorDataCenter indicates a Confluence Data Center / Server URL.
	FlavorDataCenter FlavorHint = "datacenter"
)

// Ref is a parsed Confluence reference.
type Ref struct {
	// PageID is the numeric content ID, empty if not resolvable from the input.
	PageID string
	// SpaceKey is the space key when present in the URL.
	SpaceKey string
	// Title is a human-readable title slug when present in the URL.
	Title string
	// BaseURL is the site root (scheme://host[/context]) when input was a URL.
	BaseURL string
	// Flavor is the best-effort backend flavor guess.
	Flavor FlavorHint
	// IsURL reports whether the input was a URL rather than a bare ID.
	IsURL bool
}

var (
	bareIDRe       = regexp.MustCompile(`^\d+$`)
	pagesPathRe    = regexp.MustCompile(`/pages/(\d+)(?:/([^/?#]*))?`)
	spacesPrefixRe = regexp.MustCompile(`/spaces/([^/?#]+)`)
	displayPathRe  = regexp.MustCompile(`/display/([^/?#]+)(?:/([^/?#]*))?`)
)

// Parse interprets s as either a bare page ID or a Confluence URL.
// It never errors: an unrecognised input yields a Ref with empty fields.
func Parse(s string) Ref {
	s = strings.TrimSpace(s)
	if s == "" {
		return Ref{}
	}
	if bareIDRe.MatchString(s) {
		return Ref{PageID: s}
	}
	if !strings.Contains(s, "://") {
		// Not a URL and not a bare numeric ID; treat verbatim as an ID.
		return Ref{PageID: s}
	}

	u, err := url.Parse(s)
	if err != nil {
		return Ref{}
	}
	ref := Ref{IsURL: true}
	ref.Flavor = flavorOf(u)
	ref.BaseURL = baseURLOf(u)

	path := u.Path
	// .../pages/{id}[/{title}] — used by Cloud and modern Data Center.
	if m := pagesPathRe.FindStringSubmatch(path); m != nil {
		ref.PageID = m[1]
		ref.Title = unslug(m[2])
	}
	// .../spaces/{key}/...
	if m := spacesPrefixRe.FindStringSubmatch(path); m != nil {
		ref.SpaceKey = m[1]
	}
	// Data Center: /display/{SPACE}/{Title}
	if m := displayPathRe.FindStringSubmatch(path); m != nil {
		ref.SpaceKey = m[1]
		ref.Title = unslug(m[2])
	}
	// Data Center: /pages/viewpage.action?pageId=123
	if ref.PageID == "" {
		if id := u.Query().Get("pageId"); id != "" {
			ref.PageID = id
		}
	}
	// Space overview / browse without a page id.
	if ref.SpaceKey == "" {
		if k := u.Query().Get("spaceKey"); k != "" {
			ref.SpaceKey = k
		}
	}
	return ref
}

// flavorOf guesses the flavor from host and path shape.
func flavorOf(u *url.URL) FlavorHint {
	host := strings.ToLower(u.Hostname())
	if strings.HasSuffix(host, ".atlassian.net") || strings.HasSuffix(host, ".jira.com") {
		return FlavorCloud
	}
	if strings.HasPrefix(u.Path, "/wiki/") || u.Path == "/wiki" {
		return FlavorCloud
	}
	return FlavorDataCenter
}

// baseURLOf returns the site root. For Cloud the "/wiki" context is preserved.
func baseURLOf(u *url.URL) string {
	root := u.Scheme + "://" + u.Host
	if strings.HasPrefix(u.Path, "/wiki/") || u.Path == "/wiki" {
		return root + "/wiki"
	}
	return root
}

// unslug turns a URL title slug ("My+Page" or "My%20Page") into readable text.
func unslug(s string) string {
	if s == "" {
		return ""
	}
	if dec, err := url.QueryUnescape(s); err == nil {
		s = dec
	}
	return strings.ReplaceAll(s, "+", " ")
}

package app

import (
	"context"

	"github.com/angelmsger/confluence-cli/internal/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/angelmsger/confluence-cli/internal/transport"
	"github.com/angelmsger/confluence-cli/pkg/urlref"
	"github.com/spf13/cobra"
)

// buildProbeTransport returns an unauthenticated transport used for flavor
// detection and connectivity checks.
func buildProbeTransport(s *appState) *transport.Client {
	return transport.New(transport.Options{
		Timeout:    s.timeout(),
		MaxRetries: s.cfg().Defaults.MaxRetries,
	})
}

// cmdContext returns a context bounded by the configured request timeout.
func cmdContext(s *appState) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.timeout())
}

// resolvePageID extracts a Confluence page ID from a bare ID or a page URL.
func resolvePageID(arg string) (string, error) {
	ref := urlref.Parse(arg)
	if ref.PageID == "" {
		return "", cerrors.Newf(cerrors.CategoryUsage, "NO_PAGE_ID",
			"could not resolve a page ID from %q", arg).
			WithHint("Pass a numeric page ID or a Confluence page URL.").
			WithNextSteps("confluence-cli search --text \"<keywords>\" to find the page ID.")
	}
	return ref.PageID, nil
}

// resolveCommentID extracts a comment ID from a bare ID or a comment-permalink
// URL. A plain page URL is rejected rather than silently resolved to the page
// ID — acting on the wrong content ID could delete the wrong resource.
func resolveCommentID(arg string) (string, error) {
	ref := urlref.Parse(arg)
	if ref.IsURL {
		if ref.CommentID != "" {
			return ref.CommentID, nil
		}
		return "", cerrors.Newf(cerrors.CategoryUsage, "COMMENT_URL_NO_ID",
			"%q is a page URL, not a comment reference", arg).
			WithHint("Pass the comment's own content ID, not a page URL.").
			WithNextSteps("confluence-cli comment list <page> to find comment IDs.")
	}
	if ref.PageID == "" {
		return "", cerrors.Newf(cerrors.CategoryUsage, "NO_COMMENT_ID",
			"could not resolve a comment ID from %q", arg)
	}
	return ref.PageID, nil
}

// resolveAttachmentID extracts an attachment content ID from a bare ID. A URL
// is rejected: a Confluence attachment URL does not carry the attachment's
// content ID, so resolving one would act on the wrong resource.
func resolveAttachmentID(arg string) (string, error) {
	ref := urlref.Parse(arg)
	if ref.IsURL {
		return "", cerrors.Newf(cerrors.CategoryUsage, "ATTACHMENT_URL_UNSUPPORTED",
			"%q is a URL; attachment commands need the attachment's content ID", arg).
			WithHint("Pass the attachment ID, not a URL.").
			WithNextSteps("confluence-cli attachment list <page> to find attachment IDs.")
	}
	if ref.PageID == "" {
		return "", cerrors.Newf(cerrors.CategoryUsage, "NO_ATTACHMENT_ID",
			"could not resolve an attachment ID from %q", arg)
	}
	return ref.PageID, nil
}

// addListFlags registers the shared pagination flags every list command takes.
func addListFlags(cmd *cobra.Command, limit *int, all *bool, cursor *string) {
	f := cmd.Flags()
	f.IntVar(limit, "limit", 0, "page size (default from config)")
	f.BoolVar(all, "all", false, "fetch every page of results")
	f.StringVar(cursor, "cursor", "", "start from this pagination cursor (the 'next' of a prior page)")
}

// pageInfo carries the pagination cursor for one page of a listing.
type pageInfo struct {
	Next    string
	HasMore bool
}

// listOutput is the envelope every list command emits: a page of items plus
// the cursor an agent needs to fetch the page after it.
type listOutput[T any] struct {
	Items   []T    `json:"items"`
	Next    string `json:"next,omitempty"`
	HasMore bool   `json:"has_more"`
}

// newListOutput wraps a fetched page of items into the list envelope.
func newListOutput[T any](items []T, info pageInfo) listOutput[T] {
	if items == nil {
		items = []T{}
	}
	return listOutput[T]{Items: items, Next: info.Next, HasMore: info.HasMore}
}

// collectPage fetches results for a list command. With all set it walks every
// page starting at cursor and returns the full set; otherwise it returns the
// single page at cursor plus the cursor for the page after it.
func collectPage[T any](fetch apiclient.FetchPage[T], cursor string, all bool) ([]T, pageInfo, error) {
	if all {
		var items []T
		c := cursor
		for {
			page, err := fetch(c)
			if err != nil {
				return items, pageInfo{}, err
			}
			items = append(items, page.Items...)
			if page.Next == "" {
				return items, pageInfo{}, nil
			}
			c = page.Next
		}
	}
	page, err := fetch(cursor)
	if err != nil {
		return nil, pageInfo{}, err
	}
	return page.Items, pageInfo{Next: page.Next, HasMore: page.Next != ""}, nil
}

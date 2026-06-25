package app

import (
	"testing"

	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

func TestResolvePageID(t *testing.T) {
	t.Parallel()
	id, err := resolvePageID("123456")
	if err != nil || id != "123456" {
		t.Errorf("bare id = %q, %v", id, err)
	}
	id, err = resolvePageID("https://kms.example.com/pages/viewpage.action?pageId=777")
	if err != nil || id != "777" {
		t.Errorf("page url = %q, %v", id, err)
	}
}

func TestResolveCommentID(t *testing.T) {
	t.Parallel()
	// A bare token is the comment ID.
	if id, err := resolveCommentID("c1"); err != nil || id != "c1" {
		t.Errorf("bare id = %q, %v", id, err)
	}
	// A comment permalink carries focusedCommentId — that is the target.
	id, err := resolveCommentID("https://kms.example.com/pages/viewpage.action?pageId=123&focusedCommentId=999")
	if err != nil || id != "999" {
		t.Errorf("comment url = %q, %v", id, err)
	}
	// A plain page URL must be rejected, not resolved to the page ID.
	if _, err := resolveCommentID("https://kms.example.com/pages/viewpage.action?pageId=123"); err == nil {
		t.Error("a plain page URL must not resolve to a comment ID")
	} else if cerrors.AsCLIError(err).Code != "COMMENT_URL_NO_ID" {
		t.Errorf("code = %s, want COMMENT_URL_NO_ID", cerrors.AsCLIError(err).Code)
	}
}

func TestResolveAttachmentID(t *testing.T) {
	t.Parallel()
	if id, err := resolveAttachmentID("att123"); err != nil || id != "att123" {
		t.Errorf("bare id = %q, %v", id, err)
	}
	// An attachment URL carries no attachment content ID — reject it.
	if _, err := resolveAttachmentID("https://kms.example.com/download/attachments/123/f.pdf"); err == nil {
		t.Error("an attachment URL must be rejected")
	} else if cerrors.AsCLIError(err).Code != "ATTACHMENT_URL_UNSUPPORTED" {
		t.Errorf("code = %s, want ATTACHMENT_URL_UNSUPPORTED", cerrors.AsCLIError(err).Code)
	}
}

func TestCollectPageSinglePage(t *testing.T) {
	t.Parallel()
	fetch := func(cursor string) (apiclient.ListResult[int], error) {
		return apiclient.ListResult[int]{Items: []int{1, 2}, Next: "2"}, nil
	}
	items, info, err := collectPage(fetch, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || !info.HasMore || info.Next != "2" {
		t.Errorf("items=%v info=%+v", items, info)
	}
}

func TestCollectPageLastPage(t *testing.T) {
	t.Parallel()
	fetch := func(cursor string) (apiclient.ListResult[int], error) {
		return apiclient.ListResult[int]{Items: []int{1}}, nil // no Next
	}
	_, info, err := collectPage(fetch, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if info.HasMore || info.Next != "" {
		t.Errorf("last page should not report more: %+v", info)
	}
}

func TestCollectPageAll(t *testing.T) {
	t.Parallel()
	pages := map[string]apiclient.ListResult[int]{
		"":  {Items: []int{1, 2}, Next: "2"},
		"2": {Items: []int{3, 4}, Next: "4"},
		"4": {Items: []int{5}},
	}
	calls := 0
	fetch := func(cursor string) (apiclient.ListResult[int], error) {
		calls++
		return pages[cursor], nil
	}
	items, info, err := collectPage(fetch, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 || items[0] != 1 || items[4] != 5 {
		t.Errorf("collected %v", items)
	}
	if info.HasMore {
		t.Error("--all must leave has_more false")
	}
	if calls != 3 {
		t.Errorf("expected 3 fetches, got %d", calls)
	}
}

func TestCollectPageCursorStart(t *testing.T) {
	t.Parallel()
	var gotCursor string
	fetch := func(cursor string) (apiclient.ListResult[int], error) {
		gotCursor = cursor
		return apiclient.ListResult[int]{Items: []int{9}}, nil
	}
	if _, _, err := collectPage(fetch, "50", false); err != nil {
		t.Fatal(err)
	}
	if gotCursor != "50" {
		t.Errorf("--cursor not forwarded: fetch saw %q, want 50", gotCursor)
	}
}

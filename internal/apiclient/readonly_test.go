package apiclient

import (
	"context"
	"net/http"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

// TestReadOnlyBlocksEveryMutator drives every mutating method on the wrapper
// and asserts each one returns a READONLY_BLOCKED permission error before any
// HTTP request is sent.
func TestReadOnlyBlocksEveryMutator(t *testing.T) {
	t.Parallel()
	inner, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("read-only wrapper sent an HTTP request: %s %s", r.Method, r.URL.Path)
	}))
	c := NewReadOnly(inner)
	ctx := context.Background()

	cases := []struct {
		name string
		fn   func() error
	}{
		{"CreatePage", func() error { _, err := c.CreatePage(ctx, CreatePageReq{SpaceKey: "K", Title: "T"}); return err }},
		{"UpdatePage", func() error { _, err := c.UpdatePage(ctx, UpdatePageReq{ID: "1"}); return err }},
		{"DeletePage", func() error { return c.DeletePage(ctx, DeletePageReq{ID: "1"}) }},
		{"MovePage", func() error { _, err := c.MovePage(ctx, MovePageReq{ID: "1", TargetParent: "p"}); return err }},
		{"CopyPage", func() error { _, err := c.CopyPage(ctx, CopyPageReq{SourceID: "1", Title: "T"}); return err }},
		{"RestorePage", func() error { _, err := c.RestorePage(ctx, RestorePageReq{ID: "1", Version: 2}); return err }},
		{"SetWatch", func() error { return c.SetWatch(ctx, WatchReq{PageID: "1", Watching: true}) }},
		{"AddComment", func() error { _, err := c.AddComment(ctx, AddCommentReq{PageID: "1", Body: "x"}); return err }},
		{"UpdateComment", func() error { _, err := c.UpdateComment(ctx, UpdateCommentReq{ID: "c", Body: "x"}); return err }},
		{"DeleteComment", func() error { return c.DeleteComment(ctx, DeleteCommentReq{ID: "c"}) }},
		{"UploadAttachment", func() error {
			_, err := c.UploadAttachment(ctx, UploadAttachmentReq{PageID: "1", FileName: "f", Data: []byte("d")})
			return err
		}},
		{"UpdateAttachment", func() error {
			_, err := c.UpdateAttachment(ctx, UpdateAttachmentReq{PageID: "1", AttachmentID: "a", FileName: "f", Data: []byte("d")})
			return err
		}},
		{"DeleteAttachment", func() error { return c.DeleteAttachment(ctx, DeleteAttachmentReq{AttachmentID: "a"}) }},
		{"AddLabels", func() error { _, err := c.AddLabels(ctx, AddLabelsReq{PageID: "1", Names: []string{"x"}}); return err }},
		{"RemoveLabel", func() error { return c.RemoveLabel(ctx, RemoveLabelReq{PageID: "1", Name: "x"}) }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.fn()
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tc.name)
			}
			ce := cerrors.AsCLIError(err)
			if ce.Category != cerrors.CategoryPermission {
				t.Errorf("%s: category = %s, want permission", tc.name, ce.Category)
			}
			if ce.Code != "READONLY_BLOCKED" {
				t.Errorf("%s: code = %s, want READONLY_BLOCKED", tc.name, ce.Code)
			}
			if !strings.Contains(strings.Join(ce.NextSteps, " "), "--allow-writes") {
				t.Errorf("%s: next_steps missing --allow-writes hint: %v", tc.name, ce.NextSteps)
			}
		})
	}
}

// TestReadOnlyAllowsDescribeWrite verifies that --dry-run (DescribeWrite) is
// not blocked by the wrapper, even though the underlying op is a write.
func TestReadOnlyAllowsDescribeWrite(t *testing.T) {
	t.Parallel()
	inner, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("DescribeWrite should not send HTTP: %s %s", r.Method, r.URL.Path)
	}))
	c := NewReadOnly(inner)
	plan, err := c.DescribeWrite(context.Background(), DeletePageReq{ID: "42"})
	if err != nil {
		t.Fatalf("DescribeWrite under read-only failed: %v", err)
	}
	if plan.Method != "DELETE" {
		t.Errorf("plan.Method = %q, want DELETE", plan.Method)
	}
	if !strings.Contains(plan.URL, "/content/42") {
		t.Errorf("plan.URL = %q, want substring /content/42", plan.URL)
	}
}

// TestReadOnlyAllowsReads verifies that GET methods pass through the wrapper.
func TestReadOnlyAllowsReads(t *testing.T) {
	t.Parallel()
	inner, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123","type":"page","title":"T","space":{"key":"K"}}`))
	}))
	c := NewReadOnly(inner)
	p, err := c.GetPage(context.Background(), "123", GetPageOpts{})
	if err != nil {
		t.Fatalf("GetPage under read-only failed: %v", err)
	}
	if p.ID != "123" {
		t.Errorf("page ID = %q", p.ID)
	}
}

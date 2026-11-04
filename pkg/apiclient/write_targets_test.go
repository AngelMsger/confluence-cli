package apiclient

import (
	"context"
	"net/http"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// A write that fails after it was sent may have been applied, so its error must
// never read as retryable: it says the outcome is unknown and names the read
// that shows the remote state. Before the kit's write rule a 5xx after a write
// carried the transport's retryable:true, inviting an agent to repeat it.
func TestWriteFailuresAreUnknownOutcomes(t *testing.T) {
	body := &PageBody{Value: "<p>x</p>", Format: "storage"}
	cases := []struct {
		name   string
		write  func(Client) error
		target cerrors.WriteTarget
	}{
		{"create page", func(c Client) error {
			_, err := c.CreatePage(context.Background(), CreatePageReq{SpaceKey: "ENG", Title: `Q3 "plan"`, Body: *body})
			return err
		}, newPageWriteTarget("ENG", `Q3 "plan"`)},
		{"update page", func(c Client) error {
			_, err := c.UpdatePage(context.Background(), UpdatePageReq{ID: "123", Title: "T", Body: body, ExpectVersion: 2})
			return err
		}, pageWriteTarget("123")},
		{"delete page", func(c Client) error {
			return c.DeletePage(context.Background(), DeletePageReq{ID: "123"})
		}, pageWriteTarget("123")},
		{"add comment", func(c Client) error {
			_, err := c.AddComment(context.Background(), AddCommentReq{PageID: "123", Body: "hi"})
			return err
		}, newCommentWriteTarget("123")},
		{"delete comment", func(c Client) error {
			return c.DeleteComment(context.Background(), DeleteCommentReq{ID: "9"})
		}, commentWriteTarget("9")},
		{"upload attachment", func(c Client) error {
			_, err := c.UploadAttachment(context.Background(), UploadAttachmentReq{PageID: "123", FileName: "f.txt", Data: []byte("x")})
			return err
		}, newAttachmentWriteTarget("123", "f.txt")},
		{"add labels", func(c Client) error {
			_, err := c.AddLabels(context.Background(), AddLabelsReq{PageID: "123", Names: []string{"runbook"}})
			return err
		}, labelWriteTarget("123")},
		{"watch", func(c Client) error {
			return c.SetWatch(context.Background(), WatchReq{PageID: "123", Watching: true})
		}, watchWriteTarget("123")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			ce := cerrors.AsCLIError(tc.write(c))
			if ce.Category != cerrors.CategoryServer || ce.HTTPStatus != 503 || ce.Retryable {
				t.Fatalf("got category %s, status %d, retryable %v; want a non-retryable server 503",
					ce.Category, ce.HTTPStatus, ce.Retryable)
			}
			if !strings.HasPrefix(ce.Message, "Write outcome is unknown for "+tc.target.Description) {
				t.Fatalf("message %q does not name the write target", ce.Message)
			}
			if len(ce.NextSteps) == 0 || ce.NextSteps[0] != tc.target.ReadSteps[0] {
				t.Fatalf("next steps %q, want the target's read %q", ce.NextSteps, tc.target.ReadSteps)
			}
		})
	}
}

// The read for a new page searches its space for the title; CQL and shell
// quoting must both survive a title with quotes in it.
func TestNewPageReadStepQuotesTheTitle(t *testing.T) {
	got := newPageWriteTarget("ENG", `it's "Q3"`).ReadSteps[0]
	want := `confluence-cli search 'type = page AND space = "ENG" AND title = "it'"'"'s \"Q3\""' --limit 25`
	if got != want {
		t.Fatalf("read step\n got %s\nwant %s", got, want)
	}
}

// A definite refusal shows nothing was written, so it keeps its own code and
// guidance: a version conflict still says to merge and retry.
func TestWriteRefusalPassesThrough(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	_, err := c.UpdatePage(context.Background(), UpdatePageReq{ID: "123", Title: "T",
		Body: &PageBody{Value: "x"}, ExpectVersion: 2})
	if ce := cerrors.AsCLIError(err); ce.Code != "PAGE_VERSION_CONFLICT" || strings.Contains(ce.Message, "Write outcome") {
		t.Fatalf("got %s %q; want the conflict unchanged", ce.Code, ce.Message)
	}
}

// Only a 2xx acknowledges a request. A redirect the HTTP client does not
// follow used to read as success.
func TestWriteRedirectIsNotSuccess(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	err := c.SetWatch(context.Background(), WatchReq{PageID: "123", Watching: true})
	if ce := cerrors.AsCLIError(err); err == nil || ce.Retryable || !strings.HasPrefix(ce.Message, "Write outcome is unknown") {
		t.Fatalf("got %v; want an unknown write outcome", err)
	}
}

// The server acknowledged these writes, so their errors say the write
// happened and must not invite a second one.
func TestWriteAcknowledgedButUnusable(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"undecodable", `{"id":`},
		{"no id", `{"type":"page","title":"T"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))
			_, err := c.CreatePage(context.Background(), CreatePageReq{SpaceKey: "ENG", Title: "T"})
			ce := cerrors.AsCLIError(err)
			if ce.Code != cerrors.CodeWriteSucceededResponseInvalid || ce.Retryable ||
				!strings.HasPrefix(ce.Message, `Write succeeded for new page "T" in space ENG`) {
				t.Fatalf("got %s retryable=%v %q; want %s", ce.Code, ce.Retryable, ce.Message,
					cerrors.CodeWriteSucceededResponseInvalid)
			}
		})
	}
}

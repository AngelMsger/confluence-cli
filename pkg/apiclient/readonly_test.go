package apiclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// assertCoversMethods fails unless names is exactly the method set of iface,
// so a table test cannot silently skip a method added to the interface.
func assertCoversMethods(t *testing.T, iface reflect.Type, names []string) {
	t.Helper()
	var want []string
	for i := range iface.NumMethod() {
		want = append(want, iface.Method(i).Name)
	}
	got := slices.Sorted(slices.Values(names))
	if !slices.Equal(got, want) {
		t.Fatalf("table covers %v, %s declares %v", got, iface.Name(), want)
	}
}

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
	var names []string
	for _, tc := range cases {
		names = append(names, tc.name)
	}
	assertCoversMethods(t, reflect.TypeFor[Writer](), names)

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
			if !strings.Contains(ce.Message, `"`+tc.name+`"`) {
				t.Errorf("%s: message %q does not name the operation", tc.name, ce.Message)
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

// readCall is one call a recordingReader received.
type readCall struct {
	method string
	args   []any
}

// errCannedRead is returned alongside every canned read result, so delegation
// tests also see errors pass through unchanged.
var errCannedRead = errors.New("canned read error")

// recordingReader answers every Reader method with a fixed non-zero result and
// records the call. It embeds Client only to satisfy NewReadOnly's parameter
// type: a write that reached it would panic on the nil embedded interface.
type recordingReader struct {
	Client
	calls []readCall
}

func (r *recordingReader) record(method string, args ...any) {
	r.calls = append(r.calls, readCall{method: method, args: args})
}

func (r *recordingReader) Flavor() Flavor {
	r.record("Flavor")
	return FlavorCloud
}

func (r *recordingReader) BaseURL() string {
	r.record("BaseURL")
	return "https://confluence.example.test"
}

func (r *recordingReader) Ping(ctx context.Context) (ServerInfo, error) {
	r.record("Ping", ctx)
	return ServerInfo{Flavor: FlavorCloud, Reachable: true}, errCannedRead
}

func (r *recordingReader) GetPage(ctx context.Context, id string, opt GetPageOpts) (*Page, error) {
	r.record("GetPage", ctx, id, opt)
	return &Page{ID: "1", Title: "Home"}, errCannedRead
}

func (r *recordingReader) ListChildren(ctx context.Context, id string, opt ListOpts) (ListResult[Page], error) {
	r.record("ListChildren", ctx, id, opt)
	return ListResult[Page]{Items: []Page{{ID: "2"}}, Next: "c2"}, errCannedRead
}

func (r *recordingReader) ListDescendants(ctx context.Context, id string, opt ListOpts) (ListResult[Page], error) {
	r.record("ListDescendants", ctx, id, opt)
	return ListResult[Page]{Items: []Page{{ID: "3"}}}, errCannedRead
}

func (r *recordingReader) ListPageVersions(ctx context.Context, id string, opt ListOpts) (ListResult[PageVersion], error) {
	r.record("ListPageVersions", ctx, id, opt)
	return ListResult[PageVersion]{Items: []PageVersion{{Number: 4}}}, errCannedRead
}

func (r *recordingReader) WatchStatus(ctx context.Context, pageID string) (bool, error) {
	r.record("WatchStatus", ctx, pageID)
	return true, errCannedRead
}

func (r *recordingReader) Search(ctx context.Context, cql string, opt ListOpts) (ListResult[SearchHit], error) {
	r.record("Search", ctx, cql, opt)
	return ListResult[SearchHit]{Items: []SearchHit{{ID: "5", Title: "Hit"}}}, errCannedRead
}

func (r *recordingReader) ListSpaces(ctx context.Context, opt SpaceListOpts) (ListResult[Space], error) {
	r.record("ListSpaces", ctx, opt)
	return ListResult[Space]{Items: []Space{{Key: "ENG"}}}, errCannedRead
}

func (r *recordingReader) GetSpace(ctx context.Context, key string) (*Space, error) {
	r.record("GetSpace", ctx, key)
	return &Space{Key: "ENG", Name: "Engineering"}, errCannedRead
}

func (r *recordingReader) ListComments(ctx context.Context, pageID string, opt ListOpts) (ListResult[Comment], error) {
	r.record("ListComments", ctx, pageID, opt)
	return ListResult[Comment]{Items: []Comment{{ID: "6"}}}, errCannedRead
}

func (r *recordingReader) CurrentUser(ctx context.Context) (*User, error) {
	r.record("CurrentUser", ctx)
	return &User{AccountID: "me"}, errCannedRead
}

func (r *recordingReader) SearchUsers(ctx context.Context, opt UserSearchOpts) (ListResult[User], error) {
	r.record("SearchUsers", ctx, opt)
	return ListResult[User]{Items: []User{{Username: "alice"}}}, errCannedRead
}

func (r *recordingReader) GetUser(ctx context.Context, selector string) (*User, error) {
	r.record("GetUser", ctx, selector)
	return &User{Username: "bob"}, errCannedRead
}

func (r *recordingReader) ListAttachments(ctx context.Context, pageID string, opt ListOpts) (ListResult[Attachment], error) {
	r.record("ListAttachments", ctx, pageID, opt)
	return ListResult[Attachment]{Items: []Attachment{{ID: "7"}}}, errCannedRead
}

func (r *recordingReader) GetAttachment(ctx context.Context, id string) (*Attachment, error) {
	r.record("GetAttachment", ctx, id)
	return &Attachment{ID: "8", Title: "a.txt"}, errCannedRead
}

func (r *recordingReader) DownloadAttachment(ctx context.Context, att Attachment, w io.Writer) (DownloadMeta, error) {
	r.record("DownloadAttachment", ctx, att, w)
	return DownloadMeta{ContentType: "text/plain", Bytes: 9}, errCannedRead
}

func (r *recordingReader) ListLabels(ctx context.Context, pageID string, opt ListOpts) (ListResult[Label], error) {
	r.record("ListLabels", ctx, pageID, opt)
	return ListResult[Label]{Items: []Label{{Name: "runbook"}}}, errCannedRead
}

func (r *recordingReader) DescribeWrite(ctx context.Context, op any) (WriteRequestPlan, error) {
	r.record("DescribeWrite", ctx, op)
	return WriteRequestPlan{Method: "DELETE", URL: "https://confluence.example.test/rest/api/content/1"}, errCannedRead
}

type delegationKey struct{}

// TestReadOnlyDelegatesEveryRead calls every Reader method through the
// wrapper and asserts the inner client received exactly that one call with the
// same arguments, and that its results and error came back unchanged.
func TestReadOnlyDelegatesEveryRead(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), delegationKey{}, "read-only delegation")
	page := ListOpts{Limit: 7, Cursor: "c1"}

	// argsFor builds fresh arguments on every call, so the expected values are
	// independent of the ones handed to the wrapper: a wrapper that mutated an
	// argument in place before forwarding it would otherwise change the
	// expectation along with it.
	argsFor := func() map[string][]any {
		return map[string][]any{
			"Flavor":             nil,
			"BaseURL":            nil,
			"Ping":               {ctx},
			"GetPage":            {ctx, "1", GetPageOpts{BodyFormat: "view", WithBody: true}},
			"ListChildren":       {ctx, "1", page},
			"ListDescendants":    {ctx, "1", page},
			"ListPageVersions":   {ctx, "1", page},
			"WatchStatus":        {ctx, "1"},
			"Search":             {ctx, "type = page", page},
			"ListSpaces":         {ctx, SpaceListOpts{ListOpts: page, Type: "global"}},
			"GetSpace":           {ctx, "ENG"},
			"ListComments":       {ctx, "1", page},
			"CurrentUser":        {ctx},
			"SearchUsers":        {ctx, UserSearchOpts{ListOpts: page, Query: "ali"}},
			"GetUser":            {ctx, "bob"},
			"ListAttachments":    {ctx, "1", page},
			"GetAttachment":      {ctx, "8"},
			"DownloadAttachment": {ctx, Attachment{ID: "8", Title: "a.txt"}, &bytes.Buffer{}},
			"ListLabels":         {ctx, "1", page},
			"DescribeWrite":      {ctx, DeletePageReq{ID: "1"}},
		}
	}
	assertCoversMethods(t, reflect.TypeFor[Reader](), slices.Collect(maps.Keys(argsFor())))

	call := func(c Client, method string, args []any) []any {
		in := make([]reflect.Value, len(args))
		for i, a := range args {
			in[i] = reflect.ValueOf(a)
		}
		var out []any
		for _, v := range reflect.ValueOf(c).MethodByName(method).Call(in) {
			out = append(out, v.Interface())
		}
		return out
	}

	for method := range argsFor() {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			inner := &recordingReader{}
			got := call(NewReadOnly(inner), method, argsFor()[method])
			expected := argsFor()[method]
			if want := call(&recordingReader{}, method, expected); !reflect.DeepEqual(got, want) {
				t.Errorf("results = %#v, want the inner client's %#v", got, want)
			}
			want := []readCall{{method: method, args: expected}}
			if !reflect.DeepEqual(inner.calls, want) {
				t.Errorf("inner client received %#v, want %#v", inner.calls, want)
			}
		})
	}
}

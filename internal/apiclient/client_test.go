package apiclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/angelmsger/confluence-cli/internal/transport"
)

// newTestClient builds a Data Center flavored client pointed at handler.
func newTestClient(t *testing.T, handler http.Handler) (Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(Config{
		Flavor:    FlavorDataCenter,
		BaseURL:   srv.URL,
		Transport: transport.New(transport.Options{}),
	})
	return c, srv
}

func TestGetPage(t *testing.T) {
	t.Parallel()
	var gotPath, gotQuery string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id":"123","type":"page","status":"current","title":"Design Doc",
			"space":{"key":"ENG","name":"Engineering"},
			"version":{"number":3,"when":"2025-01-02T00:00:00Z","by":{"displayName":"Alice"}},
			"ancestors":[{"id":"1","title":"Root"}],
			"body":{"storage":{"value":"<p>hi</p>","representation":"storage"}},
			"_links":{"webui":"/display/ENG/Design+Doc"}}`))
	}))

	page, err := c.GetPage(context.Background(), "123", GetPageOpts{WithBody: true})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/rest/api/content/123" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotQuery, "body.storage") {
		t.Errorf("query missing body.storage: %q", gotQuery)
	}
	if page.Title != "Design Doc" || page.SpaceKey != "ENG" {
		t.Errorf("page = %+v", page)
	}
	if page.Version == nil || page.Version.Number != 3 || page.Version.By != "Alice" {
		t.Errorf("version = %+v", page.Version)
	}
	if page.Body == nil || page.Body.Value != "<p>hi</p>" {
		t.Errorf("body = %+v", page.Body)
	}
	if len(page.Ancestors) != 1 || page.Ancestors[0].ID != "1" {
		t.Errorf("ancestors = %+v", page.Ancestors)
	}
}

func TestListChildrenPagination(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")
		w.Header().Set("Content-Type", "application/json")
		if start == "0" {
			w.Write([]byte(`{"results":[
				{"id":"a","type":"page","title":"A"},
				{"id":"b","type":"page","title":"B"}],"size":2,"limit":2}`))
		} else {
			w.Write([]byte(`{"results":[{"id":"c","type":"page","title":"C"}],"size":1,"limit":2}`))
		}
	}))

	all, err := CollectAll(func(cursor string) (ListResult[Page], error) {
		return c.ListChildren(context.Background(), "root", ListOpts{Limit: 2, Cursor: cursor})
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("collected %d pages, want 3", len(all))
	}
	if all[0].ID != "a" || all[2].ID != "c" {
		t.Errorf("unexpected order: %+v", all)
	}
}

func TestSearch(t *testing.T) {
	t.Parallel()
	var gotCQL string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCQL = r.URL.Query().Get("cql")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{
			"content":{"id":"42","type":"page","title":"Hit","space":{"key":"ENG"},
			           "_links":{"webui":"/display/ENG/Hit"}},
			"title":"Hit","excerpt":"an excerpt","lastModified":"2025-03-01T00:00:00Z"}],
			"size":1,"limit":25}`))
	}))

	res, err := c.Search(context.Background(), `text ~ "x"`, ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if gotCQL != `text ~ "x"` {
		t.Errorf("cql = %q", gotCQL)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "42" || res.Items[0].Excerpt != "an excerpt" {
		t.Errorf("hits = %+v", res.Items)
	}
	if res.Items[0].SpaceKey != "ENG" {
		t.Errorf("space key = %q", res.Items[0].SpaceKey)
	}
}

func TestListAndGetSpace(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/space/ENG") {
			w.Write([]byte(`{"id":100,"key":"ENG","name":"Engineering","type":"global"}`))
			return
		}
		w.Write([]byte(`{"results":[{"id":100,"key":"ENG","name":"Engineering","type":"global"}],"size":1,"limit":25}`))
	}))

	list, err := c.ListSpaces(context.Background(), SpaceListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Key != "ENG" || list.Items[0].ID != "100" {
		t.Errorf("spaces = %+v", list.Items)
	}
	sp, err := c.GetSpace(context.Background(), "ENG")
	if err != nil {
		t.Fatal(err)
	}
	if sp.Name != "Engineering" {
		t.Errorf("space = %+v", sp)
	}
}

func TestListComments(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"id":"c1","type":"comment",
			"body":{"storage":{"value":"<p>nice</p>","representation":"storage"}},
			"version":{"number":1}}],"size":1,"limit":25}`))
	}))
	res, err := c.ListComments(context.Background(), "123", ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "c1" || res.Items[0].PageID != "123" {
		t.Errorf("comments = %+v", res.Items)
	}
}

func TestAddComment(t *testing.T) {
	t.Parallel()
	var gotMethod string
	var gotBody []byte
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"new1","type":"comment"}`))
	}))
	cm, err := c.AddComment(context.Background(), AddCommentReq{
		PageID: "123", Body: "great page", Format: "storage",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s", gotMethod)
	}
	if !strings.Contains(string(gotBody), `"great page"`) {
		t.Errorf("request body = %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"type":"comment"`) {
		t.Errorf("request body missing comment type: %s", gotBody)
	}
	if cm.ID != "new1" {
		t.Errorf("comment = %+v", cm)
	}
}

func TestAddCommentValidation(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if _, err := c.AddComment(context.Background(), AddCommentReq{Body: "x"}); err == nil {
		t.Error("expected error for missing page ID")
	}
	if _, err := c.AddComment(context.Background(), AddCommentReq{PageID: "1"}); err == nil {
		t.Error("expected error for empty body")
	}
}

func TestAttachmentsListAndDownload(t *testing.T) {
	t.Parallel()
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/child/attachment"):
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"results":[{"id":"att1","type":"attachment","title":"file.pdf",
				"metadata":{"mediaType":"application/pdf"},
				"extensions":{"fileSize":2048},
				"_links":{"download":"/download/attachments/123/file.pdf"}}],"size":1,"limit":25}`))
		case strings.HasPrefix(r.URL.Path, "/download/"):
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("PDFDATA"))
		}
	}))

	res, err := c.ListAttachments(context.Background(), "123", ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].FileSize != 2048 {
		t.Fatalf("attachments = %+v", res.Items)
	}
	att := res.Items[0]
	if !strings.HasPrefix(att.DownloadURL, srv.URL) {
		t.Errorf("download url not absolute: %q", att.DownloadURL)
	}
	var buf strings.Builder
	meta, err := c.DownloadAttachment(context.Background(), att, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "PDFDATA" || meta.Bytes != 7 {
		t.Errorf("download = %q meta = %+v", buf.String(), meta)
	}
}

func TestHTTPErrorClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status int
		cat    cerrors.Category
	}{
		{401, cerrors.CategoryAuth},
		{403, cerrors.CategoryPermission},
		{404, cerrors.CategoryNotFound},
		{500, cerrors.CategoryServer},
	}
	for _, tc := range tests {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			t.Parallel()
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(`{"message":"boom"}`))
			}))
			_, err := c.GetPage(context.Background(), "x", GetPageOpts{})
			if err == nil {
				t.Fatal("expected error")
			}
			ce := cerrors.AsCLIError(err)
			if ce.Category != tc.cat {
				t.Errorf("category = %s, want %s", ce.Category, tc.cat)
			}
			if !strings.Contains(ce.Message, "boom") {
				t.Errorf("message should include server detail: %q", ce.Message)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()
	dc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rest/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"results":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dc.Close()

	flavor, err := Detect(context.Background(), transport.New(transport.Options{}), dc.URL)
	if err != nil {
		t.Fatal(err)
	}
	if flavor != FlavorDataCenter {
		t.Errorf("flavor = %s, want datacenter", flavor)
	}
}

// TestDetectAtlassianNetShortcut: any `*.atlassian.net` host is Cloud without
// a network call, even when the URL is incomplete or has typos in the path —
// the host suffix is the discriminator Atlassian reserves for tenants.
func TestDetectAtlassianNetShortcut(t *testing.T) {
	t.Parallel()
	cases := []string{
		"https://angelmsger.atlassian.net/wiki",
		"https://angelmsger.atlassian.net/wiki/",
		"https://angelmsger.atlassian.net",
		"angelmsger.atlassian.net/wiki",
		"HTTPS://ANGELMSGER.ATLASSIAN.NET/WiKi",
	}
	for _, in := range cases {
		in := in
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			// Use an unreachable transport to prove no network call is made.
			f, err := Detect(context.Background(),
				transport.New(transport.Options{}), in)
			if err != nil {
				t.Fatalf("Detect(%q): %v", in, err)
			}
			if f != FlavorCloud {
				t.Errorf("flavor = %s, want cloud", f)
			}
		})
	}
}

// TestDetectCloudViaTenantInfo covers a custom-domain Cloud tenant — the
// host suffix doesn't match, but `_edge/tenant_info` does. This is also the
// path that fixes the original bug: a tenant whose `/wiki/api/v2/...`
// returns a 302 to login (which the probe rejects as HTML) is still picked
// up by the tenant_info sentinel.
func TestDetectCloudViaTenantInfo(t *testing.T) {
	t.Parallel()
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_edge/tenant_info":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"cloudId":"abc","baseUrl":"https://wiki.example.com"}`))
		case "/wiki/api/v2/spaces":
			// Simulate the real-world 302-to-SSO that defeated the old probe.
			http.Redirect(w, r, "https://id.atlassian.com/login", http.StatusFound)
		case "/rest/api/space":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cloud.Close()

	f, err := Detect(context.Background(),
		transport.New(transport.Options{}), cloud.URL+"/wiki")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if f != FlavorCloud {
		t.Errorf("flavor = %s, want cloud", f)
	}
}

func TestIsAtlassianCloudHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"https://acme.atlassian.net", true},
		{"https://acme.atlassian.net/wiki", true},
		{"https://Acme.Atlassian.Net/wiki", true},
		{"acme.atlassian.net/wiki", true},
		{"https://wiki.example.com", false},
		{"https://kms.fineres.com/", false},
		{"https://attacker.atlassian.net.evil.example.com/", false},
		{"", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := isAtlassianCloudHost(tc.in); got != tc.want {
				t.Errorf("isAtlassianCloudHost(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func readAll(r *http.Request) []byte {
	b, _ := io.ReadAll(r.Body)
	return b
}

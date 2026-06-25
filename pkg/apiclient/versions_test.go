package apiclient

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestListPageVersions(t *testing.T) {
	t.Parallel()
	var gotPath string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[
			{"number":3,"when":"2025-03-01T00:00:00Z","message":"latest","minorEdit":false,
			 "by":{"displayName":"Alice"}},
			{"number":2,"when":"2025-02-01T00:00:00Z","by":{"displayName":"Bob"}}],
			"size":2,"limit":25}`))
	}))

	res, err := c.ListPageVersions(context.Background(), "123", ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/rest/api/content/123/version" {
		t.Errorf("path = %q", gotPath)
	}
	if len(res.Items) != 2 {
		t.Fatalf("versions = %+v", res.Items)
	}
	if res.Items[0].Number != 3 || res.Items[0].By != "Alice" || res.Items[0].Message != "latest" {
		t.Errorf("version[0] = %+v", res.Items[0])
	}
}

// restoreServer is a Confluence stand-in covering the three GETs and one PUT a
// restore performs.
func restoreServer(t *testing.T, putBody *[]byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/version/2"):
			w.Write([]byte(`{"number":2,"content":{"id":"123","type":"page","title":"Doc",
				"body":{"storage":{"value":"<p>v2 body</p>","representation":"storage"}}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/content/123":
			w.Write([]byte(`{"id":"123","type":"page","title":"Doc","version":{"number":7}}`))
		case r.Method == http.MethodPut:
			*putBody = readAll(r)
			w.Write([]byte(`{"id":"123","type":"page","title":"Doc","version":{"number":8}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestRestorePage(t *testing.T) {
	t.Parallel()
	var putBody []byte
	c, _ := newTestClient(t, restoreServer(t, &putBody))

	page, err := c.RestorePage(context.Background(), RestorePageReq{ID: "123", Version: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.Version == nil || page.Version.Number != 8 {
		t.Errorf("restored page = %+v", page)
	}
	body := string(putBody)
	// The restore must PUT the v2 body, bump the version to 8 and carry a
	// default restore message.
	for _, want := range []string{`v2 body`, `"number":8`, `Restored to version 2`} {
		if !strings.Contains(body, want) {
			t.Errorf("restore PUT missing %q: %s", want, body)
		}
	}
}

func TestRestorePageValidation(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if _, err := c.RestorePage(context.Background(), RestorePageReq{Version: 1}); err == nil {
		t.Error("expected error for missing page ID")
	}
	if _, err := c.RestorePage(context.Background(), RestorePageReq{ID: "1"}); err == nil {
		t.Error("expected error for missing version")
	}
}

func TestDescribeWriteRestore(t *testing.T) {
	t.Parallel()
	var putBody []byte
	c, srv := newTestClient(t, restoreServer(t, &putBody))

	plan, err := c.DescribeWrite(context.Background(), RestorePageReq{ID: "123", Version: 2})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != http.MethodPut || plan.URL != srv.URL+"/rest/api/content/123" {
		t.Errorf("plan = %s %s", plan.Method, plan.URL)
	}
	if putBody != nil {
		t.Error("dry-run must not send the PUT")
	}
	if plan.Payload == nil {
		t.Error("payload should be populated")
	}
}

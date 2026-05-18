package apiclient

import (
	"context"
	"net/http"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

func TestCreatePage(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	var gotBody []byte
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotBody = r.Method, r.URL.Path, readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"900","type":"page","title":"New Page",
			"space":{"key":"ENG"},"version":{"number":1}}`))
	}))

	page, err := c.CreatePage(context.Background(), CreatePageReq{
		SpaceKey: "ENG", Title: "New Page", ParentID: "100",
		Body: PageBody{Value: "<p>hi</p>", Format: "storage"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/api/content" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	body := string(gotBody)
	for _, want := range []string{`"type":"page"`, `"New Page"`, `"key":"ENG"`,
		`"id":"100"`, `hi`, `"representation":"storage"`} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q: %s", want, body)
		}
	}
	if page.ID != "900" {
		t.Errorf("page = %+v", page)
	}
}

func TestCreatePageValidation(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if _, err := c.CreatePage(context.Background(), CreatePageReq{Title: "x"}); err == nil {
		t.Error("expected error for missing space")
	}
	if _, err := c.CreatePage(context.Background(), CreatePageReq{SpaceKey: "ENG"}); err == nil {
		t.Error("expected error for missing title")
	}
}

func TestUpdatePageExplicitVersion(t *testing.T) {
	t.Parallel()
	var gotMethod string
	var gotBody []byte
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotBody = r.Method, readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123","type":"page","title":"T","version":{"number":6}}`))
	}))

	body := PageBody{Value: "<p>new</p>", Format: "storage"}
	_, err := c.UpdatePage(context.Background(), UpdatePageReq{
		ID: "123", Title: "T", Body: &body, ExpectVersion: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s", gotMethod)
	}
	// Explicit version 5 must yield version.number 6 with no GET round-trip.
	if !strings.Contains(string(gotBody), `"number":6`) {
		t.Errorf("request body = %s", gotBody)
	}
}

func TestUpdatePageAutoVersion(t *testing.T) {
	t.Parallel()
	var putBody []byte
	gets := 0
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			gets++
			w.Write([]byte(`{"id":"123","type":"page","title":"Current",
				"version":{"number":9},
				"body":{"storage":{"value":"<p>old</p>","representation":"storage"}}}`))
			return
		}
		putBody = readAll(r)
		w.Write([]byte(`{"id":"123","type":"page","title":"Current","version":{"number":10}}`))
	}))

	// Only a title is given: the client must GET to learn the version and body.
	_, err := c.UpdatePage(context.Background(), UpdatePageReq{ID: "123", Title: "Renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if gets != 1 {
		t.Errorf("expected 1 GET, got %d", gets)
	}
	body := string(putBody)
	if !strings.Contains(body, `"number":10`) {
		t.Errorf("version not incremented from current: %s", body)
	}
	if !strings.Contains(body, `old`) {
		t.Errorf("existing body not carried over: %s", body)
	}
}

func TestUpdatePageConflict(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			http.Error(w, `{"message":"version conflict"}`, http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123","type":"page","title":"T","version":{"number":2}}`))
	}))

	_, err := c.UpdatePage(context.Background(), UpdatePageReq{ID: "123", Title: "T"})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	ce := cerrors.AsCLIError(err)
	if ce.Category != cerrors.CategoryConflict {
		t.Errorf("category = %s, want conflict", ce.Category)
	}
	if ce.Code != "PAGE_VERSION_CONFLICT" {
		t.Errorf("code = %s", ce.Code)
	}
	if cerrors.ExitCode(err) != cerrors.ExitConflict {
		t.Errorf("exit code = %d, want %d", cerrors.ExitCode(err), cerrors.ExitConflict)
	}
}

func TestDeletePage(t *testing.T) {
	t.Parallel()
	var calls []string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.RequestURI())
		w.WriteHeader(http.StatusNoContent)
	}))

	if err := c.DeletePage(context.Background(), DeletePageReq{ID: "123"}); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "DELETE /rest/api/content/123") {
		t.Errorf("trash call = %v", calls)
	}

	calls = nil
	if err := c.DeletePage(context.Background(), DeletePageReq{ID: "123", Purge: true}); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || !strings.Contains(calls[1], "status=trashed") {
		t.Errorf("purge calls = %v", calls)
	}
}

func TestMovePage(t *testing.T) {
	t.Parallel()
	var putBody []byte
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"id":"123","type":"page","title":"Movable",
				"version":{"number":4},
				"body":{"storage":{"value":"<p>keep</p>","representation":"storage"}}}`))
			return
		}
		putBody = readAll(r)
		w.Write([]byte(`{"id":"123","type":"page","title":"Movable","version":{"number":5}}`))
	}))

	_, err := c.MovePage(context.Background(), MovePageReq{ID: "123", TargetParent: "777"})
	if err != nil {
		t.Fatal(err)
	}
	body := string(putBody)
	for _, want := range []string{`"id":"777"`, `"number":5`, `keep`} {
		if !strings.Contains(body, want) {
			t.Errorf("move request missing %q: %s", want, body)
		}
	}
}

func TestCopyPage(t *testing.T) {
	t.Parallel()
	var postBody []byte
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"id":"123","type":"page","title":"Source",
				"space":{"key":"ENG"},"ancestors":[{"id":"50","title":"Root"}],
				"body":{"storage":{"value":"<p>copy me</p>","representation":"storage"}}}`))
			return
		}
		postBody = readAll(r)
		w.Write([]byte(`{"id":"950","type":"page","title":"Duplicate"}`))
	}))

	page, err := c.CopyPage(context.Background(), CopyPageReq{SourceID: "123", Title: "Duplicate"})
	if err != nil {
		t.Fatal(err)
	}
	if page.ID != "950" {
		t.Errorf("page = %+v", page)
	}
	body := string(postBody)
	for _, want := range []string{`"Duplicate"`, `"key":"ENG"`, `"id":"50"`, `copy me`} {
		if !strings.Contains(body, want) {
			t.Errorf("copy request missing %q: %s", want, body)
		}
	}
}

func TestDescribeWriteSendsNoWrite(t *testing.T) {
	t.Parallel()
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("dry-run must not send %s", r.Method)
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	plan, err := c.DescribeWrite(context.Background(), CreatePageReq{
		SpaceKey: "ENG", Title: "Planned", Body: PageBody{Value: "<p>x</p>", Format: "storage"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != http.MethodPost {
		t.Errorf("method = %s", plan.Method)
	}
	if plan.URL != srv.URL+"/rest/api/content" {
		t.Errorf("url = %s", plan.URL)
	}
	if plan.Payload == nil {
		t.Error("payload should be populated")
	}
}

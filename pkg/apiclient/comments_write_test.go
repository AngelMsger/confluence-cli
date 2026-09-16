package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/angelmsger/confluence-cli/pkg/transport"
)

func TestAddCommentPreviewMatchesWrite(t *testing.T) {
	t.Parallel()
	for _, flavor := range []Flavor{FlavorCloud, FlavorDataCenter} {
		for _, format := range []string{"storage", "wiki"} {
			t.Run(string(flavor)+"/"+format, func(t *testing.T) {
				t.Parallel()
				calls := 0
				var method, path, body string
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					method, path, body = r.Method, r.URL.Path, string(readAll(r))
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"id":"c2","type":"comment","version":{"number":1}}`))
				}))
				t.Cleanup(srv.Close)
				c := New(Config{Flavor: flavor, BaseURL: srv.URL, Transport: transport.New(transport.Options{})})
				req := AddCommentReq{PageID: "123", ParentID: "c1", Body: "A & B\nsecond line", Format: format}
				plan, err := NewReadOnly(c).DescribeWrite(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				if calls != 0 {
					t.Fatalf("preview sent %d requests", calls)
				}
				wantPath := "/rest/api/content"
				if flavor == FlavorCloud {
					wantPath = "/wiki" + wantPath
				}
				if plan.Method != http.MethodPost || plan.URL != srv.URL+wantPath {
					t.Fatalf("plan = %+v", plan)
				}
				preview, err := json.Marshal(plan.Payload)
				if err != nil {
					t.Fatal(err)
				}
				var payload commentRequest
				if err := json.Unmarshal(preview, &payload); err != nil {
					t.Fatal(err)
				}
				if payload.Type != "comment" || payload.Container.ID != "123" || payload.Container.Type != "page" ||
					len(payload.Ancestors) != 1 || payload.Ancestors[0].ID != "c1" ||
					len(payload.Body) != 1 || payload.Body[format].Value != req.Body || payload.Body[format].Representation != format {
					t.Fatalf("preview lost comment fields: %s", preview)
				}
				created, err := c.AddComment(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				if calls != 1 || method != plan.Method || path != wantPath || body != string(preview) || created.ID != "c2" {
					t.Fatalf("live write differs: calls=%d method=%s path=%s body=%s result=%+v", calls, method, path, body, created)
				}
			})
		}
	}
}

func TestAddCommentPreviewValidation(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid comment sent an HTTP request")
	}))
	for _, tc := range []struct {
		req  AddCommentReq
		code string
	}{
		{AddCommentReq{Body: "text"}, "COMMENT_NO_PAGE"},
		{AddCommentReq{PageID: "123"}, "COMMENT_NO_BODY"},
	} {
		_, previewErr := c.DescribeWrite(context.Background(), tc.req)
		_, writeErr := c.AddComment(context.Background(), tc.req)
		for _, err := range []error{previewErr, writeErr} {
			if err == nil || cerrors.AsCLIError(err).Code != tc.code {
				t.Errorf("error = %v, want %s", err, tc.code)
			}
		}
	}
}

func TestUpdateCommentAutoVersion(t *testing.T) {
	t.Parallel()
	var putBody []byte
	gets := 0
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			gets++
			w.Write([]byte(`{"id":"c1","type":"comment","version":{"number":4}}`))
			return
		}
		putBody = readAll(r)
		w.Write([]byte(`{"id":"c1","type":"comment","version":{"number":5}}`))
	}))

	cm, err := c.UpdateComment(context.Background(), UpdateCommentReq{ID: "c1", Body: "edited"})
	if err != nil {
		t.Fatal(err)
	}
	if gets != 1 {
		t.Errorf("expected 1 GET to learn the version, got %d", gets)
	}
	body := string(putBody)
	for _, want := range []string{`"type":"comment"`, `edited`, `"number":5`} {
		if !strings.Contains(body, want) {
			t.Errorf("update PUT missing %q: %s", want, body)
		}
	}
	if cm.ID != "c1" {
		t.Errorf("comment = %+v", cm)
	}
}

func TestUpdateCommentExplicitVersion(t *testing.T) {
	t.Parallel()
	var putBody []byte
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			t.Error("explicit version must not trigger a GET")
		}
		putBody = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"c1","type":"comment","version":{"number":8}}`))
	}))

	_, err := c.UpdateComment(context.Background(), UpdateCommentReq{
		ID: "c1", Body: "edited", ExpectVersion: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(putBody), `"number":8`) {
		t.Errorf("explicit version 7 must yield number 8: %s", putBody)
	}
}

func TestDeleteComment(t *testing.T) {
	t.Parallel()
	var got string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := c.DeleteComment(context.Background(), DeleteCommentReq{ID: "c1"}); err != nil {
		t.Fatal(err)
	}
	if got != "DELETE /rest/api/content/c1" {
		t.Errorf("request = %q", got)
	}
}

func TestCommentWriteValidation(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if _, err := c.UpdateComment(context.Background(), UpdateCommentReq{Body: "x"}); err == nil {
		t.Error("expected error for missing comment ID")
	}
	if _, err := c.UpdateComment(context.Background(), UpdateCommentReq{ID: "c1"}); err == nil {
		t.Error("expected error for empty body")
	}
	if err := c.DeleteComment(context.Background(), DeleteCommentReq{}); err == nil {
		t.Error("expected error for missing comment ID")
	}
}

func TestDescribeWriteComment(t *testing.T) {
	t.Parallel()
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("dry-run must not send %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"c1","type":"comment","version":{"number":2}}`))
	}))

	plan, err := c.DescribeWrite(context.Background(), UpdateCommentReq{ID: "c1", Body: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != http.MethodPut || plan.URL != srv.URL+"/rest/api/content/c1" {
		t.Errorf("plan = %s %s", plan.Method, plan.URL)
	}

	del, err := c.DescribeWrite(context.Background(), DeleteCommentReq{ID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if del.Method != http.MethodDelete {
		t.Errorf("delete plan method = %s", del.Method)
	}
}

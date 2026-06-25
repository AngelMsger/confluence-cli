package apiclient

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

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

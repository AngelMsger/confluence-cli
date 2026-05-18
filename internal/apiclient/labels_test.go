package apiclient

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestListLabels(t *testing.T) {
	t.Parallel()
	var gotPath string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[
			{"id":"l1","name":"release-notes","prefix":"global","label":"release-notes"},
			{"id":"l2","prefix":"global","label":"reviewed"}],"size":2,"limit":25}`))
	}))

	res, err := c.ListLabels(context.Background(), "123", ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/rest/api/content/123/label" {
		t.Errorf("path = %q", gotPath)
	}
	if len(res.Items) != 2 || res.Items[0].Name != "release-notes" {
		t.Fatalf("labels = %+v", res.Items)
	}
	// The second entry has no "name"; the mapper falls back to "label".
	if res.Items[1].Name != "reviewed" {
		t.Errorf("label name fallback = %q", res.Items[1].Name)
	}
}

func TestAddLabels(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	var gotBody []byte
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotBody = r.Method, r.URL.Path, readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[
			{"id":"l1","name":"q3","prefix":"global"},
			{"id":"l2","name":"reviewed","prefix":"global"}],"size":2,"limit":25}`))
	}))

	labels, err := c.AddLabels(context.Background(), AddLabelsReq{
		PageID: "123", Names: []string{"q3", "reviewed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/api/content/123/label" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	body := string(gotBody)
	for _, want := range []string{`"prefix":"global"`, `"name":"q3"`, `"name":"reviewed"`} {
		if !strings.Contains(body, want) {
			t.Errorf("request body missing %q: %s", want, body)
		}
	}
	if len(labels) != 2 || labels[0].Name != "q3" {
		t.Errorf("labels = %+v", labels)
	}
}

func TestRemoveLabel(t *testing.T) {
	t.Parallel()
	var got string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.RequestURI()
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := c.RemoveLabel(context.Background(), RemoveLabelReq{PageID: "123", Name: "old tag"}); err != nil {
		t.Fatal(err)
	}
	if got != "DELETE /rest/api/content/123/label?name=old+tag" {
		t.Errorf("request = %q", got)
	}
}

func TestLabelWriteValidation(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if _, err := c.AddLabels(context.Background(), AddLabelsReq{Names: []string{"x"}}); err == nil {
		t.Error("expected error for missing page ID")
	}
	if _, err := c.AddLabels(context.Background(), AddLabelsReq{PageID: "1"}); err == nil {
		t.Error("expected error for no labels")
	}
	if err := c.RemoveLabel(context.Background(), RemoveLabelReq{PageID: "1"}); err == nil {
		t.Error("expected error for missing label name")
	}
}

func TestDescribeWriteLabel(t *testing.T) {
	t.Parallel()
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("dry-run must not send a request: %s %s", r.Method, r.URL.Path)
	}))

	plan, err := c.DescribeWrite(context.Background(), AddLabelsReq{
		PageID: "123", Names: []string{"q3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != http.MethodPost || plan.URL != srv.URL+"/rest/api/content/123/label" {
		t.Errorf("plan = %s %s", plan.Method, plan.URL)
	}

	rm, err := c.DescribeWrite(context.Background(), RemoveLabelReq{PageID: "123", Name: "q3"})
	if err != nil {
		t.Fatal(err)
	}
	if rm.Method != http.MethodDelete || !strings.Contains(rm.URL, "name=q3") {
		t.Errorf("remove plan = %s %s", rm.Method, rm.URL)
	}
}

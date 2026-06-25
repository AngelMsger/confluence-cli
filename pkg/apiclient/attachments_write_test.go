package apiclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestUploadAttachment(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath, gotToken, gotFileName, gotComment, gotFileBody string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotToken = r.Header.Get("X-Atlassian-Token")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile: %v", err)
		} else {
			gotFileName = hdr.Filename
			b, _ := io.ReadAll(f)
			gotFileBody = string(b)
		}
		gotComment = r.FormValue("comment")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"id":"att900","type":"attachment","title":"notes.txt",
			"extensions":{"fileSize":5,"mediaType":"text/plain"}}]}`))
	}))

	att, err := c.UploadAttachment(context.Background(), UploadAttachmentReq{
		PageID: "123", FileName: "notes.txt", Data: []byte("hello"),
		Comment: "first cut",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/api/content/123/child/attachment" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotToken != "nocheck" {
		t.Errorf("X-Atlassian-Token = %q, want nocheck", gotToken)
	}
	if gotFileName != "notes.txt" || gotFileBody != "hello" {
		t.Errorf("file part = %q / %q", gotFileName, gotFileBody)
	}
	if gotComment != "first cut" {
		t.Errorf("comment field = %q", gotComment)
	}
	if att.ID != "att900" || att.FileSize != 5 {
		t.Errorf("attachment = %+v", att)
	}
}

func TestUpdateAttachment(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = r.ParseMultipartForm(1 << 20)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"att900","type":"attachment","title":"notes.txt"}`))
	}))

	att, err := c.UpdateAttachment(context.Background(), UpdateAttachmentReq{
		PageID: "123", AttachmentID: "att900", FileName: "notes.txt", Data: []byte("v2"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/api/content/123/child/attachment/att900/data" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if att.ID != "att900" {
		t.Errorf("attachment = %+v", att)
	}
}

func TestDeleteAttachment(t *testing.T) {
	t.Parallel()
	var got string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := c.DeleteAttachment(context.Background(), DeleteAttachmentReq{AttachmentID: "att900"}); err != nil {
		t.Fatal(err)
	}
	if got != "DELETE /rest/api/content/att900" {
		t.Errorf("request = %q", got)
	}
}

func TestAttachmentWriteValidation(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if _, err := c.UploadAttachment(context.Background(), UploadAttachmentReq{FileName: "x"}); err == nil {
		t.Error("expected error for missing page ID")
	}
	if _, err := c.UploadAttachment(context.Background(), UploadAttachmentReq{PageID: "1"}); err == nil {
		t.Error("expected error for missing file name")
	}
	if _, err := c.UpdateAttachment(context.Background(), UpdateAttachmentReq{PageID: "1", FileName: "x"}); err == nil {
		t.Error("expected error for missing attachment ID")
	}
	if err := c.DeleteAttachment(context.Background(), DeleteAttachmentReq{}); err == nil {
		t.Error("expected error for missing attachment ID")
	}
}

func TestDescribeWriteAttachment(t *testing.T) {
	t.Parallel()
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("dry-run must not send a request: %s %s", r.Method, r.URL.Path)
	}))

	plan, err := c.DescribeWrite(context.Background(), UploadAttachmentReq{
		PageID: "123", FileName: "notes.txt", Data: []byte("hello"), Comment: "c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != http.MethodPost || plan.URL != srv.URL+"/rest/api/content/123/child/attachment" {
		t.Errorf("plan = %s %s", plan.Method, plan.URL)
	}
	mp, ok := plan.Payload.(MultipartPlan)
	if !ok {
		t.Fatalf("payload type = %T, want MultipartPlan", plan.Payload)
	}
	if mp.FileName != "notes.txt" || mp.FileBytes != 5 || mp.Fields["comment"] != "c" {
		t.Errorf("multipart plan = %+v", mp)
	}
}

func TestUploadAttachmentNoResult(t *testing.T) {
	t.Parallel()
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`))
	}))
	_, err := c.UploadAttachment(context.Background(), UploadAttachmentReq{
		PageID: "1", FileName: "f.txt", Data: []byte("x"),
	})
	if err == nil || !strings.Contains(err.Error(), "no attachment") {
		t.Errorf("expected an empty-result error, got %v", err)
	}
}

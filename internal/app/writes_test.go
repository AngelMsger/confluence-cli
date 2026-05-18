package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempFile writes content to a temp file and returns its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCmdAttachmentUploadDryRun(t *testing.T) {
	srv := mockConfluence(t)
	file := writeTempFile(t, "diagram.png", "PNGDATA")
	out, err := runCLI(t, srv, "attachment", "upload", "123", "--file", file, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if got["dry_run"] != true || got["method"] != "POST" {
		t.Errorf("dry-run output = %v", got)
	}
	if !strings.HasSuffix(got["url"].(string), "/rest/api/content/123/child/attachment") {
		t.Errorf("url = %v", got["url"])
	}
	payload, _ := got["payload"].(map[string]any)
	if payload["file_name"] != "diagram.png" || payload["file_bytes"].(float64) != 7 {
		t.Errorf("payload = %v", payload)
	}
}

func TestCmdAttachmentUpload(t *testing.T) {
	srv := mockConfluence(t)
	file := writeTempFile(t, "notes.txt", "hello")
	out, err := runCLI(t, srv, "attachment", "upload", "123", "--file", file)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if got["id"] != "att900" {
		t.Errorf("uploaded attachment = %v", got)
	}
}

func TestCmdAttachmentUploadNoFile(t *testing.T) {
	srv := mockConfluence(t)
	if _, err := runCLI(t, srv, "attachment", "upload", "123"); err == nil {
		t.Fatal("expected an error when --file is missing")
	}
}

func TestCmdAttachmentUpdateDryRun(t *testing.T) {
	srv := mockConfluence(t)
	file := writeTempFile(t, "notes-v2.txt", "world")
	out, err := runCLI(t, srv, "attachment", "update", "att900", "--file", file, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["dry_run"] != true {
		t.Errorf("dry-run output = %v", got)
	}
	// The update endpoint is page-scoped; the parent page (123) is resolved
	// from the attachment and the request targets its /data sub-resource.
	if !strings.HasSuffix(got["url"].(string), "/rest/api/content/123/child/attachment/att900/data") {
		t.Errorf("url = %v", got["url"])
	}
}

func TestCmdAttachmentDeleteDryRun(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "attachment", "delete", "att900", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["dry_run"] != true || got["method"] != "DELETE" {
		t.Errorf("dry-run output = %v", got)
	}
}

func TestCmdLabelList(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "label", "list", "123")
	if err != nil {
		t.Fatal(err)
	}
	got := decodeList(t, out)
	if len(got) != 1 || got[0]["name"] != "release-notes" {
		t.Errorf("labels = %v", got)
	}
}

func TestCmdLabelAdd(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "label", "add", "123", "q3", "reviewed")
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not a JSON array: %v\n%s", err, out)
	}
	if len(got) != 2 || got[0]["name"] != "q3" {
		t.Errorf("labels = %v", got)
	}
}

func TestCmdLabelAddDryRun(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "label", "add", "123", "q3", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["dry_run"] != true || got["method"] != "POST" {
		t.Errorf("dry-run output = %v", got)
	}
}

func TestCmdLabelRemoveDryRun(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "label", "remove", "123", "release-notes", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["dry_run"] != true || got["method"] != "DELETE" {
		t.Errorf("dry-run output = %v", got)
	}
	if !strings.Contains(got["url"].(string), "name=release-notes") {
		t.Errorf("url = %v", got["url"])
	}
}

func TestCmdPageHistory(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "page", "history", "123")
	if err != nil {
		t.Fatal(err)
	}
	got := decodeList(t, out)
	if len(got) != 2 || got[0]["number"].(float64) != 2 {
		t.Errorf("versions = %v", got)
	}
}

func TestCmdPageRestoreDryRun(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "page", "restore", "123", "--version", "1", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["dry_run"] != true || got["method"] != "PUT" {
		t.Errorf("dry-run output = %v", got)
	}
}

func TestCmdPageRestore(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "page", "restore", "123", "--version", "1")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["id"] != "123" {
		t.Errorf("restored page = %v", got)
	}
}

func TestCmdPageRestoreNoVersion(t *testing.T) {
	srv := mockConfluence(t)
	if _, err := runCLI(t, srv, "page", "restore", "123"); err == nil {
		t.Fatal("expected an error when --version is missing")
	}
}

func TestCmdPageWatch(t *testing.T) {
	srv := mockConfluence(t)
	for _, tc := range []struct {
		cmd      string
		watching bool
	}{{"watch", true}, {"unwatch", false}} {
		out, err := runCLI(t, srv, "page", tc.cmd, "123")
		if err != nil {
			t.Fatalf("%s: %v", tc.cmd, err)
		}
		var got map[string]any
		json.Unmarshal([]byte(out), &got)
		if got["page_id"] != "123" || got["watching"] != tc.watching {
			t.Errorf("%s output = %v", tc.cmd, got)
		}
	}
}

func TestCmdPageWatchStatus(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "page", "watch-status", "123")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["watching"] != true {
		t.Errorf("watch-status output = %v", got)
	}
}

func TestCmdPageWatchDryRun(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "page", "watch", "123", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["dry_run"] != true || got["method"] != "POST" {
		t.Errorf("dry-run output = %v", got)
	}
}

func TestCmdCommentUpdate(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "comment", "update", "c1", "--body", "edited")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["id"] != "c1" {
		t.Errorf("updated comment = %v", got)
	}
}

func TestCmdCommentUpdateDryRun(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "comment", "update", "c1", "--body", "edited", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["dry_run"] != true || got["method"] != "PUT" {
		t.Errorf("dry-run output = %v", got)
	}
}

func TestCmdCommentUpdateNoBody(t *testing.T) {
	srv := mockConfluence(t)
	if _, err := runCLI(t, srv, "comment", "update", "c1"); err == nil {
		t.Fatal("expected an error when no body is given")
	}
}

func TestCmdCommentDeleteDryRun(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "comment", "delete", "c1", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["dry_run"] != true || got["method"] != "DELETE" {
		t.Errorf("dry-run output = %v", got)
	}
}

func TestCmdCommentDelete(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "comment", "delete", "c1", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["id"] != "c1" || got["status"] != "deleted" {
		t.Errorf("delete output = %v", got)
	}
}

func TestCmdCommentDeleteRejectsPageURL(t *testing.T) {
	srv := mockConfluence(t)
	// A page URL with no focusedCommentId must NOT be resolved to the page ID
	// and deleted as if it were a comment.
	pageURL := srv.URL + "/pages/viewpage.action?pageId=123"
	_, err := runCLI(t, srv, "comment", "delete", pageURL, "--yes")
	if err == nil {
		t.Fatal("expected an error: a page URL is not a comment reference")
	}
}

func TestCmdCommentDeleteAcceptsCommentURL(t *testing.T) {
	srv := mockConfluence(t)
	// A comment permalink carries focusedCommentId — that is the content to act on.
	commentURL := srv.URL + "/pages/viewpage.action?pageId=123&focusedCommentId=c1"
	out, err := runCLI(t, srv, "comment", "delete", commentURL, "--yes")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["id"] != "c1" {
		t.Errorf("comment delete from permalink targeted %v, want c1", got["id"])
	}
}

func TestCmdAttachmentDeleteRejectsURL(t *testing.T) {
	srv := mockConfluence(t)
	if _, err := runCLI(t, srv, "attachment", "delete", srv.URL+"/download/attachments/123/f.pdf", "--yes"); err == nil {
		t.Fatal("expected an error: an attachment URL carries no attachment content ID")
	}
}

func TestCmdSearchListEnvelope(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "search", "--text", "welcome")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Items   []map[string]any `json:"items"`
		HasMore bool             `json:"has_more"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("output not a list envelope: %v\n%s", err, out)
	}
	if len(env.Items) != 1 || env.HasMore {
		t.Errorf("envelope = %+v", env)
	}
}

func TestCmdPageGetRenderNotes(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "page", "get", "790")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	notes, _ := got["render_notes"].([]any)
	if len(notes) == 0 {
		t.Fatalf("page with a view-file macro should report render_notes:\n%s", out)
	}
	if first, _ := notes[0].(string); !strings.Contains(first, "view-file") {
		t.Errorf("render_notes should name the macro: %v", notes)
	}
}

func TestCmdPageGetRaw(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "page", "get", "790", "--as", "raw")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	body, _ := got["body"].(string)
	// raw emits the storage source untouched — the macro tag must survive.
	if !strings.Contains(body, "<ac:structured-macro") {
		t.Errorf("--as raw should return the unrendered source:\n%s", body)
	}
	if got["scope_applied"] != "raw" {
		t.Errorf("scope_applied = %v, want raw", got["scope_applied"])
	}
	if _, leaked := got["render_notes"]; leaked {
		t.Errorf("raw output renders nothing, so it must carry no render_notes")
	}
}

func TestCmdPageGetRawRejectsPartialScope(t *testing.T) {
	srv := mockConfluence(t)
	if _, err := runCLI(t, srv, "page", "get", "790", "--as", "raw", "--scope", "outline"); err == nil {
		t.Fatal("expected an error: --as raw supports only --scope full")
	}
}

func TestCmdWhoami(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "whoami")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal([]byte(out), &got)
	if got["display_name"] != "Test User" || got["username"] != "tester" {
		t.Errorf("whoami output = %v", got)
	}
}

func TestCmdDoctorCurrentUser(t *testing.T) {
	srv := mockConfluence(t)
	out, err := runCLI(t, srv, "doctor", "--no-update-check")
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	json.Unmarshal([]byte(out), &report)
	checks, _ := report["checks"].([]any)
	var found map[string]any
	for _, c := range checks {
		m, _ := c.(map[string]any)
		if m["name"] == "current-user" {
			found = m
		}
	}
	if found == nil {
		t.Fatalf("doctor report has no current-user check:\n%s", out)
	}
	if found["ok"] != true || !strings.Contains(found["detail"].(string), "Test User") {
		t.Errorf("current-user check = %v", found)
	}
}

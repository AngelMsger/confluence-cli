package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
)

type sampleRec struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Space struct {
		Key string `json:"key"`
	} `json:"space"`
}

func mk(id, title, key string) sampleRec {
	r := sampleRec{ID: id, Title: title}
	r.Space.Key = key
	return r
}

func TestEmitJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := Emit(mk("1", "Hello", "ENG"), Options{Format: FormatJSON, Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got["title"] != "Hello" {
		t.Errorf("title = %v", got["title"])
	}
}

func TestEmitJSONFieldsProjection(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := Emit(mk("1", "Hello", "ENG"), Options{
		Format: FormatJSON, Fields: []string{"id", "space.key"}, Writer: &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal(buf.Bytes(), &got)
	if _, ok := got["title"]; ok {
		t.Error("title should have been projected out")
	}
	if got["space.key"] != "ENG" {
		t.Errorf("nested projection failed: %v", got)
	}
}

func TestEmitNDJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	recs := []sampleRec{mk("1", "A", "ENG"), mk("2", "B", "OPS")}
	if err := Emit(recs, Options{Format: FormatNDJSON, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("ndjson lines = %d, want 2", len(lines))
	}
	for _, l := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(l), &obj); err != nil {
			t.Errorf("line not valid JSON: %q", l)
		}
	}
}

func TestEmitTableList(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	recs := []sampleRec{mk("1", "Alpha", "ENG"), mk("2", "Beta", "OPS")}
	err := Emit(recs, Options{Format: FormatTable, Fields: []string{"id", "title"}, Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "id") || !strings.Contains(out, "title") {
		t.Errorf("table header missing:\n%s", out)
	}
	if !strings.Contains(out, "Alpha") || !strings.Contains(out, "Beta") {
		t.Errorf("table rows missing:\n%s", out)
	}
}

func TestEmitTableSingleObject(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Emit(mk("7", "Solo", "ENG"), Options{Format: FormatTable, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "FIELD") || !strings.Contains(out, "Solo") {
		t.Errorf("kv table wrong:\n%s", out)
	}
}

func TestEmitTableEmptyList(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Emit([]sampleRec{}, Options{Format: FormatTable, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no results") {
		t.Errorf("expected empty-list notice, got %q", buf.String())
	}
}

func TestEmitBadFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := Emit(mk("1", "X", "Y"), Options{Format: "xml", Writer: &buf})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if cerrors.AsCLIError(err).Category != cerrors.CategoryUsage {
		t.Errorf("category = %s", cerrors.AsCLIError(err).Category)
	}
}

func TestEmitListJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	items := []sampleRec{mk("1", "Hello", "ENG"), mk("2", "World", "DEV")}
	if err := EmitList(items, "25", true, Options{Format: FormatJSON, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Items   []map[string]any `json:"items"`
		Next    string           `json:"next"`
		HasMore bool             `json:"has_more"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not a list envelope: %v\n%s", err, buf.String())
	}
	if len(got.Items) != 2 || got.Next != "25" || !got.HasMore {
		t.Errorf("envelope = %+v", got)
	}
}

func TestEmitListJSONEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := EmitList([]sampleRec(nil), "", false, Options{Format: FormatJSON, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	// An empty list must serialize items as [] (not null) and omit next.
	if !strings.Contains(buf.String(), `"items": []`) {
		t.Errorf("empty list should emit []:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), `"next"`) {
		t.Errorf("next must be omitted when empty:\n%s", buf.String())
	}
}

func TestEmitListTable(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	items := []sampleRec{mk("1", "Hello", "ENG"), mk("2", "World", "DEV")}
	if err := EmitList(items, "25", true, Options{Format: FormatTable, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "World") {
		t.Errorf("table missing item rows:\n%s", out)
	}
	if strings.Contains(out, "has_more") {
		t.Errorf("envelope keys leaked into the table:\n%s", out)
	}
	if !strings.Contains(out, "--cursor 25") {
		t.Errorf("table should hint the next cursor:\n%s", out)
	}
}

func TestEmitListNDJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	items := []sampleRec{mk("1", "Hello", "ENG"), mk("2", "World", "DEV")}
	if err := EmitList(items, "", false, Options{Format: FormatNDJSON, Writer: &buf}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("ndjson should stream one item per line, got %d:\n%s", len(lines), buf.String())
	}
	for _, ln := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(ln), &rec); err != nil {
			t.Fatalf("ndjson line not valid JSON: %v", err)
		}
		if _, isEnvelope := rec["has_more"]; isEnvelope {
			t.Errorf("ndjson streamed the envelope, not its items: %s", ln)
		}
	}
}

func TestEmitListFields(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	items := []sampleRec{mk("1", "Hello", "ENG")}
	err := EmitList(items, "25", true, Options{Format: FormatJSON, Fields: []string{"id"}, Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Items   []map[string]any `json:"items"`
		HasMore bool             `json:"has_more"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not a list envelope: %v\n%s", err, buf.String())
	}
	// --fields projects the items; the envelope wrapper is preserved.
	if len(got.Items) != 1 || got.Items[0]["id"] != "1" {
		t.Errorf("projected items = %v", got.Items)
	}
	if _, leaked := got.Items[0]["title"]; leaked {
		t.Errorf("title should have been projected away: %v", got.Items[0])
	}
	if !got.HasMore {
		t.Error("envelope has_more was lost")
	}
}

func TestEmitError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	EmitError(cerrors.New(cerrors.CategoryAuth, "AUTH_X", "bad token"), &buf)
	var p cerrors.Payload
	if err := json.Unmarshal(buf.Bytes(), &p); err != nil {
		t.Fatalf("error output not valid JSON: %v", err)
	}
	if p.Error.Category != cerrors.CategoryAuth || p.Error.Code != "AUTH_X" {
		t.Errorf("error payload = %+v", p.Error)
	}
	if len(p.Error.NextSteps) == 0 {
		t.Error("error payload should carry next steps")
	}
}

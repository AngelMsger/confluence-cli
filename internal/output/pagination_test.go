package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestNDJSONPaginationNotice(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		items []map[string]any
	}{
		{name: "projected rows", items: []map[string]any{{"id": "1", "title": "hidden"}}},
		{name: "empty filtered page"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rows, notices bytes.Buffer
			if err := EmitList(tc.items, "opaque-cursor", true, Options{
				Format: FormatNDJSON, Fields: []string{"id"}, Writer: &rows, NoticeWriter: &notices,
			}); err != nil {
				t.Fatal(err)
			}
			wantRows := ""
			if len(tc.items) > 0 {
				wantRows = "{\"id\":\"1\"}\n"
			}
			if rows.String() != wantRows {
				t.Fatalf("stdout = %q, want only projected rows %q", rows.String(), wantRows)
			}
			var got struct {
				Notice struct {
					Pagination struct {
						Next    string `json:"next"`
						HasMore bool   `json:"has_more"`
					} `json:"pagination"`
					NextSteps []string `json:"next_steps"`
				} `json:"_notice"`
			}
			if err := json.Unmarshal(notices.Bytes(), &got); err != nil {
				t.Fatalf("stderr must contain one JSON notice: %v", err)
			}
			if got.Notice.Pagination.Next != "opaque-cursor" || !got.Notice.Pagination.HasMore {
				t.Fatalf("pagination lost: %s", notices.String())
			}
			if len(got.Notice.NextSteps) != 1 || got.Notice.NextSteps[0] != "Pass next as --cursor to retrieve the next page." {
				t.Fatalf("recovery instructions missing: %s", notices.String())
			}
		})
	}
}

func TestPaginationNoticeOnlyForIncompleteNDJSON(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		format  string
		hasMore bool
	}{
		{name: "final NDJSON page", format: FormatNDJSON},
		{name: "JSON envelope", format: FormatJSON, hasMore: true},
		{name: "table footer", format: FormatTable, hasMore: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rows, notices bytes.Buffer
			if err := EmitList(nil, "next", tc.hasMore, Options{
				Format: tc.format, Writer: &rows, NoticeWriter: &notices,
			}); err != nil {
				t.Fatal(err)
			}
			if notices.Len() != 0 {
				t.Fatalf("unexpected notice: %s", notices.String())
			}
		})
	}
}

type brokenPaginationWriter struct{}

func (brokenPaginationWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestNDJSONOutputFailureDoesNotAdvertiseContinuation(t *testing.T) {
	t.Parallel()
	var notices bytes.Buffer
	err := EmitList([]map[string]any{{"id": "1"}}, "next", true, Options{
		Format: FormatNDJSON, Writer: brokenPaginationWriter{}, NoticeWriter: &notices,
	})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("output failure = %v, want closed pipe", err)
	}
	if notices.Len() != 0 {
		t.Fatalf("failed output advertised continuation: %s", notices.String())
	}
}

func TestNDJSONNoticeFailurePreservesSuccessfulRows(t *testing.T) {
	t.Parallel()
	var rows bytes.Buffer
	if err := EmitList([]map[string]any{{"id": "1"}}, "next", true, Options{
		Format: FormatNDJSON, Writer: &rows, NoticeWriter: brokenPaginationWriter{},
	}); err != nil {
		t.Fatal(err)
	}
	if rows.String() != "{\"id\":\"1\"}\n" {
		t.Fatalf("rows = %q", rows.String())
	}
}

func TestPaginationContinuationFlag(t *testing.T) {
	t.Parallel()
	for _, format := range []string{FormatNDJSON, FormatTable} {
		t.Run(format, func(t *testing.T) {
			var rows, notices bytes.Buffer
			if err := EmitList(nil, "25", true, Options{
				Format: format, Writer: &rows, NoticeWriter: &notices, NextFlag: "--offset",
			}); err != nil {
				t.Fatal(err)
			}
			guidance := rows.String() + notices.String()
			if !strings.Contains(guidance, "--offset") || strings.Contains(guidance, "--cursor") {
				t.Fatalf("wrong continuation flag: %s", guidance)
			}
		})
	}
}

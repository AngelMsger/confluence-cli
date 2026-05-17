package apiclient

import "testing"

func TestBuildCQL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		params CQLParams
		want   string
		ok     bool
	}{
		{
			name:   "text only",
			params: CQLParams{Text: "release notes"},
			want:   `text ~ "release notes"`, ok: true,
		},
		{
			name:   "author and space",
			params: CQLParams{Author: "alice", Space: "ENG"},
			want:   `creator = "alice" AND space = "ENG"`, ok: true,
		},
		{
			name:   "type page",
			params: CQLParams{Type: "page"},
			want:   "type = page", ok: true,
		},
		{
			name:   "date range",
			params: CQLParams{After: "2025-01-01", Before: "2025-12-31"},
			want:   `lastmodified >= "2025-01-01" AND lastmodified <= "2025-12-31"`, ok: true,
		},
		{
			name:   "label and contributor",
			params: CQLParams{Label: "doc", Contributor: "bob"},
			want:   `contributor = "bob" AND label = "doc"`, ok: true,
		},
		{
			name:   "quote escaping",
			params: CQLParams{Text: `say "hi"`},
			want:   `text ~ "say \"hi\""`, ok: true,
		},
		{
			name:   "bad type",
			params: CQLParams{Type: "widget"}, ok: false,
		},
		{
			name:   "empty params",
			params: CQLParams{}, ok: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := BuildCQL(tc.params)
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, want ok = %v", err, tc.ok)
			}
			if tc.ok && got != tc.want {
				t.Errorf("BuildCQL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"https://x.atlassian.net/wiki", "https://x.atlassian.net"},
		{"https://x.atlassian.net/wiki/", "https://x.atlassian.net"},
		{"https://kms.example.com/", "https://kms.example.com"},
		{"https://kms.example.com", "https://kms.example.com"},
	}
	for _, tc := range tests {
		if got := NormalizeBaseURL(tc.in); got != tc.want {
			t.Errorf("NormalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNextOffsetToken(t *testing.T) {
	t.Parallel()
	if got := nextOffsetToken("", 25, 25); got != "25" {
		t.Errorf("full page next = %q, want 25", got)
	}
	if got := nextOffsetToken("25", 25, 25); got != "50" {
		t.Errorf("second full page next = %q, want 50", got)
	}
	if got := nextOffsetToken("", 25, 10); got != "" {
		t.Errorf("partial page next = %q, want empty", got)
	}
}

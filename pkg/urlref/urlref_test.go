package urlref

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		in       string
		wantID   string
		wantKey  string
		wantFlav FlavorHint
		wantURL  bool
		wantBase string
	}{
		{
			name: "bare numeric id", in: "123456",
			wantID: "123456", wantFlav: FlavorUnknown,
		},
		{
			name:   "cloud page url",
			in:     "https://acme.atlassian.net/wiki/spaces/ENG/pages/98765/Design+Doc",
			wantID: "98765", wantKey: "ENG", wantFlav: FlavorCloud,
			wantURL: true, wantBase: "https://acme.atlassian.net/wiki",
		},
		{
			name:   "datacenter modern pages url",
			in:     "https://kms.example.com/spaces/DEV/pages/4242/Title",
			wantID: "4242", wantKey: "DEV", wantFlav: FlavorDataCenter,
			wantURL: true, wantBase: "https://kms.example.com",
		},
		{
			name:   "datacenter viewpage action",
			in:     "https://kms.example.com/pages/viewpage.action?pageId=555",
			wantID: "555", wantFlav: FlavorDataCenter,
			wantURL: true, wantBase: "https://kms.example.com",
		},
		{
			name:    "datacenter display url",
			in:      "https://kms.example.com/display/DEV/Some+Page",
			wantKey: "DEV", wantFlav: FlavorDataCenter, wantURL: true,
		},
		{
			name:    "cloud space overview without page",
			in:      "https://acme.atlassian.net/wiki/spaces/ENG/overview",
			wantKey: "ENG", wantFlav: FlavorCloud, wantURL: true,
		},
		{
			name: "non-numeric non-url treated as id",
			in:   "abc123xyz", wantID: "abc123xyz",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Parse(tc.in)
			if got.PageID != tc.wantID {
				t.Errorf("PageID = %q, want %q", got.PageID, tc.wantID)
			}
			if got.SpaceKey != tc.wantKey {
				t.Errorf("SpaceKey = %q, want %q", got.SpaceKey, tc.wantKey)
			}
			if got.Flavor != tc.wantFlav {
				t.Errorf("Flavor = %q, want %q", got.Flavor, tc.wantFlav)
			}
			if got.IsURL != tc.wantURL {
				t.Errorf("IsURL = %v, want %v", got.IsURL, tc.wantURL)
			}
			if tc.wantBase != "" && got.BaseURL != tc.wantBase {
				t.Errorf("BaseURL = %q, want %q", got.BaseURL, tc.wantBase)
			}
		})
	}
}

func TestParseTitleUnslug(t *testing.T) {
	t.Parallel()
	got := Parse("https://acme.atlassian.net/wiki/spaces/ENG/pages/1/My+Great+Page")
	if got.Title != "My Great Page" {
		t.Errorf("Title = %q, want %q", got.Title, "My Great Page")
	}
}

func TestParseFocusedComment(t *testing.T) {
	t.Parallel()
	// A comment permalink carries both the page ID and the comment's own ID.
	got := Parse("https://kms.example.com/pages/viewpage.action?pageId=555&focusedCommentId=999")
	if got.PageID != "555" {
		t.Errorf("PageID = %q, want 555", got.PageID)
	}
	if got.CommentID != "999" {
		t.Errorf("CommentID = %q, want 999", got.CommentID)
	}
	// A plain page URL carries no comment ID.
	if plain := Parse("https://kms.example.com/pages/viewpage.action?pageId=555"); plain.CommentID != "" {
		t.Errorf("CommentID = %q, want empty for a plain page URL", plain.CommentID)
	}
}

func TestParseEmpty(t *testing.T) {
	t.Parallel()
	if got := Parse("  "); got != (Ref{}) {
		t.Errorf("Parse(blank) = %+v, want zero Ref", got)
	}
}

package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

func TestResolveHistoryWindow(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	window, err := resolveHistoryWindow("2d", "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if !window.from.Equal(now.Add(-48*time.Hour)) || !window.to.Equal(now) {
		t.Fatalf("window = %s..%s", window.from, window.to)
	}

	window, err = resolveHistoryWindow("", "2026-09-03", "2026-09-04", now)
	if err != nil {
		t.Fatal(err)
	}
	if got := window.to.Sub(window.from); got != 24*time.Hour {
		t.Fatalf("date window = %s; want 24h", got)
	}
}

// The shared time-window contract: --since excludes --from/--to, --to needs
// --from, and the window must be a non-empty [from, to).
func TestResolveHistoryWindowRejectsAmbiguousOrUnusableWindows(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, since, from, to, want string
	}{
		{name: "since with from", since: "24h", from: "2026-09-03", want: "--since cannot be combined"},
		{name: "since with to", since: "24h", to: "2026-09-04", want: "--since cannot be combined"},
		{name: "since with from and to", since: "24h", from: "2026-09-03", to: "2026-09-04", want: "--since cannot be combined"},
		{name: "to without from", to: "2026-09-04", want: "--to requires --from"},
		{name: "reversed", from: "2026-09-04", to: "2026-09-03", want: "--to must be later than --from"},
		{name: "empty", from: "2026-09-03", to: "2026-09-03", want: "--to must be later than --from"},
		{name: "from past the default end", from: "2026-09-05", want: "--to must be later than --from"},
		{name: "zero since", since: "0h", want: "invalid --since"},
		{name: "negative since", since: "-1h", want: "invalid --since"},
		{name: "date as since", since: "2026-09-03", want: "invalid --since"},
		{name: "bad from", from: "soon", want: "invalid --from"},
		{name: "bad to", from: "2026-09-03", to: "later", want: "invalid --to"},
	} {
		_, err := resolveHistoryWindow(tc.since, tc.from, tc.to, now)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want it to contain %q", tc.name, err, tc.want)
		}
	}
}

// The command reports a rejected window as one structured usage error that
// names the violated rule, and it does so before any request is sent.
func TestCmdPageHistoryRejectsBadWindowsBeforeAnyRequest(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"since with from", []string{"123", "--since", "24h", "--from", "2026-09-03"}, "--since cannot be combined with --from or --to"},
		{"since with to", []string{"123", "--since", "24h", "--to", "2026-09-04"}, "--since cannot be combined with --from or --to"},
		{"to without from", []string{"123", "--to", "2026-09-04"}, "--to requires --from"},
		{"reversed", []string{"123", "--from", "2026-09-04", "--to", "2026-09-03"}, "--to must be later than --from"},
		{"bad since", []string{"123", "--since", "soon"}, `invalid --since "soon"`},
		{"batch since with to", []string{"123", "456", "--since", "24h", "--to", "2026-09-04"}, "--since cannot be combined with --from or --to"},
		{"batch with actor", []string{"123", "456", "--actor", "me", "--since", "24h", "--from", "2026-09-03"}, "--since cannot be combined with --from or --to"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCLI(t, srv, append([]string{"page", "history"}, tc.args...)...)
			ce := cerrors.AsCLIError(err)
			if ce == nil || ce.Code != "BAD_TIME_RANGE" || ce.Category != cerrors.CategoryUsage || cerrors.ExitCode(err) != cerrors.ExitUsage {
				t.Fatalf("unexpected error: %+v", ce)
			}
			if !strings.Contains(ce.Message, tc.want) {
				t.Fatalf("message %q does not name the violated rule %q", ce.Message, tc.want)
			}
			if !strings.Contains(ce.Hint, "--to requires --from") || len(ce.NextSteps) != 2 ||
				!strings.HasPrefix(ce.NextSteps[0], "confluence-cli page history <id> --since ") {
				t.Fatalf("missing contract hint or runnable next steps: %+v", ce)
			}
			if out != "" {
				t.Fatalf("a rejected window wrote to stdout: %q", out)
			}
		})
	}
	// A batch needs a lower bound, so an upper bound alone asks for one.
	_, err := runCLI(t, srv, "page", "history", "123", "456", "--to", "2026-09-04")
	if ce := cerrors.AsCLIError(err); ce == nil || ce.Code != "HISTORY_TIME_REQUIRED" || ce.Category != cerrors.CategoryUsage {
		t.Fatalf("batch --to without --from: %+v", ce)
	}
	if requests != 0 {
		t.Fatalf("a rejected window still sent %d request(s)", requests)
	}
}

func TestCollectPageVersionsInWindowStopsAfterLowerBound(t *testing.T) {
	window, err := resolveHistoryWindow("", "2026-09-03", "2026-09-04", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	pages := map[string]apiclient.ListResult[apiclient.PageVersion]{
		"": {
			Items: []apiclient.PageVersion{
				{Number: 5, When: "2026-09-04T00:00:00Z"},
				{Number: 4, When: "2026-09-03T18:00:00Z", Actor: &apiclient.User{Username: "bob"}},
			},
			Next: "2",
		},
		"2": {
			Items: []apiclient.PageVersion{
				{Number: 3, When: "2026-09-03T12:00:00Z", Actor: &apiclient.User{UserKey: "key-1"}},
				{Number: 2, When: "2026-09-02T23:59:59Z"},
			},
			Next: "3",
		},
		"3": {Items: []apiclient.PageVersion{{Number: 1, When: "2026-09-02T00:00:00Z"}}},
	}
	calls := 0
	withinWindow, err := collectPageVersionsInWindow(func(cursor string) (apiclient.ListResult[apiclient.PageVersion], error) {
		calls++
		return pages[cursor], nil
	}, "", window)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("fetched %d pages; want 2", calls)
	}
	got, _ := filterPageVersionsByActor(withinWindow,
		&apiclient.User{Username: "alice", UserKey: "key-1"}, apiclient.FlavorDataCenter)
	if len(got) != 1 || got[0].Number != 3 {
		t.Fatalf("versions = %+v", got)
	}
}

func TestFilterPageVersionsMatchesDataCenterUserKey(t *testing.T) {
	actor := &apiclient.User{Username: "alice", UserKey: "key-1"}
	items := []apiclient.PageVersion{
		{Number: 2, Actor: &apiclient.User{UserKey: "key-1"}},
		{Number: 1, Actor: &apiclient.User{DisplayName: "Alice"}},
	}
	got, missing := filterPageVersionsByActor(items, actor, apiclient.FlavorDataCenter)
	if len(got) != 1 || got[0].Number != 2 {
		t.Fatalf("versions = %+v", got)
	}
	if missing != 1 {
		t.Fatalf("missing stable actors = %d; want 1", missing)
	}
}

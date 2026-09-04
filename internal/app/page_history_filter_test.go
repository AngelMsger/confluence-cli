package app

import (
	"testing"
	"time"

	"github.com/angelmsger/confluence-cli/pkg/apiclient"
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

func TestResolveHistoryWindowValidation(t *testing.T) {
	for _, tc := range []struct {
		since string
		from  string
		to    string
	}{
		{since: "24h", from: "2026-09-03"},
		{to: "2026-09-04"},
		{since: "0h"},
		{from: "2026-09-04", to: "2026-09-03"},
	} {
		if _, err := resolveHistoryWindow(tc.since, tc.from, tc.to, time.Now()); err == nil {
			t.Fatalf("expected error for %+v", tc)
		}
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

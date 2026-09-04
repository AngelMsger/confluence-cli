package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

type historyWindow struct {
	from time.Time
	to   time.Time
	set  bool
}

func resolveHistoryWindow(since, from, to string, now time.Time) (historyWindow, error) {
	if since != "" && (from != "" || to != "") {
		return historyWindow{}, fmt.Errorf("--since cannot be combined with --from or --to")
	}
	if to != "" && from == "" {
		return historyWindow{}, fmt.Errorf("--to requires --from")
	}
	if since == "" && from == "" {
		return historyWindow{}, nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	var start, end time.Time
	var err error
	if since != "" {
		duration, err := parseHistoryDuration(since)
		if err != nil || duration <= 0 {
			return historyWindow{}, fmt.Errorf("invalid --since %q: use a positive duration such as 24h or 7d", since)
		}
		start, end = now.Add(-duration), now
	} else {
		start, err = parseHistoryInstant(from)
		if err != nil {
			return historyWindow{}, fmt.Errorf("invalid --from %q: %w", from, err)
		}
		end = now
		if to != "" {
			end, err = parseHistoryInstant(to)
			if err != nil {
				return historyWindow{}, fmt.Errorf("invalid --to %q: %w", to, err)
			}
		}
	}
	if !end.After(start) {
		return historyWindow{}, fmt.Errorf("--to must be later than --from")
	}
	return historyWindow{from: start, to: end, set: true}, nil
}

func parseHistoryDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if len(value) > 1 {
		n, err := strconv.Atoi(value[:len(value)-1])
		if err == nil {
			switch value[len(value)-1] {
			case 'd':
				return time.Duration(n) * 24 * time.Hour, nil
			case 'w':
				return time.Duration(n) * 7 * 24 * time.Hour, nil
			}
		}
	}
	return time.ParseDuration(value)
}

func parseHistoryInstant(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05-0700",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, time.UTC); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("use RFC3339 or a date in YYYY-MM-DD form (date-only values are UTC)")
}

func collectPageVersionsInWindow(fetch apiclient.FetchPage[apiclient.PageVersion], cursor string, window historyWindow) ([]apiclient.PageVersion, error) {
	var out []apiclient.PageVersion
	for {
		page, err := fetch(cursor)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			when, err := parseHistoryInstant(item.When)
			if err != nil {
				return nil, cerrors.Wrap(err, cerrors.CategoryParse, "HISTORY_TIME_INVALID",
					fmt.Sprintf("page version has an invalid timestamp %q", item.When))
			}
			if !when.Before(window.to) {
				continue
			}
			if when.Before(window.from) {
				return out, nil
			}
			out = append(out, item)
		}
		if page.Next == "" {
			return out, nil
		}
		cursor = page.Next
	}
}

func filterPageVersionsByActor(items []apiclient.PageVersion, actor *apiclient.User, flavor apiclient.Flavor) ([]apiclient.PageVersion, int) {
	if actor == nil {
		return items, 0
	}
	out := make([]apiclient.PageVersion, 0, len(items))
	missingActors := 0
	for _, item := range items {
		if !hasStableUserID(flavor, item.Actor) {
			missingActors++
			continue
		}
		if !sameStableUser(actor, item.Actor, flavor) {
			continue
		}
		out = append(out, item)
	}
	return out, missingActors
}

func sameStableUser(left, right *apiclient.User, flavor apiclient.Flavor) bool {
	if left == nil || right == nil {
		return false
	}
	if flavor == apiclient.FlavorCloud {
		return left.AccountID != "" && right.AccountID != "" && strings.EqualFold(left.AccountID, right.AccountID)
	}
	usernameMatch := left.Username != "" && right.Username != "" && strings.EqualFold(left.Username, right.Username)
	userKeyMatch := left.UserKey != "" && right.UserKey != "" && strings.EqualFold(left.UserKey, right.UserKey)
	return usernameMatch || userKeyMatch
}

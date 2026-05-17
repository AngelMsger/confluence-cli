package app

import (
	"context"
	"fmt"
	"os"

	"github.com/angelmsger/confluence-cli/internal/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/angelmsger/confluence-cli/internal/transport"
	"github.com/angelmsger/confluence-cli/pkg/urlref"
)

// buildProbeTransport returns an unauthenticated transport used for flavor
// detection and connectivity checks.
func buildProbeTransport(s *appState) *transport.Client {
	return transport.New(transport.Options{
		Timeout:    s.timeout(),
		MaxRetries: s.cfg().Defaults.MaxRetries,
	})
}

// cmdContext returns a context bounded by the configured request timeout.
func cmdContext(s *appState) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.timeout())
}

// resolveID extracts a Confluence page ID from a bare ID or a page URL.
func resolveID(arg string) (string, error) {
	ref := urlref.Parse(arg)
	if ref.PageID == "" {
		return "", cerrors.Newf(cerrors.CategoryUsage, "NO_PAGE_ID",
			"could not resolve a page ID from %q", arg).
			WithHint("Pass a numeric page ID or a Confluence page URL.").
			WithNextSteps("confluence-cli search --text \"<keywords>\" to find the page ID.")
	}
	return ref.PageID, nil
}

// collectList fetches one page of results, or every page when all is true.
// When not collecting all and more results exist, it prints a hint to stderr.
func collectList[T any](fetch apiclient.FetchPage[T], limit int, all bool) ([]T, error) {
	if all {
		return apiclient.CollectAll(fetch, 0)
	}
	page, err := fetch("")
	if err != nil {
		return nil, err
	}
	if page.Next != "" {
		fmt.Fprintln(os.Stderr,
			"note: more results available — re-run with --all to fetch every page")
	}
	return page.Items, nil
}

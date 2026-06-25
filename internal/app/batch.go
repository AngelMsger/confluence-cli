package app

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// deletePrompt builds the confirmDelete prompt for a one-or-many delete: the
// detailed single-item form ("page 123 (moves it to the trash)") when there is
// exactly one item, or a count ("3 pages") for a batch.
func deletePrompt(noun string, items []string, suffix string) string {
	if len(items) == 1 {
		p := noun + " " + items[0]
		if suffix != "" {
			p += " " + suffix
		}
		return p
	}
	return fmt.Sprintf("%d %ss", len(items), noun)
}

// batchItem is one entry in a batch operation's aggregated result.
type batchItem struct {
	Ref    string               `json:"ref"`
	OK     bool                 `json:"ok"`
	Result any                  `json:"result,omitempty"`
	Error  *cerrors.PayloadBody `json:"error,omitempty"`
}

// collectBatchArgs returns the items a batch command should operate on. A lone
// "-" positional reads newline-separated items from r (stdin); otherwise the
// positional args are the items verbatim. Blank lines and surrounding
// whitespace are ignored, so `... | xargs`-style and here-doc input both work.
func collectBatchArgs(args []string, r io.Reader) ([]string, error) {
	if len(args) == 1 && args[0] == "-" {
		var items []string
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if s := strings.TrimSpace(sc.Text()); s != "" {
				items = append(items, s)
			}
		}
		if err := sc.Err(); err != nil {
			return nil, cerrors.Wrap(err, cerrors.CategoryUsage, "BAD_STDIN",
				"could not read items from stdin")
		}
		if len(items) == 0 {
			return nil, cerrors.New(cerrors.CategoryUsage, "NO_ITEMS",
				"no items were provided on stdin")
		}
		return items, nil
	}
	return args, nil
}

// runBatch applies do to each item. With a single item it emits the bare result
// (byte-identical to the pre-batch single-object output, so existing callers and
// scripts are unaffected). With several items it emits a {items, has_more}
// aggregate recording ok / result / error per item, runs every item even when
// some fail, and returns a summarizing error so the process exit code still
// reflects a partial failure. do should return the per-item success payload (or
// nil) and an error; it is responsible for honoring --dry-run itself.
func runBatch(s *appState, items []string, do func(item string) (any, error)) error {
	if len(items) == 1 {
		res, err := do(items[0])
		if err != nil {
			return err
		}
		return s.emit(res)
	}
	out := make([]batchItem, 0, len(items))
	failed := 0
	var last *cerrors.CLIError
	for _, it := range items {
		res, err := do(it)
		if err != nil {
			ce := cerrors.AsCLIError(err)
			body := ce.Payload().Error
			out = append(out, batchItem{Ref: it, OK: false, Error: &body})
			failed++
			last = ce
			continue
		}
		out = append(out, batchItem{Ref: it, OK: true, Result: res})
	}
	if emitErr := s.emit(map[string]any{"items": out, "has_more": false}); emitErr != nil {
		return emitErr
	}
	if failed > 0 {
		return cerrors.Newf(last.Category, "BATCH_PARTIAL_FAILURE",
			"%d of %d operations failed; see per-item results on stdout", failed, len(items))
	}
	return nil
}

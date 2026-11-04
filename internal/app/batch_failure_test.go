package app

import (
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// A live batch with failures must not invite a rerun of the batch: its ok items
// were applied. Its next steps replace the category's "retry" defaults.
func TestBatchPartialFailureLive(t *testing.T) {
	ce := cerrors.AsCLIError(batchPartialFailure(cerrors.CategoryServer, 1, 2, false))
	if ce.Code != "BATCH_PARTIAL_FAILURE" || ce.Retryable || !strings.Contains(ce.Hint, "were applied") {
		t.Fatalf("got %s retryable=%v hint %q", ce.Code, ce.Retryable, ce.Hint)
	}
	for _, step := range ce.NextSteps {
		if strings.Contains(strings.ToLower(step), "retry") {
			t.Fatalf("next step %q invites a retry of the batch", step)
		}
	}
}

// A --dry-run batch sent nothing, so it must not claim anything was applied.
func TestBatchPartialFailureDryRun(t *testing.T) {
	ce := cerrors.AsCLIError(batchPartialFailure(cerrors.CategoryNotFound, 1, 2, true))
	if strings.Contains(ce.Hint, "applied") || !strings.Contains(ce.Hint, "nothing was sent") {
		t.Fatalf("dry-run hint %q", ce.Hint)
	}
}

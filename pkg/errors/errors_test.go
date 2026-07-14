package errors

import (
	stderrors "errors"
	"testing"
)

func TestExitCodeMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cat  Category
		want int
	}{
		{CategoryUsage, ExitUsage},
		{CategoryConfig, ExitConfig},
		{CategoryAuth, ExitAuth},
		{CategoryPermission, ExitPermission},
		{CategoryNotFound, ExitNotFound},
		{CategoryRateLimit, ExitRateLimit},
		{CategoryNetwork, ExitNetwork},
		{CategoryServer, ExitServer},
		{CategoryParse, ExitParse},
		{CategoryInternal, ExitInternal},
	}
	for _, tc := range tests {
		t.Run(string(tc.cat), func(t *testing.T) {
			t.Parallel()
			err := New(tc.cat, "X", "msg")
			if got := ExitCode(err); got != tc.want {
				t.Errorf("ExitCode(%s) = %d, want %d", tc.cat, got, tc.want)
			}
		})
	}
}

func TestExitCodeNil(t *testing.T) {
	t.Parallel()
	if got := ExitCode(nil); got != ExitSuccess {
		t.Errorf("ExitCode(nil) = %d, want %d", got, ExitSuccess)
	}
}

func TestFromHTTPStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status int
		want   Category
	}{
		{401, CategoryAuth},
		{403, CategoryPermission},
		{404, CategoryNotFound},
		{429, CategoryRateLimit},
		{500, CategoryServer},
		{503, CategoryServer},
		{400, CategoryUsage},
		{422, CategoryUsage},
	}
	for _, tc := range tests {
		if got := FromHTTPStatus(tc.status); got != tc.want {
			t.Errorf("FromHTTPStatus(%d) = %s, want %s", tc.status, got, tc.want)
		}
	}
}

func TestAsCLIErrorWrapsUnknown(t *testing.T) {
	t.Parallel()
	plain := stderrors.New("boom")
	ce := AsCLIError(plain)
	if ce.Category != CategoryInternal {
		t.Errorf("Category = %s, want internal", ce.Category)
	}
	if !stderrors.Is(ce, plain) {
		t.Error("wrapped CLIError should unwrap to the original error")
	}
}

func TestAsCLIErrorPassthrough(t *testing.T) {
	t.Parallel()
	orig := New(CategoryAuth, "AUTH", "nope")
	if AsCLIError(orig) != orig {
		t.Error("AsCLIError should return an existing *CLIError unchanged")
	}
}

func TestDefaultGuidancePopulated(t *testing.T) {
	t.Parallel()
	for _, cat := range []Category{CategoryAuth, CategoryConfig, CategoryNotFound} {
		e := New(cat, "C", "m")
		if e.Hint == "" || len(e.NextSteps) == 0 {
			t.Errorf("category %s: expected hint and next steps", cat)
		}
	}
}

func TestRetryableFlag(t *testing.T) {
	t.Parallel()
	if !New(CategoryRateLimit, "C", "m").Retryable {
		t.Error("rate_limit should be retryable")
	}
	if New(CategoryAuth, "C", "m").Retryable {
		t.Error("auth should not be retryable")
	}
}

func TestPayloadShape(t *testing.T) {
	t.Parallel()
	p := New(CategoryNotFound, "NF", "missing").
		WithHTTPStatus(404).
		WithRecovery(Recovery{Action: "retry_current_command", Scope: "host", Requires: []string{"os_keychain"}}).
		Payload()
	if p.Error.Category != CategoryNotFound || p.Error.HTTPStatus != 404 {
		t.Errorf("unexpected payload: %+v", p)
	}
	if p.Error.Recovery == nil || p.Error.Recovery.Scope != "host" {
		t.Errorf("payload missing recovery: %+v", p)
	}
}

package errors

import "testing"

// The model is tested in agent-cli-kit; this checks that confluence-cli's own
// guidance is what an error built through this package carries.
func TestErrorsCarryConfluenceGuidance(t *testing.T) {
	e := New(CategoryConfig, "NO_BASE_URL", "no Confluence base URL configured")
	want := []string{"confluence-cli config init", "confluence-cli config show --explain"}
	if len(e.NextSteps) != len(want) || e.NextSteps[0] != want[0] || e.NextSteps[1] != want[1] {
		t.Fatalf("NextSteps = %v, want confluence-cli's config guidance %v", e.NextSteps, want)
	}
	if ExitCode(e) != ExitConfig {
		t.Fatalf("ExitCode = %d, want %d", ExitCode(e), ExitConfig)
	}
}

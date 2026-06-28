package app

import (
	"bytes"
	"strings"
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// runRoot executes the full command tree against an isolated (empty) config dir
// so config loading never interferes, and returns the resulting error.
func runRoot(t *testing.T, args ...string) error {
	t.Helper()
	cmd := NewRootCmd()
	cmd.SetArgs(append([]string{"--config", t.TempDir()}, args...))
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	return cmd.Execute()
}

// A pure command group must reject an unknown subcommand as a usage error
// instead of cobra's default (print help, exit 0), which reads as false success.
func TestGroupRejectsUnknownSubcommand(t *testing.T) {
	for _, group := range []string{"config", "auth", "skill"} {
		t.Run(group, func(t *testing.T) {
			err := runRoot(t, group, "zzz", "extra")
			if err == nil {
				t.Fatalf("%s: expected an error for an unknown subcommand, got nil", group)
			}
			ce := cerrors.AsCLIError(err)
			if ce.Category != cerrors.CategoryUsage || ce.Code != "UNKNOWN_COMMAND" {
				t.Fatalf("%s: got category=%q code=%q, want usage/UNKNOWN_COMMAND", group, ce.Category, ce.Code)
			}
			if cerrors.ExitCode(ce) != cerrors.ExitUsage {
				t.Fatalf("%s: exit code = %d, want %d", group, cerrors.ExitCode(ce), cerrors.ExitUsage)
			}
		})
	}
}

// A close typo should surface a "Did you mean" suggestion, mirroring the
// root-level UX cobra gives via findSuggestions.
func TestGroupSuggestsNearestSubcommand(t *testing.T) {
	err := runRoot(t, "config", "use-contexts") // typo of use-context
	if err == nil {
		t.Fatal("expected an error for use-contexts")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Did you mean") || !strings.Contains(msg, "use-context") {
		t.Fatalf("expected a use-context suggestion, got: %q", msg)
	}
}

// A bare group invocation still prints help and succeeds (exit 0).
func TestBareGroupShowsHelp(t *testing.T) {
	if err := runRoot(t, "config"); err != nil {
		t.Fatalf("bare group should show help without error, got: %v", err)
	}
}

// A valid subcommand is unaffected by the group's RunE guard.
func TestValidSubcommandStillRuns(t *testing.T) {
	if err := runRoot(t, "config", "show"); err != nil {
		t.Fatalf("valid subcommand should run, got: %v", err)
	}
}

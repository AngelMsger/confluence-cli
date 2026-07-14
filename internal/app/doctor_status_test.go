package app

import (
	"testing"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

func TestDoctorCredentialRecoveryStatus(t *testing.T) {
	t.Parallel()
	err := cerrors.New(cerrors.CategoryConfig, "CREDENTIAL_STORE_INACCESSIBLE", "hidden").
		WithRecovery(cerrors.Recovery{Action: "retry_current_command", Scope: "host"})
	if got := diagnosticStatus(err); got != "inaccessible" {
		t.Fatalf("diagnosticStatus() = %q, want inaccessible", got)
	}
	recoveryScope := diagnosticRecoveryScope(err)
	if recoveryScope != "host" {
		t.Fatalf("diagnosticRecoveryScope() = %q, want host", recoveryScope)
	}
	report := doctorReport{Checks: []doctorCheck{{RecoveryScope: recoveryScope}}}
	if !reportNeedsHostRetry(report) {
		t.Fatal("reportNeedsHostRetry() = false, want true")
	}
}

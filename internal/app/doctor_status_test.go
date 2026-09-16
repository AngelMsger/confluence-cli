package app

import (
	"os"
	"path/filepath"
	"strings"
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

func TestCompanionSkillDoctorCheck(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envSkillLoaded, "")
	dir := filepath.Join(home, ".codex", "skills", "confluence")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	version := strings.TrimPrefix(embeddedSkillVersion(), "v")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nversion: "+version+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := companionSkillDoctorCheck(); got.Status != "installed_not_loaded" || !got.OK {
		t.Fatalf("installed check = %+v", got)
	}
	t.Setenv(envSkillLoaded, "0.0.0")
	if got := companionSkillDoctorCheck(); got.Status != "reload_required" {
		t.Fatalf("reload check = %+v", got)
	}
	t.Setenv(envSkillLoaded, version)
	if got := companionSkillDoctorCheck(); got.Status != "current" || !got.OK {
		t.Fatalf("loaded check = %+v", got)
	}
}

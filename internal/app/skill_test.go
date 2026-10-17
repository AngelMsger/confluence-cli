package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angelmsger/agent-cli-kit/skill"
)

// The kit's skill package owns the version, load-state, inspection and agent
// directory rules and their behavior matrices. These tests check the wiring:
// that this CLI hands the kit its own Skill name, embedded document,
// environment variable and filesystem.

func TestEmbeddedSkillHasAVersion(t *testing.T) {
	if got := embeddedSkillVersion(); got == "(unknown)" {
		t.Fatal("the embedded SKILL.md has no version: line")
	}
}

func TestSkillInspectionComparesWithTheEmbeddedSkill(t *testing.T) {
	dir := t.TempDir()
	write := func(version string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nversion: "+version+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	embedded := embeddedSkillVersion()
	write(strings.TrimPrefix(embedded, "v"))
	want := skillResult{Agent: "test", Path: dir, Status: "installed", Version: embedded, Alignment: "current"}
	if got := inspectSkillInstall("test", dir); got != want {
		t.Fatalf("current install = %+v, want %+v", got, want)
	}
	write("0.0.0")
	want = skillResult{Agent: "test", Path: dir, Status: "installed", Version: "v0.0.0", Alignment: "outdated"}
	if got := inspectSkillInstall("test", dir); got != want {
		t.Fatalf("outdated install = %+v, want %+v", got, want)
	}
	missing := filepath.Join(dir, "missing")
	want = skillResult{Agent: "test", Path: missing, Status: "not_installed"}
	if got := inspectSkillInstall("test", missing); got != want {
		t.Fatalf("missing install = %+v, want %+v", got, want)
	}
}

func TestSkillLoadStateReadsTheHandshakeVariable(t *testing.T) {
	t.Setenv(envSkillLoaded, "")
	if got := currentSkillLoadState(); got.Status != "not_loaded" || got.Loaded {
		t.Fatalf("unset load = %+v", got)
	}
	t.Setenv(envSkillLoaded, strings.TrimPrefix(embeddedSkillVersion(), "v"))
	if got := currentSkillLoadState(); got.Status != "current" || !got.Loaded {
		t.Fatalf("current load = %+v", got)
	}
	t.Setenv(envSkillLoaded, "0.0.0")
	if got := currentSkillLoadState(); got.Status != "outdated" {
		t.Fatalf("outdated load = %+v", got)
	}
}

func TestSkillTargetsUseThisSkillsName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, agent := range skill.Agents() {
		global, err := agentDest(agent, false)
		if err != nil {
			t.Fatal(err)
		}
		if want := agent.Destination(home, skillName, false); global != want {
			t.Errorf("%s global dest = %q, want %q", agent.ID, global, want)
		}
		project, err := agentDest(agent, true)
		if err != nil {
			t.Fatal(err)
		}
		if want := agent.Destination("", skillName, true); project != want {
			t.Errorf("%s project dest = %q, want %q", agent.ID, project, want)
		}
	}
	dests, err := resolveTargets(nil, false, "base")
	if err != nil || len(dests) != 1 || dests[0].path != filepath.Join("base", skillName) {
		t.Fatalf("--dir targets = %+v, %v", dests, err)
	}
}

func TestSkillDetectionLooksUnderHomeAndTheProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	t.Chdir(project)
	if err := os.MkdirAll(filepath.Join(project, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	ids := func(agents []skill.Agent) []string {
		var out []string
		for _, agent := range agents {
			out = append(out, agent.ID)
		}
		return out
	}
	if got := ids(detectAgents(false)); !reflect.DeepEqual(got, []string{"codex"}) {
		t.Fatalf("global detection = %v", got)
	}
	if got := ids(detectAgents(true)); !reflect.DeepEqual(got, []string{"cursor"}) {
		t.Fatalf("project detection = %v", got)
	}
}

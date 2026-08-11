package app

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentIDsIncludeGrok(t *testing.T) {
	t.Parallel()
	ids := agentIDs()
	want := map[string]bool{"claude-code": true, "codex": true, "grok": true}
	if len(ids) != len(want) {
		t.Fatalf("agentIDs() = %v, want %d entries", ids, len(want))
	}
	for _, id := range ids {
		if !want[id] {
			t.Errorf("unexpected agent id %q", id)
		}
		delete(want, id)
	}
	for missing := range want {
		t.Errorf("missing agent id %q", missing)
	}
}

func TestGrokAgentDest(t *testing.T) {
	t.Parallel()
	spec, ok := agentByID("grok")
	if !ok {
		t.Fatal("grok agentSpec is missing")
	}
	if spec.homeSub != ".grok" {
		t.Fatalf("homeSub = %q, want .grok", spec.homeSub)
	}
	if spec.projectSkills != ".grok/skills" {
		t.Fatalf("projectSkills = %q, want .grok/skills", spec.projectSkills)
	}

	projectPath, err := agentDest(spec, true)
	if err != nil {
		t.Fatal(err)
	}
	wantProject := filepath.Join(".grok", "skills", "confluence")
	if projectPath != wantProject {
		t.Fatalf("project dest = %q, want %q", projectPath, wantProject)
	}

	homePath, err := agentDest(spec, false)
	if err != nil {
		t.Fatal(err)
	}
	suffix := filepath.Join(".grok", "skills", "confluence")
	if !strings.HasSuffix(homePath, suffix) {
		t.Fatalf("home dest %q does not end with %q", homePath, suffix)
	}
}

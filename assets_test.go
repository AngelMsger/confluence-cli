package confluencecli

import (
	"strings"
	"testing"
)

// codexDescriptionLimit is the maximum Skill `description` length Codex accepts;
// a longer one fails to load with "invalid description: exceeds maximum length
// of 1024 characters". Keep the embedded Skill under it.
const codexDescriptionLimit = 1024

func TestEmbeddedSkillDescriptionWithinCodexLimit(t *testing.T) {
	data, err := SkillFS.ReadFile(SkillRoot + "/SKILL.md")
	if err != nil {
		t.Fatalf("reading embedded SKILL.md: %v", err)
	}
	desc := frontmatterDescription(string(data))
	if desc == "" {
		t.Fatal("no description found in SKILL.md frontmatter")
	}
	if n := len(desc); n > codexDescriptionLimit {
		t.Errorf("SKILL.md description is %d chars, exceeds the %d-char Codex limit; shorten it",
			n, codexDescriptionLimit)
	}
}

// frontmatterDescription returns the single-line YAML `description:` value
// (quotes stripped) from a Skill's frontmatter, or "" if absent.
func frontmatterDescription(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(line, "description:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		v = strings.TrimPrefix(v, `"`)
		v = strings.TrimSuffix(v, `"`)
		return v
	}
	return ""
}

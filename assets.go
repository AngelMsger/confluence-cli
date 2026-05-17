// Package confluencecli is the module root. It exists only to embed packaged
// assets — the companion `confluence` Skill — into the CLI binary, so that
// `confluence-cli skill install` can deploy a version-matched copy regardless
// of how the binary itself was installed (npm, go install, prebuilt, source).
package confluencecli

import "embed"

// SkillFS holds the companion Skill, rooted at "skills/confluence".
//
//go:embed all:skills/confluence
var SkillFS embed.FS

// SkillRoot is the path within SkillFS at which the Skill is rooted.
const SkillRoot = "skills/confluence"

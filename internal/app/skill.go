package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	confluencecli "github.com/angelmsger/confluence-cli"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// newSkillCmd manages the companion `confluence` Skill, which is embedded in
// the binary so it always matches the installed CLI version.
func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the companion Skill for coding agents",
	}
	cmd.AddCommand(newSkillInstallCmd(), newSkillPathCmd(), newSkillShowCmd())
	return cmd
}

// skillTarget resolves the destination directory for the Skill.
func skillTarget(project bool, dir string) (string, error) {
	if dir != "" {
		return filepath.Join(dir, "confluence"), nil
	}
	base := ".claude/skills"
	if !project {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", cerrors.Wrap(err, cerrors.CategoryConfig, "NO_HOME",
				"could not determine the home directory")
		}
		base = filepath.Join(home, ".claude", "skills")
	}
	return filepath.Join(base, "confluence"), nil
}

func newSkillInstallCmd() *cobra.Command {
	var (
		project bool
		dir     string
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Deploy the embedded Skill into an agent's skills directory",
		Long: "Write the companion `confluence` Skill — bundled inside this binary —\n" +
			"into a coding agent's skills directory. Re-run it after upgrading the\n" +
			"CLI to refresh the Skill to the matching version.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dest, err := skillTarget(project, dir)
			if err != nil {
				return err
			}
			n, err := writeSkill(dest)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "installed confluence Skill %s -> %s (%d files)\n",
				embeddedSkillVersion(), dest, n)
			return nil
		},
	}
	cmd.Flags().BoolVar(&project, "project", false,
		"install into ./.claude/skills instead of ~/.claude/skills")
	cmd.Flags().StringVar(&dir, "dir", "",
		"explicit skills base directory (for agents other than Claude Code)")
	return cmd
}

func newSkillPathCmd() *cobra.Command {
	var (
		project bool
		dir     string
	)
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Print where the Skill would be installed",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dest, err := skillTarget(project, dir)
			if err != nil {
				return err
			}
			status := "not installed"
			if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err == nil {
				status = "installed"
			}
			fmt.Fprintf(os.Stdout, "%s (%s)\n", dest, status)
			return nil
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "use ./.claude/skills")
	cmd.Flags().StringVar(&dir, "dir", "", "explicit skills base directory")
	return cmd
}

func newSkillShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the embedded SKILL.md to stdout",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := confluencecli.SkillFS.ReadFile(confluencecli.SkillRoot + "/SKILL.md")
			if err != nil {
				return cerrors.Wrap(err, cerrors.CategoryInternal, "SKILL_READ",
					"failed to read the embedded Skill")
			}
			_, err = os.Stdout.Write(data)
			return err
		},
	}
}

// writeSkill copies the embedded Skill tree into dest, replacing any existing
// copy. It returns the number of files written.
func writeSkill(dest string) (int, error) {
	sub, err := fs.Sub(confluencecli.SkillFS, confluencecli.SkillRoot)
	if err != nil {
		return 0, cerrors.Wrap(err, cerrors.CategoryInternal, "SKILL_FS",
			"failed to open the embedded Skill")
	}
	// Replace any previous copy so removed files do not linger.
	if err := os.RemoveAll(dest); err != nil {
		return 0, cerrors.Wrap(err, cerrors.CategoryConfig, "SKILL_CLEAN",
			"failed to clear the existing Skill directory")
	}

	count := 0
	walkErr := fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dest, p)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(sub, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		count++
		return nil
	})
	if walkErr != nil {
		return count, cerrors.Wrap(walkErr, cerrors.CategoryConfig, "SKILL_WRITE",
			"failed to write the Skill files")
	}
	return count, nil
}

// embeddedSkillVersion reads the `version:` field from the embedded SKILL.md.
func embeddedSkillVersion() string {
	data, err := confluencecli.SkillFS.ReadFile(confluencecli.SkillRoot + "/SKILL.md")
	if err != nil {
		return "(unknown)"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "version:") {
			return "v" + strings.TrimSpace(strings.TrimPrefix(line, "version:"))
		}
	}
	return "(unknown)"
}

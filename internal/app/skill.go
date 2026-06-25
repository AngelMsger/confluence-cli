package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	confluencecli "github.com/angelmsger/confluence-cli"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// skillResult is the result shape for a skill install / uninstall / path entry.
type skillResult struct {
	Agent   string `json:"agent,omitempty"`
	Path    string `json:"path"`
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	Files   int    `json:"files,omitempty"`
}

// newSkillCmd manages the companion `confluence` Skill, which is embedded in
// the binary so it always matches the installed CLI version.
func newSkillCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the companion Skill for coding agents (Claude Code, Codex)",
	}
	cmd.AddCommand(
		newSkillInstallCmd(s),
		newSkillUninstallCmd(s),
		newSkillPathCmd(s),
		newSkillStatusCmd(s),
		newSkillShowCmd(),
	)
	return cmd
}

// newSkillStatusCmd reports, in one shot, whether the companion Skill is loaded
// into the current agent context (via the CONFLUENCE_CLI_SKILL handshake) and
// where it is installed on disk — plus the single next action to take. It is the
// command an agent runs to self-check after seeing the discovery nudge.
func newSkillStatusCmd(s *appState) *cobra.Command {
	var project bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report whether the companion Skill is loaded and installed",
		RunE: func(cmd *cobra.Command, _ []string) error {
			loaded := os.Getenv(envSkillLoaded) != ""

			installs := make([]skillResult, 0, len(agentSpecs))
			anyInstalled := false
			for _, spec := range agentSpecs {
				p, err := agentDest(spec, project)
				if err != nil {
					return err
				}
				status := "not_installed"
				if _, err := os.Stat(filepath.Join(p, "SKILL.md")); err == nil {
					status = "installed"
					anyInstalled = true
				}
				installs = append(installs, skillResult{Agent: spec.id, Path: p, Status: status})
			}
			sort.Slice(installs, func(i, j int) bool { return installs[i].Agent < installs[j].Agent })

			next := ""
			switch {
			case loaded:
				next = "" // nothing to do; the Skill is in context
			case anyInstalled:
				next = "the Skill is installed but not loaded — load it before composing commands"
			default:
				next = "run `" + constants.AppName + " skill install` then load it"
			}

			return s.emit(map[string]any{
				"loaded":           loaded,
				"loaded_env":       envSkillLoaded,
				"embedded_version": embeddedSkillVersion(),
				"installs":         installs,
				"next":             next,
			})
		},
	}
	cmd.Flags().BoolVar(&project, "project", false,
		"check the project skills dirs (./.claude/skills, ./.agents/skills) instead of $HOME")
	return cmd
}

// agentSpec describes where a coding agent loads Skills from, and how to tell
// that the agent is in use on this machine / in this project.
type agentSpec struct {
	id            string   // "claude-code" / "codex"
	homeSub       string   // dir under $HOME holding the global skills dir
	homeMarkers   []string // paths under $HOME proving the agent is installed
	projectSkills string   // skills dir relative to the project root
	projectMarks  []string // paths under the cwd proving the agent is used here
}

// agentSpecs lists every coding agent the Skill can be installed for.
var agentSpecs = []agentSpec{
	{
		id:            "claude-code",
		homeSub:       ".claude",
		homeMarkers:   []string{".claude"},
		projectSkills: ".claude/skills",
		projectMarks:  []string{".claude"},
	},
	{
		id:            "codex",
		homeSub:       ".codex",
		homeMarkers:   []string{".codex"},
		projectSkills: ".agents/skills",
		projectMarks:  []string{".agents", "AGENTS.md"},
	},
}

func agentByID(id string) (agentSpec, bool) {
	for _, s := range agentSpecs {
		if s.id == id {
			return s, true
		}
	}
	return agentSpec{}, false
}

func agentIDs() []string {
	ids := make([]string, len(agentSpecs))
	for i, s := range agentSpecs {
		ids[i] = s.id
	}
	return ids
}

// skillDest is a resolved install location for the Skill.
type skillDest struct {
	agent string // agent id, or "" for an explicit --dir target
	path  string // the `confluence` directory itself
}

// agentDest returns the `confluence` Skill directory for an agent.
func agentDest(spec agentSpec, project bool) (string, error) {
	if project {
		return filepath.Join(spec.projectSkills, "confluence"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", cerrors.Wrap(err, cerrors.CategoryConfig, "NO_HOME",
			"could not determine the home directory")
	}
	return filepath.Join(home, spec.homeSub, "skills", "confluence"), nil
}

// detectAgents returns the agents whose directories exist — globally (under
// $HOME) or, with project set, in the current directory.
func detectAgents(project bool) []agentSpec {
	var found []agentSpec
	base := "."
	if !project {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		base = home
	}
	for _, spec := range agentSpecs {
		markers := spec.homeMarkers
		if project {
			markers = spec.projectMarks
		}
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(base, m)); err == nil {
				found = append(found, spec)
				break
			}
		}
	}
	return found
}

// resolveTargets maps the install/uninstall flags to concrete destinations.
func resolveTargets(agents []string, project bool, dir string) ([]skillDest, error) {
	if dir != "" {
		if len(agents) > 0 || project {
			return nil, cerrors.New(cerrors.CategoryUsage, "SKILL_FLAGS",
				"--dir cannot be combined with --agent or --project").
				WithHint("--dir is an explicit, agent-agnostic path; drop --agent/--project")
		}
		return []skillDest{{path: filepath.Join(dir, "confluence")}}, nil
	}

	var specs []agentSpec
	if len(agents) > 0 {
		for _, a := range agents {
			spec, ok := agentByID(a)
			if !ok {
				return nil, cerrors.Newf(cerrors.CategoryUsage, "SKILL_AGENT",
					"unknown agent %q", a).
					WithHint("supported agents: " + strings.Join(agentIDs(), ", "))
			}
			specs = append(specs, spec)
		}
	} else {
		specs = detectAgents(project)
		if len(specs) == 0 {
			return nil, cerrors.New(cerrors.CategoryUsage, "SKILL_NO_AGENT",
				"no coding agent detected").
				WithHint("pass --agent (" + strings.Join(agentIDs(), ", ") +
					") or --dir <path> to choose a target explicitly")
		}
	}

	var dests []skillDest
	for _, spec := range specs {
		p, err := agentDest(spec, project)
		if err != nil {
			return nil, err
		}
		dests = append(dests, skillDest{agent: spec.id, path: p})
	}
	return dests, nil
}

func newSkillInstallCmd(s *appState) *cobra.Command {
	var (
		project bool
		dir     string
		agents  []string
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Deploy the embedded Skill into a coding agent's skills directory",
		Long: "Write the companion `confluence` Skill — bundled inside this binary —\n" +
			"into a coding agent's skills directory. With no flags it probes for\n" +
			"installed agents (Claude Code, Codex) and installs into each one found.\n" +
			"Re-run it after upgrading the CLI to refresh the Skill to the matching\n" +
			"version.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dests, err := resolveTargets(agents, project, dir)
			if err != nil {
				return err
			}
			results := make([]skillResult, 0, len(dests))
			for _, d := range dests {
				n, err := writeSkill(d.path)
				if err != nil {
					return err
				}
				results = append(results, skillResult{
					Agent: d.agent, Path: d.path, Status: "installed",
					Version: embeddedSkillVersion(), Files: n,
				})
			}
			return s.emit(results)
		},
	}
	cmd.Flags().BoolVar(&project, "project", false,
		"install into the project (./.claude/skills, ./.agents/skills) instead of $HOME")
	cmd.Flags().StringVar(&dir, "dir", "",
		"explicit skills base directory; installs into <dir>/confluence")
	cmd.Flags().StringSliceVar(&agents, "agent", nil,
		"target agents instead of auto-detecting ("+strings.Join(agentIDs(), ", ")+")")
	return cmd
}

func newSkillUninstallCmd(s *appState) *cobra.Command {
	var (
		project bool
		dir     string
		agents  []string
	)
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the companion Skill from a coding agent's skills directory",
		Long: "Delete a previously installed `confluence` Skill. With no flags it\n" +
			"probes for installed agents (Claude Code, Codex) and removes the Skill\n" +
			"from each one found.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dests, err := resolveTargets(agents, project, dir)
			if err != nil {
				return err
			}
			results := make([]skillResult, 0, len(dests))
			for _, d := range dests {
				if _, statErr := os.Stat(filepath.Join(d.path, "SKILL.md")); statErr != nil {
					results = append(results, skillResult{
						Agent: d.agent, Path: d.path, Status: "not_installed",
					})
					continue
				}
				if err := os.RemoveAll(d.path); err != nil {
					return cerrors.Wrap(err, cerrors.CategoryConfig, "SKILL_REMOVE",
						"failed to remove the Skill directory")
				}
				results = append(results, skillResult{
					Agent: d.agent, Path: d.path, Status: "removed",
				})
			}
			return s.emit(results)
		},
	}
	cmd.Flags().BoolVar(&project, "project", false,
		"remove from the project (./.claude/skills, ./.agents/skills) instead of $HOME")
	cmd.Flags().StringVar(&dir, "dir", "",
		"explicit skills base directory; removes <dir>/confluence")
	cmd.Flags().StringSliceVar(&agents, "agent", nil,
		"target agents instead of auto-detecting ("+strings.Join(agentIDs(), ", ")+")")
	return cmd
}

func newSkillPathCmd(s *appState) *cobra.Command {
	var (
		project bool
		dir     string
		agents  []string
	)
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Print where the Skill would be installed, and whether it is",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var dests []skillDest
			if dir != "" || len(agents) > 0 {
				resolved, err := resolveTargets(agents, project, dir)
				if err != nil {
					return err
				}
				dests = resolved
			} else {
				// No flags: list every known agent so the user sees all options.
				for _, spec := range agentSpecs {
					p, err := agentDest(spec, project)
					if err != nil {
						return err
					}
					dests = append(dests, skillDest{agent: spec.id, path: p})
				}
				sort.Slice(dests, func(i, j int) bool { return dests[i].agent < dests[j].agent })
			}
			results := make([]skillResult, 0, len(dests))
			for _, d := range dests {
				status := "not_installed"
				if _, err := os.Stat(filepath.Join(d.path, "SKILL.md")); err == nil {
					status = "installed"
				}
				results = append(results, skillResult{
					Agent: d.agent, Path: d.path, Status: status,
				})
			}
			return s.emit(results)
		},
	}
	cmd.Flags().BoolVar(&project, "project", false,
		"use the project skills directories instead of $HOME")
	cmd.Flags().StringVar(&dir, "dir", "", "explicit skills base directory")
	cmd.Flags().StringSliceVar(&agents, "agent", nil,
		"limit to specific agents ("+strings.Join(agentIDs(), ", ")+")")
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

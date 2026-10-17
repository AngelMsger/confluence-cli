package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angelmsger/agent-cli-kit/skill"
	confluencecli "github.com/angelmsger/confluence-cli"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// skillResult is the result shape for a skill install / uninstall / path entry.
type skillResult struct {
	Agent     string `json:"agent,omitempty"`
	Path      string `json:"path"`
	Status    string `json:"status"`
	Version   string `json:"version,omitempty"`
	Alignment string `json:"alignment,omitempty"`
	Files     int    `json:"files,omitempty"`
}

// skillName is the Skill's directory name inside an agent's skills directory.
const skillName = "confluence"

// newSkillCmd manages the companion `confluence` Skill, which is embedded in
// the binary so it always matches the installed CLI version.
func newSkillCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the companion Skill for coding agents",
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

func newSkillStatusCmd(s *appState) *cobra.Command {
	var project bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report loaded, installed and embedded Skill versions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			load := currentSkillLoadState()
			installs, err := inspectSkillInstalls(project)
			if err != nil {
				return err
			}
			anyCurrent := false
			needsRefresh := false
			for _, install := range installs {
				anyCurrent = anyCurrent || install.Alignment == "current"
				needsRefresh = needsRefresh || (install.Status == "installed" && install.Alignment != "current")
			}

			nextSteps := []string{}
			switch {
			case load.Status == "current" && needsRefresh:
				nextSteps = []string{constants.AppName + " skill install"}
			case load.Status == "current":
			case load.Loaded && anyCurrent:
				nextSteps = []string{"reload the agent context so it loads the refreshed Skill"}
			case load.Loaded:
				nextSteps = []string{
					constants.AppName + " skill install",
					"reload the agent context so it loads the refreshed Skill",
				}
			case anyCurrent:
				nextSteps = []string{"reload the agent context before composing commands"}
			default:
				nextSteps = []string{
					constants.AppName + " skill install",
					"reload the agent context so it loads the installed Skill",
				}
			}
			next := ""
			if len(nextSteps) > 0 {
				next = nextSteps[0]
			}

			return s.emit(map[string]any{
				"loaded":           load.Loaded,
				"loaded_env":       envSkillLoaded,
				"loaded_version":   load.Version,
				"loaded_status":    load.Status,
				"embedded_version": embeddedSkillVersion(),
				"installs":         installs,
				"next":             next,
				"next_steps":       nextSteps,
			})
		},
	}
	cmd.Flags().BoolVar(&project, "project", false,
		"check each agent's project skills directory instead of $HOME")
	return cmd
}

// skillDest is a resolved install location for the Skill.
type skillDest struct {
	agent string // agent id, or "" for an explicit --dir target
	path  string // the `confluence` directory itself
}

// agentDest returns the `confluence` Skill directory for an agent. The kit owns
// the directory table; this CLI owns the home lookup and its error.
func agentDest(agent skill.Agent, project bool) (string, error) {
	home := ""
	if !project {
		var err error
		if home, err = os.UserHomeDir(); err != nil {
			return "", cerrors.Wrap(err, cerrors.CategoryConfig, "NO_HOME",
				"could not determine the home directory")
		}
	}
	return agent.Destination(home, skillName, project), nil
}

// detectAgents returns the agents whose directories exist — globally (under
// $HOME) or, with project set, in the current directory.
func detectAgents(project bool) []skill.Agent {
	base := "."
	if !project {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		base = home
	}
	return skill.Detect(os.DirFS(base), project)
}

// resolveTargets maps the install/uninstall flags to concrete destinations.
func resolveTargets(agents []string, project bool, dir string) ([]skillDest, error) {
	if dir != "" {
		if len(agents) > 0 || project {
			return nil, cerrors.New(cerrors.CategoryUsage, "SKILL_FLAGS",
				"--dir cannot be combined with --agent or --project").
				WithHint("--dir is an explicit, agent-agnostic path; drop --agent/--project")
		}
		return []skillDest{{path: filepath.Join(dir, skillName)}}, nil
	}

	var specs []skill.Agent
	if len(agents) > 0 {
		for _, a := range agents {
			spec, ok := skill.AgentByID(a)
			if !ok {
				return nil, cerrors.Newf(cerrors.CategoryUsage, "SKILL_AGENT",
					"unknown agent %q", a).
					WithHint("supported agents: " + strings.Join(skill.AgentIDs(), ", "))
			}
			specs = append(specs, spec)
		}
	} else {
		specs = detectAgents(project)
		if len(specs) == 0 {
			return nil, cerrors.New(cerrors.CategoryUsage, "SKILL_NO_AGENT",
				"no coding agent detected").
				WithHint("pass --agent (" + strings.Join(skill.AgentIDs(), ", ") +
					") or --dir <path> to choose a target explicitly")
		}
	}

	var dests []skillDest
	for _, spec := range specs {
		p, err := agentDest(spec, project)
		if err != nil {
			return nil, err
		}
		dests = append(dests, skillDest{agent: spec.ID, path: p})
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
			"installed agents (" + strings.Join(skill.AgentIDs(), ", ") + ") and installs\n" +
			"into each one found. Re-run it after upgrading the CLI to refresh the\n" +
			"Skill to the matching version, then reload the agent context.",
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
					Version: embeddedSkillVersion(), Alignment: "current", Files: n,
				})
			}
			return s.emit(results)
		},
	}
	cmd.Flags().BoolVar(&project, "project", false,
		"install into each agent's project skills directory instead of $HOME")
	cmd.Flags().StringVar(&dir, "dir", "",
		"explicit skills base directory; installs into <dir>/confluence")
	cmd.Flags().StringSliceVar(&agents, "agent", nil,
		"target agents instead of auto-detecting ("+strings.Join(skill.AgentIDs(), ", ")+")")
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
			"probes for installed agents (" + strings.Join(skill.AgentIDs(), ", ") + ")\n" +
			"and removes the Skill from each one found.",
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
		"remove from each agent's project skills directory instead of $HOME")
	cmd.Flags().StringVar(&dir, "dir", "",
		"explicit skills base directory; removes <dir>/confluence")
	cmd.Flags().StringSliceVar(&agents, "agent", nil,
		"target agents instead of auto-detecting ("+strings.Join(skill.AgentIDs(), ", ")+")")
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
		Short: "Print Skill paths, installation state and version alignment",
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
				for _, spec := range skill.Agents() {
					p, err := agentDest(spec, project)
					if err != nil {
						return err
					}
					dests = append(dests, skillDest{agent: spec.ID, path: p})
				}
				sort.Slice(dests, func(i, j int) bool { return dests[i].agent < dests[j].agent })
			}
			results := make([]skillResult, 0, len(dests))
			for _, d := range dests {
				results = append(results, inspectSkillInstall(d.agent, d.path))
			}
			return s.emit(results)
		},
	}
	cmd.Flags().BoolVar(&project, "project", false,
		"use the project skills directories instead of $HOME")
	cmd.Flags().StringVar(&dir, "dir", "", "explicit skills base directory")
	cmd.Flags().StringSliceVar(&agents, "agent", nil,
		"limit to specific agents ("+strings.Join(skill.AgentIDs(), ", ")+")")
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
	return skill.Version(data)
}

func currentSkillLoadState() skill.LoadState {
	return skill.Loaded(os.Getenv(envSkillLoaded), embeddedSkillVersion())
}

func inspectSkillInstall(agent, path string) skillResult {
	install := skill.Inspect(os.DirFS(path), "SKILL.md", embeddedSkillVersion())
	return skillResult{
		Agent: agent, Path: path, Status: install.Status,
		Version: install.Version, Alignment: install.Alignment,
	}
}

func inspectSkillInstalls(project bool) ([]skillResult, error) {
	agents := skill.Agents()
	installs := make([]skillResult, 0, len(agents))
	for _, agent := range agents {
		path, err := agentDest(agent, project)
		if err != nil {
			return nil, err
		}
		installs = append(installs, inspectSkillInstall(agent.ID, path))
	}
	sort.Slice(installs, func(i, j int) bool { return installs[i].Agent < installs[j].Agent })
	return installs, nil
}

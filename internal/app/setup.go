package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
)

func newConfigSetContextCmd(s *appState) *cobra.Command {
	var overwrite, activate, dryRun bool
	cmd := &cobra.Command{Use: "set-context <name>", Short: "Configure service presets without credentials or network access", Args: cobra.ExactArgs(1),
		Long:    "Write service fields from flags, environment or .env over the named target context. Personal environment fields and secrets are ignored. Existing conflicting fields require --overwrite; unrelated contexts and user identities are preserved.",
		Example: "  confluence-cli config set-context team --base-url https://service.example.com --dry-run\n  confluence-cli config set-context team --base-url https://service.example.com --activate",
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _, err := config.ReadFile(s.cfgDir)
			if err != nil {
				return cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_READ", "failed to read config")
			}
			plan, err := config.PlanServiceContext(file, args[0], s.resolved, overwrite, activate)
			if err != nil {
				return err
			}
			if plan.Changed && !dryRun {
				if err := config.WriteFile(s.cfgDir, plan.File); err != nil {
					return cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_WRITE", "failed to write service presets")
				}
			}
			return s.emit(struct {
				config.ContextPlan
				DryRun     bool   `json:"dry_run"`
				ConfigFile string `json:"config_file"`
			}{plan, dryRun, config.ConfigFilePath(s.cfgDir)})
		},
	}
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "update conflicting service fields; preserve personal identity and credentials")
	cmd.Flags().BoolVar(&activate, "activate", false, "make this the current context")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview configuration changes without writing files or credentials")
	return cmd
}

func newAuthGuideCmd(s *appState) *cobra.Command {
	return &cobra.Command{Use: "guide", Short: "Show offline credential acquisition guidance for this service", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			g, err := config.Guide(s.cfg(), s.resolved.Sources)
			if err != nil {
				return err
			}
			if name := s.resolved.ActiveContext; name != "" {
				quoted := "'" + strings.ReplaceAll(name, "'", "'\"'\"'") + "'"
				for i, step := range g.NextSteps {
					g.NextSteps[i] = strings.Replace(step, " auth ", " --use-context "+quoted+" auth ", 1)
				}
			}
			return s.emit(g)
		},
	}
}

func printAuthGuide(cfg config.Config) error {
	g, err := config.Guide(cfg, nil)
	if err != nil {
		return err
	}
	for _, line := range g.Lines() {
		fmt.Fprintln(os.Stderr, line)
	}
	return nil
}

// setupPrefill applies presets to the actual wizard destination, not the active context.
func (s *appState) setupPrefill(name string, _ *config.NamedContext) (*config.NamedContext, error) {
	resolved, err := config.Load(config.LoadOptions{ConfigDir: s.cfgDir, Context: name, Setup: true, Flags: config.FlagValues{
		BaseURL: s.gflags.baseURL, AuthScheme: s.gflags.authScheme, CredentialURL: s.gflags.credentialURL, Flavor: s.gflags.flavor,
	}})
	if err != nil {
		return nil, err
	}
	cfg := resolved.Config
	if resolved.Sources[config.FieldAuthScheme] == "default" {
		cfg.Auth.Scheme = ""
	}
	return &config.NamedContext{Name: name, BaseURL: cfg.BaseURL, Auth: cfg.Auth, Flavor: cfg.Flavor, DetectedFlavor: cfg.DetectedFlavor}, nil
}

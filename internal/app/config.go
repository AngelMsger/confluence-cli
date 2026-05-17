package app

import (
	"context"
	"fmt"
	"os"

	"github.com/angelmsger/confluence-cli/internal/apiclient"
	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

func newConfigCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage confluence-cli configuration",
	}
	cmd.AddCommand(newConfigInitCmd(s), newConfigShowCmd(s), newConfigPathCmd(s))
	return cmd
}

func newConfigInitCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactively set up server URL and credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := config.RunWizard(os.Stdin, os.Stdout, wizardHooks(s))
			if err != nil {
				return cerrors.Wrap(err, cerrors.CategoryConfig, "INIT_ABORTED", err.Error())
			}
			if err := config.WriteConfigFile(s.cfgDir, result.Config); err != nil {
				return cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_WRITE",
					"failed to write the config file")
			}
			cred := credentialFrom(result.Config, result.Secrets)
			backend, err := auth.Save(result.Config.BaseURL, cred, s.store)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "\nConfiguration saved to %s\n", config.ConfigFilePath(s.cfgDir))
			fmt.Fprintf(os.Stdout, "Credential stored in the %s.\n", backend)
			fmt.Fprintln(os.Stdout, "\nNext steps:")
			for _, step := range config.SuggestedNextSteps() {
				fmt.Fprintf(os.Stdout, "  %s\n", step)
			}
			return nil
		},
	}
}

func newConfigShowCmd(s *appState) *cobra.Command {
	var explain bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the resolved configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := s.cfg()
			view := map[string]any{
				"server":      cfg.BaseURL,
				"flavor":      cfg.Flavor,
				"auth.scheme": cfg.Auth.Scheme,
				"auth.user":   cfg.Auth.Username,
				"format":      cfg.Defaults.Format,
				"page_size":   cfg.Defaults.PageSize,
				"timeout":     cfg.Defaults.Timeout.String(),
			}
			if explain {
				src := s.resolved.Sources
				view["server"] = explained(cfg.BaseURL, src, config.FieldServer)
				view["flavor"] = explained(cfg.Flavor, src, config.FieldFlavor)
				view["format"] = explained(cfg.Defaults.Format, src, config.FieldFormat)
			}
			return s.emit(view)
		},
	}
	cmd.Flags().BoolVar(&explain, "explain", false, "annotate each value with its source")
	return cmd
}

func explained(value string, sources map[string]string, field string) string {
	return fmt.Sprintf("%s (from %s)", value, config.ExplainField(sources, field))
}

func newConfigPathCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(os.Stdout, config.ConfigFilePath(s.cfgDir))
			return nil
		},
	}
}

// credentialFrom builds a Credential from a config + secrets pair.
func credentialFrom(cfg config.Config, secrets config.Secrets) auth.Credential {
	cred := auth.Credential{Scheme: cfg.Auth.Scheme, Username: cfg.Auth.Username}
	switch cfg.Auth.Scheme {
	case auth.SchemePAT:
		cred.Secret = secrets.PAT
	case auth.SchemeBasic:
		if secrets.Password != "" {
			cred.Secret = secrets.Password
		} else {
			cred.Secret = secrets.APIToken
		}
	}
	return cred
}

// wizardHooks builds the live detection / validation callbacks for `config init`.
func wizardHooks(s *appState) config.WizardHooks {
	return config.WizardHooks{
		DetectFlavor: func(baseURL string) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), s.timeout())
			defer cancel()
			tc := buildProbeTransport(s)
			f, err := apiclient.Detect(ctx, tc, baseURL)
			return string(f), err
		},
		Validate: func(cfg config.Config, secrets config.Secrets) error {
			ctx, cancel := context.WithTimeout(context.Background(), s.timeout())
			defer cancel()
			cred := credentialFrom(cfg, secrets)
			if err := cred.Validate(); err != nil {
				return err
			}
			client, _, err := apiclient.Build(ctx, apiclient.BuildParams{
				BaseURL:       cfg.BaseURL,
				Flavor:        cfg.Flavor,
				AuthDecorator: cred.Decorator(),
				Timeout:       cfg.Defaults.Timeout,
				MaxRetries:    cfg.Defaults.MaxRetries,
			})
			if err != nil {
				return err
			}
			_, err = client.Ping(ctx)
			return err
		},
	}
}

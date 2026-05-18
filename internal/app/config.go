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

// contextRow is the result shape for `config get-contexts`.
type contextRow struct {
	Current    bool   `json:"current"`
	Name       string `json:"name"`
	Server     string `json:"server,omitempty"`
	Flavor     string `json:"flavor,omitempty"`
	AuthScheme string `json:"auth_scheme,omitempty"`
}

// configInitOutput is the result shape emitted by `config init`.
type configInitOutput struct {
	ConfigFile string              `json:"config_file"`
	Contexts   []initContextResult `json:"contexts"`
	NextSteps  []string            `json:"next_steps"`
}

type initContextResult struct {
	Name              string `json:"name"`
	CredentialBackend string `json:"credential_backend"`
}

func newConfigCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage confluence-cli configuration",
	}
	cmd.AddCommand(
		newConfigInitCmd(s), newConfigShowCmd(s), newConfigPathCmd(s),
		newConfigGetContextsCmd(s), newConfigUseContextCmd(s), newConfigDeleteContextCmd(s),
	)
	return cmd
}

func newConfigInitCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactively set up server URL and credentials",
		Long: "Run the interactive setup wizard. It collects a server URL, detects\n" +
			"the flavor, validates a credential and stores it. The wizard can also\n" +
			"configure additional named contexts for working with several servers.",
		Example: "  confluence-cli config init",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// The wizard's prompts are human interaction — keep them on stderr
			// so stdout carries only the final JSON result.
			result, err := config.RunWizard(os.Stdin, os.Stderr, wizardHooks(s))
			if err != nil {
				return cerrors.Wrap(err, cerrors.CategoryConfig, "INIT_ABORTED", err.Error())
			}
			if err := config.WriteFile(s.cfgDir, result.File); err != nil {
				return cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_WRITE",
					"failed to write the config file")
			}
			out := configInitOutput{
				ConfigFile: config.ConfigFilePath(s.cfgDir),
				NextSteps:  config.SuggestedNextSteps(),
			}
			for _, cr := range result.Creds {
				cred := credentialFromContext(cr.Context, cr.Secrets)
				backend, err := auth.Save(cr.Context.BaseURL, cred, s.store)
				if err != nil {
					return err
				}
				out.Contexts = append(out.Contexts, initContextResult{
					Name:              cr.Context.Name,
					CredentialBackend: fmt.Sprint(backend),
				})
			}
			return s.emit(out)
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
			// Surface the context only when more than one is configured, so
			// single-context users never see the concept.
			if len(s.resolved.ContextNames) > 1 {
				view["context"] = s.resolved.ActiveContext
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
			return s.emit(map[string]any{"path": config.ConfigFilePath(s.cfgDir)})
		},
	}
}

// credentialFrom builds a Credential from a config + secrets pair.
func credentialFrom(cfg config.Config, secrets config.Secrets) auth.Credential {
	return credentialOf(cfg.Auth, secrets)
}

// credentialFromContext builds a Credential for a single named context.
func credentialFromContext(nc config.NamedContext, secrets config.Secrets) auth.Credential {
	return credentialOf(nc.Auth, secrets)
}

// credentialOf builds a Credential from auth settings and transient secrets.
func credentialOf(ac config.AuthConfig, secrets config.Secrets) auth.Credential {
	cred := auth.Credential{Scheme: ac.Scheme, Username: ac.Username}
	switch ac.Scheme {
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

// readConfigFile loads the config file for the context subcommands, mapping a
// missing file to a clear error.
func readConfigFile(s *appState) (config.File, error) {
	file, exists, err := config.ReadFile(s.cfgDir)
	if err != nil {
		return config.File{}, cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_READ",
			"failed to read the config file")
	}
	if !exists || len(file.Contexts) == 0 {
		return config.File{}, cerrors.New(cerrors.CategoryConfig, "NO_CONFIG",
			"no configured contexts").
			WithHint("Run `confluence-cli config init` to create one.")
	}
	return file, nil
}

func newConfigGetContextsCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "get-contexts",
		Short: "List the configured contexts",
		Long: "List every context in the config file. The current context — the one\n" +
			"used when --use-context is not given — is marked.",
		Example: "  confluence-cli config get-contexts\n" +
			"  confluence-cli config get-contexts --format table",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			file, _, err := config.ReadFile(s.cfgDir)
			if err != nil {
				return cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_READ",
					"failed to read the config file")
			}
			rows := make([]contextRow, 0, len(file.Contexts))
			for _, c := range file.Contexts {
				rows = append(rows, contextRow{
					Current:    c.Name == file.CurrentContext,
					Name:       c.Name,
					Server:     c.BaseURL,
					Flavor:     c.Flavor,
					AuthScheme: c.Auth.Scheme,
				})
			}
			return s.emit(rows)
		},
	}
}

func newConfigUseContextCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use-context <name>",
		Short: "Switch the current context",
		Long: "Set the current context — the server used by default. Override it for\n" +
			"a single command with the global --use-context flag instead.",
		Example: "  confluence-cli config use-context staging",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			file, err := readConfigFile(s)
			if err != nil {
				return err
			}
			if _, ok := file.Context(name); !ok {
				return cerrors.Newf(cerrors.CategoryConfig, "UNKNOWN_CONTEXT",
					"context %q is not defined", name).
					WithHint("Run `confluence-cli config get-contexts` to list defined contexts.")
			}
			file.CurrentContext = name
			if err := config.WriteFile(s.cfgDir, file); err != nil {
				return cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_WRITE",
					"failed to write the config file")
			}
			return s.emit(map[string]any{"context": name, "status": "current"})
		},
	}
	cmd.ValidArgsFunction = completeContextNames(s)
	return cmd
}

func newConfigDeleteContextCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete-context <name>",
		Short:   "Delete a context and its stored credential",
		Example: "  confluence-cli config delete-context staging",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			file, err := readConfigFile(s)
			if err != nil {
				return err
			}
			target, ok := file.Context(name)
			if !ok {
				return cerrors.Newf(cerrors.CategoryConfig, "UNKNOWN_CONTEXT",
					"context %q is not defined", name).
					WithHint("Run `confluence-cli config get-contexts` to list defined contexts.")
			}
			if len(file.Contexts) == 1 {
				return cerrors.New(cerrors.CategoryUsage, "LAST_CONTEXT",
					"cannot delete the only context")
			}
			scheme := target.Auth.Scheme
			if scheme == "" {
				scheme = auth.SchemePAT
			}
			_ = auth.Forget(target.BaseURL, scheme, s.store)

			kept := file.Contexts[:0]
			for _, c := range file.Contexts {
				if c.Name != name {
					kept = append(kept, c)
				}
			}
			file.Contexts = kept
			if file.CurrentContext == name {
				file.CurrentContext = file.Contexts[0].Name
			}
			if err := config.WriteFile(s.cfgDir, file); err != nil {
				return cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_WRITE",
					"failed to write the config file")
			}
			return s.emit(map[string]any{"context": name, "status": "deleted"})
		},
	}
	cmd.ValidArgsFunction = completeContextNames(s)
	return cmd
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

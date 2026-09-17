package app

import (
	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
)

func newAuthCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Inspect and manage stored credentials",
	}
	cmd.AddCommand(newAuthGuideCmd(s), newAuthStatusCmd(s), newAuthLoginCmd(s), newAuthLogoutCmd(s))
	return cmd
}

// authStatus is the result shape for `auth status`.
type authStatus struct {
	Server     string `json:"server"`
	Flavor     string `json:"flavor"`
	Scheme     string `json:"scheme"`
	Username   string `json:"username,omitempty"`
	Configured bool   `json:"configured"`
	Secret     string `json:"secret,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

func newAuthStatusCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether a usable credential is configured",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := s.cfg()
			st := authStatus{
				Server: cfg.BaseURL, Flavor: cfg.Flavor,
				Scheme: cfg.Auth.Scheme, Username: cfg.Auth.Username,
			}
			cred, err := auth.Resolve(cfg, s.resolved.Secrets, s.store)
			if err != nil {
				st.Configured = false
				st.Detail = cerrors.AsCLIError(err).Message
			} else {
				st.Configured = true
				st.Secret = cred.Redacted().Secret
			}
			return s.emit(st)
		},
	}
}

func newAuthLoginCmd(s *appState) *cobra.Command {
	return &cobra.Command{Use: "login", Short: "Verify and store personal credentials for the configured service", Args: cobra.NoArgs,
		Long: "Reuse the configured service, show its credential acquisition page, and store a verified credential together with its username. Requires a terminal; use credential environment variables for non-interactive execution.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := s.cfg()
			if _, _, err := loginFile(s, cfg, auth.Credential{Scheme: cfg.Auth.Scheme, Username: cfg.Auth.Username}); err != nil {
				return err
			}
			if !stdinIsTTY() {
				return cerrors.New(cerrors.CategoryConfig, "AUTH_LOGIN_NEEDS_TTY", "auth login needs an interactive terminal").WithHint("Use the documented CONFLUENCE credential environment variables for non-interactive execution.").WithNextSteps("confluence-cli auth guide")
			}
			if err := printAuthGuide(cfg); err != nil {
				return err
			}
			cred := auth.Credential{Scheme: cfg.Auth.Scheme, Username: cfg.Auth.Username}
			var err error
			if cred.Scheme == auth.SchemeBasic && cred.Username == "" {
				cred.Username, err = promptLine("Username or email", "")
				if err != nil {
					return err
				}
				cred.Username = config.NormalizeUsername(cred.Username)
			}
			cred.Secret, err = promptSecret(secretLabel(cred.Scheme))
			if err != nil {
				return err
			}
			backend, err := completeLogin(s, cfg, cred, s.loginServices())
			if err != nil {
				return err
			}
			return s.emit(map[string]any{"server": cfg.BaseURL, "scheme": cred.Scheme, "credential_backend": backend, "status": "stored"})
		},
	}
}

func newAuthLogoutCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored credential for the configured server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := s.cfg()
			if cfg.BaseURL == "" {
				return cerrors.New(cerrors.CategoryConfig, "NO_SERVER",
					"no server URL configured")
			}
			scheme := cfg.Auth.Scheme
			if scheme == "" {
				scheme = config.SchemePAT
			}
			if err := auth.Forget(cfg.BaseURL, scheme, s.store); err != nil {
				return err
			}
			return s.emit(map[string]any{"server": cfg.BaseURL, "status": "removed"})
		},
	}
}

func secretLabel(scheme string) string {
	if scheme == auth.SchemeBasic {
		return "Password or API token"
	}
	return "Personal Access Token"
}

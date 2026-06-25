package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	cmd.AddCommand(newAuthStatusCmd(s), newAuthLoginCmd(s), newAuthLogoutCmd(s))
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
	return &cobra.Command{
		Use:   "login",
		Short: "Store a credential for the configured server",
		Long:  "Prompt for a secret and store it securely. Run `config init` first if the server URL is not set.",
		Example: "  confluence-cli auth login\n" +
			"  confluence-cli --use-context staging auth login",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := s.cfg()
			if cfg.BaseURL == "" {
				return cerrors.New(cerrors.CategoryConfig, "NO_SERVER",
					"no server URL configured").
					WithNextSteps("confluence-cli config init")
			}
			// auth login is interactive: it prompts for a secret on stdin. With
			// no terminal (a sandboxed agent, CI without a PTY) the read would
			// block forever, so fail fast and point at the non-interactive paths.
			if !stdinIsTTY() {
				return cerrors.New(cerrors.CategoryConfig, "AUTH_LOGIN_NEEDS_TTY",
					"auth login needs an interactive terminal to prompt for the secret").
					WithHint("Run `confluence-cli auth login` yourself in a terminal, or provide credentials via environment variables (CONFLUENCE_PERSONAL_ACCESS_TOKEN, or CONFLUENCE_USERNAME + CONFLUENCE_API_TOKEN / CONFLUENCE_PASSWORD).")
			}
			r := bufio.NewReader(os.Stdin)
			cred := auth.Credential{Scheme: cfg.Auth.Scheme, Username: cfg.Auth.Username}
			if cred.Scheme == "" {
				cred.Scheme = auth.SchemePAT
			}
			if cred.Scheme == auth.SchemeBasic && cred.Username == "" {
				cred.Username = ask(r, "Username or email")
			}
			cred.Secret = ask(r, secretLabel(cred.Scheme))
			if err := cred.Validate(); err != nil {
				return err
			}
			backend, err := auth.Save(cfg.BaseURL, cred, s.store)
			if err != nil {
				return err
			}
			return s.emit(map[string]any{
				"server":             cfg.BaseURL,
				"scheme":             cred.Scheme,
				"credential_backend": fmt.Sprint(backend),
				"status":             "stored",
			})
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

func ask(r *bufio.Reader, label string) string {
	// Prompts are human interaction — write them to stderr so stdout stays
	// clean JSON.
	fmt.Fprintf(os.Stderr, "%s: ", label)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

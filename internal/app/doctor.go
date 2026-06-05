package app

import (
	"context"
	"net/http"
	"time"

	"github.com/angelmsger/confluence-cli/internal/apiclient"
	"github.com/angelmsger/confluence-cli/internal/auth"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/angelmsger/confluence-cli/internal/update"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	"github.com/spf13/cobra"
)

// updateCheckTimeout caps the release-update lookup so an offline or slow
// network never stalls `doctor` for the full request timeout.
const updateCheckTimeout = 5 * time.Second

// doctorCheck is a single diagnostic result.
type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// doctorReport is the result shape for `doctor`.
type doctorReport struct {
	Healthy bool           `json:"healthy"`
	Checks  []doctorCheck  `json:"checks"`
	Update  *update.Status `json:"update,omitempty"`
}

func newDoctorCmd(s *appState) *cobra.Command {
	var skipUpdate bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose configuration, credentials and connectivity",
		Example: "  confluence-cli doctor\n" +
			"  confluence-cli doctor --no-update-check",
		RunE: func(cmd *cobra.Command, _ []string) error {
			report := runDoctor(s, skipUpdate)
			if err := s.emit(report); err != nil {
				return err
			}
			if !report.Healthy {
				return cerrors.New(cerrors.CategoryConfig, "DOCTOR_UNHEALTHY",
					"one or more diagnostic checks failed").
					WithNextSteps("Review the failing checks above.",
						"confluence-cli config init")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipUpdate, "no-update-check", false,
		"skip the check for a newer confluence-cli release")
	return cmd
}

func runDoctor(s *appState, skipUpdate bool) doctorReport {
	var checks []doctorCheck
	cfg := s.cfg()

	// 1. Configuration.
	cfgOK := cfg.BaseURL != ""
	checks = append(checks, doctorCheck{
		Name: "configuration", OK: cfgOK,
		Detail: pick(cfgOK, "server URL = "+cfg.BaseURL, "no server URL configured"),
	})

	// 2. Credentials.
	cred, credErr := auth.Resolve(cfg, s.resolved.Secrets, s.store)
	credOK := credErr == nil
	checks = append(checks, doctorCheck{
		Name: "credentials", OK: credOK,
		Detail: pick(credOK, "scheme = "+cred.Scheme, detailOf(credErr)),
	})

	// 3. Connectivity + flavor (only when prerequisites pass).
	var (
		client    apiclient.Client
		reachable bool
		doctorCtx context.Context
	)
	if cfgOK && credOK {
		ctx, cancel := cmdContext(s)
		defer cancel()
		doctorCtx = ctx
		var err error
		client, _, err = apiclient.Build(ctx, apiclient.BuildParams{
			BaseURL:       cfg.BaseURL,
			Flavor:        cfg.Flavor,
			AuthDecorator: cred.Decorator(),
			Timeout:       cfg.Defaults.Timeout,
			MaxRetries:    cfg.Defaults.MaxRetries,
		})
		if err != nil {
			client = nil
			checks = append(checks, doctorCheck{Name: "connectivity", OK: false, Detail: detailOf(err)})
		} else {
			info, pingErr := client.Ping(ctx)
			reachable = pingErr == nil
			checks = append(checks, doctorCheck{
				Name: "connectivity", OK: reachable,
				Detail: pick(reachable,
					"reachable, flavor = "+string(info.Flavor), detailOf(pingErr)),
			})
		}
	} else {
		checks = append(checks, doctorCheck{
			Name: "connectivity", OK: false,
			Detail: "skipped: fix configuration and credentials first",
		})
	}

	// Healthy reflects only the checks above; the current-user probe that
	// follows is informational and never fails the command.
	healthy := true
	for _, c := range checks {
		if !c.OK {
			healthy = false
		}
	}

	// 4. Current user — informational only. A failure here does not affect
	// the healthy verdict (some locked-down deployments restrict it).
	if client != nil && reachable {
		user, userErr := client.CurrentUser(doctorCtx)
		checks = append(checks, doctorCheck{
			Name: "current-user", OK: userErr == nil,
			Detail: pick(userErr == nil,
				"authenticated as "+userDisplay(user), detailOf(userErr)),
		})
	}

	report := doctorReport{Healthy: healthy, Checks: checks}

	// Release-update check: informational only, it never affects Healthy and
	// never fails the command — being out of date is not a misconfiguration.
	if !skipUpdate {
		ctx, cancel := updateContext(s)
		defer cancel()
		st := update.Check(ctx, &http.Client{Timeout: updateCheckTimeout}, constants.Version)
		report.Update = &st
	}
	return report
}

// updateContext bounds the release-update lookup by updateCheckTimeout, or the
// configured request timeout when that is shorter.
func updateContext(s *appState) (context.Context, context.CancelFunc) {
	d := updateCheckTimeout
	if t := s.timeout(); t > 0 && t < d {
		d = t
	}
	return context.WithTimeout(context.Background(), d)
}

func pick(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

// userDisplay renders a user for the doctor detail line, preferring the
// display name and falling back to the username or account ID.
func userDisplay(u *apiclient.User) string {
	switch {
	case u == nil:
		return "unknown user"
	case u.DisplayName != "":
		return u.DisplayName
	case u.Username != "":
		return u.Username
	default:
		return u.AccountID
	}
}

func detailOf(err error) string {
	if err == nil {
		return ""
	}
	return cerrors.AsCLIError(err).Message
}

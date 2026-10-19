package app

import (
	"context"
	"net/http"
	"time"

	"github.com/angelmsger/agent-cli-kit/update"
	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// updateCheckTimeout caps the release-update lookup so an offline or slow
// network never stalls `doctor` for the full request timeout.
const updateCheckTimeout = 5 * time.Second

// doctorCheck is a single diagnostic result.
type doctorCheck struct {
	Name          string `json:"name"`
	OK            bool   `json:"ok"`
	Status        string `json:"status"`
	Detail        string `json:"detail"`
	RecoveryScope string `json:"recovery_scope,omitempty"`
}

// doctorReport is the result shape for `doctor`.
type doctorReport struct {
	Healthy   bool           `json:"healthy"`
	Checks    []doctorCheck  `json:"checks"`
	NextSteps []string       `json:"next_steps,omitempty"`
	Update    *update.Status `json:"update,omitempty"`
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
				steps := []string{
					"Review the failing checks and their status/recovery_scope fields above.",
					"When recovery_scope is host, retry `confluence-cli doctor` with access to the host user environment.",
				}
				// A stored login goes after the host retry and before any sign-in.
				steps = append(append(steps, report.NextSteps...),
					"Only configure credentials if the host retry also reports them missing.")
				err := cerrors.New(cerrors.CategoryConfig, "DOCTOR_UNHEALTHY",
					"one or more diagnostic checks failed").
					WithNextSteps(steps...)
				if reportNeedsHostRetry(report) {
					err.WithRecovery(cerrors.Recovery{
						Action: "retry_current_command", Scope: "host",
						Requires: []string{"user_home", "os_keychain"},
					})
				}
				return err
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
		Name: "configuration", OK: cfgOK, Status: pick(cfgOK, "ok", "missing"),
		Detail: pick(cfgOK, "server URL = "+cfg.BaseURL, "no server URL configured"),
	})

	// 2. Credentials.
	cred, credErr := auth.Resolve(cfg, s.resolved.Secrets, s.store)
	credOK := credErr == nil
	checks = append(checks, doctorCheck{
		Name: "credentials", OK: credOK,
		Status: diagnosticStatus(credErr), RecoveryScope: diagnosticRecoveryScope(credErr),
		Detail: pick(credOK, "scheme = "+cred.Scheme, detailOf(credErr)),
	})

	// 3. Connectivity + flavor (only when prerequisites pass).
	var (
		client    apiclient.Client
		reachable bool
		doctorCtx context.Context
		connErr   error
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
			client, connErr = nil, err
			checks = append(checks, doctorCheck{Name: "connectivity", OK: false, Status: diagnosticStatus(err), Detail: detailOf(err)})
		} else {
			info, pingErr := client.Ping(ctx)
			reachable, connErr = pingErr == nil, pingErr
			checks = append(checks, doctorCheck{
				Name: "connectivity", OK: reachable, Status: diagnosticStatus(pingErr),
				Detail: pick(reachable,
					"reachable, flavor = "+string(info.Flavor), detailOf(pingErr)),
			})
		}
	} else {
		checks = append(checks, doctorCheck{
			Name: "connectivity", OK: false, Status: "skipped",
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
			Name: "current-user", OK: userErr == nil, Status: diagnosticStatus(userErr),
			Detail: pick(userErr == nil,
				"authenticated as "+userDisplay(user), detailOf(userErr)),
		})
	}
	checks = append(checks, companionSkillDoctorCheck())

	report := doctorReport{Healthy: healthy, Checks: checks}
	if step := cfg.ReuseStep; step != "" && cfgOK && authUnhealthy(credErr, connErr) {
		// A stored login may replace a missing or rejected one; reuse reads
		// the same store, so an unreadable store is recovered on the host.
		report.NextSteps = []string{step}
	}

	// Release-update check: informational only, it never affects Healthy and
	// never fails the command — being out of date is not a misconfiguration.
	if !skipUpdate {
		ctx, cancel := updateContext(s)
		defer cancel()
		st := releaseChecker.Check(ctx, &http.Client{Timeout: updateCheckTimeout}, constants.Version)
		report.Update = &st
	}
	return report
}

func companionSkillDoctorCheck() doctorCheck {
	load := currentSkillLoadState()
	installs, err := inspectSkillInstalls(false)
	if err != nil {
		return doctorCheck{Name: "companion-skill", Status: "invalid", Detail: err.Error()}
	}
	projectInstalls, err := inspectSkillInstalls(true)
	if err != nil {
		return doctorCheck{Name: "companion-skill", Status: "invalid", Detail: err.Error()}
	}
	installs = append(installs, projectInstalls...)
	currentInstalled := false
	installed := false
	for _, item := range installs {
		installed = installed || item.Status == "installed"
		currentInstalled = currentInstalled || item.Alignment == "current"
	}
	switch {
	case load.Status == "current":
		return doctorCheck{Name: "companion-skill", OK: true, Status: "current",
			Detail: "loaded Skill " + load.Version + " matches " + embeddedSkillVersion()}
	case load.Loaded && currentInstalled:
		return doctorCheck{Name: "companion-skill", Status: "reload_required",
			Detail: "current Skill is installed; reload the agent context to load it"}
	case load.Loaded:
		return doctorCheck{Name: "companion-skill", Status: load.Status,
			Detail: "run `" + constants.AppName + " skill install`, then reload the agent context"}
	case currentInstalled:
		return doctorCheck{Name: "companion-skill", OK: true, Status: "installed_not_loaded",
			Detail: "current Skill is installed; reload the agent context to load it"}
	case installed:
		return doctorCheck{Name: "companion-skill", Status: "outdated",
			Detail: "run `" + constants.AppName + " skill install`, then reload the agent context"}
	default:
		return doctorCheck{Name: "companion-skill", Status: "not_installed",
			Detail: "run `" + constants.AppName + " skill install`, then reload the agent context"}
	}
}

func diagnosticStatus(err error) string {
	if err == nil {
		return "ok"
	}
	ce := cerrors.AsCLIError(err)
	switch ce.Code {
	case "CREDENTIAL_STORE_INACCESSIBLE":
		return "inaccessible"
	case "CREDENTIAL_NOT_VISIBLE_OR_MISSING":
		return "missing_or_inaccessible"
	}
	switch ce.Category {
	case cerrors.CategoryAuth, cerrors.CategoryPermission:
		return "rejected_by_server"
	case cerrors.CategoryNetwork, cerrors.CategoryServer:
		return "unreachable"
	default:
		return "invalid"
	}
}

func diagnosticRecoveryScope(err error) string {
	if err == nil {
		return ""
	}
	if recovery := cerrors.AsCLIError(err).Recovery; recovery != nil {
		return recovery.Scope
	}
	return ""
}

// authUnhealthy reports whether the credentials are missing or invalid, or
// the server rejected them (HTTP 401), as opposed to an unreadable store, a
// permission or a network failure.
func authUnhealthy(credErr, connErr error) bool {
	if credErr != nil {
		return cerrors.AsCLIError(credErr).Code != "CREDENTIAL_STORE_INACCESSIBLE"
	}
	if connErr == nil {
		return false
	}
	ce := cerrors.AsCLIError(connErr)
	return ce.Category == cerrors.CategoryAuth && ce.HTTPStatus == 401
}

func reportNeedsHostRetry(report doctorReport) bool {
	for _, check := range report.Checks {
		if check.RecoveryScope == "host" {
			return true
		}
	}
	return false
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

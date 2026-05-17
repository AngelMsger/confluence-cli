package app

import (
	"github.com/angelmsger/confluence-cli/internal/apiclient"
	"github.com/angelmsger/confluence-cli/internal/auth"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

// doctorCheck is a single diagnostic result.
type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// doctorReport is the result shape for `doctor`.
type doctorReport struct {
	Healthy bool          `json:"healthy"`
	Checks  []doctorCheck `json:"checks"`
}

func newDoctorCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose configuration, credentials and connectivity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			report := runDoctor(s)
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
}

func runDoctor(s *appState) doctorReport {
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
	if cfgOK && credOK {
		ctx, cancel := cmdContext(s)
		defer cancel()
		client, _, err := apiclient.Build(ctx, apiclient.BuildParams{
			BaseURL:       cfg.BaseURL,
			Flavor:        cfg.Flavor,
			AuthDecorator: cred.Decorator(),
			Timeout:       cfg.Defaults.Timeout,
			MaxRetries:    cfg.Defaults.MaxRetries,
		})
		if err != nil {
			checks = append(checks, doctorCheck{Name: "connectivity", OK: false, Detail: detailOf(err)})
		} else {
			info, pingErr := client.Ping(ctx)
			checks = append(checks, doctorCheck{
				Name: "connectivity", OK: pingErr == nil,
				Detail: pick(pingErr == nil,
					"reachable, flavor = "+string(info.Flavor), detailOf(pingErr)),
			})
		}
	} else {
		checks = append(checks, doctorCheck{
			Name: "connectivity", OK: false,
			Detail: "skipped: fix configuration and credentials first",
		})
	}

	healthy := true
	for _, c := range checks {
		if !c.OK {
			healthy = false
		}
	}
	return doctorReport{Healthy: healthy, Checks: checks}
}

func pick(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func detailOf(err error) string {
	if err == nil {
		return ""
	}
	return cerrors.AsCLIError(err).Message
}

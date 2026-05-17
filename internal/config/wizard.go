package config

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/angelmsger/confluence-cli/pkg/constants"
)

// WizardHooks lets the caller plug live behaviour into the init wizard without
// config depending on the API client. Either hook may be nil.
type WizardHooks struct {
	// DetectFlavor probes baseURL and returns "cloud" or "datacenter".
	DetectFlavor func(baseURL string) (string, error)
	// Validate performs a live credential check with the chosen settings.
	Validate func(cfg Config, secrets Secrets) error
}

// WizardResult is the outcome of a completed init wizard.
type WizardResult struct {
	Config  Config
	Secrets Secrets
}

// RunWizard drives the interactive `config init` flow over in/out.
func RunWizard(in io.Reader, out io.Writer, hooks WizardHooks) (*WizardResult, error) {
	r := bufio.NewReader(in)
	cfg := configFromMap(defaultLayer())

	fmt.Fprintln(out, "confluence-cli setup")
	fmt.Fprintln(out, "--------------------")

	cfg.BaseURL = prompt(r, out, "Confluence base URL", cfg.BaseURL, true)

	cfg.Flavor = promptChoice(r, out, "Backend flavor",
		[]string{FlavorAuto, FlavorCloud, FlavorDataCenter}, FlavorAuto)
	if cfg.Flavor == FlavorAuto && hooks.DetectFlavor != nil {
		if detected, err := hooks.DetectFlavor(cfg.BaseURL); err == nil {
			cfg.DetectedFlavor = detected
			fmt.Fprintf(out, "  detected flavor: %s\n", detected)
		} else {
			fmt.Fprintf(out, "  flavor detection failed (%v); continuing with auto\n", err)
		}
	}

	cfg.Auth.Scheme = promptChoice(r, out, "Auth scheme",
		[]string{SchemePAT, SchemeBasic}, SchemePAT)

	var secrets Secrets
	switch cfg.Auth.Scheme {
	case SchemePAT:
		secrets.PAT = prompt(r, out, "Personal Access Token", "", true)
	case SchemeBasic:
		cfg.Auth.Username = prompt(r, out, "Username or email", cfg.Auth.Username, true)
		secret := prompt(r, out, "Password or API token", "", true)
		// Cloud basic auth uses an API token; Data Center uses a password.
		if cfg.Flavor == FlavorCloud || cfg.DetectedFlavor == FlavorCloud {
			secrets.APIToken = secret
		} else {
			secrets.Password = secret
		}
	}

	if hooks.Validate != nil {
		fmt.Fprintln(out, "Validating credentials...")
		if err := hooks.Validate(cfg, secrets); err != nil {
			fmt.Fprintf(out, "  validation failed: %v\n", err)
			if !promptYesNo(r, out, "Save configuration anyway?", false) {
				return nil, fmt.Errorf("aborted: credential validation failed")
			}
		} else {
			fmt.Fprintln(out, "  credentials OK")
		}
	}

	return &WizardResult{Config: cfg, Secrets: secrets}, nil
}

func prompt(r *bufio.Reader, out io.Writer, label, def string, required bool) string {
	for {
		if def != "" {
			fmt.Fprintf(out, "%s [%s]: ", label, def)
		} else {
			fmt.Fprintf(out, "%s: ", label)
		}
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			line = def
		}
		if line == "" && required {
			fmt.Fprintln(out, "  value is required")
			continue
		}
		return line
	}
}

func promptChoice(r *bufio.Reader, out io.Writer, label string, choices []string, def string) string {
	for {
		v := prompt(r, out, fmt.Sprintf("%s (%s)", label, strings.Join(choices, "/")), def, true)
		for _, c := range choices {
			if strings.EqualFold(v, c) {
				return c
			}
		}
		fmt.Fprintf(out, "  choose one of: %s\n", strings.Join(choices, ", "))
	}
}

func promptYesNo(r *bufio.Reader, out io.Writer, label string, def bool) bool {
	d := "n"
	if def {
		d = "y"
	}
	v := strings.ToLower(prompt(r, out, label+" (y/n)", d, true))
	return v == "y" || v == "yes"
}

// SuggestedNextSteps returns commands to print after a successful init.
func SuggestedNextSteps() []string {
	return []string{
		constants.AppName + " auth status",
		constants.AppName + " doctor",
	}
}

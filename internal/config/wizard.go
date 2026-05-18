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

// ContextResult is one configured context together with its transient secrets.
type ContextResult struct {
	Context NamedContext
	Secrets Secrets
}

// WizardResult is the outcome of a completed init wizard: a config File plus
// the per-context secrets to persist into the keychain.
type WizardResult struct {
	File  File
	Creds []ContextResult
}

// RunWizard drives the interactive `config init` flow over in/out. It always
// configures one context (named "default"); a single-context user sees no
// mention of contexts beyond a final "Add another context?" prompt.
func RunWizard(in io.Reader, out io.Writer, hooks WizardHooks) (*WizardResult, error) {
	r := bufio.NewReader(in)

	fmt.Fprintln(out, "confluence-cli setup")
	fmt.Fprintln(out, "--------------------")

	result := &WizardResult{
		File: File{
			CurrentContext: DefaultContextName,
			Defaults:       configFromMap(defaultLayer()).Defaults,
		},
	}
	used := map[string]bool{}
	name := DefaultContextName
	for {
		cr, err := runContextWizard(r, out, hooks, name)
		if err != nil {
			return nil, err
		}
		result.File.Contexts = append(result.File.Contexts, cr.Context)
		result.Creds = append(result.Creds, cr)
		used[name] = true

		if !promptYesNo(r, out, "Add another context?", false) {
			break
		}
		for {
			name = prompt(r, out, "Context name", "", true)
			if used[name] {
				fmt.Fprintf(out, "  context %q is already configured; choose another name\n", name)
				continue
			}
			break
		}
	}
	return result, nil
}

// runContextWizard collects the settings for a single named context.
func runContextWizard(r *bufio.Reader, out io.Writer, hooks WizardHooks, name string) (ContextResult, error) {
	if name != DefaultContextName {
		fmt.Fprintf(out, "\nContext %q\n", name)
	}
	nc := NamedContext{Name: name}

	nc.BaseURL = prompt(r, out, "Confluence base URL", "", true)

	nc.Flavor = promptChoice(r, out, "Backend flavor",
		[]string{FlavorAuto, FlavorCloud, FlavorDataCenter}, FlavorAuto)
	if nc.Flavor == FlavorAuto && hooks.DetectFlavor != nil {
		if detected, err := hooks.DetectFlavor(nc.BaseURL); err == nil {
			nc.DetectedFlavor = detected
			fmt.Fprintf(out, "  detected flavor: %s\n", detected)
		} else {
			fmt.Fprintf(out, "  flavor detection failed (%v); continuing with auto\n", err)
		}
	}

	nc.Auth.Scheme = promptChoice(r, out, "Auth scheme",
		[]string{SchemePAT, SchemeBasic}, SchemePAT)

	var secrets Secrets
	switch nc.Auth.Scheme {
	case SchemePAT:
		secrets.PAT = prompt(r, out, "Personal Access Token", "", true)
	case SchemeBasic:
		nc.Auth.Username = prompt(r, out, "Username or email", "", true)
		secret := prompt(r, out, "Password or API token", "", true)
		// Cloud basic auth uses an API token; Data Center uses a password.
		if nc.Flavor == FlavorCloud || nc.DetectedFlavor == FlavorCloud {
			secrets.APIToken = secret
		} else {
			secrets.Password = secret
		}
	}

	if hooks.Validate != nil {
		fmt.Fprintln(out, "Validating credentials...")
		if err := hooks.Validate(contextConfig(nc), secrets); err != nil {
			fmt.Fprintf(out, "  validation failed: %v\n", err)
			if !promptYesNo(r, out, "Save configuration anyway?", false) {
				return ContextResult{}, fmt.Errorf("aborted: credential validation failed")
			}
		} else {
			fmt.Fprintln(out, "  credentials OK")
		}
	}

	return ContextResult{Context: nc, Secrets: secrets}, nil
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

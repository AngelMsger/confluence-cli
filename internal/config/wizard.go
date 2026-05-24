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

// WizardInputs carries optional state the caller has already loaded: the
// previously persisted config file and a way to fetch the stored secret for a
// given context. Both fields may be nil — the wizard then runs as a pure
// fresh-setup flow.
type WizardInputs struct {
	// Existing is the previously persisted config file. nil — or a value with
	// no contexts — means there is nothing to edit.
	Existing *File
	// LoadSecret returns the secret currently stored for nc, mapped into the
	// right Secrets field for nc's scheme + flavor. The second return is false
	// when no secret is stored. Used to honour "press Enter to keep current"
	// on secret prompts when editing an existing context.
	LoadSecret func(nc NamedContext) (Secrets, bool)
}

// ContextResult is one configured context together with its transient secrets.
type ContextResult struct {
	Context Context
	// Secrets are the credentials to persist for this context. When the user
	// chose to keep an existing stored secret, the secret is pre-loaded here
	// via WizardInputs.LoadSecret so the caller can save unconditionally.
	Secrets Secrets
}

// Context is an alias used in ContextResult so existing call sites keep
// working. The wizard always operates on NamedContext.
type Context = NamedContext

// WizardResult is the outcome of a completed init wizard.
type WizardResult struct {
	File  File
	Creds []ContextResult
}

// PromptDriver is the I/O surface the wizard talks to. PlainDriver mirrors the
// historical text prompts; SurveyDriver (built on github.com/AlecAivazis/survey/v2)
// adds arrow-key selection and styled placeholders for human interaction.
//
// Implementations are free to style output however they like, but they must
// honour these contracts:
//   - AskText: when def != "" it is the value returned on empty input;
//     example is a hint only and never auto-fills.
//   - AskChoice / AskSelect: the returned value is one of the supplied
//     choices / SelectItem.Value entries.
//   - AskSecretOptional: returns (v, keep=true, nil) when the user accepts the
//     existing value without supplying a new one.
type PromptDriver interface {
	// Section prints a section header (e.g. Context "default").
	Section(title string)
	// Notice prints an informational line (detected flavor, validation status).
	Notice(msg string)
	AskText(label, def, example string, required bool) (string, error)
	AskChoice(label string, choices []string, def string) (string, error)
	AskSelect(label string, items []SelectItem, def string) (string, error)
	AskSecret(label string) (string, error)
	AskSecretOptional(label string) (value string, keep bool, err error)
	AskConfirm(label string, def bool) (bool, error)
}

// SelectItem is a value plus a human label for AskSelect prompts.
type SelectItem struct {
	Value string
	Label string
}

const (
	exampleBaseURL  = "https://your-site.atlassian.net/wiki"
	exampleUsername = "you@example.com"
)

// contextPicks bundles the raw answers the user supplied for one context.
// It is the seam between any UI driver (plain text, huh TUI, future variants)
// and the deterministic mapping into a ContextResult: drivers gather inputs
// into a contextPicks; assembleContextResult turns it into the final
// Secrets-routed result. Keeping the routing rules in one place avoids
// "APIToken vs Password by flavor" bugs drifting between paths.
type contextPicks struct {
	Name           string
	BaseURL        string
	Flavor         string
	DetectedFlavor string // populated post-flavor-detection hook
	Scheme         string
	Username       string // only meaningful when Scheme == basic
	Secret         string // raw secret input; ignored when KeepSecret
	KeepSecret     bool   // user opted to retain the previously stored secret
}

// defaultSchemeForFlavor picks the auth scheme we should suggest based on the
// backend flavor. Cloud's id.atlassian.com API tokens only authenticate with
// HTTP Basic (email + token); Bearer/PAT against Cloud returns 403. PAT
// (Bearer) is Data Center 7.9+. When the flavor is unknown we default to PAT
// — that is the historical default and matches Data Center, where users are
// most likely to have come from before this wizard learned to suggest basic.
func defaultSchemeForFlavor(flavor, detected string) string {
	if flavor == FlavorCloud || detected == FlavorCloud {
		return SchemeBasic
	}
	return SchemePAT
}

// assembleContextResult turns raw picks (plus any kept secret loaded from the
// store) into a ContextResult. PAT routes the secret into Secrets.PAT; basic
// routes into Secrets.APIToken on Cloud and Secrets.Password on Data Center.
// When picks.KeepSecret is true the secret is sourced from kept rather than
// picks.Secret.
func assembleContextResult(picks contextPicks, kept Secrets) ContextResult {
	nc := NamedContext{
		Name:           picks.Name,
		BaseURL:        picks.BaseURL,
		Flavor:         picks.Flavor,
		DetectedFlavor: picks.DetectedFlavor,
		Auth: AuthConfig{
			Scheme:   picks.Scheme,
			Username: picks.Username,
		},
	}
	isCloud := picks.Flavor == FlavorCloud || picks.DetectedFlavor == FlavorCloud

	var secrets Secrets
	switch picks.Scheme {
	case SchemePAT:
		if picks.KeepSecret {
			secrets.PAT = kept.PAT
		} else {
			secrets.PAT = picks.Secret
		}
	case SchemeBasic:
		sec := picks.Secret
		if picks.KeepSecret {
			sec = kept.APIToken
			if sec == "" {
				sec = kept.Password
			}
		}
		if isCloud {
			secrets.APIToken = sec
		} else {
			secrets.Password = sec
		}
	}
	return ContextResult{Context: nc, Secrets: secrets}
}

// RunWizard drives the interactive `config init` flow. When inputs.Existing
// carries contexts, the user is asked whether to edit one, add a new one, or
// replace the configuration; otherwise the wizard runs a fresh setup.
func RunWizard(d PromptDriver, hooks WizardHooks, inputs WizardInputs) (*WizardResult, error) {
	d.Notice("confluence-cli setup")
	d.Notice("--------------------")

	hasExisting := inputs.Existing != nil && len(inputs.Existing.Contexts) > 0

	result := &WizardResult{
		File: File{
			CurrentContext: DefaultContextName,
			Defaults:       configFromMap(defaultLayer()).Defaults,
		},
	}

	editTarget := ""
	keepOthers := false

	if hasExisting {
		announceExisting(d, inputs.Existing)
		action, err := d.AskChoice("What would you like to do",
			[]string{"edit", "add", "replace"}, "edit")
		if err != nil {
			return nil, err
		}
		switch action {
		case "edit":
			if len(inputs.Existing.Contexts) == 1 {
				editTarget = inputs.Existing.Contexts[0].Name
			} else {
				items := make([]SelectItem, 0, len(inputs.Existing.Contexts))
				for _, c := range inputs.Existing.Contexts {
					items = append(items, SelectItem{Value: c.Name, Label: c.Name + " — " + c.BaseURL})
				}
				editTarget, err = d.AskSelect("Edit which context", items, inputs.Existing.CurrentContext)
				if err != nil {
					return nil, err
				}
			}
			keepOthers = true
		case "add":
			keepOthers = true
		case "replace":
			// keepOthers stays false; fresh-setup behaviour below.
		}
		if keepOthers {
			result.File.CurrentContext = inputs.Existing.CurrentContext
			result.File.Defaults = inputs.Existing.Defaults
		}
	}

	used := map[string]bool{}
	if keepOthers {
		for _, c := range inputs.Existing.Contexts {
			if c.Name == editTarget {
				continue
			}
			result.File.Contexts = append(result.File.Contexts, c)
			used[c.Name] = true
		}
	}

	name := DefaultContextName
	switch {
	case editTarget != "":
		name = editTarget
	case hasExisting && keepOthers:
		// "add" against existing contexts — let the user pick the new name.
		n, err := promptNewContextName(d, used)
		if err != nil {
			return nil, err
		}
		name = n
	}

	for {
		var prefill *NamedContext
		if editTarget != "" && name == editTarget {
			for i := range inputs.Existing.Contexts {
				if inputs.Existing.Contexts[i].Name == editTarget {
					prefill = &inputs.Existing.Contexts[i]
					break
				}
			}
		}

		cr, err := runContextWizard(d, hooks, inputs, name, prefill)
		if err != nil {
			return nil, err
		}
		result.File.Contexts = append(result.File.Contexts, cr.Context)
		result.Creds = append(result.Creds, cr)
		used[name] = true

		// After the first iteration, any further contexts are always brand-new.
		editTarget = ""

		more, err := d.AskConfirm("Add another context?", false)
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
		n, err := promptNewContextName(d, used)
		if err != nil {
			return nil, err
		}
		name = n
	}
	return result, nil
}

// announceExisting prints a short summary of the existing contexts via the
// driver's Notice channel.
func announceExisting(d PromptDriver, f *File) {
	d.Notice("Existing configuration:")
	for _, c := range f.Contexts {
		mark := "  "
		if c.Name == f.CurrentContext {
			mark = "* "
		}
		server := c.BaseURL
		if server == "" {
			server = "(no server)"
		}
		d.Notice(fmt.Sprintf("%s%s — %s", mark, c.Name, server))
	}
}

func promptNewContextName(d PromptDriver, used map[string]bool) (string, error) {
	for {
		name, err := d.AskText("New context name", "", "production", true)
		if err != nil {
			return "", err
		}
		if used[name] {
			d.Notice(fmt.Sprintf("  context %q is already configured; choose another name", name))
			continue
		}
		return name, nil
	}
}

// runContextWizard collects the settings for a single named context. When
// prefill is non-nil, its values are offered as defaults that the user can
// accept by pressing Enter; otherwise example placeholders are shown.
func runContextWizard(d PromptDriver, hooks WizardHooks, inputs WizardInputs, name string, prefill *NamedContext) (ContextResult, error) {
	if name != DefaultContextName || prefill != nil {
		d.Section(fmt.Sprintf("Context %q", name))
	}
	picks := contextPicks{Name: name}

	baseDef := ""
	if prefill != nil {
		baseDef = prefill.BaseURL
	}
	url, err := d.AskText("Confluence base URL", baseDef, exampleBaseURL, true)
	if err != nil {
		return ContextResult{}, err
	}
	picks.BaseURL = url

	flavorDef := FlavorAuto
	if prefill != nil && prefill.Flavor != "" {
		flavorDef = prefill.Flavor
	}
	flavor, err := d.AskChoice("Backend flavor",
		[]string{FlavorAuto, FlavorCloud, FlavorDataCenter}, flavorDef)
	if err != nil {
		return ContextResult{}, err
	}
	picks.Flavor = flavor
	if picks.Flavor == FlavorAuto && hooks.DetectFlavor != nil {
		if detected, err := hooks.DetectFlavor(picks.BaseURL); err == nil {
			picks.DetectedFlavor = detected
			d.Notice(fmt.Sprintf("  detected flavor: %s", detected))
		} else {
			d.Notice(fmt.Sprintf("  flavor detection failed (%v); continuing with auto", err))
		}
	}

	schemeDef := defaultSchemeForFlavor(picks.Flavor, picks.DetectedFlavor)
	if prefill != nil && prefill.Auth.Scheme != "" {
		schemeDef = prefill.Auth.Scheme
	}
	scheme, err := d.AskChoice("Auth scheme",
		[]string{SchemePAT, SchemeBasic}, schemeDef)
	if err != nil {
		return ContextResult{}, err
	}
	picks.Scheme = scheme

	// "Press Enter to keep current" is only meaningful when the scheme did
	// not change and a secret is actually stored for the prefill identity.
	var kept Secrets
	keepable := false
	if prefill != nil && inputs.LoadSecret != nil && prefill.Auth.Scheme == picks.Scheme {
		if loaded, ok := inputs.LoadSecret(*prefill); ok {
			keepable = true
			kept = loaded
		}
	}

	switch picks.Scheme {
	case SchemePAT:
		if keepable {
			v, keep, err := d.AskSecretOptional("Personal Access Token")
			if err != nil {
				return ContextResult{}, err
			}
			picks.Secret, picks.KeepSecret = v, keep
		} else {
			v, err := d.AskSecret("Personal Access Token")
			if err != nil {
				return ContextResult{}, err
			}
			picks.Secret = v
		}
	case SchemeBasic:
		userDef := ""
		if prefill != nil {
			userDef = prefill.Auth.Username
		}
		user, err := d.AskText("Username or email", userDef, exampleUsername, true)
		if err != nil {
			return ContextResult{}, err
		}
		picks.Username = user

		if keepable {
			v, keep, err := d.AskSecretOptional("Password or API token")
			if err != nil {
				return ContextResult{}, err
			}
			picks.Secret, picks.KeepSecret = v, keep
		} else {
			v, err := d.AskSecret("Password or API token")
			if err != nil {
				return ContextResult{}, err
			}
			picks.Secret = v
		}
	}

	result := assembleContextResult(picks, kept)

	if hooks.Validate != nil {
		d.Notice("Validating credentials...")
		if err := hooks.Validate(contextConfig(result.Context), result.Secrets); err != nil {
			d.Notice(fmt.Sprintf("  validation failed: %v", err))
			ok, err2 := d.AskConfirm("Save configuration anyway?", false)
			if err2 != nil {
				return ContextResult{}, err2
			}
			if !ok {
				return ContextResult{}, fmt.Errorf("aborted: credential validation failed")
			}
		} else {
			d.Notice("  credentials OK")
		}
	}

	return result, nil
}

// PlainDriver is the historical text-prompt UI. Prompts and notices are
// written to Out; input is read from In.
type PlainDriver struct {
	In  io.Reader
	Out io.Writer
	r   *bufio.Reader
}

// NewPlainDriver returns a PlainDriver writing to out and reading from in.
func NewPlainDriver(in io.Reader, out io.Writer) *PlainDriver {
	return &PlainDriver{In: in, Out: out}
}

func (p *PlainDriver) reader() *bufio.Reader {
	if p.r == nil {
		p.r = bufio.NewReader(p.In)
	}
	return p.r
}

// Section prints a blank line and a section header so the user sees a visual
// break in the prompt stream.
func (p *PlainDriver) Section(title string) {
	fmt.Fprintf(p.Out, "\n%s\n", title)
}

func (p *PlainDriver) Notice(msg string) {
	fmt.Fprintln(p.Out, msg)
}

func (p *PlainDriver) AskText(label, def, example string, required bool) (string, error) {
	return p.text(label, def, example, required), nil
}

func (p *PlainDriver) AskChoice(label string, choices []string, def string) (string, error) {
	for {
		v := p.text(fmt.Sprintf("%s (%s)", label, strings.Join(choices, "/")), def, "", true)
		for _, c := range choices {
			if strings.EqualFold(v, c) {
				return c, nil
			}
		}
		fmt.Fprintf(p.Out, "  choose one of: %s\n", strings.Join(choices, ", "))
	}
}

func (p *PlainDriver) AskSelect(label string, items []SelectItem, def string) (string, error) {
	choices := make([]string, 0, len(items))
	for _, it := range items {
		choices = append(choices, it.Value)
	}
	return p.AskChoice(label, choices, def)
}

func (p *PlainDriver) AskSecret(label string) (string, error) {
	return p.text(label, "", "", true), nil
}

func (p *PlainDriver) AskSecretOptional(label string) (string, bool, error) {
	fmt.Fprintf(p.Out, "%s [press Enter to keep current]: ", label)
	line, _ := p.reader().ReadString('\n')
	line = strings.TrimSpace(line)
	return line, line == "", nil
}

func (p *PlainDriver) AskConfirm(label string, def bool) (bool, error) {
	d := "n"
	if def {
		d = "y"
	}
	v := strings.ToLower(p.text(label+" (y/n)", d, "", true))
	return v == "y" || v == "yes", nil
}

// text is the inner prompt helper that AskText / AskChoice / AskConfirm /
// AskSecret all funnel through.
func (p *PlainDriver) text(label, def, example string, required bool) string {
	for {
		switch {
		case def != "":
			fmt.Fprintf(p.Out, "%s [%s]: ", label, def)
		case example != "":
			fmt.Fprintf(p.Out, "%s (e.g. %s): ", label, example)
		default:
			fmt.Fprintf(p.Out, "%s: ", label)
		}
		line, _ := p.reader().ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			line = def
		}
		if line == "" && required {
			fmt.Fprintln(p.Out, "  value is required")
			continue
		}
		return line
	}
}

// SuggestedNextSteps returns commands to print after a successful init.
func SuggestedNextSteps() []string {
	return []string{
		constants.AppName + " auth status",
		constants.AppName + " doctor",
	}
}

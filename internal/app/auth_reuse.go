// Reuse a verified native credential by associating its identity with an existing
// team context. Secrets stay in their original store; public setup stays offline.
package app

import (
	"errors"
	"reflect"
	"sort"
	"strings"

	"github.com/angelmsger/confluence-cli/internal/auth"
	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
)

type reuseServices struct {
	Resolve func(config.Config) (auth.Credential, error)
	Verify  func(config.Config, auth.Credential) error
	Read    func(string) (config.File, bool, error)
	Write   func(string, config.File) error
}

type reuseResult struct {
	Context       string `json:"context"`
	SourceContext string `json:"source_context,omitempty"`
	State         string `json:"state"`
	Changed       bool   `json:"changed"`
	Verified      bool   `json:"verified"`
	DryRun        bool   `json:"dry_run"`
	Reason        string `json:"reason,omitempty"`
}

func newAuthReuseCmd(s *appState) *cobra.Command {
	var dryRun bool
	var from string
	cmd := &cobra.Command{
		Use:   "reuse",
		Short: "Reuse an existing login in the selected context without signing in again",
		Long: "Match stored contexts by complete service URL, authentication scheme and service scope.\n" +
			"Verify a matching stored credential before filling a missing personal identity in\n" +
			"the selected context. Never copy secrets, replace an existing identity, activate\n" +
			"a context, or read credentials from environment variables. Dry-run performs\n" +
			"read-only verification and reports the same proposed association. An unchanged\n" +
			"or unavailable result is not proof of authentication; use auth status to check.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			services := reuseServices{
				Resolve: func(cfg config.Config) (auth.Credential, error) { return auth.Resolve(cfg, config.Secrets{}, s.store) },
				Verify:  func(cfg config.Config, cred auth.Credential) error { return verifyCredential(s, cfg, cred) },
				Read:    config.ReadFile, Write: config.WriteFile,
			}
			result, err := reuseAuthentication(s, from, dryRun, services)
			if err != nil {
				return err
			}
			return s.emit(result)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "verify and preview the identity association without writing")
	cmd.Flags().StringVar(&from, "from-context", "", "choose the source context when multiple identities match; list names with 'config get-contexts'")
	return cmd
}

func reuseAuthentication(s *appState, from string, dryRun bool, services reuseServices) (reuseResult, error) {
	result := reuseResult{Context: s.resolved.ActiveContext, State: "unavailable", DryRun: dryRun}
	file, _, err := services.Read(s.cfgDir)
	if err != nil {
		return result, cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_READ", "failed to read authentication contexts")
	}
	target, exists := file.Context(result.Context)
	if !exists || target.BaseURL == "" {
		return result, cerrors.New(cerrors.CategoryConfig, "AUTH_REUSE_TARGET_MISSING", "prepare the destination context before reusing a login").WithNextSteps(constants.AppName + " config set-context <name> --base-url <url>")
	}
	result.Context = target.Name
	targetCfg := reuseContextConfig(target, file.Defaults)
	if !sameReuseService(targetCfg, s.cfg()) {
		return result, cerrors.New(cerrors.CategoryConflict, "AUTH_REUSE_TARGET_MISMATCH", "service overrides differ from the stored destination; no identity was changed").WithNextSteps(constants.AppName + " --use-context " + target.Name + " config show")
	}

	if from != "" {
		source, ok := file.Context(from)
		if !ok {
			return result, cerrors.New(cerrors.CategoryNotFound, "AUTH_REUSE_SOURCE_NOT_FOUND", "the selected source context does not exist").WithNextSteps(constants.AppName + " config get-contexts")
		}
		if !sameReuseService(targetCfg, reuseContextConfig(source, file.Defaults)) {
			return result, cerrors.New(cerrors.CategoryConflict, "AUTH_REUSE_SOURCE_MISMATCH", "the source context belongs to a different service or authentication scope").WithNextSteps(constants.AppName + " config get-contexts")
		}
	}
	if targetCfg.Auth.Scheme == "none" {
		result.State = "unchanged"
		result.Reason = "the service does not use authentication"
		return result, nil
	}
	// A populated destination belongs to the user. Reuse fills missing identity,
	// never silently changes which account a named context represents.
	if target.Auth.Username != "" {
		result.State = "unchanged"
		result.Reason = "destination identity is already configured"
		return result, nil
	}
	candidates := []config.NamedContext{}
	for _, source := range file.Contexts {
		if strings.EqualFold(source.Name, target.Name) || (from != "" && !strings.EqualFold(source.Name, from)) {
			continue
		}
		if !sameReuseService(targetCfg, reuseContextConfig(source, file.Defaults)) {
			continue
		}
		// Equal native keys already share the credential. No identity data means
		// nothing needs migrating; the separate auth check decides readiness.
		if source.Auth.Username == "" && auth.AccountKey(source.BaseURL, reuseContextConfig(source, file.Defaults).Auth.Scheme) == auth.AccountKey(target.BaseURL, targetCfg.Auth.Scheme) {
			continue
		}
		candidates = append(candidates, source)
	}
	if len(candidates) > 0 {
		// Token/session schemes can already identify a working destination without
		// a username. Preserve that login instead of associating another source.
		credential, err := services.Resolve(targetCfg)
		if err == nil {
			err = credential.Validate()
		}
		if err == nil {
			err = services.Verify(reuseVerificationConfig(targetCfg), credential)
			if err == nil {
				result.State = "unchanged"
				result.Verified = true
				result.Reason = "destination already has a verified login"
				return result, nil
			}
			failure := cerrors.AsCLIError(err)
			if failure.Category != cerrors.CategoryAuth || failure.HTTPStatus != 401 {
				return result, err
			}
		} else if !unavailableReuseCredential(err) && cerrors.AsCLIError(err).Code != "CREDENTIAL_NOT_VISIBLE_OR_MISSING" {
			return result, err
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	type match struct {
		source     config.NamedContext
		credential auth.Credential
	}
	matches := []match{}
	seen := map[string]bool{}
	for _, source := range candidates {
		cfg := reuseContextConfig(source, file.Defaults)
		credential, err := services.Resolve(cfg)
		if err != nil {
			if unavailableReuseCredential(err) {
				continue
			}
			return result, err
		}
		if err = services.Verify(reuseVerificationConfig(cfg), credential); err != nil {
			if failure := cerrors.AsCLIError(err); failure.Category == cerrors.CategoryAuth && failure.HTTPStatus == 401 {
				continue
			}
			return result, err
		}
		key := auth.AccountKey(source.BaseURL, credential.Scheme) + "\x00" + credential.Username
		if seen[key] {
			continue
		}
		seen[key] = true
		matches = append(matches, match{source, credential})
	}
	if len(matches) == 0 {
		result.Reason = "no matching stored identity can be reused"
		return result, nil
	}
	if len(matches) > 1 {
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.source.Name
		}
		return result, cerrors.New(cerrors.CategoryConflict, "AUTH_REUSE_AMBIGUOUS", "multiple matching login identities are available; choose a source context").WithDetails(map[string]any{"contexts": names}).WithNextSteps(constants.AppName+" config get-contexts", constants.AppName+" --use-context "+reuseContextArg(target.Name)+" auth reuse --from-context <name> --dry-run")
	}
	chosen := matches[0]
	updated := target
	updated.Auth.Username = chosen.credential.Username
	updated.Auth.Scheme = chosen.credential.Scheme
	// Preserve the native store key for equivalent URL spellings without copying
	// a secret. The normalized complete service URL has already been matched.
	if auth.AccountKey(updated.BaseURL, updated.Auth.Scheme) != auth.AccountKey(chosen.source.BaseURL, chosen.credential.Scheme) {
		updated.BaseURL = chosen.source.BaseURL
	}
	result.SourceContext = chosen.source.Name
	result.Verified = true
	result.Changed = !reflect.DeepEqual(target, updated)
	if !result.Changed {
		result.State = "unchanged"
		return result, nil
	}
	result.State = "available"
	if dryRun {
		return result, nil
	}

	current, _, err := services.Read(s.cfgDir)
	if err != nil {
		return result, cerrors.Wrap(err, cerrors.CategoryConfig, "CONFIG_READ", "failed to recheck authentication contexts")
	}
	if !reflect.DeepEqual(file, current) {
		return result, cerrors.New(cerrors.CategoryConflict, "AUTH_REUSE_CONFIG_CHANGED", "configuration changed during verification; no identity was saved").WithNextSteps(constants.AppName + " --use-context " + target.Name + " auth reuse --dry-run")
	}
	currentCredential, err := services.Resolve(reuseContextConfig(chosen.source, file.Defaults))
	if err != nil {
		return result, err
	}
	if !reflect.DeepEqual(currentCredential, chosen.credential) {
		return result, cerrors.New(cerrors.CategoryConflict, "AUTH_REUSE_CREDENTIAL_CHANGED", "stored credential changed during verification; no identity was saved").WithNextSteps(constants.AppName + " --use-context " + reuseContextArg(target.Name) + " auth reuse --dry-run")
	}
	next := file
	next.Contexts = append([]config.NamedContext(nil), file.Contexts...)
	for i, c := range next.Contexts {
		if c.Name == target.Name {
			next.Contexts[i] = updated
			break
		}
	}
	if err = services.Write(s.cfgDir, next); err != nil {
		return result, cerrors.Wrap(err, cerrors.CategoryConfig, "AUTH_REUSE_WRITE_FAILED", "verified identity could not be saved; credentials were not changed")
	}
	result.State = "reused"
	return result, nil
}

func reuseContextConfig(c config.NamedContext, defaults config.Defaults) config.Config {
	return config.StoredContext(c, defaults)
}

func sameReuseService(a, b config.Config) bool {
	left, e1 := config.NormalizeServiceURL(a.BaseURL)
	right, e2 := config.NormalizeServiceURL(b.BaseURL)
	return e1 == nil && e2 == nil && left == right && a.Auth.Scheme == b.Auth.Scheme && reuseFlavor(a) == reuseFlavor(b)
}

func unavailableReuseCredential(err error) bool {
	var ce *cerrors.CLIError
	if !errors.As(err, &ce) {
		return false
	}
	// Store visibility failures require host access, not a new login. Preserve
	// their structured recovery; only incomplete local identity is skippable.
	return ce.Code == "AUTH_NO_TOKEN" || ce.Code == "AUTH_NO_BASIC" || ce.Code == "AUTH_NO_BEARER" || ce.Code == "AUTH_BAD_SESSION"
}

func reuseContextArg(name string) string { return "'" + strings.ReplaceAll(name, "'", "'\\''") + "'" }

func reuseFlavor(cfg config.Config) string {
	if (cfg.Flavor == "auto" || cfg.Flavor == "") && (cfg.DetectedFlavor == "cloud" || cfg.DetectedFlavor == "datacenter") {
		return cfg.DetectedFlavor
	}
	return cfg.Flavor
}

func reuseVerificationConfig(cfg config.Config) config.Config {
	cfg.Flavor = reuseFlavor(cfg)
	return cfg
}

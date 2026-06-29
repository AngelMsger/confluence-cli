package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/angelmsger/confluence-cli/internal/output"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	"github.com/spf13/cobra"
)

const (
	// envContextOverride names the context for one invocation; it is also the
	// env half of the "explicit selection" signal.
	envContextOverride = "CONFLUENCE_CONTEXT"
	// envNoContextHint opts out of the runtime multi-context nudge entirely.
	envNoContextHint = "CONFLUENCE_CLI_NO_CONTEXT_HINT"
)

// contextReminderBlock returns the plain-text block appended to --help when the
// config defines more than one context, so an agent reading help sees which
// instance commands will actually hit and how to target a different one.
//
// It is best-effort and never blocks help: any error resolving or loading the
// config yields "" (help still renders), as does a single-context (or no)
// config. cobra does not run PersistentPreRunE for --help, so this loads the
// config itself rather than reading s.resolved (which is nil at help time).
func contextReminderBlock(s *appState) string {
	cfgDir := s.gflags.configPath
	if cfgDir == "" {
		d, err := config.ResolveConfigDir()
		if err != nil {
			return ""
		}
		cfgDir = d
	}
	resolved, err := config.Load(config.LoadOptions{
		ConfigDir: cfgDir,
		Context:   s.gflags.useContext,
		Flags: config.FlagValues{
			BaseURL: s.gflags.baseURL,
			Flavor:  s.gflags.flavor,
			Format:  s.gflags.format,
			Timeout: s.gflags.timeout,
		},
	})
	if err != nil || len(resolved.ContextNames) <= 1 {
		return ""
	}

	names := resolved.ContextNames
	var head string
	if resolved.ActiveContext == "" {
		head = fmt.Sprintf("No active context selected (%d configured: %s).",
			len(names), strings.Join(names, ", "))
	} else {
		head = fmt.Sprintf("Active context: %s  (%d configured: %s; %s)",
			resolved.ActiveContext, len(names), strings.Join(names, ", "),
			contextSourceLabel(resolved.ContextSource))
	}
	return head + "\n" +
		"Multiple contexts are configured — verify you are targeting the intended one.\n" +
		"Override per call with --use-context <name> or " + envContextOverride + "=<name>, or\n" +
		"change the default with `" + constants.AppName + " config use-context <name>`.\n"
}

// maybeContextHint nudges an agent that is about to run a real command against
// one of several configured contexts without having picked one explicitly — the
// case where it can silently hit the wrong instance and trust the result.
//
// It runs from PersistentPreRunE after the config is loaded, so it reads the
// already-resolved state. It stays silent (single structured _notice to stderr,
// stdout untouched) when:
//   - the config has 0 or 1 context (nothing to disambiguate),
//   - the context was selected explicitly (--use-context / CONFLUENCE_CONTEXT),
//   - a human is at the terminal (stderr is a TTY),
//   - the command is a setup/meta command or a non-runnable group, or
//   - the nudge is opted out (CONFLUENCE_CLI_NO_CONTEXT_HINT).
//
// Unlike the Skill hint it is NOT silenced by the Skill handshake: loading the
// Skill does not tell the agent which context to target. It self-silences the
// moment the agent selects a context explicitly.
func maybeContextHint(cmd *cobra.Command, s *appState) {
	if !contextHintApplies(cmd, s) {
		return
	}
	active := s.resolved.ActiveContext
	target := active
	if target == "" {
		target = "the resolved context"
	}
	output.EmitNotice(os.Stderr, map[string]any{"_notice": map[string]any{
		"context": map[string]any{
			"active":       active,
			"available":    s.resolved.ContextNames,
			"selected_via": s.resolved.ContextSource,
			"message": fmt.Sprintf("Multiple contexts are configured and this one was selected implicitly (%s). "+
				"Confirm %q is the intended target before trusting results.",
				contextSourceLabel(s.resolved.ContextSource), target),
			"override": "--use-context <name> or " + envContextOverride + "=<name>",
			"silence":  "set " + envNoContextHint + "=1 to suppress",
		},
	}})
}

// contextHintApplies holds the gating for the runtime multi-context nudge,
// split out so it can be unit-tested without capturing stderr.
func contextHintApplies(cmd *cobra.Command, s *appState) bool {
	if os.Getenv(envNoContextHint) != "" {
		return false
	}
	if s.resolved == nil || len(s.resolved.ContextNames) <= 1 {
		return false
	}
	if s.resolved.ContextSelectedExplicitly() {
		return false
	}
	if stderrIsTTY() {
		return false
	}
	return cmd.Runnable() && !skillHintSkip(cmd)
}

// contextSourceLabel renders a config.ContextSource* value as a human phrase
// for the help block and the runtime notice.
func contextSourceLabel(source string) string {
	switch source {
	case config.ContextSourceFlag:
		return "selected via --use-context"
	case config.ContextSourceEnv:
		return "selected via " + envContextOverride
	case config.ContextSourceCurrent:
		return "selected via the saved current_context"
	case config.ContextSourceSingle:
		return "the only configured context"
	case config.ContextSourceDefault:
		return `selected as the "default" context`
	default:
		return "no context selected"
	}
}

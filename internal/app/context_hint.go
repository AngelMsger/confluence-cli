package app

import (
	"os"

	"github.com/angelmsger/agent-cli-kit/output"
	"github.com/angelmsger/confluence-cli/internal/config"
	"github.com/spf13/cobra"
)

// envNoContextHint opts out of the runtime multi-context nudge entirely.
const envNoContextHint = config.EnvNoContextHint

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
	return config.ReminderBlock(resolved.ActiveContext, resolved.ContextSource, resolved.ContextNames)
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
	output.EmitNotice(os.Stderr, config.ContextNotice(s.resolved.ActiveContext, s.resolved.ContextSource, s.resolved.ContextNames))
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

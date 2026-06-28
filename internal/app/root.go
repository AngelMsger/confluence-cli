// Package app wires the cobra command tree and runs the CLI.
package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/angelmsger/confluence-cli/internal/cliflags"
	"github.com/angelmsger/confluence-cli/internal/output"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the full cobra command tree. It exists so tooling — most
// notably the docs generator (cmd/gen-docs) — can walk the same command tree
// the CLI runs, keeping generated reference docs in lock-step with --help.
func NewRootCmd() *cobra.Command { return newRootCmd() }

// newRootCmd builds the command tree, discarding the appState handle. Callers
// that need the state (Execute, to emit the post-run update notice) use
// newRootCmdWithState instead.
func newRootCmd() *cobra.Command {
	root, _ := newRootCmdWithState()
	return root
}

// Execute builds and runs the root command, returning a process exit code.
func Execute() int {
	root, state := newRootCmdWithState()
	// Absorb common LLM argv slips (--userId -> --user-id, --limit100 ->
	// --limit 100) before cobra parses, echoing each fix to stderr so the data
	// on stdout is untouched and the agent learns the canonical form.
	if corrected, corrections := cliflags.Normalize(os.Args[1:], cliflags.Collect(root)); len(corrections) > 0 {
		root.SetArgs(corrected)
		output.EmitNotice(os.Stderr, map[string]any{"_notice": map[string]any{"corrections": corrections}})
	}
	cmd, err := root.ExecuteC()
	// Surface an available-update notice on stderr regardless of whether the
	// command succeeded: it is purely informational, and an agent whose commands
	// often fail should still learn a newer release exists. ExecuteC gives us the
	// command that actually ran so the skip list still applies. Best-effort and
	// bounded — it never affects the exit code.
	maybeNotifyUpdate(state, cmd)
	if err != nil {
		// cobra flag/usage errors are not CLIErrors; classify them as usage.
		ce := cerrors.AsCLIError(err)
		if ce.Category == cerrors.CategoryInternal && !isCLIError(err) {
			ce = cerrors.Wrap(err, cerrors.CategoryUsage, "USAGE", err.Error())
		}
		output.EmitError(ce, os.Stderr)
		return cerrors.ExitCode(ce)
	}
	return 0
}

func isCLIError(err error) bool {
	_, ok := err.(*cerrors.CLIError)
	return ok
}

// newRootCmdWithState assembles the full command tree and returns the appState
// it is wired to, so Execute can emit the update notice after the command runs.
func newRootCmdWithState() (*cobra.Command, *appState) {
	state := &appState{}

	root := &cobra.Command{
		Use:   constants.AppName,
		Short: "Use a Confluence instance as a knowledge base for coding agents",
		Long: "confluence-cli reads Confluence pages, searches via CQL, creates and\n" +
			"edits pages, and manages comments. It supports Confluence Cloud and\n" +
			"Data Center / Server, and emits agent-friendly JSON with structured errors.\n\n" +
			"AGENT NOTE: a companion Skill (\"confluence\") carries the canonical usage,\n" +
			"safety modes, and env setup and is the source of truth for driving this CLI.\n" +
			"If you are an agent, load that Skill before composing commands. Check status\n" +
			"with `confluence-cli skill status`; install it with `confluence-cli skill install`.",
		Version:       versionString(),
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// config / auth subcommands manage configuration themselves and
			// must run even when nothing is configured yet.
			output.SetErrorPretty(state.gflags.pretty)
			// Nudge agents that shell out without the companion Skill loaded.
			maybeSkillHint(cmd)
			return state.load()
		},
		// The available-update notice is emitted from Execute (after ExecuteC),
		// not a PersistentPostRunE: cobra skips the Post hooks when a command
		// fails, but the notice should surface on failure too.
	}

	pf := root.PersistentFlags()
	pf.StringVar(&state.gflags.baseURL, "base-url", "", "Confluence site URL (overrides config)")
	pf.StringVar(&state.gflags.flavor, "flavor", "", "backend flavor: cloud, datacenter or auto")
	pf.StringVarP(&state.gflags.format, "format", "f", "", "output format: json, table or ndjson")
	pf.StringVar(&state.gflags.fields, "fields", "", "comma-separated dot-path fields to keep")
	pf.StringVar(&state.gflags.timeout, "timeout", "", "request timeout, e.g. 30s")
	pf.StringVar(&state.gflags.configPath, "config", "", "config directory (default ~/.angelmsger/confluence, falling back to ~/.confluence when only that exists)")
	pf.StringVar(&state.gflags.useContext, "use-context", "", "use a named context for this invocation")
	pf.BoolVarP(&state.gflags.verbose, "verbose", "v", false, "verbose diagnostics on stderr")
	pf.BoolVar(&state.gflags.pretty, "pretty", false,
		"human-friendly mode for interactive terminal use only (agents/scripts should omit): TUI in `config init`, colorized JSON elsewhere; errors without a TTY")
	pf.BoolVar(&state.gflags.allowWrites, "allow-writes", false,
		"override read-only mode (defaults.read_only / CONFLUENCE_CLI_READ_ONLY) for this invocation")

	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return cerrors.Wrap(err, cerrors.CategoryUsage, "BAD_FLAG", err.Error())
	})
	// `confluence-cli --version` prints the same line as the `version` command.
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")

	enumComplete(root, "format", "json", "table", "ndjson")
	enumComplete(root, "flavor", "cloud", "datacenter", "auto")

	root.AddCommand(
		newPageCmd(state),
		newSearchCmd(state),
		newSpaceCmd(state),
		newCommentCmd(state),
		newAttachmentCmd(state),
		newLabelCmd(state),
		newConfigCmd(state),
		newAuthCmd(state),
		newDoctorCmd(state),
		newWhoamiCmd(state),
		newUserCmd(state),
		newSkillCmd(state),
		newVersionCmd(),
	)
	enforceSubcommands(root)
	return root, state
}

// enforceSubcommands makes every pure command group (one with subcommands but no
// action of its own) reject an unknown subcommand. Cobra's default for such a
// non-runnable parent is to print help and exit 0 — which reads as a successful
// no-op to agents and scripts that typo a subcommand (e.g. `config use-contexts`
// for `config use-context`). Cobra only flags unknown commands at the root (via
// legacyArgs), so nested groups need this. The root itself is left alone — it is
// already covered — and runnable leaves are untouched. Applied tree-wide so any
// group at any depth is covered automatically.
func enforceSubcommands(cmd *cobra.Command) {
	for _, child := range cmd.Commands() {
		enforceSubcommands(child)
	}
	if cmd.HasParent() && cmd.HasSubCommands() && !cmd.Runnable() {
		requireSubcommand(cmd)
	}
}

// requireSubcommand gives a group command a RunE so it survives cobra's
// non-runnable help-and-exit-0 path: a bare invocation still prints help, but an
// unknown subcommand becomes a structured usage error (exit 2), mirroring the
// "unknown command ... Did you mean" UX cobra gives at the root.
func requireSubcommand(cmd *cobra.Command) {
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return c.Help()
		}
		msg := fmt.Sprintf("unknown command %q for %q", args[0], c.CommandPath())
		// SuggestionsFor honors SuggestionsMinimumDistance but, unlike cobra's
		// root-level findSuggestions, does not default it — so set it here to get
		// the same "Did you mean" behavior (e.g. use-contexts -> use-context).
		if c.SuggestionsMinimumDistance <= 0 {
			c.SuggestionsMinimumDistance = 2
		}
		if s := c.SuggestionsFor(args[0]); len(s) > 0 {
			msg += "\n\nDid you mean this?\n\t" + strings.Join(s, "\n\t")
		}
		return cerrors.New(cerrors.CategoryUsage, "UNKNOWN_COMMAND", msg).
			WithNextSteps(c.CommandPath() + " --help")
	}
}

// versionString renders the version, commit and build time as one line.
func versionString() string {
	return fmt.Sprintf("%s (commit %s, built %s)",
		constants.Version, constants.Commit, constants.BuildTime)
}

// newVersionCmd prints build metadata. It mirrors the `--version` flag.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(os.Stdout, "%s %s\n", constants.AppName, versionString())
			return nil
		},
	}
}

package app

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/angelmsger/confluence-cli/internal/output"
	"github.com/angelmsger/confluence-cli/internal/update"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	"github.com/spf13/cobra"
)

// notifyTimeout bounds the release lookup performed on a cache miss so a
// routine command is never stalled by a slow or offline network. The result
// is cached for 24h (see update.Cached), so at most one command per day pays
// this cost.
const notifyTimeout = 800 * time.Millisecond

// noUpdateNotifierEnv silences the in-response update notice when set to a
// truthy value, mirroring lark-cli's LARKSUITE_CLI_NO_UPDATE_NOTIFIER.
const noUpdateNotifierEnv = "CONFLUENCE_CLI_NO_UPDATE_NOTIFIER"

// updateCommandHint is the command the notice tells the user/agent to run to
// upgrade. There is no `confluence-cli update` subcommand, so it points at the
// npm package the binary is distributed through.
const updateCommandHint = "npm install -g @angelmsger/confluence-cli@latest"

// updateNoticeSkip lists the top-level command groups that must not emit the
// update notice: doctor reports updates itself; the rest are setup/meta
// commands where the notice would be noise.
var updateNoticeSkip = map[string]bool{
	"doctor": true, "version": true, "config": true, "auth": true,
	"skill": true, "completion": true, "help": true,
	"__complete": true, "__completeNoDesc": true,
}

// maybeNotifyUpdate emits a one-line {"_notice":{"update":{…}}} to stderr when
// a newer release is available. It runs from the root PersistentPostRunE, i.e.
// only after a command has succeeded, and never returns an error: a failed or
// skipped check simply produces no notice. The notice goes to stderr so the
// stdout data contract is untouched while agents still see it via the shell.
func maybeNotifyUpdate(s *appState, cmd *cobra.Command) {
	if envTruthy(os.Getenv(noUpdateNotifierEnv)) {
		return
	}
	if updateNoticeSkip[topLevelName(cmd)] {
		return
	}
	if s.cfgDir == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()
	st := update.Cached(ctx, &http.Client{Timeout: notifyTimeout}, s.cfgDir, constants.Version)
	if !st.Available {
		return
	}
	output.EmitNotice(os.Stderr, map[string]any{
		"_notice": map[string]any{
			"update": map[string]any{
				"current": st.Current,
				"latest":  st.Latest,
				"command": updateCommandHint,
				"detail":  st.Detail,
			},
		},
	})
}

// topLevelName returns the name of the first-level subcommand under root (e.g.
// "config" for `config init`, "doctor" for `doctor`), used to scope the
// update-notice skip list to whole command groups.
func topLevelName(cmd *cobra.Command) string {
	c := cmd
	for c.Parent() != nil && c.Parent().Parent() != nil {
		c = c.Parent()
	}
	return c.Name()
}

// envTruthy parses a flag-style truthy env string ("1", "true", "yes", "on").
func envTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

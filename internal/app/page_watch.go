package app

import (
	"github.com/angelmsger/confluence-cli/internal/apiclient"
	"github.com/spf13/cobra"
)

// watchOutput is the result shape for the page-watch commands.
type watchOutput struct {
	PageID   string `json:"page_id"`
	Watching bool   `json:"watching"`
}

// newPageSetWatchCmd builds `page watch` or `page unwatch`; watching selects
// which.
func newPageSetWatchCmd(s *appState, watching bool) *cobra.Command {
	use, short, example := "watch <id|url>",
		"Watch a page (subscribe to its notifications)",
		"  confluence-cli page watch 123456"
	if !watching {
		use, short, example = "unwatch <id|url>",
			"Stop watching a page",
			"  confluence-cli page unwatch 123456"
	}
	var dryRun bool
	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolveID(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.WatchReq{PageID: id, Watching: watching}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			if err := client.SetWatch(ctx, req); err != nil {
				return err
			}
			return s.emit(watchOutput{PageID: id, Watching: watching})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}

func newPageWatchCmd(s *appState) *cobra.Command   { return newPageSetWatchCmd(s, true) }
func newPageUnwatchCmd(s *appState) *cobra.Command { return newPageSetWatchCmd(s, false) }

func newPageWatchStatusCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "watch-status <id|url>",
		Short:   "Report whether you watch a page",
		Example: "  confluence-cli page watch-status 123456",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolveID(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			watching, err := client.WatchStatus(ctx, id)
			if err != nil {
				return err
			}
			return s.emit(watchOutput{PageID: id, Watching: watching})
		},
	}
	return cmd
}

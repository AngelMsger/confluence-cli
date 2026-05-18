package app

import (
	"github.com/angelmsger/confluence-cli/internal/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

func newPageHistoryCmd(s *appState) *cobra.Command {
	var (
		limit  int
		all    bool
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "history <id|url>",
		Short: "List a page's version history",
		Example: "  confluence-cli page history 123456\n" +
			"  confluence-cli page history 123456 --all --format table",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolvePageID(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			items, info, err := collectPage(func(cursor string) (apiclient.ListResult[apiclient.PageVersion], error) {
				return client.ListPageVersions(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: cursor})
			}, cursor, all)
			if err != nil {
				return err
			}
			return s.emitList(items, info)
		},
	}
	addListFlags(cmd, &limit, &all, &cursor)
	return cmd
}

func newPageRestoreCmd(s *appState) *cobra.Command {
	var (
		version int
		message string
		dryRun  bool
	)
	cmd := &cobra.Command{
		Use:   "restore <id|url> --version <N>",
		Short: "Restore a page to an earlier version",
		Long: "Republish an earlier version's body as a new version. The restore is\n" +
			"non-destructive: the version history is left intact. Run `page history`\n" +
			"first to find the version number to restore.",
		Example: "  confluence-cli page restore 123456 --version 3\n" +
			"  confluence-cli page restore 123456 --version 3 --message \"roll back bad edit\"",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolvePageID(args[0])
			if err != nil {
				return err
			}
			if version <= 0 {
				return cerrors.New(cerrors.CategoryUsage, "RESTORE_NO_VERSION",
					"--version must be a positive version number to restore")
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.RestorePageReq{ID: id, Version: version, Message: message}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			page, err := client.RestorePage(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(pageMetaOutput(page))
		},
	}
	f := cmd.Flags()
	f.IntVar(&version, "version", 0, "the version number to restore")
	f.StringVar(&message, "message", "", "version comment for the restore")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}

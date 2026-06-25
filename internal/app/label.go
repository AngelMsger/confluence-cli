package app

import (
	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	"github.com/spf13/cobra"
)

func newLabelCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "List, add and remove page labels",
	}
	cmd.AddCommand(newLabelListCmd(s), newLabelAddCmd(s), newLabelRemoveCmd(s))
	return cmd
}

func newLabelListCmd(s *appState) *cobra.Command {
	var (
		limit  int
		all    bool
		cursor string
	)
	cmd := &cobra.Command{
		Use:     "list <page-id|url>",
		Short:   "List the labels on a page",
		Example: "  confluence-cli label list 123456",
		Args:    cobra.ExactArgs(1),
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
			items, info, err := collectPage(func(cursor string) (apiclient.ListResult[apiclient.Label], error) {
				return client.ListLabels(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: cursor})
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

func newLabelAddCmd(s *appState) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "add <page-id|url> <label>...",
		Short: "Add one or more labels to a page",
		Example: "  confluence-cli label add 123456 release-notes\n" +
			"  confluence-cli label add 123456 q3 reviewed --dry-run",
		Args: cobra.MinimumNArgs(2),
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
			req := apiclient.AddLabelsReq{PageID: id, Names: args[1:]}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			labels, err := client.AddLabels(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(labels)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}

func newLabelRemoveCmd(s *appState) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "remove <page-id|url> <label>",
		Short:   "Remove a label from a page",
		Example: "  confluence-cli label remove 123456 release-notes",
		Args:    cobra.ExactArgs(2),
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
			req := apiclient.RemoveLabelReq{PageID: id, Name: args[1]}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			if err := client.RemoveLabel(ctx, req); err != nil {
				return err
			}
			return s.emit(map[string]any{"id": id, "label": args[1], "status": "removed"})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}

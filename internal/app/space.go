package app

import (
	"github.com/angelmsger/confluence-cli/internal/apiclient"
	"github.com/spf13/cobra"
)

func newSpaceCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "space",
		Short: "List and inspect Confluence spaces",
	}
	cmd.AddCommand(newSpaceListCmd(s), newSpaceGetCmd(s))
	return cmd
}

func newSpaceListCmd(s *appState) *cobra.Command {
	var (
		spaceType string
		limit     int
		all       bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List spaces",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			items, err := collectList(func(cursor string) (apiclient.ListResult[apiclient.Space], error) {
				return client.ListSpaces(ctx, apiclient.SpaceListOpts{
					ListOpts: apiclient.ListOpts{Limit: limit, Cursor: cursor},
					Type:     spaceType,
				})
			}, limit, all)
			if err != nil {
				return err
			}
			return s.emit(items)
		},
	}
	cmd.Flags().StringVar(&spaceType, "type", "", "filter by type: global or personal")
	cmd.Flags().IntVar(&limit, "limit", 0, "page size (default from config)")
	cmd.Flags().BoolVar(&all, "all", false, "fetch every page of results")
	enumComplete(cmd, "type", "global", "personal")
	return cmd
}

func newSpaceGetCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:               "get <key>",
		Short:             "Fetch a single space by key",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSpaceKeys(s),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			space, err := client.GetSpace(ctx, args[0])
			if err != nil {
				return err
			}
			return s.emit(space)
		},
	}
}

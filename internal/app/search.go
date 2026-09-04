package app

import (
	"strings"

	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	"github.com/spf13/cobra"
)

func newSearchCmd(s *appState) *cobra.Command {
	var (
		params apiclient.CQLParams
		limit  int
		all    bool
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "search [cql]",
		Short: "Search pages with CQL or filter flags",
		Long: "Search Confluence content. Provide a raw CQL string as the argument,\n" +
			"or build one from filter flags (--text, --author, --space, ...).",
		Example: "  # filter flags are combined into a CQL query\n" +
			"  confluence-cli search --text \"release process\" --space ENG --type page\n\n" +
			"  # or pass raw CQL directly\n" +
			"  confluence-cli search 'creator = \"jdoe\" AND created >= \"2025-01-01\"' --all",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := cmdContext(s)
			defer cancel()
			var client apiclient.Client
			var cql string
			if len(args) == 1 && args[0] != "" {
				cql = args[0]
			} else {
				built, err := apiclient.BuildCQL(params)
				if err != nil {
					return err
				}
				cql = built
				if strings.EqualFold(params.Author, "me") || strings.EqualFold(params.Contributor, "me") {
					client, err = s.newClient(ctx)
					if err != nil {
						return err
					}
					user, err := resolveUserSelector(ctx, client, "me")
					if err != nil {
						return err
					}
					selector := stableUserID(client.Flavor(), user)
					if strings.EqualFold(params.Author, "me") {
						params.Author = selector
					}
					if strings.EqualFold(params.Contributor, "me") {
						params.Contributor = selector
					}
					cql, err = apiclient.BuildCQL(params)
					if err != nil {
						return err
					}
				}
			}
			if client == nil {
				var err error
				client, err = s.newClient(ctx)
				if err != nil {
					return err
				}
			}
			items, info, err := collectPage(func(cursor string) (apiclient.ListResult[apiclient.SearchHit], error) {
				return client.Search(ctx, cql, apiclient.ListOpts{Limit: limit, Cursor: cursor})
			}, cursor, all)
			if err != nil {
				return err
			}
			return s.emitList(items, info)
		},
	}
	f := cmd.Flags()
	f.StringVar(&params.Text, "text", "", "free-text match (CQL: text ~)")
	f.StringVar(&params.Author, "author", "", "original creator (CQL: creator =); find IDs with `user search`")
	f.StringVar(&params.Contributor, "contributor", "", "any contributor (CQL: contributor =); find IDs with `user search`")
	f.StringVar(&params.Space, "space", "", "space key (CQL: space =)")
	f.StringVar(&params.Label, "label", "", "label (CQL: label =)")
	f.StringVar(&params.Type, "type", "", "content type: page, blogpost, comment, attachment")
	f.StringVar(&params.After, "after", "", "modified on/after date, e.g. 2025-01-01")
	f.StringVar(&params.Before, "before", "", "modified on/before date, e.g. 2025-12-31")
	addListFlags(cmd, &limit, &all, &cursor)
	enumComplete(cmd, "type", "page", "blogpost", "comment", "attachment")
	return cmd
}

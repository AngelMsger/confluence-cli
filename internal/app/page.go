package app

import (
	"github.com/angelmsger/confluence-cli/internal/apiclient"
	"github.com/angelmsger/confluence-cli/internal/render"
	"github.com/spf13/cobra"
)

// pageOutput is the result shape for `page get`.
type pageOutput struct {
	ID           string                `json:"id"`
	Title        string                `json:"title"`
	SpaceKey     string                `json:"space_key,omitempty"`
	Status       string                `json:"status,omitempty"`
	URL          string                `json:"url,omitempty"`
	Version      *apiclient.Version    `json:"version,omitempty"`
	Ancestors    []apiclient.PageRef   `json:"ancestors,omitempty"`
	Outline      []render.OutlineEntry `json:"outline,omitempty"`
	Body         string                `json:"body,omitempty"`
	ScopeApplied string                `json:"scope_applied,omitempty"`
	Truncated    bool                  `json:"truncated,omitempty"`
}

func newPageCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Read Confluence pages",
	}
	cmd.AddCommand(newPageGetCmd(s), newPageChildrenCmd(s), newPageDescendantsCmd(s))
	return cmd
}

func newPageGetCmd(s *appState) *cobra.Command {
	var (
		bodyFormat string
		detail     string
		scope      string
		section    string
		keyword    string
		as         string
		noBody     bool
	)
	cmd := &cobra.Command{
		Use:   "get <id|url>",
		Short: "Fetch a page and render its body",
		Long: "Fetch a single page. Use --scope to read only part of the body:\n" +
			"  outline  list the headings (start here when the structure is unknown)\n" +
			"  section  one section, identified by --section <id> from the outline\n" +
			"  keyword  blocks matching --keyword, with their heading for context\n" +
			"  full     the entire body (default)",
		Args: cobra.ExactArgs(1),
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
			page, err := client.GetPage(ctx, id, apiclient.GetPageOpts{
				WithBody:   !noBody,
				BodyFormat: bodyFormat,
			})
			if err != nil {
				return err
			}
			out := pageOutput{
				ID: page.ID, Title: page.Title, SpaceKey: page.SpaceKey,
				Status: page.Status, URL: page.URL,
				Version: page.Version, Ancestors: page.Ancestors,
			}
			if !noBody && page.Body != nil {
				rendered, err := render.Render(page.Body.Value, render.Options{
					Scope: scope, Detail: detail, As: as,
					SectionID: section, Keyword: keyword,
				})
				if err != nil {
					return err
				}
				out.Outline = rendered.Outline
				out.Body = rendered.Body
				out.ScopeApplied = rendered.ScopeApplied
				out.Truncated = rendered.Truncated
			}
			return s.emit(out)
		},
	}
	f := cmd.Flags()
	f.StringVar(&bodyFormat, "body-format", "storage", "source body format: storage or view")
	f.StringVar(&detail, "detail", "simple", "block detail: simple, with-ids or full")
	f.StringVar(&scope, "scope", "full", "read scope: full, outline, section or keyword")
	f.StringVar(&section, "section", "", "section ID (with --scope section)")
	f.StringVar(&keyword, "keyword", "", "keyword (with --scope keyword)")
	f.StringVar(&as, "as", "markdown", "render body as markdown or text")
	f.BoolVar(&noBody, "no-body", false, "fetch metadata only, skip the body")
	enumComplete(cmd, "body-format", "storage", "view")
	enumComplete(cmd, "detail", "simple", "with-ids", "full")
	enumComplete(cmd, "scope", "full", "outline", "section", "keyword")
	enumComplete(cmd, "as", "markdown", "text")
	return cmd
}

func newPageChildrenCmd(s *appState) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "children <id|url>",
		Short: "List the direct child pages of a page",
		Args:  cobra.ExactArgs(1),
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
			items, err := collectList(func(cursor string) (apiclient.ListResult[apiclient.Page], error) {
				return client.ListChildren(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: cursor})
			}, limit, all)
			if err != nil {
				return err
			}
			return s.emit(items)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "page size (default from config)")
	cmd.Flags().BoolVar(&all, "all", false, "fetch every page of results")
	return cmd
}

func newPageDescendantsCmd(s *appState) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "descendants <id|url>",
		Short: "List all descendant pages of a page",
		Args:  cobra.ExactArgs(1),
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
			items, err := collectList(func(cursor string) (apiclient.ListResult[apiclient.Page], error) {
				return client.ListDescendants(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: cursor})
			}, limit, all)
			if err != nil {
				return err
			}
			return s.emit(items)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "page size (default from config)")
	cmd.Flags().BoolVar(&all, "all", false, "fetch every page of results")
	return cmd
}

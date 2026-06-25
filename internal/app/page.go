package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/angelmsger/confluence-cli/internal/render"
	"github.com/angelmsger/confluence-cli/pkg/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
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
	// RenderNotes lists content the markdown/text renderer could not represent
	// (e.g. unrendered macros). When non-empty, re-read with --as raw.
	RenderNotes []string `json:"render_notes,omitempty"`
	// OutputPath and Bytes are set when --output wrote the body to a file
	// instead of inlining it. Body is then omitted from stdout so a large page
	// never floods the agent's context — the content lives on disk.
	OutputPath string `json:"output_path,omitempty"`
	Bytes      int    `json:"bytes,omitempty"`
}

// dryRunOutput is the result shape emitted for a --dry-run write.
type dryRunOutput struct {
	DryRun  bool   `json:"dry_run"`
	Method  string `json:"method"`
	URL     string `json:"url"`
	Payload any    `json:"payload,omitempty"`
}

func newPageCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Read and write Confluence pages",
	}
	cmd.AddCommand(
		newPageGetCmd(s), newPageChildrenCmd(s), newPageDescendantsCmd(s),
		newPageCreateCmd(s), newPageUpdateCmd(s), newPageDeleteCmd(s),
		newPageMoveCmd(s), newPageCopyCmd(s),
		newPageHistoryCmd(s), newPageRestoreCmd(s),
		newPageWatchCmd(s), newPageUnwatchCmd(s), newPageWatchStatusCmd(s),
	)
	return cmd
}

// pageMetaOutput projects a page into the result shape used by write commands
// (metadata only, no rendered body).
func pageMetaOutput(p *apiclient.Page) pageOutput {
	return pageOutput{
		ID: p.ID, Title: p.Title, SpaceKey: p.SpaceKey,
		Status: p.Status, URL: p.URL, Version: p.Version,
		Ancestors: p.Ancestors,
	}
}

// emitDryRun resolves a write request into the HTTP request it would send and
// emits that plan instead of performing the write.
func emitDryRun(s *appState, client apiclient.Client, ctx context.Context, op any) error {
	out, err := dryRunPlan(client, ctx, op)
	if err != nil {
		return err
	}
	return s.emit(out)
}

// dryRunPlan resolves a write request into its dry-run output object without
// emitting it, so batch commands can collect one plan per item.
func dryRunPlan(client apiclient.Client, ctx context.Context, op any) (dryRunOutput, error) {
	plan, err := client.DescribeWrite(ctx, op)
	if err != nil {
		return dryRunOutput{}, err
	}
	return dryRunOutput{
		DryRun: true, Method: plan.Method, URL: plan.URL, Payload: plan.Payload,
	}, nil
}

// resolvePageBody reads the body for a write command from --body or --body-file
// and converts it to a storage- or wiki-format PageBody. provided is false when
// neither flag was set, signalling the caller to keep the existing body.
func resolvePageBody(cmd *cobra.Command, body, bodyFile, format string) (apiclient.PageBody, bool, error) {
	if !cmd.Flags().Changed("body") && !cmd.Flags().Changed("body-file") {
		return apiclient.PageBody{}, false, nil
	}
	text := body
	if cmd.Flags().Changed("body-file") {
		var raw []byte
		var err error
		if bodyFile == "-" {
			raw, err = io.ReadAll(os.Stdin)
		} else {
			raw, err = os.ReadFile(bodyFile)
		}
		if err != nil {
			return apiclient.PageBody{}, false, cerrors.Wrap(err, cerrors.CategoryUsage,
				"PAGE_BODY_READ", "failed to read page body")
		}
		text = string(raw)
	}
	text = strings.TrimSpace(text)
	switch format {
	case "storage", "wiki":
		return apiclient.PageBody{Value: text, Format: format}, true, nil
	case "markdown":
		return apiclient.PageBody{Value: render.MarkdownToStorage(text, os.Stderr), Format: "storage"}, true, nil
	default:
		return apiclient.PageBody{}, false, cerrors.Newf(cerrors.CategoryUsage,
			"PAGE_BAD_FORMAT", "unknown body format %q (use storage, wiki or markdown)", format)
	}
}

// stdinIsTTY reports whether stdin is an interactive terminal.
func stdinIsTTY() bool { return isTerminal(os.Stdin) }

// confirmDelete enforces the delete safety guard: --yes skips it, a non-TTY
// without --yes is refused, and a TTY prompts on stderr. prompt describes the
// thing being deleted, e.g. "page 123 (moves it to the trash)".
func confirmDelete(prompt string, yes bool) error {
	if yes {
		return nil
	}
	if !stdinIsTTY() {
		return cerrors.New(cerrors.CategoryUsage, "DELETE_NEEDS_YES",
			"delete requires --yes when stdin is not a terminal").
			WithHint("Re-run with --yes to confirm the deletion.")
	}
	fmt.Fprintf(os.Stderr, "Delete %s? [y/N] ", prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
		return cerrors.New(cerrors.CategoryUsage, "DELETE_ABORTED", "deletion cancelled")
	}
	return nil
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
		outputPath string
	)
	cmd := &cobra.Command{
		Use:   "get <id|url>",
		Short: "Fetch a page and render its body",
		Long: "Fetch a single page. Use --scope to read only part of the body:\n" +
			"  outline  list the headings (start here when the structure is unknown)\n" +
			"  section  one section, identified by --section <id> from the outline\n" +
			"  keyword  blocks matching --keyword, with their heading for context\n" +
			"  full     the entire body (default)\n\n" +
			"Rendering to markdown/text drops content it cannot represent (macros,\n" +
			"images); when that happens the result carries a render_notes field.\n" +
			"Use --as raw to get the untouched source body instead.",
		Example: "  # render the whole page as Markdown\n" +
			"  confluence-cli page get 123456\n\n" +
			"  # list the headings, then read just one section\n" +
			"  confluence-cli page get 123456 --scope outline\n" +
			"  confluence-cli page get 123456 --scope section --section sec-2\n\n" +
			"  # get the untouched storage XHTML (macros and all)\n" +
			"  confluence-cli page get 123456 --as raw\n\n" +
			"  # a page URL works in place of an ID\n" +
			"  confluence-cli page get https://wiki.example.com/pages/viewpage.action?pageId=123456",
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
				if as == "raw" {
					// raw emits the body exactly as fetched, with no rendering;
					// slicing the unparsed source is not supported.
					if scope != render.ScopeFull {
						return cerrors.New(cerrors.CategoryUsage, "RAW_NEEDS_FULL_SCOPE",
							"--as raw supports only --scope full").
							WithHint("Drop --scope, or drop --as raw to read a section.")
					}
					out.Body = page.Body.Value
					out.ScopeApplied = "raw"
				} else {
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
					out.RenderNotes = rendered.Notes
				}
			}
			if outputPath != "" {
				if noBody {
					return cerrors.New(cerrors.CategoryUsage, "OUTPUT_NEEDS_BODY",
						"--output has nothing to write together with --no-body").
						WithHint("Drop --no-body to write the page body to the file.")
				}
				if err := os.WriteFile(outputPath, []byte(out.Body), 0o644); err != nil {
					return cerrors.Wrap(err, cerrors.CategoryUsage, "OUTPUT_WRITE",
						"failed to write the page body to "+outputPath).
						WithHint("Check the path is writable and its parent directory exists.")
				}
				out.Bytes = len(out.Body)
				out.OutputPath = outputPath
				// Body now lives on disk; keep it out of stdout so a large page
				// never floods the agent's context.
				out.Body = ""
			}
			return s.emit(out)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&outputPath, "output", "o", "",
		"write the page body to a file; stdout then carries only metadata (id, title, output_path, bytes)")
	f.StringVar(&bodyFormat, "body-format", "storage", "source body format: storage or view")
	f.StringVar(&detail, "detail", "simple", "block detail: simple, with-ids or full")
	f.StringVar(&scope, "scope", "full", "read scope: full, outline, section or keyword")
	f.StringVar(&section, "section", "", "section ID (with --scope section)")
	f.StringVar(&keyword, "keyword", "", "keyword (with --scope keyword)")
	f.StringVar(&as, "as", "markdown", "output form: markdown, text or raw (unrendered source)")
	f.BoolVar(&noBody, "no-body", false, "fetch metadata only, skip the body")
	enumComplete(cmd, "body-format", "storage", "view")
	enumComplete(cmd, "detail", "simple", "with-ids", "full")
	enumComplete(cmd, "scope", "full", "outline", "section", "keyword")
	enumComplete(cmd, "as", "markdown", "text", "raw")
	return cmd
}

func newPageChildrenCmd(s *appState) *cobra.Command {
	var (
		limit  int
		all    bool
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "children <id|url>",
		Short: "List the direct child pages of a page",
		Example: "  confluence-cli page children 123456\n" +
			"  confluence-cli page children 123456 --all --format table",
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
			items, info, err := collectPage(func(cursor string) (apiclient.ListResult[apiclient.Page], error) {
				return client.ListChildren(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: cursor})
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

// addBodyFlags registers the shared --body/--body-file/--body-format flags.
func addBodyFlags(cmd *cobra.Command, body, bodyFile, format *string) {
	f := cmd.Flags()
	f.StringVar(body, "body", "", "body text")
	f.StringVar(bodyFile, "body-file", "", "read body from a file ('-' for stdin)")
	f.StringVar(format, "body-format", "storage", "body format: storage, wiki or markdown")
	enumComplete(cmd, "body-format", "storage", "wiki", "markdown")
}

func newPageCreateCmd(s *appState) *cobra.Command {
	var (
		space, title, parent   string
		body, bodyFile, format string
		dryRun                 bool
	)
	cmd := &cobra.Command{
		Use:   "create --space <KEY> --title <TITLE>",
		Short: "Create a new page",
		Long: "Create a page in a space. Use --parent to nest it under an existing\n" +
			"page. The body may be storage-format XHTML, Confluence wiki markup or\n" +
			"Markdown (--body-format markdown, converted on a best-effort basis).",
		Example: "  # create a page from a Markdown file, nested under a parent\n" +
			"  confluence-cli page create --space ENG --title \"Release Notes\" \\\n" +
			"    --parent 123456 --body-format markdown --body-file notes.md\n\n" +
			"  # preview the request without sending it\n" +
			"  confluence-cli page create --space ENG --title Draft --body \"<p>hi</p>\" --dry-run",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if space == "" {
				return cerrors.New(cerrors.CategoryUsage, "PAGE_NO_SPACE",
					"--space is required to create a page")
			}
			if title == "" {
				return cerrors.New(cerrors.CategoryUsage, "PAGE_NO_TITLE",
					"--title is required to create a page")
			}
			parentID := ""
			if parent != "" {
				id, err := resolvePageID(parent)
				if err != nil {
					return err
				}
				parentID = id
			}
			pageBody, _, err := resolvePageBody(cmd, body, bodyFile, format)
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.CreatePageReq{
				SpaceKey: space, Title: title, ParentID: parentID, Body: pageBody,
			}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			page, err := client.CreatePage(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(pageMetaOutput(page))
		},
	}
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "space key for the new page")
	f.StringVar(&title, "title", "", "page title")
	f.StringVar(&parent, "parent", "", "parent page ID or URL")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	addBodyFlags(cmd, &body, &bodyFile, &format)
	_ = cmd.RegisterFlagCompletionFunc("space", completeSpaceKeys(s))
	return cmd
}

func newPageUpdateCmd(s *appState) *cobra.Command {
	var (
		title, body, bodyFile, format string
		version                       int
		minor                         bool
		message                       string
		dryRun                        bool
	)
	cmd := &cobra.Command{
		Use:   "update <id|url>",
		Short: "Update a page's title and/or body",
		Long: "Update an existing page. Omitted fields are kept as-is. The new\n" +
			"version is the current version + 1; pass --version to assert the\n" +
			"version you last read (a mismatch fails with a conflict error).",
		Example: "  # retitle a page, keeping its body\n" +
			"  confluence-cli page update 123456 --title \"New Title\"\n\n" +
			"  # replace the body from Markdown, asserting the version last read\n" +
			"  confluence-cli page update 123456 --body-format markdown --body-file body.md --version 7",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolvePageID(args[0])
			if err != nil {
				return err
			}
			pageBody, bodyProvided, err := resolvePageBody(cmd, body, bodyFile, format)
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("title") && !bodyProvided {
				return cerrors.New(cerrors.CategoryUsage, "PAGE_UPDATE_NOOP",
					"nothing to update: pass --title and/or --body")
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.UpdatePageReq{
				ID: id, Title: title, ExpectVersion: version,
				Minor: minor, Message: message,
			}
			if bodyProvided {
				req.Body = &pageBody
			}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			page, err := client.UpdatePage(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(pageMetaOutput(page))
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "new page title (kept when omitted)")
	f.IntVar(&version, "version", 0, "expected current version (fetched when omitted)")
	f.BoolVar(&minor, "minor", false, "mark the edit as minor")
	f.StringVar(&message, "message", "", "version comment for the edit")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	addBodyFlags(cmd, &body, &bodyFile, &format)
	return cmd
}

func newPageDeleteCmd(s *appState) *cobra.Command {
	var (
		purge  bool
		yes    bool
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "delete <id|url>...",
		Short: "Delete one or more pages (move them to the trash)",
		Long: "Move a page to the trash. With --purge the trashed page is then\n" +
			"permanently removed. Pass several IDs to delete them in one run, or a\n" +
			"single '-' to read newline-separated IDs from stdin. Deletion requires\n" +
			"--yes (or an interactive confirmation when stdin is a terminal); --yes\n" +
			"applies to the whole batch.",
		Example: "  confluence-cli page delete 123456 --yes\n" +
			"  confluence-cli page delete 123456 123457 --purge --yes\n" +
			"  confluence-cli search --text obsolete --format json | jq -r '.items[].id' | confluence-cli page delete - --yes",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := collectBatchArgs(args, cmd.InOrStdin())
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			if !dryRun {
				if cerr := confirmDelete(deletePrompt("page", items, "(moves it to the trash)"), yes); cerr != nil {
					return cerr
				}
			}
			status := "trashed"
			if purge {
				status = "purged"
			}
			return runBatch(s, items, func(arg string) (any, error) {
				id, rerr := resolvePageID(arg)
				if rerr != nil {
					return nil, rerr
				}
				req := apiclient.DeletePageReq{ID: id, Purge: purge}
				if dryRun {
					return dryRunPlan(client, ctx, req)
				}
				if derr := client.DeletePage(ctx, req); derr != nil {
					return nil, derr
				}
				return map[string]any{"id": id, "status": status}, nil
			})
		},
	}
	f := cmd.Flags()
	f.BoolVar(&purge, "purge", false, "permanently delete (removes the trashed page)")
	f.BoolVar(&yes, "yes", false, "skip the deletion confirmation")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}

func newPageMoveCmd(s *appState) *cobra.Command {
	var (
		targetParent, targetSpace string
		dryRun                    bool
	)
	cmd := &cobra.Command{
		Use:   "move <id|url>",
		Short: "Move a page under a new parent and/or space",
		Example: "  # reparent a page\n" +
			"  confluence-cli page move 123456 --target-parent 999\n\n" +
			"  # move a page into another space\n" +
			"  confluence-cli page move 123456 --target-space DOCS",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolvePageID(args[0])
			if err != nil {
				return err
			}
			if targetParent == "" && targetSpace == "" {
				return cerrors.New(cerrors.CategoryUsage, "PAGE_MOVE_NO_TARGET",
					"specify --target-parent and/or --target-space")
			}
			parentID := ""
			if targetParent != "" {
				if parentID, err = resolvePageID(targetParent); err != nil {
					return err
				}
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.MovePageReq{
				ID: id, TargetParent: parentID, TargetSpace: targetSpace,
			}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			page, err := client.MovePage(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(pageMetaOutput(page))
		},
	}
	f := cmd.Flags()
	f.StringVar(&targetParent, "target-parent", "", "new parent page ID or URL")
	f.StringVar(&targetSpace, "target-space", "", "new space key")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	_ = cmd.RegisterFlagCompletionFunc("target-space", completeSpaceKeys(s))
	return cmd
}

func newPageCopyCmd(s *appState) *cobra.Command {
	var (
		title, space, parent string
		dryRun               bool
	)
	cmd := &cobra.Command{
		Use:   "copy <id|url> --title <TITLE>",
		Short: "Copy a page's title and body to a new page",
		Long: "Create a new page from an existing one. The copy is shallow: it\n" +
			"carries the title and body only, not child pages or attachments.",
		Example: "  confluence-cli page copy 123456 --title \"Copy of the spec\"\n" +
			"  confluence-cli page copy 123456 --title Draft --space SANDBOX",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolvePageID(args[0])
			if err != nil {
				return err
			}
			if title == "" {
				return cerrors.New(cerrors.CategoryUsage, "PAGE_NO_TITLE",
					"--title is required for the copied page")
			}
			parentID := ""
			if parent != "" {
				if parentID, err = resolvePageID(parent); err != nil {
					return err
				}
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.CopyPageReq{
				SourceID: id, Title: title, SpaceKey: space, ParentID: parentID,
			}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			page, err := client.CopyPage(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(pageMetaOutput(page))
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "title for the copied page")
	f.StringVar(&space, "space", "", "space key for the copy (default: source space)")
	f.StringVar(&parent, "parent", "", "parent page for the copy (default: source parent)")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	_ = cmd.RegisterFlagCompletionFunc("space", completeSpaceKeys(s))
	return cmd
}

func newPageDescendantsCmd(s *appState) *cobra.Command {
	var (
		limit  int
		all    bool
		cursor string
	)
	cmd := &cobra.Command{
		Use:     "descendants <id|url>",
		Short:   "List all descendant pages of a page",
		Example: "  confluence-cli page descendants 123456 --all",
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
			items, info, err := collectPage(func(cursor string) (apiclient.ListResult[apiclient.Page], error) {
				return client.ListDescendants(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: cursor})
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

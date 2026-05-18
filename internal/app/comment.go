package app

import (
	"io"
	"os"
	"strings"

	"github.com/angelmsger/confluence-cli/internal/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/angelmsger/confluence-cli/internal/render"
	"github.com/spf13/cobra"
)

// commentOutput is the result shape for comment commands.
type commentOutput struct {
	ID       string             `json:"id"`
	PageID   string             `json:"page_id,omitempty"`
	ParentID string             `json:"parent_id,omitempty"`
	URL      string             `json:"url,omitempty"`
	Version  *apiclient.Version `json:"version,omitempty"`
	Body     string             `json:"body,omitempty"`
}

func newCommentCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Read and post page comments",
	}
	cmd.AddCommand(
		newCommentListCmd(s), newCommentAddCmd(s),
		newCommentUpdateCmd(s), newCommentDeleteCmd(s),
	)
	return cmd
}

func toCommentOutput(c apiclient.Comment, as string) commentOutput {
	out := commentOutput{
		ID: c.ID, PageID: c.PageID, ParentID: c.ParentID,
		URL: c.URL, Version: c.Version,
	}
	if c.Body != nil {
		if rendered, err := render.Render(c.Body.Value, render.Options{
			Scope: render.ScopeFull, As: as,
		}); err == nil {
			out.Body = rendered.Body
		} else {
			out.Body = c.Body.Value
		}
	}
	return out
}

func newCommentListCmd(s *appState) *cobra.Command {
	var (
		limit  int
		all    bool
		cursor string
		as     string
	)
	cmd := &cobra.Command{
		Use:   "list <id|url>",
		Short: "List the footer comments of a page",
		Example: "  confluence-cli comment list 123456\n" +
			"  confluence-cli comment list 123456 --all --as text",
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
			items, info, err := collectPage(func(cursor string) (apiclient.ListResult[apiclient.Comment], error) {
				return client.ListComments(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: cursor})
			}, cursor, all)
			if err != nil {
				return err
			}
			out := make([]commentOutput, 0, len(items))
			for _, c := range items {
				out = append(out, toCommentOutput(c, as))
			}
			return s.emitList(out, info)
		},
	}
	addListFlags(cmd, &limit, &all, &cursor)
	cmd.Flags().StringVar(&as, "as", "markdown", "render comment bodies as markdown or text")
	enumComplete(cmd, "as", "markdown", "text")
	return cmd
}

func newCommentAddCmd(s *appState) *cobra.Command {
	var (
		body     string
		bodyFile string
		parent   string
		format   string
	)
	cmd := &cobra.Command{
		Use:   "add <id|url>",
		Short: "Post a comment on a page",
		Long: "Post a footer comment on a page. Use --parent to reply to an existing\n" +
			"comment.",
		Example: "  confluence-cli comment add 123456 --body \"Looks good to me.\"\n\n" +
			"  # reply to a comment, reading the body from stdin\n" +
			"  echo \"Agreed.\" | confluence-cli comment add 123456 --parent 789 --body-file -",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolvePageID(args[0])
			if err != nil {
				return err
			}
			text, err := readCommentBody(body, bodyFile)
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			created, err := client.AddComment(ctx, apiclient.AddCommentReq{
				PageID: id, ParentID: parent, Body: text, Format: format,
			})
			if err != nil {
				return err
			}
			return s.emit(toCommentOutput(*created, "markdown"))
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "comment body text")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "read body from a file ('-' for stdin)")
	cmd.Flags().StringVar(&parent, "parent", "", "parent comment ID, to post a reply")
	cmd.Flags().StringVar(&format, "body-format", "storage", "body format: storage or wiki")
	enumComplete(cmd, "body-format", "storage", "wiki")
	return cmd
}

func newCommentUpdateCmd(s *appState) *cobra.Command {
	var (
		body     string
		bodyFile string
		format   string
		version  int
		dryRun   bool
	)
	cmd := &cobra.Command{
		Use:   "update <comment-id|url>",
		Short: "Edit a comment's body",
		Long: "Replace a footer comment's body. The new version is the current\n" +
			"version + 1; pass --version to assert the version you last read.",
		Example: "  confluence-cli comment update 789 --body \"Updated: looks good.\"\n" +
			"  echo \"Revised.\" | confluence-cli comment update 789 --body-file -",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolveCommentID(args[0])
			if err != nil {
				return err
			}
			text, err := readCommentBody(body, bodyFile)
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.UpdateCommentReq{
				ID: id, Body: text, Format: format, ExpectVersion: version,
			}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			updated, err := client.UpdateComment(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(toCommentOutput(*updated, "markdown"))
		},
	}
	f := cmd.Flags()
	f.StringVar(&body, "body", "", "new comment body text")
	f.StringVar(&bodyFile, "body-file", "", "read body from a file ('-' for stdin)")
	f.StringVar(&format, "body-format", "storage", "body format: storage or wiki")
	f.IntVar(&version, "version", 0, "expected current version (fetched when omitted)")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	enumComplete(cmd, "body-format", "storage", "wiki")
	return cmd
}

func newCommentDeleteCmd(s *appState) *cobra.Command {
	var yes, dryRun bool
	cmd := &cobra.Command{
		Use:   "delete <comment-id|url>",
		Short: "Delete a comment",
		Long: "Delete a comment by its content ID. Deletion requires --yes, or an\n" +
			"interactive confirmation when stdin is a terminal.",
		Example: "  confluence-cli comment delete 789 --yes",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolveCommentID(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.DeleteCommentReq{ID: id}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			if err := confirmDelete("comment "+id, yes); err != nil {
				return err
			}
			if err := client.DeleteComment(ctx, req); err != nil {
				return err
			}
			return s.emit(map[string]any{"id": id, "status": "deleted"})
		},
	}
	f := cmd.Flags()
	f.BoolVar(&yes, "yes", false, "skip the deletion confirmation")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}

// readCommentBody resolves the comment text from --body or --body-file.
func readCommentBody(body, bodyFile string) (string, error) {
	if body != "" {
		return body, nil
	}
	if bodyFile == "" {
		return "", cerrors.New(cerrors.CategoryUsage, "COMMENT_NO_BODY",
			"provide comment text with --body or --body-file")
	}
	var raw []byte
	var err error
	if bodyFile == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(bodyFile)
	}
	if err != nil {
		return "", cerrors.Wrap(err, cerrors.CategoryUsage, "COMMENT_BODY_READ",
			"failed to read comment body")
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", cerrors.New(cerrors.CategoryUsage, "COMMENT_NO_BODY",
			"comment body is empty")
	}
	return text, nil
}

package app

import (
	"os"

	"github.com/angelmsger/confluence-cli/internal/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

func newAttachmentCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "List and download page attachments",
	}
	cmd.AddCommand(newAttachmentListCmd(s), newAttachmentDownloadCmd(s))
	return cmd
}

func newAttachmentListCmd(s *appState) *cobra.Command {
	var (
		limit int
		all   bool
	)
	cmd := &cobra.Command{
		Use:   "list <id|url>",
		Short: "List the attachments of a page",
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
			items, err := collectList(func(cursor string) (apiclient.ListResult[apiclient.Attachment], error) {
				return client.ListAttachments(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: cursor})
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

// downloadOutput is the result shape for `attachment download`.
type downloadOutput struct {
	AttachmentID string `json:"attachment_id"`
	Output       string `json:"output"`
	ContentType  string `json:"content_type,omitempty"`
	Bytes        int64  `json:"bytes"`
}

func newAttachmentDownloadCmd(s *appState) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "download <attachment-id|url>",
		Short: "Download an attachment's content",
		Long:  "Download an attachment by its content ID. Use --output - to stream to stdout.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attID, err := resolveID(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			att, err := client.GetAttachment(ctx, attID)
			if err != nil {
				return err
			}

			dest := output
			if dest == "" {
				dest = att.Title
				if dest == "" {
					dest = "attachment-" + attID
				}
			}

			var w *os.File
			toStdout := dest == "-"
			if toStdout {
				w = os.Stdout
			} else {
				w, err = os.Create(dest)
				if err != nil {
					return cerrors.Wrap(err, cerrors.CategoryUsage, "ATTACH_WRITE",
						"failed to create output file")
				}
				defer w.Close()
			}

			meta, err := client.DownloadAttachment(ctx, *att, w)
			if err != nil {
				return err
			}
			if toStdout {
				return nil // raw bytes already written to stdout
			}
			return s.emit(downloadOutput{
				AttachmentID: attID, Output: dest,
				ContentType: meta.ContentType, Bytes: meta.Bytes,
			})
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path ('-' for stdout)")
	return cmd
}

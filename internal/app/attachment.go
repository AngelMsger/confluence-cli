package app

import (
	"io"
	"os"
	"path/filepath"

	"github.com/angelmsger/confluence-cli/internal/apiclient"
	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/spf13/cobra"
)

func newAttachmentCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "Upload, list, download and delete page attachments",
	}
	cmd.AddCommand(
		newAttachmentListCmd(s), newAttachmentDownloadCmd(s),
		newAttachmentUploadCmd(s), newAttachmentUpdateCmd(s), newAttachmentDeleteCmd(s),
	)
	return cmd
}

// readUploadFile reads an attachment's bytes from a path, or from stdin when
// the path is "-".
func readUploadFile(path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, cerrors.Wrap(err, cerrors.CategoryUsage, "ATTACH_FILE_READ",
				"failed to read attachment from stdin")
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, cerrors.Wrap(err, cerrors.CategoryUsage, "ATTACH_FILE_READ",
			"failed to read attachment file")
	}
	return data, nil
}

func newAttachmentListCmd(s *appState) *cobra.Command {
	var (
		limit  int
		all    bool
		cursor string
	)
	cmd := &cobra.Command{
		Use:     "list <id|url>",
		Short:   "List the attachments of a page",
		Example: "  confluence-cli attachment list 123456",
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
			items, info, err := collectPage(func(cursor string) (apiclient.ListResult[apiclient.Attachment], error) {
				return client.ListAttachments(ctx, id, apiclient.ListOpts{Limit: limit, Cursor: cursor})
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
		Use:   "download <attachment-id>",
		Short: "Download an attachment's content",
		Long:  "Download an attachment by its content ID. Use --output - to stream to stdout.",
		Example: "  confluence-cli attachment download att12345 --output spec.pdf\n" +
			"  confluence-cli attachment download att12345 --output -",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attID, err := resolveAttachmentID(args[0])
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

func newAttachmentUploadCmd(s *appState) *cobra.Command {
	var (
		file, name, comment string
		minor, dryRun       bool
	)
	cmd := &cobra.Command{
		Use:   "upload <page-id|url> --file <PATH>",
		Short: "Attach a file to a page",
		Long: "Upload a file as a new attachment on a page. Use --file - to read the\n" +
			"file from stdin (--name is then required). When an attachment with the\n" +
			"same name already exists, Confluence stores the upload as a new version.",
		Example: "  confluence-cli attachment upload 123456 --file diagram.png\n" +
			"  confluence-cli attachment upload 123456 --file report.pdf --comment \"Q3 figures\"\n" +
			"  confluence-cli attachment upload 123456 --file - --name notes.txt --dry-run",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pageID, err := resolvePageID(args[0])
			if err != nil {
				return err
			}
			if file == "" {
				return cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_FILE",
					"--file is required to upload an attachment")
			}
			fileName := name
			if fileName == "" {
				if file == "-" {
					return cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_NAME",
						"--name is required when reading the file from stdin")
				}
				fileName = filepath.Base(file)
			}
			data, err := readUploadFile(file)
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.UploadAttachmentReq{
				PageID: pageID, FileName: fileName, Data: data,
				Comment: comment, MinorEdit: minor,
			}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			att, err := client.UploadAttachment(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(att)
		},
	}
	f := cmd.Flags()
	f.StringVar(&file, "file", "", "path to the file to upload ('-' for stdin)")
	f.StringVar(&name, "name", "", "attachment file name (default: base name of --file)")
	f.StringVar(&comment, "comment", "", "version comment for the attachment")
	f.BoolVar(&minor, "minor", false, "mark the upload as a minor edit")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}

func newAttachmentUpdateCmd(s *appState) *cobra.Command {
	var (
		file, name, comment string
		minor, dryRun       bool
	)
	cmd := &cobra.Command{
		Use:   "update <attachment-id> --file <PATH>",
		Short: "Replace an attachment's content with a new version",
		Long: "Upload new content for an existing attachment, creating a new version.\n" +
			"The attachment keeps its name unless --name is given. Use --file - to\n" +
			"read from stdin.",
		Example: "  confluence-cli attachment update att12345 --file diagram-v2.png\n" +
			"  confluence-cli attachment update att12345 --file report.pdf --comment \"fix totals\"",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attID, err := resolveAttachmentID(args[0])
			if err != nil {
				return err
			}
			if file == "" {
				return cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_FILE",
					"--file is required to update an attachment")
			}
			data, err := readUploadFile(file)
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			// The update endpoint is page-scoped; resolve the parent page.
			att, err := client.GetAttachment(ctx, attID)
			if err != nil {
				return err
			}
			if att.PageID == "" {
				return cerrors.New(cerrors.CategoryNotFound, "ATTACH_NO_CONTAINER",
					"could not resolve the page this attachment belongs to")
			}
			fileName := name
			if fileName == "" {
				fileName = att.Title
			}
			if fileName == "" && file != "-" {
				fileName = filepath.Base(file)
			}
			if fileName == "" {
				return cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_NAME",
					"--name is required when the file name cannot be determined")
			}
			req := apiclient.UpdateAttachmentReq{
				PageID: att.PageID, AttachmentID: attID, FileName: fileName,
				Data: data, Comment: comment, MinorEdit: minor,
			}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			updated, err := client.UpdateAttachment(ctx, req)
			if err != nil {
				return err
			}
			return s.emit(updated)
		},
	}
	f := cmd.Flags()
	f.StringVar(&file, "file", "", "path to the new content ('-' for stdin)")
	f.StringVar(&name, "name", "", "attachment file name (default: current name)")
	f.StringVar(&comment, "comment", "", "version comment for the update")
	f.BoolVar(&minor, "minor", false, "mark the update as a minor edit")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}

func newAttachmentDeleteCmd(s *appState) *cobra.Command {
	var yes, dryRun bool
	cmd := &cobra.Command{
		Use:   "delete <attachment-id>",
		Short: "Delete an attachment",
		Long: "Delete an attachment by its content ID. Deletion requires --yes, or an\n" +
			"interactive confirmation when stdin is a terminal.",
		Example: "  confluence-cli attachment delete att12345 --yes",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attID, err := resolveAttachmentID(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := cmdContext(s)
			defer cancel()
			client, err := s.newClient(ctx)
			if err != nil {
				return err
			}
			req := apiclient.DeleteAttachmentReq{AttachmentID: attID}
			if dryRun {
				return emitDryRun(s, client, ctx, req)
			}
			if err := confirmDelete("attachment "+attID, yes); err != nil {
				return err
			}
			if err := client.DeleteAttachment(ctx, req); err != nil {
				return err
			}
			return s.emit(map[string]any{"id": attID, "status": "deleted"})
		},
	}
	f := cmd.Flags()
	f.BoolVar(&yes, "yes", false, "skip the deletion confirmation")
	f.BoolVar(&dryRun, "dry-run", false, "print the request without sending it")
	return cmd
}

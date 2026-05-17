# Attachments

## List a page's attachments

```bash
confluence-cli attachment list <id|url>
confluence-cli attachment list <id|url> --all
```

Each attachment has `id`, `title`, `media_type`, `file_size`, `page_id` and
`download_url`. The `id` is what `attachment download` needs.

## Download an attachment

```bash
# write to a file named after the attachment in the current directory
confluence-cli attachment download <attachment-id>

# choose the path
confluence-cli attachment download <attachment-id> --output ./spec.pdf

# stream raw bytes to stdout (e.g. to pipe elsewhere)
confluence-cli attachment download <attachment-id> --output -
```

Workflow: run `attachment list` on the page first to discover attachment IDs,
then `attachment download` with the chosen ID.

When writing to a file, the command prints a JSON summary (`attachment_id`,
`output`, `content_type`, `bytes`). With `--output -` the raw content goes to
stdout and nothing else is printed.

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

## Upload a file to a page

```bash
# attach a file; the attachment name defaults to the file's base name
confluence-cli attachment upload <page-id|url> --file ./diagram.png

# override the name, add a version comment
confluence-cli attachment upload <page-id|url> --file ./report.pdf \
  --name "Q3 Report.pdf" --comment "Q3 figures"

# read the file from stdin (--name is then required)
cat notes.txt | confluence-cli attachment upload <page-id|url> --file - --name notes.txt

# preview the request without sending it
confluence-cli attachment upload <page-id|url> --file ./diagram.png --dry-run
```

Uploading a file whose name matches an existing attachment stores it as a new
version of that attachment rather than creating a duplicate.

## Replace an attachment's content

```bash
# upload new content for an existing attachment (creates a new version)
confluence-cli attachment update <attachment-id|url> --file ./diagram-v2.png
```

`update` takes the **attachment** ID (run `attachment list` to find it). The
attachment keeps its name unless `--name` is given.

## Delete an attachment

```bash
confluence-cli attachment delete <attachment-id|url> --yes
```

Deletion requires `--yes` (or an interactive confirmation when stdin is a
terminal). Use `--dry-run` to preview the request.

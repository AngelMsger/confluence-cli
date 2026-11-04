package apiclient

import (
	"context"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// A write changes remote state, so a failure after the request was sent can
// still have applied it. The family's write-outcome rule (cerrors.FromWrite)
// keeps such a failure from reading as retryable: only a definite refusal
// passes through; anything else says the outcome is unknown and points at the
// read below, so an agent reconciles before it repeats the write. Each target
// names what the write changes and a bounded, read-only command that shows it.

// doWriteJSON sends a JSON write and classifies its failure with the write rule.
func (c *apiClient) doWriteJSON(ctx context.Context, method, path string, body, out any, target cerrors.WriteTarget) error {
	return cerrors.FromWrite(c.doJSON(ctx, method, path, nil, body, out), target)
}

// doWriteMultipart sends an upload and classifies its failure with the write rule.
func (c *apiClient) doWriteMultipart(ctx context.Context, method, path string, file multipartFile, fields map[string]string, out any, target cerrors.WriteTarget) error {
	return cerrors.FromWrite(c.doMultipart(ctx, method, path, file, fields, out), target)
}

// contentReadStep reads any content by its ID. The v1 content endpoint serves
// pages, comments and attachments alike, so page get --no-body shows a
// comment's or attachment's current version and status too, and a 404 once it
// is deleted.
func contentReadStep(id string) string {
	return "confluence-cli page get " + cerrors.ShellArg(id) + " --no-body"
}

func pageWriteTarget(id string) cerrors.WriteTarget {
	return cerrors.WriteTarget{
		Description: "page " + id,
		ReadSteps:   []string{contentReadStep(id)},
	}
}

// newPageWriteTarget is for a create or copy: the new page has no ID yet, so
// the read searches its space for the title.
func newPageWriteTarget(space, title string) cerrors.WriteTarget {
	cql := "type = page AND space = " + quote(space) + " AND title = " + quote(title)
	return cerrors.WriteTarget{
		Description: "new page " + quote(title) + " in space " + space,
		ReadSteps:   []string{"confluence-cli search " + cerrors.ShellArg(cql) + " --limit 25"},
	}
}

func newCommentWriteTarget(pageID string) cerrors.WriteTarget {
	return cerrors.WriteTarget{
		Description: "comment on page " + pageID,
		ReadSteps:   []string{"confluence-cli comment list " + cerrors.ShellArg(pageID) + " --limit 25"},
	}
}

func commentWriteTarget(id string) cerrors.WriteTarget {
	return cerrors.WriteTarget{
		Description: "comment " + id,
		ReadSteps:   []string{contentReadStep(id)},
	}
}

func newAttachmentWriteTarget(pageID, fileName string) cerrors.WriteTarget {
	return cerrors.WriteTarget{
		Description: "attachment " + quote(fileName) + " on page " + pageID,
		ReadSteps:   []string{"confluence-cli attachment list " + cerrors.ShellArg(pageID) + " --limit 25"},
	}
}

func attachmentWriteTarget(id string) cerrors.WriteTarget {
	return cerrors.WriteTarget{
		Description: "attachment " + id,
		ReadSteps:   []string{contentReadStep(id)},
	}
}

func labelWriteTarget(pageID string) cerrors.WriteTarget {
	return cerrors.WriteTarget{
		Description: "labels on page " + pageID,
		ReadSteps:   []string{"confluence-cli label list " + cerrors.ShellArg(pageID) + " --limit 25"},
	}
}

func watchWriteTarget(pageID string) cerrors.WriteTarget {
	return cerrors.WriteTarget{
		Description: "your watch on page " + pageID,
		ReadSteps:   []string{"confluence-cli page watch-status " + cerrors.ShellArg(pageID)},
	}
}

// requireCreatedID reports an acknowledged create whose response omitted the
// new content's ID: the write happened, but nothing can address it.
func requireCreatedID(id string, target cerrors.WriteTarget) error {
	if id == "" {
		return cerrors.MissingWriteIdentity(target)
	}
	return nil
}

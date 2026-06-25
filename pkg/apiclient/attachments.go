package apiclient

import (
	"context"
	"io"
	"net/http"
	"net/url"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// ListAttachments lists the attachments of a page.
func (c *apiClient) ListAttachments(ctx context.Context, pageID string, opt ListOpts) (ListResult[Attachment], error) {
	limit := c.limitOf(opt)
	q := offsetQuery(opt.Cursor, limit)
	q.Set("expand", "version,metadata,extensions")

	path := c.v1Base() + "/content/" + url.PathEscape(pageID) + "/child/attachment"
	var raw rawContentList
	if err := c.getJSON(ctx, path, q, &raw); err != nil {
		return ListResult[Attachment]{}, err
	}
	res := ListResult[Attachment]{Next: nextOffsetToken(opt.Cursor, limit, len(raw.Results))}
	for _, r := range raw.Results {
		res.Items = append(res.Items, *c.mapAttachment(r, pageID))
	}
	return res, nil
}

// GetAttachment fetches a single attachment's metadata by content ID.
func (c *apiClient) GetAttachment(ctx context.Context, id string) (*Attachment, error) {
	q := url.Values{}
	q.Set("expand", "metadata,extensions,container")
	var raw rawContent
	if err := c.getJSON(ctx, c.v1Base()+"/content/"+url.PathEscape(id), q, &raw); err != nil {
		return nil, err
	}
	return c.mapAttachment(raw, ""), nil
}

// DownloadAttachment streams an attachment's binary content into w.
func (c *apiClient) DownloadAttachment(ctx context.Context, att Attachment, w io.Writer) (DownloadMeta, error) {
	if att.DownloadURL == "" {
		return DownloadMeta{}, cerrors.New(cerrors.CategoryUsage, "ATTACH_NO_URL",
			"attachment has no download URL")
	}
	req, err := http.NewRequest(http.MethodGet, c.absURL(att.DownloadURL), nil)
	if err != nil {
		return DownloadMeta{}, cerrors.Wrap(err, cerrors.CategoryUsage, "BAD_REQUEST",
			"failed to build download request")
	}
	resp, err := c.http.Do(ctx, req)
	if err != nil {
		return DownloadMeta{}, cerrors.Wrap(err, cerrors.CategoryNetwork, "NETWORK",
			"attachment download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return DownloadMeta{}, c.httpError(resp)
	}
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return DownloadMeta{}, cerrors.Wrap(err, cerrors.CategoryNetwork, "NETWORK",
			"attachment download interrupted")
	}
	return DownloadMeta{ContentType: resp.Header.Get("Content-Type"), Bytes: n}, nil
}

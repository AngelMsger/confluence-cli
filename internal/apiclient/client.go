package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strings"

	cerrors "github.com/angelmsger/confluence-cli/internal/errors"
	"github.com/angelmsger/confluence-cli/internal/transport"
	"github.com/angelmsger/confluence-cli/pkg/constants"
)

// Client is the flavor-agnostic Confluence API surface. All methods return
// normalized models; flavor-specific request shapes are hidden.
type Client interface {
	Flavor() Flavor
	BaseURL() string
	Ping(ctx context.Context) (ServerInfo, error)

	GetPage(ctx context.Context, id string, opt GetPageOpts) (*Page, error)
	ListChildren(ctx context.Context, id string, opt ListOpts) (ListResult[Page], error)
	ListDescendants(ctx context.Context, id string, opt ListOpts) (ListResult[Page], error)

	CreatePage(ctx context.Context, req CreatePageReq) (*Page, error)
	UpdatePage(ctx context.Context, req UpdatePageReq) (*Page, error)
	DeletePage(ctx context.Context, req DeletePageReq) error
	MovePage(ctx context.Context, req MovePageReq) (*Page, error)
	CopyPage(ctx context.Context, req CopyPageReq) (*Page, error)
	DescribeWrite(ctx context.Context, op any) (WriteRequestPlan, error)

	ListPageVersions(ctx context.Context, id string, opt ListOpts) (ListResult[PageVersion], error)
	RestorePage(ctx context.Context, req RestorePageReq) (*Page, error)

	WatchStatus(ctx context.Context, pageID string) (bool, error)
	SetWatch(ctx context.Context, req WatchReq) error

	Search(ctx context.Context, cql string, opt ListOpts) (ListResult[SearchHit], error)

	ListSpaces(ctx context.Context, opt SpaceListOpts) (ListResult[Space], error)
	GetSpace(ctx context.Context, key string) (*Space, error)

	ListComments(ctx context.Context, pageID string, opt ListOpts) (ListResult[Comment], error)
	AddComment(ctx context.Context, req AddCommentReq) (*Comment, error)

	ListAttachments(ctx context.Context, pageID string, opt ListOpts) (ListResult[Attachment], error)
	GetAttachment(ctx context.Context, id string) (*Attachment, error)
	DownloadAttachment(ctx context.Context, att Attachment, w io.Writer) (DownloadMeta, error)
	UploadAttachment(ctx context.Context, req UploadAttachmentReq) (*Attachment, error)
	UpdateAttachment(ctx context.Context, req UpdateAttachmentReq) (*Attachment, error)
	DeleteAttachment(ctx context.Context, req DeleteAttachmentReq) error

	ListLabels(ctx context.Context, pageID string, opt ListOpts) (ListResult[Label], error)
	AddLabels(ctx context.Context, req AddLabelsReq) ([]Label, error)
	RemoveLabel(ctx context.Context, req RemoveLabelReq) error
}

// apiClient is the single Client implementation. Per-flavor behaviour is
// selected by the flavor field and the helpers in dialect.go / mapping.go.
type apiClient struct {
	flavor   Flavor
	baseURL  string // site root, no trailing slash (Cloud includes /wiki)
	pageSize int
	http     *transport.Client
}

// Config configures a Client.
type Config struct {
	Flavor    Flavor
	BaseURL   string
	PageSize  int
	Transport *transport.Client
}

// New builds a Client. The transport must already carry the auth decorator.
func New(cfg Config) Client {
	ps := cfg.PageSize
	if ps <= 0 {
		ps = constants.DefaultPageSize
	}
	if ps > constants.MaxPageSize {
		ps = constants.MaxPageSize
	}
	return &apiClient{
		flavor:   cfg.Flavor,
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		pageSize: ps,
		http:     cfg.Transport,
	}
}

func (c *apiClient) Flavor() Flavor  { return c.flavor }
func (c *apiClient) BaseURL() string { return c.baseURL }

// limitOf returns the effective page size for a ListOpts.
func (c *apiClient) limitOf(opt ListOpts) int {
	if opt.Limit > 0 {
		if opt.Limit > constants.MaxPageSize {
			return constants.MaxPageSize
		}
		return opt.Limit
	}
	return c.pageSize
}

// getJSON performs a GET and decodes the JSON body into out.
func (c *apiClient) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, out)
}

// doJSON performs an HTTP request and decodes a JSON response into out.
// Non-2xx responses are converted into structured *errors.CLIError values.
func (c *apiClient) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return cerrors.Wrap(err, cerrors.CategoryInternal, "ENCODE", "failed to encode request body")
		}
		reqBody = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, endpoint, reqBody)
	if err != nil {
		return cerrors.Wrap(err, cerrors.CategoryUsage, "BAD_REQUEST", "failed to build request")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(ctx, req)
	if err != nil {
		return cerrors.Wrap(err, cerrors.CategoryNetwork, "NETWORK",
			fmt.Sprintf("request to %s failed", endpoint))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return c.httpError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return cerrors.Wrap(err, cerrors.CategoryParse, "DECODE",
			"failed to decode server response")
	}
	return nil
}

// multipartFile is the binary file part of a multipart/form-data upload.
type multipartFile struct {
	FieldName string
	FileName  string
	Data      []byte
}

// doMultipart performs a multipart/form-data request: file is the binary part
// and fields carries any additional text form fields. It sets the
// X-Atlassian-Token header to bypass Confluence's XSRF guard, which every
// upload endpoint requires. The JSON response, if any, is decoded into out.
func (c *apiClient) doMultipart(ctx context.Context, method, path string, file multipartFile, fields map[string]string, out any) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	encErr := func(err error) error {
		return cerrors.Wrap(err, cerrors.CategoryInternal, "ENCODE", "failed to build multipart body")
	}
	fw, err := mw.CreateFormFile(file.FieldName, file.FileName)
	if err != nil {
		return encErr(err)
	}
	if _, err := fw.Write(file.Data); err != nil {
		return encErr(err)
	}
	// Write fields in a stable order so the request body is deterministic.
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := mw.WriteField(k, fields[k]); err != nil {
			return encErr(err)
		}
	}
	if err := mw.Close(); err != nil {
		return encErr(err)
	}

	endpoint := c.baseURL + path
	req, err := http.NewRequest(method, endpoint, &buf)
	if err != nil {
		return cerrors.Wrap(err, cerrors.CategoryUsage, "BAD_REQUEST", "failed to build request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Atlassian-Token", "nocheck")

	resp, err := c.http.Do(ctx, req)
	if err != nil {
		return cerrors.Wrap(err, cerrors.CategoryNetwork, "NETWORK",
			fmt.Sprintf("request to %s failed", endpoint))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return c.httpError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return cerrors.Wrap(err, cerrors.CategoryParse, "DECODE",
			"failed to decode server response")
	}
	return nil
}

// httpError turns a non-2xx response into a classified CLIError.
func (c *apiClient) httpError(resp *http.Response) error {
	cat := cerrors.FromHTTPStatus(resp.StatusCode)
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	msg := fmt.Sprintf("Confluence returned HTTP %d", resp.StatusCode)
	if detail := extractAPIMessage(snippet); detail != "" {
		msg += ": " + detail
	}
	if resp.StatusCode == http.StatusConflict {
		return cerrors.New(cat, "PAGE_VERSION_CONFLICT",
			msg+" — the page changed since it was last read").
			WithHTTPStatus(resp.StatusCode).
			WithHint("Re-fetch the page to get its current version, then retry.").
			WithNextSteps(
				"confluence-cli page get <id> --no-body",
				"Retry the update with --version set to the version just read.")
	}
	return cerrors.New(cat, "HTTP_"+http.StatusText(resp.StatusCode), msg).
		WithHTTPStatus(resp.StatusCode)
}

// extractAPIMessage best-effort extracts a human message from a Confluence
// JSON error body (both v1 and v2 use a "message" field).
func extractAPIMessage(raw []byte) string {
	var v struct {
		Message string `json:"message"`
		Errors  []struct {
			Title string `json:"title"`
		} `json:"errors"`
	}
	if json.Unmarshal(raw, &v) == nil {
		if v.Message != "" {
			return v.Message
		}
		if len(v.Errors) > 0 && v.Errors[0].Title != "" {
			return v.Errors[0].Title
		}
	}
	return ""
}

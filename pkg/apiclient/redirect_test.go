package apiclient

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/angelmsger/agent-cli-kit/doctor"
	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
	"github.com/angelmsger/confluence-cli/pkg/transport"
)

// A read that loops through redirects ends at the tenth hop, which is
// reported with the kit's redirect rule: its own code (HTTP_Found), not
// retryable, a hint naming --base-url and doctor first. 0.24.0 reported
// NETWORK (exit 8, retryable). doctor reports it as redirected.
func TestReadRedirectLoopIsReportedNotRetried(t *testing.T) {
	var sent atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	c := New(Config{Flavor: FlavorDataCenter, BaseURL: srv.URL, Transport: transport.New(transport.Options{MaxRetries: 2})})
	_, err := c.Ping(context.Background())
	ce := cerrors.AsCLIError(err)
	if ce == nil || ce.Code != "HTTP_Found" || ce.HTTPStatus != http.StatusFound || ce.Retryable ||
		!strings.Contains(ce.Hint, "check --base-url first") ||
		len(ce.NextSteps) == 0 || ce.NextSteps[0] != "confluence-cli doctor" {
		t.Fatalf("got %+v; want HTTP_Found 302, not retryable", ce)
	}
	if got := sent.Load(); got != 10 {
		t.Fatalf("requests = %d, want 10 (no retry)", got)
	}
	if got := doctor.Status(err); got != doctor.StatusRedirected {
		t.Fatalf("doctor.Status = %q, want redirected", got)
	}
	if got := doctor.Detail(err); !strings.Contains(got, "Confluence returned HTTP 302. The server redirected the request to ") {
		t.Fatalf("doctor.Detail = %q", got)
	}
}

// An attachment download that gets a 3xx back (a 304, or the tenth hop of a
// loop) is an error, not an empty file. 0.24.0 took any status below 400 as
// the attachment.
func TestAttachmentDownloadRedirectIsAnError(t *testing.T) {
	for _, status := range []int{http.StatusNotModified, http.StatusFound} {
		c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if status == http.StatusFound {
				http.Redirect(w, r, r.URL.Path, status)
				return
			}
			w.WriteHeader(status)
		}))
		var out bytes.Buffer
		_, err := c.DownloadAttachment(context.Background(), Attachment{DownloadURL: srv.URL + "/download/attachments/1/a.txt"}, &out)
		ce := cerrors.AsCLIError(err)
		if ce == nil || ce.HTTPStatus != status || ce.Retryable || out.Len() != 0 {
			t.Fatalf("%d: got %+v with %d bytes written; want the %d as an error", status, ce, out.Len(), status)
		}
	}
}

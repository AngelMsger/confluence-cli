package transport

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"
)

// doWithRetry runs req, retrying transient failures for idempotent methods.
// Non-idempotent requests (e.g. POST) are never retried, so a write is sent at
// most once. A transient HTTP status that exhausts retries is returned to the
// caller as a normal response for status classification.
func (c *Client) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	idempotent := req.Method == http.MethodGet || req.Method == http.MethodHead

	for attempt := 0; ; attempt++ {
		resp, err := c.doer.Do(req)
		if err == nil && !isTransientStatus(resp.StatusCode) {
			return resp, nil
		}
		if !idempotent || attempt >= c.maxRetries {
			return resp, err
		}

		delay := c.backoff(attempt, resp)
		if resp != nil {
			drainAndClose(resp.Body)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// backoff computes the wait before the next attempt. A Retry-After header (on
// 429/503) takes precedence over linear backoff.
func (c *Client) backoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil && secs >= 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return time.Duration(attempt+1) * c.baseDelay
}

func isTransientStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

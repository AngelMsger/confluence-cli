package transport

import (
	"context"
	"net/http"
	"time"

	"github.com/angelmsger/confluence-cli/pkg/constants"
)

// Options configures a Client.
type Options struct {
	// Doer executes requests. Defaults to an *http.Client with Timeout.
	Doer Doer
	// Timeout is the per-request timeout when Doer is nil.
	Timeout time.Duration
	// MaxRetries is the number of additional attempts after the first failure.
	MaxRetries int
	// RetryBaseDelay is the base backoff delay; the Nth retry waits N*base.
	RetryBaseDelay time.Duration
	// Decorators are applied to every request in order.
	Decorators []Decorator
}

// Client is a retrying HTTP client. It is flavor-agnostic: callers build fully
// formed *http.Request values and Client only adds decorators and retries.
type Client struct {
	doer       Doer
	maxRetries int
	baseDelay  time.Duration
	decorators []Decorator
}

// New builds a Client from Options, filling defaults.
func New(opt Options) *Client {
	c := &Client{
		doer:       opt.Doer,
		maxRetries: opt.MaxRetries,
		baseDelay:  opt.RetryBaseDelay,
		decorators: opt.Decorators,
	}
	if c.doer == nil {
		timeout := opt.Timeout
		if timeout == 0 {
			timeout = constants.DefaultTimeout
		}
		c.doer = &http.Client{Timeout: timeout}
	}
	if c.baseDelay == 0 {
		c.baseDelay = 500 * time.Millisecond
	}
	return c
}

// Do sends req, applying decorators and retrying transient failures. The
// returned response (if any) has a readable, non-nil Body that the caller must
// close. The context bounds the whole retry sequence.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	for _, d := range c.decorators {
		d(req)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", constants.UserAgent())
	}
	return c.doWithRetry(ctx, req)
}

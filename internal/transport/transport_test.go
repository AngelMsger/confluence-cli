package transport

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeDoer returns scripted responses/errors on successive calls.
type fakeDoer struct {
	statuses []int // one entry per expected call; 0 means return netErr
	netErr   error
	calls    int
	lastReq  *http.Request
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.lastReq = req
	idx := f.calls
	f.calls++
	if idx >= len(f.statuses) {
		idx = len(f.statuses) - 1
	}
	st := f.statuses[idx]
	if st == 0 {
		return nil, f.netErr
	}
	return &http.Response{
		StatusCode: st,
		Body:       io.NopCloser(strings.NewReader("body")),
		Header:     http.Header{},
	}, nil
}

func newReq(t *testing.T, method string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, "http://example.test/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestDoSuccessNoRetry(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{statuses: []int{200}}
	c := New(Options{Doer: doer, MaxRetries: 3, RetryBaseDelay: time.Millisecond})
	resp, err := c.Do(context.Background(), newReq(t, http.MethodGet))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if doer.calls != 1 {
		t.Errorf("calls = %d, want 1", doer.calls)
	}
}

func TestDoRetriesOn503ThenSucceeds(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{statuses: []int{503, 503, 200}}
	c := New(Options{Doer: doer, MaxRetries: 3, RetryBaseDelay: time.Millisecond})
	resp, err := c.Do(context.Background(), newReq(t, http.MethodGet))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if doer.calls != 3 {
		t.Errorf("calls = %d, want 3", doer.calls)
	}
}

func TestDoRetriesExhaustedReturnsLastResponse(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{statuses: []int{429, 429, 429, 429}}
	c := New(Options{Doer: doer, MaxRetries: 2, RetryBaseDelay: time.Millisecond})
	resp, err := c.Do(context.Background(), newReq(t, http.MethodGet))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}
	if doer.calls != 3 { // 1 initial + 2 retries
		t.Errorf("calls = %d, want 3", doer.calls)
	}
}

func TestDoDoesNotRetryPost(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{statuses: []int{503, 200}}
	c := New(Options{Doer: doer, MaxRetries: 3, RetryBaseDelay: time.Millisecond})
	resp, err := c.Do(context.Background(), newReq(t, http.MethodPost))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if doer.calls != 1 {
		t.Errorf("POST must not be retried, calls = %d, want 1", doer.calls)
	}
}

func TestDoAppliesDecoratorsAndUserAgent(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{statuses: []int{200}}
	c := New(Options{
		Doer:       doer,
		Decorators: []Decorator{func(r *http.Request) { r.Header.Set("X-Test", "yes") }},
	})
	resp, err := c.Do(context.Background(), newReq(t, http.MethodGet))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if doer.lastReq.Header.Get("X-Test") != "yes" {
		t.Error("decorator header not applied")
	}
	if doer.lastReq.Header.Get("User-Agent") == "" {
		t.Error("default User-Agent not set")
	}
}

func TestDoContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{statuses: []int{503}}
	c := New(Options{Doer: doer, MaxRetries: 5, RetryBaseDelay: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Do(ctx, newReq(t, http.MethodGet)); err == nil {
		t.Error("expected context cancellation error")
	}
}

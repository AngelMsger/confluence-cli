// Package transport provides a flavor-agnostic HTTP layer: a thin client that
// applies request decorators (auth, user-agent) and retries transient failures.
//
// This package backs the confluence-cli command layer and is also importable as a library;
// see the repository README. The CLI relies on its retry, timeout and decorator
// behavior; change it additively and keep the defaults stable rather than tuning
// them for a single local call site.
package transport

import "net/http"

// Doer executes an HTTP request. *http.Client satisfies it; tests inject fakes.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Decorator mutates an outgoing request before it is sent (e.g. adds headers).
type Decorator func(*http.Request)

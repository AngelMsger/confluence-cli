// Package transport provides a flavor-agnostic HTTP layer: a thin client that
// applies request decorators (auth, user-agent) and retries transient failures.
package transport

import "net/http"

// Doer executes an HTTP request. *http.Client satisfies it; tests inject fakes.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Decorator mutates an outgoing request before it is sent (e.g. adds headers).
type Decorator func(*http.Request)

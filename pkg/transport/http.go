// Package transport provides a flavor-agnostic HTTP layer: a thin client that
// applies request decorators (auth, user-agent) and retries transient failures.
// The client lives in github.com/angelmsger/agent-cli-kit/transport, shared by
// every CLI in the family; this package re-exports it with confluence-cli's
// User-Agent as the default.
//
// This package backs the confluence-cli command layer and is also importable as a library;
// see the repository README. The CLI relies on its retry, timeout and decorator
// behavior; change it additively and keep the defaults stable rather than tuning
// them for a single local call site.
package transport

import (
	kit "github.com/angelmsger/agent-cli-kit/transport"
	"github.com/angelmsger/confluence-cli/pkg/constants"
)

type (
	// Doer executes an HTTP request. *http.Client satisfies it; tests inject fakes.
	Doer = kit.Doer
	// Decorator mutates an outgoing request before it is sent (e.g. adds headers).
	Decorator = kit.Decorator
	// Client is a retrying HTTP client.
	Client = kit.Client
	// Options configures a Client.
	Options = kit.Options
)

// New returns a Client. Requests that carry no User-Agent get confluence-cli's.
func New(opt Options) *Client {
	if opt.UserAgent == "" {
		opt.UserAgent = constants.UserAgent()
	}
	return kit.New(opt)
}

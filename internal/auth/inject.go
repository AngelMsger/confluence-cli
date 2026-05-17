package auth

import (
	"encoding/base64"
	"net/http"

	"github.com/angelmsger/confluence-cli/internal/transport"
)

// Header returns the Authorization header value for the credential.
func (c Credential) Header() string {
	switch c.Scheme {
	case SchemePAT:
		return "Bearer " + c.Secret
	case SchemeBasic:
		raw := c.Username + ":" + c.Secret
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
	default:
		return ""
	}
}

// Decorator returns a transport.Decorator that authenticates every request.
func (c Credential) Decorator() transport.Decorator {
	header := c.Header()
	return func(req *http.Request) {
		if header != "" {
			req.Header.Set("Authorization", header)
		}
	}
}

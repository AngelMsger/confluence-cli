// Package auth models Confluence credentials, resolves them from configuration
// or secure storage, and applies them to outgoing HTTP requests.
package auth

import (
	"net/url"
	"strings"

	cerrors "github.com/angelmsger/confluence-cli/pkg/errors"
)

// Scheme identifies an authentication scheme.
const (
	SchemePAT   = "pat"   // Bearer Personal Access Token (Data Center 7.9+)
	SchemeBasic = "basic" // HTTP Basic (DC: user+password; Cloud: email+API token)
)

// Credential is a fully resolved credential ready to authenticate requests.
type Credential struct {
	Scheme   string
	Username string // basic only
	Secret   string // PAT token, or password / API token
}

// Validate reports whether the credential is internally consistent.
func (c Credential) Validate() error {
	switch c.Scheme {
	case SchemePAT:
		if c.Secret == "" {
			return cerrors.New(cerrors.CategoryConfig, "AUTH_NO_TOKEN",
				"no Personal Access Token configured")
		}
	case SchemeBasic:
		if c.Username == "" || c.Secret == "" {
			return cerrors.New(cerrors.CategoryConfig, "AUTH_NO_BASIC",
				"basic auth requires both a username and a password/token")
		}
	default:
		return cerrors.Newf(cerrors.CategoryConfig, "AUTH_BAD_SCHEME",
			"unknown auth scheme %q", c.Scheme)
	}
	return nil
}

// Redacted returns a copy safe for logging: the secret is masked.
func (c Credential) Redacted() Credential {
	c.Secret = maskSecret(c.Secret)
	return c
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(s)-4) + s[len(s)-4:]
}

// AccountKey derives the keychain account identifier for a base URL and scheme.
// It is stable across runs so credentials can be located later.
func AccountKey(baseURL, scheme string) string {
	host := baseURL
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		host = u.Host
	}
	return host + ":" + scheme
}

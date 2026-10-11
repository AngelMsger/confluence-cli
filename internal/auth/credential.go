// Package auth declares confluence-cli's auth schemes and resolves credentials
// from configuration or secure storage. The credential model, header encoding
// and recovery errors come from agent-cli-kit/auth; this package registers
// Confluence's schemes and wording with it.
package auth

import (
	kit "github.com/angelmsger/agent-cli-kit/auth"
	"github.com/angelmsger/confluence-cli/pkg/constants"
	_ "github.com/angelmsger/confluence-cli/pkg/errors" // installs Confluence's error guidance
)

// Scheme identifies an authentication scheme.
const (
	SchemePAT   = "pat"   // Bearer Personal Access Token (Data Center 7.9+)
	SchemeBasic = "basic" // HTTP Basic (DC: user+password; Cloud: email+API token)
)

func init() {
	kit.Register(&kit.Product{
		Schemes: []kit.Scheme{
			{Name: SchemePAT, Kind: kit.Bearer,
				Missing: "no Personal Access Token configured"},
			{Name: SchemeBasic, Kind: kit.Basic,
				Missing: "basic auth requires both a username and a password/token"},
		},
		DefaultScheme:    SchemePAT,
		Name:             "Confluence",
		Doctor:           constants.AppName + " doctor",
		MissingStep:      "Only if the host retry also reports missing credentials, run `" + constants.AppName + " config init` in the user's terminal or set CONFLUENCE_* environment variables.",
		InaccessibleStep: "Do not run `config init` unless the same check also fails in the host environment.",
	})
}

// Credential is a fully resolved credential ready to authenticate requests.
// Its Validate, Header, Decorator and Redacted methods follow the schemes
// registered above.
type Credential = kit.Credential

// AccountKey derives the keychain account identifier for a base URL and scheme.
func AccountKey(baseURL, scheme string) string { return kit.AccountKey(baseURL, scheme) }

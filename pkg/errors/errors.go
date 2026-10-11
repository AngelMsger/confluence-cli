// Package errors defines the structured error model used across the CLI.
//
// Every user-facing failure is a *CLIError carrying a Category, a stable Code,
// a human message, an agent-facing hint and concrete next steps. The model
// itself lives in github.com/angelmsger/agent-cli-kit/errors, shared by every
// CLI in the family; this package re-exports it and installs confluence-cli's
// recovery guidance (see hints.go), so any error built here or through the
// kit carries confluence-cli's next steps.
//
// This package is shared by the confluence-cli command layer and is also importable as a
// library; see the repository README. The Category set, the stable Codes, and
// their deterministic mapping to exit codes are an agent-facing contract relied
// on by the CLI and its companion Skill. Add new codes rather than renaming or
// repurposing existing ones, and keep the category -> exit-code mapping stable.
package errors

import kit "github.com/angelmsger/agent-cli-kit/errors"

func init() { kit.SetGuidance(defaultGuidance) }

type (
	// Category classifies a failure. It drives both the exit code and the hints.
	Category = kit.Category
	// CLIError is the single error type surfaced to the user.
	CLIError = kit.CLIError
	// Recovery tells an agent what kind of retry can fix a failure.
	Recovery = kit.Recovery
	// Payload is the JSON envelope written to stderr on failure.
	Payload = kit.Payload
	// PayloadBody is the body of Payload.
	PayloadBody = kit.PayloadBody
)

// Categories.
const (
	CategoryUsage      = kit.CategoryUsage
	CategoryConfig     = kit.CategoryConfig
	CategoryAuth       = kit.CategoryAuth
	CategoryPermission = kit.CategoryPermission
	CategoryNotFound   = kit.CategoryNotFound
	CategoryConflict   = kit.CategoryConflict
	CategoryRateLimit  = kit.CategoryRateLimit
	CategoryNetwork    = kit.CategoryNetwork
	CategoryServer     = kit.CategoryServer
	CategoryParse      = kit.CategoryParse
	CategoryInternal   = kit.CategoryInternal
)

// Exit codes, one per category.
const (
	ExitSuccess    = kit.ExitSuccess
	ExitInternal   = kit.ExitInternal
	ExitUsage      = kit.ExitUsage
	ExitConfig     = kit.ExitConfig
	ExitAuth       = kit.ExitAuth
	ExitPermission = kit.ExitPermission
	ExitNotFound   = kit.ExitNotFound
	ExitRateLimit  = kit.ExitRateLimit
	ExitNetwork    = kit.ExitNetwork
	ExitServer     = kit.ExitServer
	ExitParse      = kit.ExitParse
	ExitConflict   = kit.ExitConflict
)

// Constructors and helpers; see agent-cli-kit/errors for their documentation.
var (
	New            = kit.New
	Newf           = kit.Newf
	Wrap           = kit.Wrap
	AsCLIError     = kit.AsCLIError
	ExitCode       = kit.ExitCode
	FromHTTPStatus = kit.FromHTTPStatus
)

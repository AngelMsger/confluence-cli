// Package errors defines the structured error model used across the CLI.
//
// Every user-facing failure is a *CLIError carrying a Category, a stable Code,
// a human message, an agent-facing hint and concrete next steps. The category
// maps deterministically to a process exit code (see codes.go) and to recovery
// guidance (see hints.go).
package errors

import (
	stderrors "errors"
	"fmt"
)

// Category classifies a failure. It drives both the exit code and the hints.
type Category string

const (
	CategoryUsage      Category = "usage"
	CategoryConfig     Category = "config"
	CategoryAuth       Category = "auth"
	CategoryPermission Category = "permission"
	CategoryNotFound   Category = "not_found"
	CategoryConflict   Category = "conflict"
	CategoryRateLimit  Category = "rate_limit"
	CategoryNetwork    Category = "network"
	CategoryServer     Category = "server"
	CategoryParse      Category = "parse"
	CategoryInternal   Category = "internal"
)

// CLIError is the single error type surfaced to the user. It is JSON-encodable
// (see Payload) and unwraps to any wrapped cause.
type CLIError struct {
	Category   Category
	Code       string
	Message    string
	Hint       string
	NextSteps  []string
	Retryable  bool
	HTTPStatus int
	cause      error
}

// Error implements the error interface.
func (e *CLIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Message
}

// Unwrap exposes the wrapped cause for errors.Is / errors.As.
func (e *CLIError) Unwrap() error { return e.cause }

// New builds a CLIError for the given category. The hint and next steps are
// filled from the category defaults unless overridden later via the With* helpers.
func New(cat Category, code, message string) *CLIError {
	e := &CLIError{Category: cat, Code: code, Message: message}
	e.Hint, e.NextSteps = defaultGuidance(cat)
	e.Retryable = isRetryable(cat)
	return e
}

// Newf is New with a printf-formatted message.
func Newf(cat Category, code, format string, args ...any) *CLIError {
	return New(cat, code, fmt.Sprintf(format, args...))
}

// Wrap attaches a cause to a freshly built CLIError.
func Wrap(cause error, cat Category, code, message string) *CLIError {
	e := New(cat, code, message)
	e.cause = cause
	return e
}

// WithHint overrides the hint text.
func (e *CLIError) WithHint(hint string) *CLIError { e.Hint = hint; return e }

// WithNextSteps overrides the suggested next steps.
func (e *CLIError) WithNextSteps(steps ...string) *CLIError { e.NextSteps = steps; return e }

// WithHTTPStatus records the originating HTTP status code.
func (e *CLIError) WithHTTPStatus(status int) *CLIError { e.HTTPStatus = status; return e }

// WithCause attaches an underlying cause.
func (e *CLIError) WithCause(cause error) *CLIError { e.cause = cause; return e }

// AsCLIError converts any error into a *CLIError, classifying unknown errors
// as internal. A nil error returns nil.
func AsCLIError(err error) *CLIError {
	if err == nil {
		return nil
	}
	var ce *CLIError
	if stderrors.As(err, &ce) {
		return ce
	}
	return Wrap(err, CategoryInternal, "INTERNAL", err.Error())
}

// Payload is the JSON shape written to stderr for a failure.
type Payload struct {
	Error PayloadBody `json:"error"`
}

// PayloadBody is the inner object of Payload.
type PayloadBody struct {
	Category   Category `json:"category"`
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Hint       string   `json:"hint,omitempty"`
	NextSteps  []string `json:"next_steps,omitempty"`
	Retryable  bool     `json:"retryable"`
	HTTPStatus int      `json:"http_status,omitempty"`
}

// Payload renders the error as its JSON-encodable form.
func (e *CLIError) Payload() Payload {
	return Payload{Error: PayloadBody{
		Category:   e.Category,
		Code:       e.Code,
		Message:    e.Message,
		Hint:       e.Hint,
		NextSteps:  e.NextSteps,
		Retryable:  e.Retryable,
		HTTPStatus: e.HTTPStatus,
	}}
}

func isRetryable(cat Category) bool {
	switch cat {
	case CategoryRateLimit, CategoryNetwork, CategoryServer:
		return true
	default:
		return false
	}
}

package errors

// Exit codes. Each category maps to exactly one code so a calling agent can
// branch on the process exit status without parsing stderr.
const (
	ExitSuccess    = 0
	ExitInternal   = 1
	ExitUsage      = 2
	ExitConfig     = 3
	ExitAuth       = 4
	ExitPermission = 5
	ExitNotFound   = 6
	ExitRateLimit  = 7
	ExitNetwork    = 8
	ExitServer     = 9
	ExitParse      = 10
)

// ExitCode returns the process exit code for an error. A nil error yields
// ExitSuccess; an unclassified error yields ExitInternal.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	ce := AsCLIError(err)
	switch ce.Category {
	case CategoryUsage:
		return ExitUsage
	case CategoryConfig:
		return ExitConfig
	case CategoryAuth:
		return ExitAuth
	case CategoryPermission:
		return ExitPermission
	case CategoryNotFound:
		return ExitNotFound
	case CategoryRateLimit:
		return ExitRateLimit
	case CategoryNetwork:
		return ExitNetwork
	case CategoryServer:
		return ExitServer
	case CategoryParse:
		return ExitParse
	default:
		return ExitInternal
	}
}

// FromHTTPStatus classifies an HTTP status code into a Category. Used by the
// API client to turn non-2xx responses into structured errors.
func FromHTTPStatus(status int) Category {
	switch {
	case status == 401:
		return CategoryAuth
	case status == 403:
		return CategoryPermission
	case status == 404:
		return CategoryNotFound
	case status == 429:
		return CategoryRateLimit
	case status >= 500:
		return CategoryServer
	case status >= 400:
		return CategoryUsage
	default:
		return CategoryServer
	}
}

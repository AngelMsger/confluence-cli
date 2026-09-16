package errors

// defaultGuidance returns the default hint and next-step commands for a
// category. Callers may override these via WithHint / WithNextSteps when more
// specific guidance is available.
func defaultGuidance(cat Category) (hint string, steps []string) {
	switch cat {
	case CategoryUsage:
		return "The command was invoked incorrectly. Check flags and arguments.",
			[]string{"confluence-cli <command> --help"}
	case CategoryConfig:
		return "No usable configuration was found or it is invalid.",
			[]string{"confluence-cli config init", "confluence-cli config show --explain"}
	case CategoryAuth:
		return "The server rejected the credentials. The token may be expired or wrong.",
			[]string{"confluence-cli auth status", "confluence-cli config init"}
	case CategoryPermission:
		return "The credentials are valid but lack permission for this resource.",
			[]string{"Verify the account can access the page/space in a browser."}
	case CategoryNotFound:
		return "The requested page, space or attachment does not exist.",
			[]string{"confluence-cli search --text \"<keywords>\"", "Double-check the ID or URL."}
	case CategoryConflict:
		return "The resource changed since it was last read (version conflict).",
			[]string{"Read the current content and version, merge the intended changes, then retry with that version."}
	case CategoryRateLimit:
		return "The server is rate limiting requests.",
			[]string{"Wait before retrying reads; narrow large queries.", "Before retrying a write, verify that the previous attempt did not apply."}
	case CategoryNetwork:
		return "The request failed because of a network error (DNS, TLS or timeout).",
			[]string{"confluence-cli doctor", "Check --base-url and network connectivity.", "After a write timeout, verify the remote result before retrying; the change may already have applied."}
	case CategoryServer:
		return "The Confluence server returned an internal error.",
			[]string{"Retry reads later; verify the remote result before retrying writes.", "confluence-cli doctor"}
	case CategoryParse:
		return "A response could not be parsed or rendered.",
			[]string{"Inspect the error code; for page rendering failures, use page get <id> --as raw.", "After a write, verify the remote result before retrying; the change may already have applied."}
	default:
		return "An unexpected internal error occurred.",
			[]string{"Retry with --verbose for details."}
	}
}

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
			[]string{"Re-fetch the resource to get its current version, then retry."}
	case CategoryRateLimit:
		return "The server is rate limiting requests. Retry after a short wait.",
			[]string{"Wait and retry; reduce --limit or avoid --all for large queries."}
	case CategoryNetwork:
		return "The server could not be reached (DNS, TLS or timeout).",
			[]string{"confluence-cli doctor", "Check --base-url and network connectivity."}
	case CategoryServer:
		return "The Confluence server returned an internal error.",
			[]string{"Retry later.", "confluence-cli doctor"}
	case CategoryParse:
		return "A response could not be parsed or rendered.",
			[]string{"Retry with --format json and --scope full to inspect raw content."}
	default:
		return "An unexpected internal error occurred.",
			[]string{"Retry with --verbose for details."}
	}
}

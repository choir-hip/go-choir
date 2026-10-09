package searchplane

import "regexp"

var (
	// A URL's query string can carry the provider key (SerpAPI's api_key)
	// and the caller's query; neither belongs in a stored or returned summary.
	urlQueryPattern = regexp.MustCompile(`(https?://[^\s"'?#]+)\?[^\s"']*`)
	// key=value / key: value pairs outside a URL.
	secretPairPattern = regexp.MustCompile(`(?i)\b(api[_-]?key|apikey|key|token|access[_-]?token|secret|password)(\s*[=:]\s*)[^\s"'&,;]+`)
	// "API key sk-..." echoed back by a provider.
	secretEchoPattern = regexp.MustCompile(`(?i)\b(api[ _-]?key|token)(\s+)[A-Za-z0-9_\-\.]{12,}`)
)

// RedactSummary strips URL query strings and credential-shaped values from a
// provider error summary (docs/problems/search-plane-cooldown-on-caller-cancel-2026-10-09.md).
// Summaries are stored in the health database and returned to computers.
func RedactSummary(msg string) string {
	msg = urlQueryPattern.ReplaceAllString(msg, "$1?[redacted]")
	msg = secretPairPattern.ReplaceAllString(msg, "$1$2[redacted]")
	return secretEchoPattern.ReplaceAllString(msg, "$1$2[redacted]")
}

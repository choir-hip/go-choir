// Package publicsite is the one place the paper's public base URL is
// configured: the host a published piece's read_url names
// (CORPUSD_PUBLIC_BASE_URL, default https://choir.news). corpusd mints
// read_url and recognises a captured own-story URL from it, and the gateway's
// host fetch serves the paper's own public pages for the same host, so they
// must read one value.
package publicsite

import "strings"

const (
	// EnvBaseURL is the setting corpusd reads.
	EnvBaseURL = "CORPUSD_PUBLIC_BASE_URL"
	// DefaultBaseURL is the host a read_url names when the setting is unset.
	DefaultBaseURL = "https://choir.news"
)

// BaseURL is the configured public base URL without a trailing slash, or the
// default.
func BaseURL(getenv func(string) string) string {
	if v := strings.TrimRight(strings.TrimSpace(getenv(EnvBaseURL)), "/"); v != "" {
		return v
	}
	return DefaultBaseURL
}

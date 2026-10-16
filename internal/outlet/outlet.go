// Package outlet names the publisher of a captured URL: the media outlet,
// as distinct from the sources making claims inside it (owner §24,
// docs/reports/owner-notes-2026-10-14.md). The publish gate counts distinct
// outlets (docs/problems/capture-and-production-are-coupled-2026-10-16.md).
//
// An outlet is the registrable host of the URL, except on platforms where
// one host carries many publishers (a Telegram channel, an X account), where
// it is the platform plus the account. Distinct outlets are a floor, not
// proof of independence: wire copy syndicated across outlets reads as
// several.
package outlet

import (
	"net/url"
	"strings"
)

// secondLevel are labels under a two-letter country code that a registrar
// sells names beneath (bbc.co.uk, abc.net.au), so the registrable host keeps
// three labels. It is a short approximation of the public suffix list; a
// miss over-merges outlets, which holds a story rather than publishing it.
var secondLevel = map[string]bool{
	"co": true, "com": true, "org": true, "net": true, "gov": true, "ac": true,
	"edu": true, "or": true, "ne": true, "go": true, "gob": true, "nic": true,
}

// platformAliases names one platform's hosts by one name.
var platformAliases = map[string]string{
	"t.me": "t.me", "telegram.me": "t.me",
	"x.com": "x.com", "twitter.com": "x.com", "mobile.twitter.com": "x.com",
}

// notOutlets are registrable hosts that publish no reporting of their own:
// aggregators and search pages link to it, and syndicators republish other
// outlets' copy. A capture of one corroborates nothing, so it names no
// outlet (offline event study, 2026-10-16: an article and its Hacker News
// thread read as two outlets).
var notOutlets = map[string]bool{
	"ycombinator.com": true, "google.com": true, "reddit.com": true, "lobste.rs": true,
	"yahoo.com": true, "aol.com": true, "aol.co.uk": true, "msn.com": true,
	"bing.com": true, "duckduckgo.com": true, "flipboard.com": true, "newsbreak.com": true,
}

// Of returns the outlet key of a URL, or "" when it names no outlet.
func Of(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || !strings.Contains(host, ".") {
		return ""
	}
	if wrapped, ok := unwrapProxy(host, raw); ok {
		return Of(wrapped)
	}
	if platform, ok := platformAliases[strings.TrimPrefix(host, "www.")]; ok {
		account := firstPathSegment(u.Path, platform)
		if account == "" {
			return ""
		}
		return platform + "/" + account
	}
	labels := strings.Split(host, ".")
	keep := 2
	if n := len(labels); n >= 3 && len(labels[n-1]) == 2 && secondLevel[labels[n-2]] {
		keep = 3
	}
	if len(labels) < keep {
		return ""
	}
	key := strings.Join(labels[len(labels)-keep:], ".")
	if notOutlets[key] {
		return ""
	}
	return key
}

// unwrapProxy returns the URL a reader proxy or archive wraps:
// r.jina.ai/<url> and web.archive.org/web/<timestamp>/<url>.
func unwrapProxy(host, raw string) (string, bool) {
	var rest string
	switch host {
	case "r.jina.ai":
		_, rest, _ = strings.Cut(raw, "r.jina.ai/")
	case "web.archive.org":
		_, after, ok := strings.Cut(raw, "/web/")
		if !ok {
			return "", false
		}
		_, rest, _ = strings.Cut(after, "/")
	default:
		return "", false
	}
	if !strings.HasPrefix(rest, "http://") && !strings.HasPrefix(rest, "https://") {
		return "", false
	}
	return rest, true
}

// firstPathSegment is the account a platform URL names: t.me/<channel>/<id>,
// t.me/s/<channel> (the web preview), x.com/<account>/status/<id>.
func firstPathSegment(path, platform string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if platform == "t.me" && len(parts) > 1 && parts[0] == "s" {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(parts[0])
}

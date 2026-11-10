package hostfetch

// The own-site exception (docs/problems/own-published-pages-unreadable-by-host-fetch-2026-11-10.md).
//
// The address guard refuses any fetch that resolves to one of the host's own
// interface addresses, because the host's public address reaches listeners the
// guest floor drops. The paper's public host is the host, so no computer could
// read a published story of the paper, and no capture of one could exist.
//
// The exception is closed. A GET for the configured public host (the one
// public base URL configuration, not a literal) whose path is exactly the
// plain-text export route, or a /pub/texture/<slug>[/v<n>] page, is served
// from the host's loopback instead of the public address. The request sent is
// rebuilt from validated parts, so nothing the guest sent (cookie,
// authorization, any header) reaches the public surface; redirects are not
// followed (the guest re-sends each hop through this same check); the answer
// is relayed through the same bounded path as any fetch. Everything else on
// that host stays refused by the guard.

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/publicsite"
)

const (
	// EnvOwnSiteURL is the loopback address of the public surface (the
	// proxy). Unset, the exception is off and own-host fetches stay refused.
	EnvOwnSiteURL = "GATEWAY_OWN_SITE_URL"

	ownExportPath = "/api/platform/publications/export"
	ownPagePrefix = "/pub/texture/"
	// ownSlugMax bounds a slug.
	ownSlugMax = 200
)

// OwnSite serves the paper's own public pages from the host's loopback.
type OwnSite struct {
	scheme string
	host   string // as the public base URL writes it (with a port, if any)
	loop   *url.URL
	client *http.Client
}

// asciiLower lowercases A-Z only. Unicode case folding (U+017F folds to s,
// U+212A to k) would match a host that is not the public host.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// Host is the public host the exception is keyed on.
func (o *OwnSite) Host() string { return o.host }

// NewOwnSite builds the exception for publicBase (https://choir.news) served
// from loopbackURL (http://127.0.0.1:8082). A loopback URL that is not a
// loopback address is refused, so the exception can never point off the host.
func NewOwnSite(publicBase, loopbackURL string) (*OwnSite, error) {
	base, err := url.Parse(strings.TrimSpace(publicBase))
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" || base.User != nil ||
		(base.Path != "" && base.Path != "/") || base.RawQuery != "" || base.Fragment != "" {
		return nil, fmt.Errorf("own site: public base URL %q is not a bare http(s) origin", publicBase)
	}
	loop, err := url.Parse(strings.TrimSpace(loopbackURL))
	if err != nil || loop.Scheme != "http" || loop.User != nil || loop.Host == "" || (loop.Path != "" && loop.Path != "/") {
		return nil, fmt.Errorf("own site: loopback URL %q is not a bare http origin", loopbackURL)
	}
	if h := loop.Hostname(); h != "localhost" {
		if ip := net.ParseIP(h); ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("own site: loopback URL %q is not a loopback address", loopbackURL)
		}
	}
	return &OwnSite{
		scheme: base.Scheme,
		host:   asciiLower(base.Host),
		loop:   loop,
		client: &http.Client{
			Timeout:       MaxTimeout,
			Transport:     &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

// OwnSiteFromEnv reads the exception from the environment: nil (off) when
// GATEWAY_OWN_SITE_URL is unset, else the public base URL from the one public
// site configuration.
func OwnSiteFromEnv(getenv func(string) string) (*OwnSite, error) {
	loop := strings.TrimSpace(getenv(EnvOwnSiteURL))
	if loop == "" {
		return nil, nil
	}
	return NewOwnSite(publicsite.BaseURL(getenv), loop)
}

// validOwnRoute reports whether route is /pub/texture/<slug> or
// /pub/texture/<slug>/v<n>, n a positive decimal without a leading zero.
func validOwnRoute(route string) bool {
	rest, ok := strings.CutPrefix(route, ownPagePrefix)
	if !ok || rest == "" {
		return false
	}
	slug, version, hasVersion := strings.Cut(rest, "/")
	if hasVersion {
		n, ok := strings.CutPrefix(version, "v")
		if !ok || n == "" || n[0] == '0' || strings.Trim(n, "0123456789") != "" || len(n) > 9 {
			return false
		}
	}
	if slug == "" || len(slug) > ownSlugMax {
		return false
	}
	for _, r := range slug {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// build returns the loopback request for target when it is exactly the
// public host's export route or a published page, else ok false (the guard
// decides). The request is built from validated parts only.
func (o *OwnSite) build(ctx context.Context, method string, target *url.URL) (*http.Request, bool) {
	if o == nil || method != http.MethodGet {
		return nil, false
	}
	if target.Scheme != o.scheme || target.User != nil || asciiLower(target.Host) != o.host ||
		target.RawPath != "" || target.Fragment != "" || target.Opaque != "" {
		return nil, false
	}
	var path, rawQuery string
	switch {
	case target.Path == ownExportPath:
		q, err := url.ParseQuery(target.RawQuery)
		if err != nil || len(q) != 3 {
			return nil, false
		}
		route, format, raw := q["route"], q["format"], q["raw"]
		if len(route) != 1 || len(format) != 1 || len(raw) != 1 || format[0] != "txt" || raw[0] != "1" || !validOwnRoute(route[0]) {
			return nil, false
		}
		path = ownExportPath
		rawQuery = url.Values{"route": {route[0]}, "format": {"txt"}, "raw": {"1"}}.Encode()
	case strings.HasPrefix(target.Path, ownPagePrefix) && target.RawQuery == "" && validOwnRoute(target.Path):
		path = target.Path
	default:
		return nil, false
	}
	u := *o.loop
	u.Path, u.RawQuery = path, rawQuery
	up, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, false
	}
	up.Host = o.host
	up.Header = http.Header{
		"User-Agent": {"choir-host-fetch/1 (own-site)"},
		"Accept":     {"text/plain, text/html;q=0.9, */*;q=0.1"},
	}
	return up, true
}

// Package hostfetch is the host fetch verb (SDC NET-01): a guest's public
// web fetches (research Read, content import, podcast and YouTube fetches,
// source polls) make their network hop on the host, behind the gateway's
// POST /provider/v1/fetch, where the address guard, budgets and a log line
// per fetch run out of the guest's reach
// (docs/problems/guest-fetches-public-internet-itself-2026-10-21.md).
//
// The verb is one HTTP hop. The host never follows a redirect: it returns
// the 3xx to the guest, whose sourcefetch client re-checks the target and
// sends the next hop through the host again. There is no allowlist: any
// public address is reachable, exactly as from the guest today. Moving the
// hop moves where the check runs, not what is reachable.
//
// A computer uses the verb only when vmctl wrote a grant on its credential
// disk (CHOIR_HOST_FETCH_COMPUTERS; off while empty). A granted computer
// never falls back to dialing itself.
package hostfetch

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Path is the gateway route.
	Path = "/provider/v1/fetch"
	// StatusHeader carries the upstream status on a relayed answer. Its
	// absence means the gateway answered itself (a refusal or an error).
	StatusHeader = "X-Choir-Fetch-Status"
	// HeadersHeader carries the upstream headers, base64url of JSON.
	HeadersHeader = "X-Choir-Fetch-Headers"
	// MaxRequestBody bounds the body a guest sends upstream (InnerTube
	// posts are a few KiB).
	MaxRequestBody = 1 << 20
	// MaxResponseBody bounds a relayed body: above the largest caller cap
	// (25 MiB documents, internal/content) with room for GDELT zips.
	MaxResponseBody = 32 << 20
	// MaxTimeout bounds one upstream hop; callers' own timeouts are shorter
	// and end the hop sooner through the request context.
	MaxTimeout = 90 * time.Second
)

// Request is the guest's description of one hop.
type Request struct {
	Method string      `json:"method"`
	URL    string      `json:"url"`
	Header http.Header `json:"header,omitempty"`
	Body   []byte      `json:"body,omitempty"`
}

// hopHeaders are never relayed in either direction (RFC 9110 §7.6.1), nor
// proxy credentials.
var hopHeaders = []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Proxy-Connection", "Te", "Trailer", "Transfer-Encoding", "Upgrade", "Host"}

func stripHop(h http.Header) http.Header {
	out := h.Clone()
	if out == nil {
		return http.Header{}
	}
	for _, v := range out.Values("Connection") {
		for _, name := range strings.Split(v, ",") {
			if name = strings.TrimSpace(name); name != "" {
				out.Del(name)
			}
		}
	}
	for _, name := range hopHeaders {
		out.Del(name)
	}
	return out
}

// EncodeHeaders packs headers into one header-safe value.
func EncodeHeaders(h http.Header) string {
	raw, _ := json.Marshal(h)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeHeaders reverses EncodeHeaders.
func DecodeHeaders(v string) (http.Header, error) {
	h := http.Header{}
	if v == "" {
		return h, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("host fetch headers: %w", err)
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		return nil, fmt.Errorf("host fetch headers: %w", err)
	}
	return h, nil
}

const (
	// GrantFile is the grant's name on the credential disk.
	GrantFile = "host-fetch.json"
	// GrantGranted is the only status that turns the route on.
	GrantGranted = "granted"
	// ClassTestToken in the host list grants every test computer.
	ClassTestToken = "class:test"
	// BasisComputer and BasisClassTest say why a computer was granted.
	BasisComputer  = "computer"
	BasisClassTest = ClassTestToken
)

// Grant is the record vmctl writes on a granted computer's credential disk.
type Grant struct {
	ComputerID string `json:"computer_id"`
	Status     string `json:"status"`
	Basis      string `json:"basis"`
	GrantedAt  string `json:"granted_at"`
}

// ReadGrant reads dir's grant. found is false when there is none.
func ReadGrant(dir string) (g Grant, found bool, err error) {
	raw, err := os.ReadFile(filepath.Join(dir, GrantFile))
	if errors.Is(err, os.ErrNotExist) {
		return Grant{}, false, nil
	}
	if err != nil {
		return Grant{}, true, err
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		return Grant{}, true, err
	}
	return g, true, nil
}

// GuestRoute is the guest daemon's decision at boot: nil (fetch directly,
// as today) unless dir holds a grant for computerID; for a granted computer
// a Transport to the gateway, or, with no gateway, a route that refuses
// every fetch. status is a line for the boot log.
func GuestRoute(dir, computerID, gatewayURL string, token func() string) (http.RoundTripper, string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, "off (no credential directory)"
	}
	g, found, err := ReadGrant(dir)
	switch {
	case !found:
		return nil, "off"
	case err != nil:
		return nil, "off (unreadable grant: " + err.Error() + ")"
	case g.Status != GrantGranted:
		return nil, "off (the host withheld the route)"
	case strings.TrimSpace(computerID) == "" || g.ComputerID != computerID:
		return nil, "off (the grant names another computer)"
	}
	gatewayURL = strings.TrimRight(strings.TrimSpace(gatewayURL), "/")
	if gatewayURL == "" {
		return Refusing("granted, but this computer has no gateway URL"), "on, refusing every fetch (no gateway URL)"
	}
	return &Transport{BaseURL: gatewayURL, Token: token}, "on (" + g.Basis + ")"
}

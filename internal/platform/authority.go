package platform

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// guestOwnershipLookup resolves the live computer bound to a tap-sourced
// request. vmctl is the ownership authority for this join.
type guestOwnershipLookup interface {
	LookupGuestContext(context.Context, string) (*vmctl.GuestOwnership, error)
}

// trustedInternalTransport recognizes host-only transports. X-Internal-Caller
// remains a marker for host services, never an authority signal by itself.
func trustedInternalTransport(r *http.Request) bool {
	if r == nil {
		return false
	}
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if remoteAddr == "" || remoteAddr == "@" || strings.HasPrefix(remoteAddr, "/") {
		return true
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	// The documentation ranges (httptest's RemoteAddr) count only inside a
	// Go test binary.
	return host == "127.0.0.1" || host == "::1" || hostauth.TestNetworkAllowed(host)
}

func tapSourcedTransport(r *http.Request) bool {
	if r == nil || trustedInternalTransport(r) {
		return false
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	return net.ParseIP(host) != nil
}

func bindTapCaller(ctx context.Context, lookup guestOwnershipLookup, r *http.Request) (*vmctl.GuestOwnership, error) {
	if !tapSourcedTransport(r) || lookup == nil {
		return nil, fmt.Errorf("guest caller binding unavailable")
	}
	ownership, err := lookup.LookupGuestContext(ctx, r.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("lookup guest ownership: %w", err)
	}
	if ownership == nil || !ownership.Found || strings.TrimSpace(ownership.ComputerID) == "" {
		return nil, fmt.Errorf("guest caller is not bound to a live computer")
	}
	return ownership, nil
}

func trustedInternalCaller(r *http.Request) bool {
	// Transport, marker, then the host-internal credential
	// (docs/problems/loopback-is-a-credential-2026-10-14.md).
	return trustedInternalTransport(r) && r.Header.Get("X-Internal-Caller") == "true" && hostauth.Authorize(r)
}

// requireInternalOrBoundGuestWrite is the one rule for a write a computer
// sends corpusd (captures, support verdicts): a host-internal caller, or
// the guest vmctl binds to exactly the owner and computer the write names.
// A guest reaches corpusd directly at its platform URL (choir.platform_url),
// not through the proxy, so every guest write route needs this rule, not
// trustedInternalCaller alone (B12).
func requireInternalOrBoundGuestWrite(ctx context.Context, lookup guestOwnershipLookup, r *http.Request, ownerID, computerID string) error {
	if trustedInternalCaller(r) {
		return nil
	}
	ownership, err := bindTapCaller(ctx, lookup, r)
	if err != nil {
		return errInternalCallerRequired
	}
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(computerID) == "" ||
		strings.TrimSpace(ownerID) != strings.TrimSpace(ownership.UserID) ||
		strings.TrimSpace(computerID) != strings.TrimSpace(ownership.ComputerID) {
		return errInternalCallerRequired
	}
	return nil
}

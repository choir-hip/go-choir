package platform

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

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
	return host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "192.0.2.")
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
	return trustedInternalTransport(r) && r.Header.Get("X-Internal-Caller") == "true"
}

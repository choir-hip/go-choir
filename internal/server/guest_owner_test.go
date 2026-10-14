package server

import (
	"net/http/httptest"
	"testing"
)

// Failure modes pinned here
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F2):
// an in-guest process on loopback is believed as the owner; the loopback
// opt-in leaks into the default; an in-process call (choir.ProductAPI) or a
// test-binary TEST-NET-1 request stops being host-side; a stranger's address
// passes.
func TestGuestHostCallerAndAuthenticatedOwner(t *testing.T) {
	t.Setenv(EnvGuestTrustLoopback, "")
	request := func(remote, owner string) (bool, string, error) {
		req := httptest.NewRequest("GET", "/api/runs", nil)
		req.RemoteAddr = remote
		if owner != "" {
			req.Header.Set("X-Authenticated-User", owner)
		}
		user, err := AuthenticatedOwner(req)
		return GuestHostCaller(req), user, err
	}

	for _, remote := range []string{"127.0.0.1:40000", "[::1]:40000", "10.9.9.9:40000", "@", "/run/socket", "not-an-address"} {
		host, user, err := request(remote, "owner-1")
		if host || err == nil || user != "" {
			t.Errorf("remote %q: host=%v user=%q err=%v, want refused", remote, host, user, err)
		}
	}
	for _, remote := range []string{"", "192.0.2.1:1234"} {
		host, user, err := request(remote, "owner-1")
		if !host || err != nil || user != "owner-1" {
			t.Errorf("remote %q: host=%v user=%q err=%v, want the owner", remote, host, user, err)
		}
	}
	if _, _, err := request("", ""); err == nil {
		t.Error("an empty owner header was accepted")
	}

	t.Setenv(EnvGuestTrustLoopback, "1")
	if host, user, err := request("127.0.0.1:40000", "owner-1"); !host || err != nil || user != "owner-1" {
		t.Errorf("loopback with the development opt-in: host=%v user=%q err=%v", host, user, err)
	}
	if host, _, _ := request("10.9.9.9:40000", "owner-1"); host {
		t.Error("the loopback opt-in admitted a non-loopback stranger")
	}
}

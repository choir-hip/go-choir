package autoputer

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// hostSourcedRequest builds a GET request that satisfies server.HostPeerCaller
// via a TEST-NET-1 synthetic host RemoteAddr. Loopback is refused by the
// strict gate: in-guest callers must not drive the diag oracle.
func hostSourcedRequest(rawURL string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, rawURL, nil)
	req.RemoteAddr = "192.0.2.44:40000"
	return req
}

// The S1a refusal matrix uses mode=http to assert forged identity headers from
// inside the guest; the oracle must relay the allowlisted pair and report the
// target's status verbatim.
func TestDiagHTTPProbeRelaysAllowlistedHeadersAndStatus(t *testing.T) {
	var gotInternal, gotUser string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInternal = r.Header.Get("X-Internal-Caller")
		gotUser = r.Header.Get("X-Authenticated-User")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("denied-body"))
	}))
	defer target.Close()
	addr := strings.TrimPrefix(target.URL, "http://")
	q := fmt.Sprintf("addr=%s&mode=http&path=/internal/vmctl/list"+
		"&header=X-Internal-Caller%%3A%%20true&header=X-Authenticated-User%%3A%%20user-A", addr)
	req := hostSourcedRequest("/internal/diag/tcp-dial?" + q)
	rec := httptest.NewRecorder()
	handleDiagTCPDial(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("probe handler status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode probe response: %v", err)
	}
	if out["kind"] != "guest_http_probe" {
		t.Fatalf("kind = %v, want guest_http_probe", out["kind"])
	}
	if out["status"] != float64(http.StatusForbidden) {
		t.Fatalf("status = %v, want 403", out["status"])
	}
	if !strings.Contains(fmt.Sprint(out["body_prefix"]), "denied-body") {
		t.Fatalf("body_prefix = %v, want denied-body", out["body_prefix"])
	}
	if gotInternal != "true" || gotUser != "user-A" {
		t.Fatalf("target saw headers internal=%q user=%q, want forged pair", gotInternal, gotUser)
	}
}

// Arbitrary headers would turn the oracle into a credential relay; only the
// forged-identity pair is permitted.
func TestDiagHTTPProbeRefusesNonAllowlistedHeader(t *testing.T) {
	u := "/internal/diag/tcp-dial?addr=127.0.0.1:8083&mode=http&header=Authorization%3A%20Bearer%20x"
	rec := httptest.NewRecorder()
	handleDiagTCPDial(rec, hostSourcedRequest(u))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for non-allowlisted header", rec.Code)
	}
}

// The POST legs are the ones that prove authority binding: a GET to a
// POST-only route is 405 pre- and post-fix. POST is gated to a path
// allowlist so the oracle cannot mutate arbitrary state.
func TestDiagHTTPProbePostReachesAllowlistedRefusalTarget(t *testing.T) {
	var gotMethod, gotInternal string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotInternal = r.Header.Get("X-Internal-Caller")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer target.Close()
	addr := strings.TrimPrefix(target.URL, "http://")

	q := fmt.Sprintf("addr=%s&mode=http&method=post&path=/internal/computers/platform-updates/offer"+
		"&header=X-Internal-Caller%%3A%%20true", addr)
	req := hostSourcedRequest("/internal/diag/tcp-dial?" + q)
	rec := httptest.NewRecorder()
	handleDiagTCPDial(rec, req)

	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["status"] != float64(http.StatusForbidden) {
		t.Fatalf("status = %v, want 403", out["status"])
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("target saw method %q, want POST", gotMethod)
	}
	if gotInternal != "true" {
		t.Fatalf("target saw X-Internal-Caller %q, want true", gotInternal)
	}
}

// POST to a non-allowlisted path must be refused by the oracle itself.
func TestDiagHTTPProbePostRefusesNonAllowlistedPath(t *testing.T) {
	req := hostSourcedRequest("/internal/diag/tcp-dial?addr=127.0.0.1:8086&mode=http&method=post&path=/internal/vmctl/refresh")
	rec := httptest.NewRecorder()
	handleDiagTCPDial(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for non-allowlisted POST path", rec.Code)
	}
}

// The probe is GET-only by construction: it must never carry a caller's method
// or body to the target.
func TestDiagTCPDialRejectsNonGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/internal/diag/tcp-dial?addr=127.0.0.1:8083&mode=http", nil)
	req.RemoteAddr = "192.0.2.44:40000"
	rec := httptest.NewRecorder()
	handleDiagTCPDial(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// A non-host-sourced caller must be refused before any dial happens: the oracle
// exists for the refusal matrix, not as a guest self-service SSRF surface.
func TestDiagHTTPProbeRefusesGuestCaller(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/internal/diag/tcp-dial?addr=127.0.0.1:8083&mode=http", nil)
	// Non-loopback, non-gateway, non-TEST-NET-1 source => not host-sourced.
	req.RemoteAddr = net.JoinHostPort("203.0.113.9", "40000")
	rec := httptest.NewRecorder()
	handleDiagTCPDial(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// An in-guest caller (loopback) is refused: the diag oracle is a host-peer
// instrument only. Terminal/zot/capsule children must not run host-internal
// probes from inside the guest even though they share the guest's tap address.
func TestDiagHTTPProbeRefusesLoopbackCaller(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/internal/diag/tcp-dial?addr=127.0.0.1:8083&mode=http", nil)
	req.RemoteAddr = "127.0.0.1:40000"
	rec := httptest.NewRecorder()
	handleDiagTCPDial(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for in-guest loopback caller", rec.Code)
	}
}

package agentcore

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// The legs are selected only by an enum on the internal route; anything
// else is refused before any worker starts (docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md, O6, O7).
func TestInternalIsolationSelfTestLegSelector(t *testing.T) {
	for raw, want := range map[string]string{"leg=oom": "oom", "leg=seccomp": "seccomp"} {
		r := httptest.NewRequest(http.MethodPost, internalIsolationSelfTestPath+"?"+raw, nil)
		got, err := internalIsolationSelfTestLeg(r)
		if err != nil || got != want {
			t.Errorf("%q: leg=%q err=%v, want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "leg=OOM", "leg=capsule", "leg=oom&leg=oom", "leg=", "leg=oom&source=for{}", "leg=seccomp&filter=workload"} {
		r := httptest.NewRequest(http.MethodPost, internalIsolationSelfTestPath+"?"+raw, nil)
		if leg, err := internalIsolationSelfTestLeg(r); err == nil {
			t.Errorf("%q: accepted as %q", raw, leg)
		}
	}
}

// Review bunch 22 item 1: the owner-facing /api route runs only the
// confinement legs; a leg there is refused before anything runs.
func TestIsolationSelfTestAPIRouteRefusesTheLegs(t *testing.T) {
	h := &APIHandler{}
	for _, leg := range []string{"oom", "seccomp"} {
		r := httptest.NewRequest(http.MethodPost, "/api/runtime/isolation-selftest?leg="+leg, nil)
		r.Header.Set("X-Authenticated-User", "owner")
		w := httptest.NewRecorder()
		h.HandleIsolationSelfTest(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("leg=%s on /api: status = %d, want 400", leg, w.Code)
		}
	}
}

// Review bunch 22 item 1: the internal route admits only the host peer. This
// test runs hostauth in enforce mode, so it also checks the credential, which
// the deployed guest does not yet require (it runs off mode: the transport
// check alone gates the route; review bunch 27 E1).
func TestInternalIsolationSelfTestRefusesNonHostCallers(t *testing.T) {
	t.Setenv(server.EnvGuestTrustLoopback, "")
	const token = "selftest-gate-test-token-0123456789abcdef0123456"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	h := &APIHandler{}
	gate := hostauth.Middleware(http.HandlerFunc(h.HandleInternalIsolationSelfTest))
	for _, tc := range []struct {
		name, addr, token, query string
		want                     int
	}{
		{"stranger with the token", "203.0.113.9:4000", token, "leg=oom", http.StatusForbidden},
		{"in-guest loopback with the token", "127.0.0.1:4000", token, "leg=oom", http.StatusForbidden},
		{"host peer without the token", "192.0.2.1:4000", "", "leg=seccomp", http.StatusForbidden},
		{"host peer with a wrong token", "192.0.2.1:4000", "wrong-token-0123456789abcdef0123456789abcdef", "leg=oom", http.StatusForbidden},
		// Passes the gate, then the selector refuses before anything runs.
		{"host peer with the token, bad leg", "192.0.2.1:4000", token, "leg=bogus", http.StatusBadRequest},
	} {
		r := httptest.NewRequest(http.MethodPost, internalIsolationSelfTestPath+"?"+tc.query, nil)
		r.RemoteAddr = tc.addr
		if tc.token != "" {
			r.Header.Set(hostauth.Header, tc.token)
		}
		w := httptest.NewRecorder()
		gate.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.name, w.Code, tc.want)
		}
	}
}

func TestIsolationSelfTestRefusesAnUnknownLegBeforeRunning(t *testing.T) {
	h := &APIHandler{}
	r := httptest.NewRequest(http.MethodPost, "/api/runtime/isolation-selftest?leg=anything", nil)
	r.Header.Set("X-Authenticated-User", "owner")
	w := httptest.NewRecorder()
	h.HandleIsolationSelfTest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// Who can call it: only the host peer carrying the owner (the proxy, vmctl).
// An in-guest caller (loopback, any other address) is refused.
func TestIsolationSelfTestRefusesCallersOutsideTheHostPeer(t *testing.T) {
	t.Setenv(server.EnvGuestTrustLoopback, "")
	h := &APIHandler{}
	for _, addr := range []string{"127.0.0.1:4000", "203.0.113.9:4000"} {
		r := httptest.NewRequest(http.MethodPost, "/api/runtime/isolation-selftest?leg=oom", nil)
		r.RemoteAddr = addr
		r.Header.Set("X-Authenticated-User", "owner")
		w := httptest.NewRecorder()
		h.HandleIsolationSelfTest(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", addr, w.Code)
		}
	}
}

// The leg runs the research desk's production worker config: the class that
// reads untrusted input, under its real budget.
func TestIsolationSelfTestOOMLegUsesTheProductionResearchWorker(t *testing.T) {
	cfg := deskOOMSelfTestConfig("computer-selftest")
	if cfg.Role != "research" {
		t.Fatalf("role = %q, want research", cfg.Role)
	}
	if cfg.Base.Bin != deskWorkerBinary() || cfg.Base.WorkerArg != deskWorkerArg || cfg.Base.Session.MemoryLimitBytes != deskWorkerMemoryLimitBytes {
		t.Fatalf("not the production desk worker config: %+v", cfg.Base)
	}
}

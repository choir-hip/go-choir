package agentcore

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// The OOM leg is selected only by an enum; anything else is refused before
// any worker starts (docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md, O6, O7).
func TestIsolationSelfTestLegSelector(t *testing.T) {
	for raw, want := range map[string]string{"": "", "leg=oom": "oom"} {
		r := httptest.NewRequest(http.MethodPost, "/api/runtime/isolation-selftest?"+raw, nil)
		got, err := isolationSelfTestLeg(r)
		if err != nil || got != want {
			t.Errorf("%q: leg=%q err=%v, want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"leg=OOM", "leg=capsule", "leg=oom&leg=oom", "leg=", "source=for{}"} {
		r := httptest.NewRequest(http.MethodPost, "/api/runtime/isolation-selftest?"+raw, nil)
		if leg, err := isolationSelfTestLeg(r); err == nil {
			t.Errorf("%q: accepted as %q", raw, leg)
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

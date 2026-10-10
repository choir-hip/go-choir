package vmctl

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// docs/problems/vmctl-health-fate-shares-a-hung-guest-2026-10-10.md: one
// guest that accepted TCP but never answered made every vmctl health call
// take 8 s, so the deploy gate and choir.news reported vmctl down. Failure
// modes pinned:
//   - health probes a guest (a hung guest delays health again);
//   - a guest never probed reads as idle (health would advertise an idle
//     computer the sweep has not checked);
//   - health ignores the sweep's answer (an idle guest never shows idle).
func TestHealthNeverProbesAGuest(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(time.Minute)
	var probes atomic.Int32
	busy := false
	reg.setGuestBusyProbeForTest(func(*VMOwnership) bool {
		probes.Add(1)
		return busy
	})
	if _, err := reg.ResolveOrAssign("hung-guest-user"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("hung-guest-user", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-2 * time.Hour)
	reg.mu.Unlock()

	health := func() {
		t.Helper()
		rec := httptest.NewRecorder()
		NewHandler(reg).HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("health = %d", rec.Code)
		}
	}
	health()
	if n := probes.Load(); n != 0 {
		t.Fatalf("health probed guests %d times, want 0", n)
	}
	if idle := reg.IdleOwnershipsSeen(); len(idle) != 0 {
		t.Fatalf("an unprobed guest read as idle: %d", len(idle))
	}
	if idle := reg.CheckIdleOwnerships(); len(idle) != 1 {
		t.Fatalf("sweep idle = %d, want 1", len(idle))
	}
	probes.Store(0)
	health()
	if n := probes.Load(); n != 0 {
		t.Fatalf("health probed guests %d times after a sweep, want 0", n)
	}
	if idle := reg.IdleOwnershipsSeen(); len(idle) != 1 {
		t.Fatalf("health idle after the sweep = %d, want 1", len(idle))
	}
}

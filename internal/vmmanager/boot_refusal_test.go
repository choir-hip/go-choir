package vmmanager

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// refusedGuestHealthHandler serves the guest's typed boot refusal: the host
// must parse it without reading guest logs.
func refusedGuestHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":             "refused",
			"kind":               "recovery_tail_excess",
			"reason":             "recovery tail 423720 events exceeds 10000; publish a fresher base (local=0 W=148431 H=572151)",
			"computer_id":        "computer-blocked",
			"local_sequence":     0,
			"watermark_sequence": 148431,
			"target_sequence":    572151,
			"empty_store":        true,
			"chain_exists":       true,
		})
	}
}

func TestProbeGuestHealthParsesTypedBootRefusal(t *testing.T) {
	server := httptest.NewServer(refusedGuestHealthHandler())
	defer server.Close()
	m := &Manager{cfg: ManagerConfig{HealthCheckTimeout: 2 * time.Second}}
	probe := m.probeGuestHealthDetailed(server.URL)
	if probe.BootRefusal == nil {
		t.Fatalf("typed boot refusal not parsed: %+v", probe)
	}
	if probe.BootRefusal.Kind != "recovery_tail_excess" || probe.BootRefusal.WatermarkSequence != 148431 ||
		probe.BootRefusal.TargetSequence != 572151 || !probe.BootRefusal.EmptyStore || !probe.BootRefusal.ChainExists {
		t.Fatalf("refusal = %+v", probe.BootRefusal)
	}
	if probe.Healthy || probe.ReplayInProgress {
		t.Fatalf("refusal must not read as healthy or replaying: %+v", probe)
	}
}

func TestWaitForGuestReadyReturnsTypedRefusalPromptly(t *testing.T) {
	server := httptest.NewServer(refusedGuestHealthHandler())
	defer server.Close()
	m := &Manager{cfg: ManagerConfig{BootReadyTimeout: time.Minute, ReplayStallTimeout: time.Second, HealthCheckTimeout: time.Second}}
	start := time.Now()
	err := m.waitForGuestReady(server.URL, nil, nil)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("typed refusal waited %s, want prompt propagation", elapsed)
	}
	var refused *GuestBootRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want *GuestBootRefusedError", err)
	}
	if refused.ComputerID != "computer-blocked" || refused.WatermarkSequence != 148431 || refused.TargetSequence != 572151 {
		t.Fatalf("refusal = %+v", refused)
	}
	kind, reason, local, watermark, target, empty, chain := refused.GuestBootRefusal()
	if kind != "recovery_tail_excess" || local != 0 || watermark != 148431 || target != 572151 || !empty || !chain {
		t.Fatalf("structural refusal = %q %q %d %d %d %t %t", kind, reason, local, watermark, target, empty, chain)
	}
	if !strings.Contains(reason, "recovery tail 423720 events exceeds 10000") {
		t.Fatalf("reason = %q", reason)
	}

	// The persisted boot receipt carries the same witness structurally.
	timeline := newBootTimeline("vm-refused", "cold")
	recordGuestRefusalOnTimeline(timeline, err)
	if timeline.Refusal == nil {
		t.Fatal("boot receipt must carry the typed refusal witness")
	}
	if timeline.Refusal.Kind != "recovery_tail_excess" || timeline.Refusal.WatermarkSequence != 148431 || timeline.Refusal.TargetSequence != 572151 {
		t.Fatalf("receipt refusal = %+v", timeline.Refusal)
	}
}

func TestWaitForGuestReadyFailsFastWhenGuestProcessExited(t *testing.T) {
	m := &Manager{cfg: ManagerConfig{BootReadyTimeout: time.Minute, HealthCheckTimeout: 200 * time.Millisecond}}
	start := time.Now()
	err := m.waitForGuestReady("http://127.0.0.1:1", nil, func() bool { return false })
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("guest exit detection waited %s, want fail-fast", elapsed)
	}
	var exited *GuestProcessExitedError
	if !errors.As(err, &exited) {
		t.Fatalf("err = %v, want *GuestProcessExitedError", err)
	}
}

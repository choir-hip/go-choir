package vmctl

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A restore is an accepted act that runs in the guest after its request has
// been answered (docs/problems/restore-cannot-finish-inside-its-ten-minute-
// request-2026-11-14.md, failure list 18). Nothing else on the guest looks
// like work to vmctl: no run, no self-development operation, no capsule. A
// computer that is mid-restore must read as busy to the idle sweep, the host
// stop and the automatic restart, and a deploy's refusal must name it, or
// each of them can end the restore half done.

func restoreHealthGuest(t *testing.T, inFlight bool) *VMOwnership {
	t.Helper()
	body := `{"running_runs":0,"desk_pending_mutations":0,"selfdev_active_operations":0,"live_capsules":0}`
	if inFlight {
		body = `{"running_runs":0,"desk_pending_mutations":0,"selfdev_active_operations":0,"live_capsules":0,"restore_in_flight":true}`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &VMOwnership{ComputerURL: server.URL}
}

func TestRestoreInFlightIsBusyForVmctl(t *testing.T) {
	t.Run("the idle sweep", func(t *testing.T) {
		if httpGuestHealth()(restoreHealthGuest(t, false)).Busy {
			t.Fatal("an idle guest reads busy")
		}
		if !httpGuestHealth()(restoreHealthGuest(t, true)).Busy {
			t.Fatal("a guest mid-restore reads idle: the sweep would hibernate it")
		}
	})
	t.Run("the host stop", func(t *testing.T) {
		if !httpHostStopHealth()(restoreHealthGuest(t, false)).busyCaptureAllowed() {
			t.Fatal("an idle guest is not offered the busy capture")
		}
		if httpHostStopHealth()(restoreHealthGuest(t, true)).busyCaptureAllowed() {
			t.Fatal("a guest mid-restore is offered the busy capture")
		}
	})
	t.Run("the automatic restart", func(t *testing.T) {
		zero := 0
		e := newCrashLoopEnv(t)
		e.reg.setHostStopHealthProbeForTest(func(*VMOwnership) hostStopHealth {
			return hostStopHealth{Answered: true, LiveCapsules: &zero, RestoreInFlight: true}
		})
		e.crash(t)
		out := e.wake(t)
		if out.status != http.StatusServiceUnavailable || out.kind != string(RecoveryRefusalGuestWorking) {
			t.Fatalf("wake of a guest mid-restore = %+v, want the guest_working refusal", out)
		}
		if got := e.recovers(); got != 0 {
			t.Fatalf("a computer mid-restore was restarted: recoveries = %d", got)
		}
	})
	t.Run("the deploy refusal names it", func(t *testing.T) {
		code, _ := guestRefusalCode(`{"error":"busy","outcome":"busy","restore_in_flight":true}`)
		if code != "restore_in_flight" {
			t.Fatalf("refusal code = %q, want restore_in_flight", code)
		}
	})
}

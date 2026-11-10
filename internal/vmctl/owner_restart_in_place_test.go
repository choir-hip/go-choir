package vmctl

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The operator's restart in place (docs/problems/operator-cannot-restart-a-current-computer-2026-11-10.md):
// the platform operator's restart on a computer already on its release is a
// planned, marker-backed restart to the SAME version, through the same
// trackedDeployRefresh path an update takes. Only the operator route sends
// restart_in_place; the owner's click is unchanged and still answers current.
//
// Ways this could fail, and the test that covers each:
//   - the marker is written but the restart fails: the marker must be
//     cleared and the answer must be failed, never restarted
//     (TestRestartInPlaceMarkerWrittenButRestartFails);
//   - restart of a computer mid-boot or stopped: nothing is marked or
//     booted; a state other than active is skipped not_active, never woken
//     (TestRestartInPlaceNeverTouchesANonActiveComputer);
//   - a concurrent refresh sweep (or a second operator call) and the restart:
//     one refresh per computer at a time, the loser is deferred
//     refresh_in_flight and never reaches the guest
//     (TestRestartInPlaceMeetsAnInFlightRefresh);
//   - a marker left behind and consumed by a later crash boot, which would
//     wrongly resume: a refused or failed restart leaves no marker (failure
//     clears it; a busy guest writes none), and the guest ignores a marker
//     older than 15 minutes (plannedRestartMaxAge)
//     (TestRestartInPlaceMarkerWrittenButRestartFails,
//     TestRestartInPlaceKeepsTheGuestCorrectnessRefusals);
//   - a busy computer: running runs alone do not defer it (allow_busy is
//     sent), but a self-development apply, a live capsule, a pending
//     decision each refuse with their code and nothing restarts
//     (TestRestartInPlaceKeepsTheGuestCorrectnessRefusals);
//   - the verb used to loop a computer: a second restart of the same
//     computer within restartInPlaceMinGap is refused restart_rate_bound,
//     before the guest is asked; a failed restart counts; the bound is per
//     computer; it is in memory like the tracker's failed-target map, so a
//     vmctl restart forgets it (a vmctl restart is itself a crash-rule boot
//     of every computer, not the loop this bound stops)
//     (TestRestartInPlaceRateBound, ...MarkerWrittenButRestartFails,
//     TestRestartInPlaceRateBoundIsPerComputer);
//   - the owner's click gaining the new power: without restart_in_place a
//     current computer is still current and untouched
//     (TestOwnerUpdateOnACurrentComputerStillDoesNothing);
//   - two operator calls at once (check-then-act): the reservation is taken
//     atomically before the guest is asked, so the second is refused
//     restart_rate_bound while the first is still running and the guest sees
//     one marker and one restart (TestRestartInPlaceConcurrentCallsRestartOnce);
//     a reservation that restarted nothing is given back
//     (TestRestartInPlaceMeetsAnInFlightRefresh, ...GuestCorrectnessRefusals);
//   - a wedged guest (health and marker both time out): the probe and the
//     marker call are bounded, a refused call never probes, and the restart
//     still happens, markerless, answered work=closed; a healthy guest still
//     refuses (TestRestartInPlaceOfAWedgedGuestIsMarkerlessAndSaysSo,
//     TestRestartInPlaceRateRefusalNeverProbesTheGuest,
//     TestRestartInPlaceKeepsTheGuestCorrectnessRefusals);
//   - an update pending: the verb applies it and says refreshed, not
//     restarted (TestRestartInPlaceOnAComputerBehindIsAnUpdate).

func postRestartInPlace(t *testing.T, h *Handler, user string) (int, deployRefreshResponse) {
	t.Helper()
	return postDeployRefresh(t, h, `{"user_id":"`+user+`","desktop_id":"primary","target":"attacker-chosen","owner_busy":true,"restart_in_place":true}`)
}

// currentFixture is a computer serving the release it would boot, with a
// clock the test moves.
func currentFixture(t *testing.T) (*Handler, *mockVMManager, *fakeGuestMarker, *time.Time) {
	t.Helper()
	h, mgr, fake := newRingOnlyFixture(t)
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "commit-r1" }
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	h.SetBehindRefresh(0, func() time.Time { return now })
	return h, mgr, fake, &now
}

func TestOwnerUpdateOnACurrentComputerStillDoesNothing(t *testing.T) {
	h, mgr, fake, _ := currentFixture(t)
	code, resp := postOwnerUpdate(t, h, "owner-up")
	if code != http.StatusOK || resp.Outcome != deployRefreshCurrent || len(fake.marks) != 0 || len(mgr.refreshes) != 0 {
		t.Fatalf("owner click on a current computer: %d %+v marks=%v refreshes=%v", code, resp, fake.marks, mgr.refreshes)
	}
}

func TestRestartInPlaceRestartsACurrentComputerWithItsMarker(t *testing.T) {
	h, mgr, fake, _ := currentFixture(t)
	code, resp := postRestartInPlace(t, h, "owner-up")
	if code != http.StatusOK || resp.Outcome != deployRefreshRestarted || !resp.PlannedRestart || resp.Target != "commit-r1" {
		t.Fatalf("%d %+v, want 200 restarted, planned, target commit-r1", code, resp)
	}
	if len(fake.marks) != 1 || !strings.HasSuffix(fake.marks[0], "@commit-r1") || len(fake.allowBusy) != 1 || !fake.allowBusy[0] {
		t.Fatalf("marks=%v allow_busy=%v, want one marker for the same version, allow_busy", fake.marks, fake.allowBusy)
	}
	if resp.Work != "resumed" {
		t.Fatalf("work = %q, want resumed", resp.Work)
	}
	if len(mgr.refreshes) != 1 || fake.refreshesAtMark[0] != 0 {
		t.Fatalf("refreshes=%v at-mark=%v, want the marker written before the one restart", mgr.refreshes, fake.refreshesAtMark)
	}
	if fake.clears != 0 {
		t.Fatalf("a successful restart cleared its marker (%d)", fake.clears)
	}
	if pending := h.pendingDeployRefreshes(); len(pending) != 0 {
		t.Fatalf("restart left a pending entry: %+v", pending)
	}
}

func TestRestartInPlaceOnAComputerBehindIsAnUpdate(t *testing.T) {
	h, mgr, _, _ := currentFixture(t)
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "old" }
	code, resp := postRestartInPlace(t, h, "owner-up")
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || len(mgr.refreshes) != 1 {
		t.Fatalf("%d %+v, want refreshed (an update, not a same-version restart)", code, resp)
	}
}

func TestRestartInPlaceMarkerWrittenButRestartFails(t *testing.T) {
	h, mgr, fake, now := currentFixture(t)
	mgr.refreshError = errors.New("firecracker refused")
	code, resp := postRestartInPlace(t, h, "owner-up")
	if code != http.StatusInternalServerError || resp.Outcome != deployRefreshFailed {
		t.Fatalf("%d %+v, want 500 failed", code, resp)
	}
	if fake.clears != 1 {
		t.Fatalf("marker clears = %d, want 1: a marker must not outlive a restart that did not happen", fake.clears)
	}
	// A failed restart counts toward the bound: the verb cannot loop a failing boot.
	// (The registry also marks the computer failed, so a second call would
	// answer not_active first; the bound itself is what is asserted.)
	if _, refusal := h.deployRefreshState().beginRestartInPlace("owner-up", "primary", now.Add(time.Minute), restartInPlaceMinGap); refusal == "" {
		t.Fatal("a failed restart was not charged to the bound")
	}
}

func TestRestartInPlaceNeverTouchesANonActiveComputer(t *testing.T) {
	for _, state := range []VMState{VMStateStopped, VMStateBooting, VMStateFailed} {
		h, mgr, fake, _ := currentFixture(t)
		h.registry.mu.Lock()
		for _, o := range h.registry.ownerships {
			o.State = state
		}
		h.registry.mu.Unlock()
		bootsBefore := len(mgr.boots)
		code, resp := postRestartInPlace(t, h, "owner-up")
		if code != http.StatusConflict || resp.Outcome != deployRefreshSkipped || resp.ReasonCode != ownerReasonNotActive {
			t.Fatalf("%s: %d %+v, want skipped not_active", state, code, resp)
		}
		if len(fake.marks) != 0 || len(mgr.refreshes) != 0 || len(mgr.boots) != bootsBefore {
			t.Fatalf("%s: marked or booted: %v %v %v", state, fake.marks, mgr.refreshes, mgr.boots)
		}
	}
}

func TestRestartInPlaceMeetsAnInFlightRefresh(t *testing.T) {
	h, mgr, fake, _ := currentFixture(t)
	release, _ := h.deployRefreshState().begin("owner-up", "primary")
	code, resp := postRestartInPlace(t, h, "owner-up")
	release()
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || resp.ReasonCode != ownerReasonInFlight {
		t.Fatalf("%d %+v, want deferred refresh_in_flight", code, resp)
	}
	if len(fake.marks) != 0 || len(mgr.refreshes) != 0 {
		t.Fatalf("the loser reached the guest: %v %v", fake.marks, mgr.refreshes)
	}
	// The deferral restarted nothing, so it is not charged to the bound.
	if code, resp = postRestartInPlace(t, h, "owner-up"); code != http.StatusOK || resp.Outcome != deployRefreshRestarted {
		t.Fatalf("after the in-flight deferral: %d %+v, want restarted", code, resp)
	}
}

func TestRestartInPlaceKeepsTheGuestCorrectnessRefusals(t *testing.T) {
	cases := []struct{ name, detail, code string }{
		{"capsule", `{"outcome":"busy","allow_busy":true,"live_capsules":1}`, "live_capsule"},
		{"apply", `{"outcome":"busy","allow_busy":true,"selfdev_active_operations":1}`, "selfdev_active"},
		{"candidate", `{"outcome":"busy","reason":"selfdev_awaiting_decision","selfdev_awaiting_decision":1}`, "selfdev_awaiting_decision"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, mgr, fake, now := currentFixture(t)
			fake.outcome, fake.detail = guestMarkBusy, tc.detail
			code, resp := postRestartInPlace(t, h, "owner-up")
			if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || resp.ReasonCode != tc.code || len(resp.Refusal) == 0 {
				t.Fatalf("%d %+v, want 409 deferred %s with the guest's body", code, resp, tc.code)
			}
			if len(mgr.refreshes) != 0 {
				t.Fatalf("restarted despite the refusal: %v", mgr.refreshes)
			}
			// A refusal restarted nothing: it is not charged to the bound, and
			// once the guest settles the next ask goes through.
			*now = now.Add(time.Second)
			fake.outcome, fake.detail = guestMarkWritten, ""
			if code, resp = postRestartInPlace(t, h, "owner-up"); code != http.StatusOK || resp.Outcome != deployRefreshRestarted {
				t.Fatalf("after the guest settled: %d %+v", code, resp)
			}
		})
	}
}

func TestRestartInPlaceRateBound(t *testing.T) {
	h, mgr, fake, now := currentFixture(t)
	if code, resp := postRestartInPlace(t, h, "owner-up"); code != http.StatusOK || resp.Outcome != deployRefreshRestarted {
		t.Fatalf("first: %d %+v", code, resp)
	}
	*now = now.Add(restartInPlaceMinGap - time.Second)
	code, resp := postRestartInPlace(t, h, "owner-up")
	if code != http.StatusConflict || resp.Outcome != deployRefreshSkipped || resp.ReasonCode != ownerReasonRestartRate || resp.Reason == "" {
		t.Fatalf("inside the window: %d %+v, want 409 skipped restart_rate_bound", code, resp)
	}
	if len(fake.marks) != 1 || len(mgr.refreshes) != 1 {
		t.Fatalf("the refused call reached the guest or restarted: %v %v", fake.marks, mgr.refreshes)
	}
	*now = now.Add(2 * time.Second)
	if code, resp = postRestartInPlace(t, h, "owner-up"); code != http.StatusOK || resp.Outcome != deployRefreshRestarted || len(mgr.refreshes) != 2 {
		t.Fatalf("after the window: %d %+v", code, resp)
	}
}

func TestRestartInPlaceRateBoundIsPerComputer(t *testing.T) {
	h, _, _, now := currentFixture(t)
	if _, resp := postRestartInPlace(t, h, "owner-up"); resp.Outcome != deployRefreshRestarted {
		t.Fatalf("first: %+v", resp)
	}
	tr := h.deployRefreshState()
	if _, refusal := tr.beginRestartInPlace("owner-up", "primary", *now, restartInPlaceMinGap); refusal == "" {
		t.Fatal("the restarted computer is not bounded")
	}
	finish, refusal := tr.beginRestartInPlace("someone-else", "primary", *now, restartInPlaceMinGap)
	if refusal != "" {
		t.Fatal("the bound is shared across computers")
	}
	finish(false)
}

// The coordinator's review: the reservation is atomic. The first call is held
// inside the restart; the second must be refused, not restart again.
func TestRestartInPlaceConcurrentCallsRestartOnce(t *testing.T) {
	h, mgr, fake, _ := currentFixture(t)
	mgr.refreshStarted = make(chan struct{})
	mgr.refreshRelease = make(chan struct{})
	type result struct {
		code int
		resp deployRefreshResponse
	}
	first := make(chan result, 1)
	go func() {
		code, resp := postRestartInPlace(t, h, "owner-up")
		first <- result{code, resp}
	}()
	<-mgr.refreshStarted
	code, resp := postRestartInPlace(t, h, "owner-up")
	if code != http.StatusConflict || resp.Outcome != deployRefreshSkipped || resp.ReasonCode != ownerReasonRestartRate {
		t.Fatalf("second concurrent call: %d %+v, want restart_rate_bound", code, resp)
	}
	close(mgr.refreshRelease)
	r := <-first
	if r.code != http.StatusOK || r.resp.Outcome != deployRefreshRestarted {
		t.Fatalf("first call: %d %+v", r.code, r.resp)
	}
	if len(fake.marks) != 1 || len(mgr.refreshes) != 1 {
		t.Fatalf("marks=%v refreshes=%v, want exactly one of each", fake.marks, mgr.refreshes)
	}
}

// deadlineMarker records whether the marker call was bounded.
type deadlineMarker struct {
	*fakeGuestMarker
	bounded bool
}

func (d *deadlineMarker) mark(ctx context.Context, own VMOwnership, target string, allowBusy bool) (guestMarkOutcome, string) {
	_, d.bounded = ctx.Deadline()
	return d.fakeGuestMarker.mark(ctx, own, target, allowBusy)
}

// A wedged guest: /health never answers and the marker call times out. The
// restart still happens, as a crash boot (no marker), and says so.
func TestRestartInPlaceOfAWedgedGuestIsMarkerlessAndSaysSo(t *testing.T) {
	oldProbe, oldMark := restartProbeTimeout, restartMarkTimeout
	restartProbeTimeout, restartMarkTimeout = 10*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { restartProbeTimeout, restartMarkTimeout = oldProbe, oldMark })
	h, mgr, fake, _ := currentFixture(t)
	probes := 0
	h.guestServedCommit = func(ctx context.Context, _ VMOwnership) string {
		probes++
		<-ctx.Done() // a wedged guest answers nothing; only the bound ends the probe
		return ""
	}
	fake.outcome, fake.detail = guestMarkUnavailable, "context deadline exceeded"
	dm := &deadlineMarker{fakeGuestMarker: fake}
	h.guestMarker = dm
	code, resp := postRestartInPlace(t, h, "owner-up")
	if code != http.StatusOK || (resp.Outcome != deployRefreshRefreshed && resp.Outcome != deployRefreshRestarted) {
		t.Fatalf("%d %+v, want the restart to happen", code, resp)
	}
	if resp.PlannedRestart || resp.Work != "closed" || len(mgr.refreshes) != 1 {
		t.Fatalf("%+v refreshes=%v, want markerless, work=closed, one restart", resp, mgr.refreshes)
	}
	if probes != 1 {
		t.Fatalf("served commit probed %d times, want once", probes)
	}
	if !dm.bounded {
		t.Fatal("the marker call had no deadline")
	}
	// A guest that predates the marker endpoint is the same: no /health
	// busy check, a markerless restart.
	h2, mgr2, fake2, _ := currentFixture(t)
	fake2.outcome = guestMarkUnsupported
	if code, resp = postRestartInPlace(t, h2, "owner-up"); code != http.StatusOK || resp.Work != "closed" || len(mgr2.refreshes) != 1 {
		t.Fatalf("old guest: %d %+v", code, resp)
	}
}

// A refused call asks nothing of the guest: no probe, no marker.
func TestRestartInPlaceRateRefusalNeverProbesTheGuest(t *testing.T) {
	h, _, fake, _ := currentFixture(t)
	if _, resp := postRestartInPlace(t, h, "owner-up"); resp.Outcome != deployRefreshRestarted {
		t.Fatalf("first: %+v", resp)
	}
	probes, marks := 0, len(fake.marks)
	h.guestServedCommit = func(context.Context, VMOwnership) string { probes++; return "commit-r1" }
	if _, resp := postRestartInPlace(t, h, "owner-up"); resp.ReasonCode != ownerReasonRestartRate {
		t.Fatalf("second: %+v", resp)
	}
	if probes != 0 || len(fake.marks) != marks {
		t.Fatalf("a refused call reached the guest: probes=%d marks=%v", probes, fake.marks)
	}
}

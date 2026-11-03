package agentcore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Self-opened rounds starve the deploy refresh
// (docs/problems/self-opened-rounds-starve-the-deploy-refresh-2026-11-03.md):
// a test arm refused every refresh with a live capsule, and between rounds
// admission bound the next capsule before vmctl asked again. A refresh the
// host asked for now holds the next engineering admission, so the refresh
// finds the computer idle between rounds.
//
// Ways this could fail, written first:
//   - the refusal is not remembered, so admission binds the next capsule
//     and the arm stays on the stale build (the incident);
//   - the hold never ends: a refresh that never lands (a decision hold, a
//     run that never ends without allow_busy) stops engineering for ever;
//   - the hold outlives the host's interest: vmctl forgot the computer
//     (refreshed, failed, held), and the guest still waits;
//   - the host is not told the guest holds, so its backoff leaves the
//     drained gap unseen for up to an hour;
//   - a new release inherits a spent hold and the arm starts on a stale
//     build again;
//   - the hold is not visible: the owner sees a round that does not start
//     and no reason;
//   - the hold cuts live work, or the held request is lost at the restart
//     (it must stay open and unbound, which a planned boot keeps).

func refreshHoldClock(f *admissionFixture, at *time.Time) {
	f.rt.admissionNow = func() time.Time { return *at }
}

// askEveryMinuteUntil moves the clock to until with the host's one-minute
// sweep asking on the way, as vmctl does while the guest holds.
func askEveryMinuteUntil(t *testing.T, f *admissionFixture, now *time.Time, until time.Time, target string) {
	t.Helper()
	for now.Before(until) {
		*now = now.Add(time.Minute)
		if now.After(until) {
			*now = until
		}
		if code, body := postRefreshAsk(t, f, target); code != http.StatusConflict {
			t.Fatalf("ask at %s: status=%d body=%s, want 409", now.Format(time.RFC3339), code, body)
		}
	}
}

func postRefreshAsk(t *testing.T, f *admissionFixture, target string) (int, string) {
	t.Helper()
	return postPlannedRestartURL(t, &APIHandler{rt: f.rt}, "/internal/runtime/planned-restart", `{"target":"`+target+`"}`)
}

// fakeRunningRun makes the guest busy with a running run and no capsule, the
// refusal a computer without allow_busy gives while management is in a turn.
func fakeRunningRun(f *admissionFixture) func() {
	f.rt.runningMu.Lock()
	f.rt.running["run-refresh-hold"] = func() {}
	f.rt.runningMu.Unlock()
	return func() {
		f.rt.runningMu.Lock()
		delete(f.rt.running, "run-refresh-hold")
		f.rt.runningMu.Unlock()
	}
}

func requireRefreshHeld(t *testing.T, f *admissionFixture, index int, target, want string) {
	t.Helper()
	err := f.rt.admitEngineeringAssignment(context.Background(), f.get(index))
	var busy *engineeringAdmissionBusy
	if err == nil || !errors.As(err, &busy) || !errors.Is(err, ErrEngineeringAdmissionBusy) {
		t.Fatalf("%s: assignment %d: %v, want a retryable refresh hold", want, index, err)
	}
	if busy.Why != engineeringWaitRefresh || !strings.Contains(err.Error(), target) {
		t.Fatalf("%s: refusal %v (why %q), want %q naming %s", want, err, busy.Why, engineeringWaitRefresh, target)
	}
}

func TestRefreshAskedForHoldsTheNextRoundUntilTheRefreshLands(t *testing.T) {
	f := newAdmissionFixture(t, 2)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(f.rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(f.rt.cfg.StorePath)
	wakes := captureAdmissionWakes(f.rt)
	f.open(0)
	f.bind(0)

	// The round is live: the refresh is refused, and the guest says it holds
	// new work for it, so the host keeps asking every minute.
	code, body := postRefreshAsk(t, f, "e4f1c685b")
	if code != http.StatusConflict || !strings.Contains(body, `"holds_new_work":true`) {
		t.Fatalf("live round: status=%d body=%s, want 409 with holds_new_work", code, body)
	}

	// Management judges and casts the next round at once.
	f.finish(0)
	queued := f.open(1)
	requireRefreshHeld(t, f, 1, "e4f1c685b", "next round while a refresh is pending")

	content := fmt.Sprintf(`{"assignment_id":%q,"attempt":%d}`, queued.AssignmentID, queued.Binding.Attempt)
	if err := f.rt.HandleDelegatedAssignmentSpawnDeadline(context.Background(), f.seed.OwnerID, f.computer, f.seed.ParentAgentID, content); err != nil {
		t.Fatalf("held spawn wake: %v", err)
	}
	if armed := f.spawnWakesFor(*wakes, 1); len(armed) != 1 {
		t.Fatalf("held request armed %d retries, want 1", len(armed))
	}
	held := f.get(1)
	if held.Disposition != types.EngineeringAssignmentOpen || held.CapsuleDisposition != types.EngineeringCapsuleUnbound {
		t.Fatalf("held request is %s/%s, want open and unbound (a planned boot keeps it)", held.Disposition, held.CapsuleDisposition)
	}
	rec, err := f.s.GetCommitmentRecord(context.Background(), f.seed.OwnerID, f.computer, engineeringRefreshWaitRecordID(queued.AssignmentID, queued.Binding.Attempt, "e4f1c685b"))
	if err != nil || rec == nil {
		t.Fatalf("no wait record on the ledger: %v", err)
	}
	if rec.Provenance.ContextRef != queued.Binding.TrajectoryID || !strings.Contains(rec.Prediction.Hypothesis, "waiting for the update to e4f1c685b") {
		t.Fatalf("wait record %+v does not tell the owner the round waits for the update", rec)
	}

	// The next ask finds the computer idle between rounds and takes the
	// planned, marker-backed refresh.
	if code, body := postRefreshAsk(t, f, "e4f1c685b"); code != http.StatusOK {
		t.Fatalf("between rounds: status=%d body=%s, want 200", code, body)
	}
	if _, planned := ConsumePlannedRestartMarker(f.rt.cfg.StorePath, time.Now().UTC()); !planned {
		t.Fatal("the refresh between rounds wrote no planned-restart marker")
	}
}

func TestRefreshHoldIsBoundedPerEpisodeAndPerRelease(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	t0 := time.Now().UTC()
	now := t0
	refreshHoldClock(f, &now)
	defer fakeRunningRun(f)()
	f.open(0)

	if code, body := postRefreshAsk(t, f, "rel-1"); code != http.StatusConflict || !strings.Contains(body, `"holds_new_work":true`) {
		t.Fatalf("status=%d body=%s, want 409 with holds_new_work", code, body)
	}
	requireRefreshHeld(t, f, 0, "rel-1", "first hold")

	askEveryMinuteUntil(t, f, &now, t0.Add(refreshHoldMax-time.Minute), "rel-1")
	requireRefreshHeld(t, f, 0, "rel-1", "inside the bound")

	// The refresh could not land (a run that does not end on a computer
	// without allow_busy): at the bound the held round starts on the current
	// build.
	askEveryMinuteUntil(t, f, &now, t0.Add(refreshHoldMax+time.Minute), "rel-1")
	if code, body := postRefreshAsk(t, f, "rel-1"); code != http.StatusConflict || !strings.Contains(body, `"holds_new_work":true`) {
		t.Fatalf("past the bound: status=%d body=%s, want 409 still holding the next boundary", code, body)
	}
	requireAdmitted(t, f, 0, "a refresh that does not land stops holding the round at the bound")

	// Review 1: the bound is per episode. The next round boundary holds
	// again, so the refresh gets another gap in the same release instead of
	// being given up on.
	askEveryMinuteUntil(t, f, &now, t0.Add(refreshHoldMax+2*time.Minute), "rel-1")
	requireRefreshHeld(t, f, 0, "rel-1", "the next round boundary of the same release")

	// A new release is a new refresh: it holds too.
	if code, body := postRefreshAsk(t, f, "rel-2"); code != http.StatusConflict || !strings.Contains(body, `"holds_new_work":true`) {
		t.Fatalf("new release: status=%d body=%s, want holds_new_work", code, body)
	}
	requireRefreshHeld(t, f, 0, "rel-2", "new release")
}

// Review 1: an ask after the hold lapsed (the host stopped asking for a
// while: an unreachable guest, a slow sweep) starts a new episode; it must not
// inherit a spent bound and leave the release unheld for good.
func TestRefreshHoldRearmsAfterTheHostResumesAsking(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	t0 := time.Now().UTC()
	now := t0
	refreshHoldClock(f, &now)
	defer fakeRunningRun(f)()
	f.open(0)
	if code, _ := postRefreshAsk(t, f, "rel-1"); code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", code)
	}
	requireRefreshHeld(t, f, 0, "rel-1", "first hold")
	now = t0.Add(refreshHoldMax + time.Hour)
	if code, _ := postRefreshAsk(t, f, "rel-1"); code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", code)
	}
	requireRefreshHeld(t, f, 0, "rel-1", "the host asks again after a lapse")
}

func TestRefreshHoldLapsesWhenTheHostStopsAsking(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	t0 := time.Now().UTC()
	now := t0
	refreshHoldClock(f, &now)
	defer fakeRunningRun(f)()
	f.open(0)

	if code, _ := postRefreshAsk(t, f, "rel-1"); code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", code)
	}
	now = t0.Add(refreshAskFresh + time.Second)
	requireAdmitted(t, f, 0, "the host has not asked within the freshness window")
}

func TestRefreshHoldClearedWhenTheHostWithdraws(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	defer fakeRunningRun(f)()
	f.open(0)
	if code, _ := postRefreshAsk(t, f, "rel-1"); code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", code)
	}
	requireRefreshHeld(t, f, 0, "rel-1", "before withdrawal")
	req := httptest.NewRequest(http.MethodDelete, "/internal/runtime/planned-restart", nil)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	(&APIHandler{rt: f.rt}).HandleInternalPlannedRestart(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("withdraw: %d %s", rec.Code, rec.Body.String())
	}
	requireAdmitted(t, f, 0, "the host withdrew the refresh")
}

// A live assignment keeps its slot: the hold only stops new binds, and the
// slot refusal still names the live assignment first.
func TestRefreshHoldNeverTouchesLiveWork(t *testing.T) {
	f := newAdmissionFixture(t, 2)
	f.open(0)
	f.bind(0)
	if code, _ := postRefreshAsk(t, f, "rel-1"); code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", code)
	}
	f.open(1)
	requireBusy(t, f, 1, 0, "the live assignment still holds the slot")
	if live := f.get(0); live.Disposition != types.EngineeringAssignmentBound || live.CapsuleDisposition != types.EngineeringCapsuleActive {
		t.Fatalf("the hold touched live work: %s/%s", live.Disposition, live.CapsuleDisposition)
	}
}

// A verification checks a candidate that is frozen or awaiting its decision,
// which holds the refresh anyway (problem 22): holding the verification would
// only delay it by the whole bound. A distinct verification queued behind a
// held implementation request still waits its FIFO turn (queued_ahead), at
// most one episode of the bound; this test checks only the kind gate.
func TestRefreshHoldDoesNotHoldAVerification(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	defer fakeRunningRun(f)()
	f.open(0)
	if code, _ := postRefreshAsk(t, f, "rel-1"); code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", code)
	}
	verification := f.get(0)
	verification.Binding.Kind = types.EngineeringAssignmentVerification
	if err := f.rt.admitEngineeringAssignment(context.Background(), verification); err != nil {
		t.Fatalf("a verification waited for the refresh: %v", err)
	}
	requireRefreshHeld(t, f, 0, "rel-1", "the implementation request still waits, its bound unspent")
}

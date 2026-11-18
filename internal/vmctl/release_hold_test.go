package vmctl

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The deploy moves the guest-release link inside its nixos switch and
// restarts vmctl only later (docs/problems/paper-crash-boot-holds-work-six-
// minutes-2026-10-22.md, "Seen again, worse", 2026-11-18). In that gap the
// old vmctl must start no refresh: the new vmctl would close the boot it
// began as interrupted, and the planned-restart marker would be spent on it.

// moveLink is the deploy's activation moving the link under a running vmctl.
func (f *ringFixture) moveLink(id, commit string) {
	f.mgr.current, f.mgr.commit = id, commit
}

// enrollDeEarlier puts d@e in the pending set aimed at commit, the state a
// deferred or ring-enrolled refresh leaves for the sweep.
func (f *ringFixture) enrollDeEarlier(h *Handler, commit string) {
	h.deployRefreshState().ensure(deployRefreshPendingEntry{
		UserID: f.de.UserID, DesktopID: "primary", VMID: f.de.VMID, ComputerID: f.de.ComputerID,
		Target: commit, LastReason: "test",
	})
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestReleaseLinkMovedUnderRunningVmctlStartsNoRefresh(t *testing.T) {
	f := newRingHandlerFixture(t)
	logs := captureLog(t)
	f.moveLink("r2", "commit-r2")
	f.enrollDeEarlier(f.h, "commit-r2")
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		f.h.TickRingWalk(ctx)
		f.h.SweepDeployRefreshPending(ctx)
	}
	if len(f.mgr.refreshes) != 0 || len(f.fake.marks) != 0 {
		t.Fatalf("a vmctl the deploy is about to replace started a refresh: marks %v refreshes %v", f.fake.marks, f.mgr.refreshes)
	}
	if n := strings.Count(logs.String(), "vmctl: refresh hold: started"); n != 1 {
		t.Fatalf("the hold must be logged once when it starts, got %d:\n%s", n, logs.String())
	}
	if st := f.h.ringWalkRunner().status(10); st.State.Walk != nil {
		t.Fatalf("the old vmctl began a walk toward the new release: %+v", st.State.Walk)
	}
}

// A fresh vmctl (the deploy's restart) reads the moved link as its own
// starting release and walks and refreshes as before.
func TestFreshVmctlAfterTheLinkMovedRefreshes(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.moveLink("r2", "commit-r2")
	h2 := NewHandler(f.reg)
	dir := t.TempDir()
	if err := h2.SetRingWalkPath(filepath.Join(dir, "ring-walk.json"), filepath.Join(dir, "ring-walk-history.jsonl")); err != nil {
		t.Fatal(err)
	}
	h2.guestMarker = f.fake
	h2.guestServedCommit = func(context.Context, VMOwnership) string { return "" }
	h2.PrimeRingWalk()
	f.enrollDeEarlier(h2, "commit-r2")
	logs := captureLog(t)
	ctx := context.Background()
	h2.TickRingWalk(ctx)
	h2.SweepDeployRefreshPending(ctx)
	if len(f.mgr.refreshes) != 1 {
		t.Fatalf("a fresh vmctl must refresh d@e: marks %v refreshes %v", f.fake.marks, f.mgr.refreshes)
	}
	if strings.Contains(logs.String(), "refresh hold") {
		t.Fatalf("a fresh vmctl held its refresh:\n%s", logs.String())
	}
}

// The deploy failed and rolled the link back: the hold ends with one line and
// the sweep proceeds on the old release.
func TestReleaseHoldEndsWhenTheLinkReturns(t *testing.T) {
	f := newRingHandlerFixture(t)
	logs := captureLog(t)
	ctx := context.Background()
	f.moveLink("r2", "commit-r2")
	f.h.TickRingWalk(ctx)
	f.moveLink("r1", "commit-r1")
	f.enrollDeEarlier(f.h, "commit-r1")
	f.h.TickRingWalk(ctx)
	f.h.SweepDeployRefreshPending(ctx)
	if len(f.mgr.refreshes) != 1 {
		t.Fatalf("after the link returned the sweep must run: marks %v refreshes %v", f.fake.marks, f.mgr.refreshes)
	}
	out := logs.String()
	if strings.Count(out, "vmctl: refresh hold: started") != 1 || strings.Count(out, "vmctl: refresh hold: ended") != 1 {
		t.Fatalf("want one started and one ended line:\n%s", out)
	}
}

// The deploy died after the link moved and never restarted vmctl: the hold
// ends at its bound, says so once, and the sweep runs.
func TestReleaseHoldExpiresAtItsBound(t *testing.T) {
	f := newRingHandlerFixture(t)
	logs := captureLog(t)
	clock := time.Now()
	f.h.replacement.now = func() time.Time { return clock }
	ctx := context.Background()
	f.moveLink("r2", "commit-r2")
	f.enrollDeEarlier(f.h, "commit-r2")
	f.h.SweepDeployRefreshPending(ctx)
	clock = clock.Add(releaseHoldBound - time.Minute)
	f.h.SweepDeployRefreshPending(ctx)
	if len(f.mgr.refreshes) != 0 {
		t.Fatalf("held inside its bound, yet refreshed: %v", f.mgr.refreshes)
	}
	clock = clock.Add(2 * time.Minute)
	f.h.SweepDeployRefreshPending(ctx)
	f.h.SweepDeployRefreshPending(ctx)
	if len(f.mgr.refreshes) != 1 {
		t.Fatalf("past the bound the sweep must run: marks %v refreshes %v", f.fake.marks, f.mgr.refreshes)
	}
	out := logs.String()
	if strings.Count(out, "refresh hold: started") != 1 || strings.Count(out, "without a restart") != 1 {
		t.Fatalf("want one started line and one expiry line:\n%s", out)
	}
}

// The hold stands down only the ticker's paths: the deploy's own request
// (served by whichever vmctl answers it) is unchanged.
func TestReleaseHoldLeavesTheDeploysOwnRefreshAlone(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.moveLink("r2", "commit-r2")
	f.h.TickRingWalk(context.Background())
	code, resp := postDeployRefresh(t, f.h, `{"user_id":"ring-de","desktop_id":"primary","target":"commit-r2"}`)
	if code != 200 || resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("the deploy's own refresh during a hold: %d %+v", code, resp)
	}
}

// Review finding 1: the hold is decided before each refresh of a sweep, not
// once per sweep. The link moves while the first refresh is in flight; the
// later entries are neither refreshed nor touched.
func TestReleaseHoldIsCheckedBeforeEachRefreshOfASweep(t *testing.T) {
	f := newRingHandlerFixture(t)
	logs := captureLog(t)
	f.enrollDeEarlier(f.h, "commit-r1")
	f.h.deployRefreshState().ensure(deployRefreshPendingEntry{
		UserID: f.o3.UserID, DesktopID: "primary", VMID: f.o3.VMID, ComputerID: f.o3.ComputerID,
		Target: "commit-r1", LastReason: "test",
	})
	f.mgr.mockVMManager.onRefresh = func(string) { f.moveLink("r2", "commit-r2") }
	f.h.SweepDeployRefreshPending(context.Background())
	if n := len(f.mgr.refreshes); n != 1 {
		t.Fatalf("the sweep refreshed %d computers; the link moved after the first: %v", n, f.mgr.refreshes)
	}
	refreshed := f.mgr.refreshes[0]
	var untouched *VMOwnership
	for _, own := range []*VMOwnership{f.de, f.o3} {
		if own.VMID != refreshed {
			untouched = own
		}
	}
	var kept *deployRefreshPendingEntry
	for _, e := range f.h.pendingDeployRefreshes() {
		if e.VMID == untouched.VMID {
			e := e
			kept = &e
		}
	}
	if kept == nil || kept.Attempts != 0 || !kept.LastAttemptAt.IsZero() {
		t.Fatalf("the held entry's pending state changed: %+v", kept)
	}
	if got := f.reg.GetOwnershipForDesktop(untouched.UserID, "primary"); len(got.DeployGrants) != 0 {
		t.Fatalf("a deploy grant was spent on a held computer: %+v", got.DeployGrants)
	}
	if n := strings.Count(logs.String(), "vmctl: refresh hold: started"); n != 1 {
		t.Fatalf("hold line count %d:\n%s", n, logs.String())
	}
}

// Review finding 1 (smaller window): one tick reads the link once, so the
// hold decision and the walk's observation cannot disagree.
func TestTickReadsTheLinkOnceForHoldAndWalk(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.mgr.reads, f.mgr.flipAfter, f.mgr.flipID = 0, 1, "r2"
	f.h.TickRingWalk(context.Background())
	if st := f.h.ringWalkRunner().status(10); st.State.Walk != nil {
		t.Fatalf("a link that moved between two reads started a walk in the old vmctl: %+v", st.State.Walk)
	}
}

// Review finding 1: a wake the walk asks for is a boot too; it is not
// started once the link has moved.
func TestRingWakeIsHeldWhenTheLinkMoved(t *testing.T) {
	f := newRingHandlerFixture(t)
	woke := 0
	f.h.ringWaker = func(ringAction) { woke++ }
	f.moveLink("r2", "commit-r2")
	f.h.applyRingActions(time.Now().UTC(), []ringAction{{Kind: ringActionWake, UserID: "ring-de", DesktopID: "primary", ComputerID: f.de.ComputerID, VMID: f.de.VMID}})
	if woke != 0 {
		t.Fatalf("a wake started under a hold")
	}
}

// Review finding 2: a link that does not resolve at start is no baseline.
func TestUnreadableLinkAtStartIsNotABaseline(t *testing.T) {
	f := newRingHandlerFixture(t)
	logs := captureLog(t)
	f.moveLink("", "")
	f.h.adoptCurrentRelease()
	f.moveLink("r1", "commit-r1")
	f.enrollDeEarlier(f.h, "commit-r1")
	f.h.SweepDeployRefreshPending(context.Background())
	if len(f.mgr.refreshes) != 1 || strings.Contains(logs.String(), "refresh hold") {
		t.Fatalf("the first readable release must be the baseline, not a move: refreshes %v\n%s", f.mgr.refreshes, logs.String())
	}
}

// Review finding 2: an unreadable link after a good baseline is no
// information: it starts no hold (and ends none).
func TestUnreadableLinkAfterBaselineStartsNoHold(t *testing.T) {
	f := newRingHandlerFixture(t)
	logs := captureLog(t)
	ctx := context.Background()
	f.moveLink("", "")
	f.h.TickRingWalk(ctx)
	f.h.SweepDeployRefreshPending(ctx)
	f.moveLink("r1", "commit-r1")
	f.h.SweepDeployRefreshPending(ctx)
	if strings.Contains(logs.String(), "refresh hold") {
		t.Fatalf("a read failure was taken for a move:\n%s", logs.String())
	}
}

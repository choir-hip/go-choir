package vmctl

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// D2 S4, resume then update (docs/design/density-d2-snapshots-survive-deploys-
// 2026-10-26.md 5.1): every release-bound snapshot resume enrolls the computer
// in the deploy-refresh pending set as "woke_behind"; the sweep resolves the
// current release itself, forgets a computer that is not behind, waits for the
// user-idle gate, and then updates through the ordinary planned, marker-backed
// refresh.
//
// Failure modes, written first:
//   - a legacy resume enrolls (a refresh of a computer that is not behind);
//   - the entry trusts the validation-time "behind" and misses a switch that
//     landed between validation and the load;
//   - the update restarts a computer in front of its user (no idle gate);
//   - a CI deploy's refresh is held by the idle gate because the entry kept
//     its kind;
//   - the update kills a busy computer, or loops on a failed refresh;
//   - a hibernated computer's entry lives on and wakes a later refresh;
//   - a vmctl restart loses the kind and so the gate.

type behindManager struct {
	*snapshotMockVMManager
	mu      sync.Mutex
	current string
	commit  string
	byVM    map[string]GuestReleaseVMStatus
}

func (b *behindManager) GuestReleases() (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
func (b *behindManager) GuestReleaseHealth() (string, map[string]GuestReleaseVMStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.current, b.byVM
}
func (b *behindManager) CurrentGuestReleaseInfo() (string, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.current, b.commit
}
func (b *behindManager) setCurrent(id, commit string) {
	b.mu.Lock()
	b.current, b.commit = id, commit
	for vm, st := range b.byVM {
		st.Behind = st.GuestRelease != id
		b.byVM[vm] = st
	}
	b.mu.Unlock()
}

type behindFixture struct {
	h     *Handler
	reg   *OwnershipRegistry
	mgr   *behindManager
	fake  *fakeGuestMarker
	own   *VMOwnership
	clock *time.Time
}

// newBehindFixture: a hibernated computer with a snapshot of release r1, the
// current release r1 at validation time, then woken from the snapshot.
func newBehindFixture(t *testing.T, user, snapshotRelease string) *behindFixture {
	t.Helper()
	snap := &snapshotMockVMManager{mockVMManager: &mockVMManager{
		refreshResponse: &VMInstanceInfo{HostURL: "http://127.0.0.1:9043", Epoch: 7, Healthy: true, State: "running"},
		getVMs:          make(map[string]*VMInstanceInfo),
	}}
	mgr := &behindManager{snapshotMockVMManager: snap, current: "r1", commit: "commit-r1", byVM: map[string]GuestReleaseVMStatus{}}
	reg, initial := newSnapshotTestRegistry(t, snap, user)
	reg.SetVMManager(mgr)
	snap.snapshotResp = &VMInstanceInfo{HostURL: initial.ComputerURL, Epoch: initial.Epoch, Healthy: true, State: "running", GuestRelease: snapshotRelease}
	if snapshotRelease != "" {
		mgr.byVM[initial.VMID] = GuestReleaseVMStatus{GuestRelease: snapshotRelease, Behind: snapshotRelease != "r1"}
	}
	h := NewHandler(reg)
	fake := &fakeGuestMarker{mgr: snap.mockVMManager, outcome: guestMarkWritten}
	h.guestMarker = fake
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "" }
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	f := &behindFixture{h: h, reg: reg, mgr: mgr, fake: fake, clock: &now}
	h.SetBehindRefresh(2*time.Minute, func() time.Time { return *f.clock })
	own, err := reg.ResumeVM(user)
	if err != nil {
		t.Fatalf("ResumeVM: %v", err)
	}
	f.own = own
	return f
}

func (f *behindFixture) setActive(t *testing.T, ago time.Duration) {
	t.Helper()
	f.reg.mu.Lock()
	defer f.reg.mu.Unlock()
	o := f.reg.ownerships[ownershipKey(f.own.UserID, f.own.DesktopID)]
	o.LastActiveAt = f.clock.Add(-ago)
}

func (f *behindFixture) setState(state VMState) {
	f.reg.mu.Lock()
	defer f.reg.mu.Unlock()
	f.reg.ownerships[ownershipKey(f.own.UserID, f.own.DesktopID)].State = state
}

func (f *behindFixture) sweep() { f.h.SweepDeployRefreshPending(context.Background()) }

func (f *behindFixture) refreshes() int {
	f.mgr.mockVMManager.mu.Lock()
	defer f.mgr.mockVMManager.mu.Unlock()
	return len(f.mgr.mockVMManager.refreshes)
}

// 1: every release-bound resume enrolls one entry with a non-empty placeholder
// target; a legacy resume enrolls nothing.
func TestReleaseBoundResumeEnrolls(t *testing.T) {
	f := newBehindFixture(t, "behind-1", "r1")
	pending := f.h.pendingDeployRefreshes()
	if len(pending) != 1 || pending[0].Kind != deployRefreshKindWokeBehind || pending[0].Release != "r1" || pending[0].Target == "" || pending[0].UserID != "behind-1" || pending[0].VMID != f.own.VMID {
		t.Fatalf("pending = %+v; want one woke_behind entry for release r1 with a non-empty target", pending)
	}
	if pending[0].LastReason != "resumed on release r1" {
		t.Fatalf("last reason = %q", pending[0].LastReason)
	}
	legacy := newBehindFixture(t, "legacy-1", "")
	if got := legacy.h.pendingDeployRefreshes(); len(got) != 0 {
		t.Fatalf("a legacy resume enrolled: %+v", got)
	}
}

// 2: the race. A resume validated while r1 was current is enrolled with r1's
// commit as the placeholder; the link then moves to r2 before the first sweep:
// the sweep resolves r2, sets the target to r2's commit and, after the gate,
// refreshes. A resume onto the release that is still current is forgotten.
func TestBehindSweepResolvesTheCurrentReleaseItself(t *testing.T) {
	f := newBehindFixture(t, "race", "r1")
	f.mgr.setCurrent("r2", "commit-r2")
	f.setActive(t, 10*time.Minute)
	f.sweep()
	if f.refreshes() != 1 {
		t.Fatalf("refreshes = %d; want one (the release moved after validation)", f.refreshes())
	}
	if len(f.fake.marks) != 1 || f.fake.marks[0] != f.own.VMID+"@commit-r2" {
		t.Fatalf("marks = %v; want the marker asked for r2's commit", f.fake.marks)
	}
	if len(f.h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("pending after the refresh = %+v", f.h.pendingDeployRefreshes())
	}

	still := newBehindFixture(t, "race-still", "r1")
	still.setActive(t, 10*time.Minute)
	still.sweep()
	if still.refreshes() != 0 || len(still.fake.marks) != 0 || len(still.h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("a resume onto the current release: refreshes=%d marks=%v pending=%+v; want forgotten with no contact", still.refreshes(), still.fake.marks, still.h.pendingDeployRefreshes())
	}
}

// 3: the user-idle gate.
func TestBehindSweepWaitsForTheUserIdleGate(t *testing.T) {
	f := newBehindFixture(t, "gate", "r1")
	f.mgr.setCurrent("r2", "commit-r2")
	f.setActive(t, 30*time.Second)
	f.sweep()
	if f.refreshes() != 0 || len(f.fake.marks) != 0 || len(f.h.pendingDeployRefreshes()) != 1 {
		t.Fatalf("within the gate: refreshes=%d marks=%v pending=%+v; want skipped and kept, guest not contacted", f.refreshes(), f.fake.marks, f.h.pendingDeployRefreshes())
	}
	*f.clock = f.clock.Add(2 * time.Minute)
	f.sweep()
	if f.refreshes() != 1 {
		t.Fatalf("after the gate: refreshes=%d; want one", f.refreshes())
	}
	if len(f.fake.marks) != 1 {
		t.Fatalf("marks = %v; want the planned-restart marker asked once", f.fake.marks)
	}
}

// 4: busy keeps the entry; a refresh or a failed refresh forgets it.
func TestBehindSweepBusyKeepsRefreshedAndFailedForget(t *testing.T) {
	f := newBehindFixture(t, "busy", "r1")
	f.mgr.setCurrent("r2", "commit-r2")
	f.setActive(t, 10*time.Minute)
	f.fake.outcome = guestMarkBusy
	f.sweep()
	pending := f.h.pendingDeployRefreshes()
	if f.refreshes() != 0 || len(pending) != 1 || pending[0].Kind != deployRefreshKindWokeBehind || pending[0].Target != "commit-r2" {
		t.Fatalf("busy: refreshes=%d pending=%+v; want kept as woke_behind on r2's commit", f.refreshes(), pending)
	}
	f.sweep()
	if len(f.fake.marks) != 2 || f.refreshes() != 0 {
		t.Fatalf("a busy guest is asked again every sweep without being killed: marks=%v", f.fake.marks)
	}
	setFakeOutcome(f.fake, guestMarkWritten)
	f.sweep()
	if f.refreshes() != 1 || len(f.h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("idle at last: refreshes=%d pending=%+v", f.refreshes(), f.h.pendingDeployRefreshes())
	}

	failing := newBehindFixture(t, "failing-behind", "r1")
	failing.mgr.setCurrent("r2", "commit-r2")
	failing.setActive(t, 10*time.Minute)
	failing.mgr.mockVMManager.mu.Lock()
	failing.mgr.mockVMManager.refreshError = context.DeadlineExceeded
	failing.mgr.mockVMManager.mu.Unlock()
	failing.sweep()
	failing.sweep()
	if len(failing.fake.marks) != 1 || len(failing.h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("a failed refresh must not loop: marks=%v pending=%+v", failing.fake.marks, failing.h.pendingDeployRefreshes())
	}
}

// 5: a guest that already serves the target is forgotten.
func TestBehindSweepAlreadyServingTargetIsForgotten(t *testing.T) {
	f := newBehindFixture(t, "serves", "r1")
	f.mgr.setCurrent("r2", "commit-r2")
	f.setActive(t, 10*time.Minute)
	f.h.guestServedCommit = func(context.Context, VMOwnership) string { return "commit-r2" }
	f.sweep()
	if f.refreshes() != 0 || len(f.h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("already on the target: refreshes=%d pending=%+v", f.refreshes(), f.h.pendingDeployRefreshes())
	}
}

// 6: a vmctl restart reloads the entry with its kind and release.
func TestBehindEntrySurvivesAVMCTLRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pending.json")
	f := newBehindFixture(t, "durable-behind", "r1")
	if err := f.h.SetDeployRefreshPendingPath(path); err != nil {
		t.Fatal(err)
	}
	// Re-enroll so the entry is saved to the path.
	f.h.enrollResumed(*f.own, "r1")
	h2 := NewHandler(f.reg)
	if err := h2.SetDeployRefreshPendingPath(path); err != nil {
		t.Fatal(err)
	}
	pending := h2.pendingDeployRefreshes()
	if len(pending) != 1 || pending[0].Kind != deployRefreshKindWokeBehind || pending[0].Release != "r1" {
		t.Fatalf("reloaded pending = %+v; want the woke_behind entry for r1", pending)
	}
}

// 7: a CI deploy's refresh of the same computer settles and forgets the
// entry; deferred as busy it replaces the entry with an empty kind, and the
// gate no longer applies.
func TestCIDeployRefreshReplacesTheBehindKind(t *testing.T) {
	f := newBehindFixture(t, "ci-replaces", "r1")
	f.mgr.setCurrent("r2", "commit-r2")
	f.setActive(t, 10*time.Second) // inside the gate
	f.fake.outcome = guestMarkBusy
	code, resp := postDeployRefresh(t, f.h, `{"user_id":"ci-replaces","desktop_id":"primary","target":"deadbeef"}`)
	if resp.Outcome != deployRefreshDeferred {
		t.Fatalf("deploy refresh: %d %+v; want deferred busy", code, resp)
	}
	pending := f.h.pendingDeployRefreshes()
	if len(pending) != 1 || pending[0].Kind != "" || pending[0].Release != "" || pending[0].Target != "deadbeef" {
		t.Fatalf("pending = %+v; want the deploy's target with an empty kind", pending)
	}
	// Inside the idle gate, but a deploy's refresh is never held by it.
	setFakeOutcome(f.fake, guestMarkWritten)
	f.sweep()
	if f.refreshes() != 1 {
		t.Fatalf("a deploy refresh was held by the user-idle gate: refreshes=%d", f.refreshes())
	}

	// A CI refresh that goes through settles (forgets) the entry.
	g := newBehindFixture(t, "ci-settles", "r1")
	g.mgr.setCurrent("r2", "commit-r2")
	code, resp = postDeployRefresh(t, g.h, `{"user_id":"ci-settles","desktop_id":"primary","target":"deadbeef"}`)
	if len(g.h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("after a CI refresh (%d %+v) pending = %+v", code, resp, g.h.pendingDeployRefreshes())
	}
}

// 8: a hibernated computer's entry is forgotten at the next sweep.
func TestBehindEntryOfAHibernatedComputerIsForgotten(t *testing.T) {
	f := newBehindFixture(t, "hibernated-again", "r1")
	f.mgr.setCurrent("r2", "commit-r2")
	f.setState(VMStateHibernated)
	f.sweep()
	if f.refreshes() != 0 || len(f.fake.marks) != 0 || len(f.h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("hibernated: refreshes=%d marks=%v pending=%+v; want forgotten without contact", f.refreshes(), f.fake.marks, f.h.pendingDeployRefreshes())
	}
}

// 9: an entry that is not woke_behind is unaffected by the gate.
func TestNonBehindEntryIgnoresTheGate(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "plain-entry")
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "0ld" }
	h.SetBehindRefresh(2*time.Minute, time.Now)
	fake.outcome = guestMarkBusy
	postDeployRefresh(t, h, `{"user_id":"plain-entry","desktop_id":"primary","target":"761d94c5"}`)
	setFakeOutcome(fake, guestMarkWritten)
	h.registry.mu.Lock()
	h.registry.ownerships[ownershipKey("plain-entry", "primary")].LastActiveAt = time.Now()
	h.registry.mu.Unlock()
	h.SweepDeployRefreshPending(context.Background())
	if len(mgr.refreshes) != 1 {
		t.Fatalf("a deploy entry waited for the user-idle gate: refreshes=%v", mgr.refreshes)
	}
}

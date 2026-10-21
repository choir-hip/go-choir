package vmctl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmmanager"
)

// A deploy is an external event: it re-arms the one automatic retry of a
// failure-stopped always-on computer, once per deploy target, and reaches
// protected computers that are not active
// (docs/problems/vmctl-failed-protected-computer-waits-a-day-and-deploy-cannot-reach-it-2026-10-21.md).
//
// Ways this could fail, written before the code (the problem record's A and B
// lists are the long form):
//   - a deploy of a new target does not revive a failure-stopped paper, so a
//     rollback waits out the 24-hour clock (A1);
//   - the same target re-arms every time it is posted, a crash loop under one
//     build (A2), or many targets spawn without a failure stop in between (A6);
//   - an ordinary or held computer is re-armed or booted (A3, B3);
//   - the always-on sweep with no deploy gets more than one retry a window
//     (A4);
//   - a refusal that spawned nothing burns the one retry (A5);
//   - the rescue boots over a live guest, or stops one that is working (B1, B2);
//   - a pending rescue is dropped while the guest still lives (B4);
//   - the rescue writes a planned-restart marker, or a failed boot is retried
//     every minute (B6, B7).

const (
	rescueUser = "paper-owner"
	rescueVM   = "vm-paper-rescue"
)

type rescueFixture struct {
	h    *Handler
	reg  *OwnershipRegistry
	mock *mockVMManager
	ctrl *processControlVMManager
	fake *fakeGuestMarker
	now  *time.Time
}

// newRescueFixture builds a registry with one computer owned by rescueUser.
// protected makes it a critical_protected platform computer (the paper).
func newRescueFixture(t *testing.T, protected bool) *rescueFixture {
	t.Helper()
	ctrl := newProcessControlVMManager()
	reg := NewOwnershipRegistry("")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(ctrl)
	now := time.Date(2026, 10, 21, 16, 0, 0, 0, time.UTC)
	reg.clock = func() time.Time { return now }
	reg.guestNetworkProbe = func(string) bool { return true }
	own, err := reg.ResolveOrAssign(rescueUser)
	if err != nil {
		t.Fatal(err)
	}
	cfg := WarmnessPolicyConfig{}
	if protected {
		cfg.PlatformComputerIDs = map[string]bool{own.ComputerID: true}
	}
	reg.SetWarmnessPolicyConfig(cfg)
	h := NewHandler(reg)
	fake := &fakeGuestMarker{outcome: guestMarkWritten}
	h.guestMarker = fake
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "" }
	f := &rescueFixture{h: h, reg: reg, mock: ctrl.mockVMManager, ctrl: ctrl, fake: fake, now: &now}
	f.mock.boots, f.mock.resumes, f.mock.recovers, f.mock.refreshes = nil, nil, nil, nil
	return f
}

// spawns counts every way vmctl can start a guest process.
func (f *rescueFixture) spawns() int {
	f.mock.mu.Lock()
	defer f.mock.mu.Unlock()
	return len(f.mock.boots) + len(f.mock.resumes) + len(f.mock.recovers) + len(f.mock.refreshes)
}

func (f *rescueFixture) own() VMOwnership {
	return *f.reg.GetOwnershipForDesktop(rescueUser, PrimaryDesktopID)
}

// failStop puts the computer where the wedge detector leaves it: stopped, by
// failure, with no process.
func (f *rescueFixture) failStop(stoppedBy string) {
	f.reg.mu.Lock()
	own := f.reg.ownerships[ownershipKey(rescueUser, PrimaryDesktopID)]
	own.State, own.StoppedBy = VMStateStopped, stoppedBy
	f.reg.mu.Unlock()
	f.mock.mu.Lock()
	delete(f.mock.getVMs, own.VMID)
	f.mock.mu.Unlock()
	delete(f.ctrl.live, own.VMID)
}

func (f *rescueFixture) deploy(t *testing.T, target string) (int, deployRefreshResponse) {
	t.Helper()
	return postDeployRefresh(t, f.h, fmt.Sprintf(`{"user_id":%q,"desktop_id":"primary","target":%q}`, rescueUser, target))
}

func TestDeployGrantReArmsTheRetryOncePerTargetForProtectedComputersOnly(t *testing.T) {
	reg, _ := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetWarmnessPolicyConfig(tierConfig())
	recent := reg.now().UTC().Format(time.RFC3339Nano)
	addOwnership(reg, &VMOwnership{VMID: "vm-paper", UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByWedged, FailureWarmedAt: recent})
	addOwnership(reg, &VMOwnership{VMID: "vm-plain", UserID: "plain", ComputerID: "computer-plain", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByWedged, FailureWarmedAt: recent})
	addOwnership(reg, &VMOwnership{VMID: "vm-held", UserID: "held", ComputerID: "computer-held", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, WarmnessClass: WarmnessClassCriticalProtected, State: VMStateStopped, StoppedBy: stoppedByWedged, FailureWarmedAt: recent, HoldStatus: &MaintenanceHold{}})

	if got := warmTargetIDs(reg); len(got) != 0 {
		t.Fatalf("retry spent: warm targets = %v, want none", got)
	}
	if !reg.GrantDeployRetry("vm-paper", "t1") {
		t.Fatal("a new deploy target did not re-arm the protected computer")
	}
	if got := warmTargetIDs(reg); len(got) != 1 || got[0] != "vm-paper" {
		t.Fatalf("after the grant: warm targets = %v, want vm-paper", got)
	}
	// Spent, it stays spent for the same target (A2) ...
	if !reg.recordFailureWarm("vm-paper") {
		t.Fatal("the granted retry could not be spent")
	}
	reg.mu.Lock()
	reg.vmByID["vm-paper"].State, reg.vmByID["vm-paper"].StoppedBy = VMStateStopped, stoppedByWedged
	reg.mu.Unlock()
	if reg.GrantDeployRetry("vm-paper", "t1") {
		t.Fatal("the same target re-armed the retry twice")
	}
	if got := warmTargetIDs(reg); len(got) != 0 {
		t.Fatalf("same target again: warm targets = %v, want none", got)
	}
	// ... and the next target is a new external event.
	if !reg.GrantDeployRetry("vm-paper", "t2") {
		t.Fatal("a second target did not re-arm")
	}
	// An ordinary computer and a held one are never re-armed (A3).
	if reg.GrantDeployRetry("vm-plain", "t1") {
		t.Fatal("an ordinary computer was re-armed")
	}
	if reg.GrantDeployRetry("vm-held", "t1") {
		t.Fatal("a held computer was re-armed")
	}
	if reg.GrantDeployRetry("vm-missing", "t1") || reg.GrantDeployRetry("vm-paper", "  ") {
		t.Fatal("an unknown computer or an empty target re-armed")
	}
}

func TestSweepWithoutADeployStaysBoundedToOneRetryPerWindow(t *testing.T) {
	reg, now := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetWarmnessPolicyConfig(tierConfig())
	addOwnership(reg, &VMOwnership{VMID: "vm-paper", UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByWedged})
	// A deploy target was granted long ago; the grant is not a clock reset.
	reg.GrantDeployRetry("vm-paper", "t1")
	for i := 0; i < 3; i++ {
		if !reg.recordFailureWarm("vm-paper") && i == 0 {
			t.Fatal("first retry refused")
		}
		reg.mu.Lock()
		reg.vmByID["vm-paper"].State, reg.vmByID["vm-paper"].StoppedBy = VMStateStopped, stoppedByWedged
		reg.mu.Unlock()
		*now = now.Add(failureWarmWindow / 4)
		if got := warmTargetIDs(reg); len(got) != 0 && i < 3 {
			t.Fatalf("pass %d: sweep allowed a second retry inside the window: %v", i, got)
		}
	}
	*now = now.Add(failureWarmWindow)
	if got := warmTargetIDs(reg); len(got) != 1 {
		t.Fatalf("after the window: warm targets = %v, want one", got)
	}
}

func TestSelectDeployRefreshListsProtectedFailureStoppedComputersAsRescue(t *testing.T) {
	ownerships := []ownershipResponse{
		{VMID: "vm-wedged", UserID: "a", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: string(WarmnessClassCriticalProtected), StoppedBy: stoppedByWedged},
		{VMID: "vm-failed", UserID: "b", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: string(WarmnessClassPremiumAlwaysOn), StoppedBy: stoppedByRecoveryFailed},
		{VMID: "vm-restart", UserID: "c", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: string(WarmnessClassCriticalProtected), StoppedBy: "vmctl-restart"},
		{VMID: "vm-idle", UserID: "d", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: string(WarmnessClassCriticalProtected), StoppedBy: "idle"},
		{VMID: "vm-plain", UserID: "e", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: string(WarmnessClassPrimary), StoppedBy: stoppedByWedged},
		{VMID: "vm-held", UserID: "f", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: string(WarmnessClassCriticalProtected), StoppedBy: stoppedByWedged, Held: true},
		{VMID: "vm-active", UserID: "g", DesktopID: "primary", State: "active", Kind: VMKindInteractive, WarmnessClass: string(WarmnessClassCriticalProtected)},
	}
	plan := selectDeployRefresh(ownerships)
	rescue, plain := map[string]bool{}, map[string]bool{}
	for _, c := range plan.Candidates {
		if c.Rescue {
			rescue[c.VMID] = true
		} else {
			plain[c.VMID] = true
		}
	}
	for _, want := range []string{"vm-wedged", "vm-failed", "vm-restart"} {
		if !rescue[want] {
			t.Errorf("%s not listed as a rescue candidate: %+v", want, plan.Candidates)
		}
	}
	if len(rescue) != 3 || !plain["vm-active"] || len(plain) != 1 {
		t.Fatalf("candidates = %+v, want three rescue and the active one", plan.Candidates)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].VMID != "vm-held" {
		t.Fatalf("skipped = %+v, want only the held computer", plan.Skipped)
	}
}

func TestDeployBootsAFailureStoppedPaperOncePerTargetAndAsACrashBoot(t *testing.T) {
	f := newRescueFixture(t, true)
	// The retry was spent at 16:45 on the bad release; wedged again at 16:59.
	f.reg.mu.Lock()
	f.reg.ownerships[ownershipKey(rescueUser, PrimaryDesktopID)].FailureWarmedAt = f.now.UTC().Format(time.RFC3339Nano)
	f.reg.mu.Unlock()
	f.failStop(stoppedByWedged)
	*f.now = f.now.Add(14 * time.Minute)
	if got := warmTargetIDs(f.reg); len(got) != 0 {
		t.Fatalf("before the deploy the sweep must stay held: %v", got)
	}

	code, resp := f.deploy(t, "revert-1")
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || resp.PlannedRestart {
		t.Fatalf("deploy of a new target: %d %+v, want 200 refreshed as a crash boot", code, resp)
	}
	if f.spawns() != 1 {
		t.Fatalf("spawns = %d, want exactly one", f.spawns())
	}
	if len(f.fake.marks) != 0 {
		t.Fatalf("the rescue asked the guest for a planned-restart marker: %v", f.fake.marks)
	}
	if own := f.own(); own.State != VMStateActive || own.StoppedBy != "" {
		t.Fatalf("after the rescue: %+v", own)
	}

	// It wedges again under the same build, and the same deploy is posted
	// again (CI re-run, retry sweep, duplicate request): nothing spawns.
	f.failStop(stoppedByWedged)
	for i := 0; i < 5; i++ {
		_, resp = f.deploy(t, "revert-1")
		if resp.Outcome == deployRefreshRefreshed {
			t.Fatalf("post %d of the same target booted again: %+v", i, resp)
		}
	}
	if got := warmTargetIDs(f.reg); len(got) != 0 {
		t.Fatalf("the sweep revived it under the same build: %v", got)
	}
	if f.spawns() != 1 {
		t.Fatalf("spawns = %d after the same target again, want 1", f.spawns())
	}
	// A different build is a new external event.
	if _, resp = f.deploy(t, "revert-2"); resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("the next target did not boot it: %+v", resp)
	}
	if f.spawns() != 2 {
		t.Fatalf("spawns = %d, want 2 for two targets", f.spawns())
	}
}

func TestDeployLoopBoundIsOneSpawnPerTargetEvenWithManyPosts(t *testing.T) {
	f := newRescueFixture(t, true)
	const targets = 6
	for i := 0; i < targets; i++ {
		target := fmt.Sprintf("build-%d", i)
		f.failStop(stoppedByWedged)
		if _, resp := f.deploy(t, target); resp.Outcome != deployRefreshRefreshed {
			t.Fatalf("target %d: %+v, want a boot", i, resp)
		}
		// The build wedges again. Every driver that can ask for a boot asks
		// many times; none may spawn under the same build.
		f.failStop(stoppedByWedged)
		for post := 0; post < 4; post++ {
			f.deploy(t, target)
			f.h.SweepDeployRefreshPending(t.Context())
			f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes)
		}
		if want := i + 1; f.spawns() != want {
			t.Fatalf("after target %d spawns = %d, want %d (one per target)", i, f.spawns(), want)
		}
		// And a day later the backstop allows exactly one sweep retry.
		*f.now = f.now.Add(failureWarmWindow + time.Minute)
		if got := f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes); got != 1 {
			t.Fatalf("after the window: warmed %d, want 1", got)
		}
		f.failStop(stoppedByWedged)
		if got := f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes); got != 0 {
			t.Fatalf("a second sweep retry inside the window warmed %d", got)
		}
		// The sweep's retry also spawned: account for it before the next target.
		f.mock.mu.Lock()
		f.mock.boots = f.mock.boots[:len(f.mock.boots)-1]
		f.mock.mu.Unlock()
	}
}

func TestDeployDoesNotBootAHealthyPaperOrAnOrdinaryComputer(t *testing.T) {
	// A running paper takes the ordinary planned refresh: one refresh, a marker.
	f := newRescueFixture(t, true)
	if _, resp := f.deploy(t, "t1"); resp.Outcome != deployRefreshRefreshed || !resp.PlannedRestart || f.spawns() != 1 || len(f.fake.marks) != 1 {
		t.Fatalf("running paper: %+v spawns=%d marks=%v, want one planned refresh", resp, f.spawns(), f.fake.marks)
	}
	// An ordinary computer that failed is not booted by a deploy (B3).
	g := newRescueFixture(t, false)
	g.failStop(stoppedByWedged)
	if _, resp := g.deploy(t, "t1"); resp.Outcome != deployRefreshSkipped || g.spawns() != 0 {
		t.Fatalf("ordinary failure-stopped computer: %+v spawns=%d, want skipped and none", resp, g.spawns())
	}
	// Stopped by an idle sweep is not a failure; a deploy leaves it to its next start.
	h := newRescueFixture(t, true)
	h.failStop("idle")
	if _, resp := h.deploy(t, "t1"); resp.Outcome != deployRefreshSkipped || h.spawns() != 0 {
		t.Fatalf("idle-stopped paper: %+v spawns=%d, want skipped and none", resp, h.spawns())
	}
}

func TestDeployNeverBootsOverALiveGuestAndNeverStopsOne(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStop("vmctl-restart")
	own := f.own()
	f.ctrl.live[own.VMID] = []int{4242}
	f.ctrl.reattachError = errors.New("route authority unavailable")

	code, resp := f.deploy(t, "revert-1")
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred {
		t.Fatalf("live guest that will not reattach: %d %+v, want 409 deferred", code, resp)
	}
	if f.spawns() != 0 || len(f.ctrl.reaped) != 0 || len(f.mock.stops) != 0 {
		t.Fatalf("spawns=%d reaped=%v stops=%v: a deploy touched a live guest", f.spawns(), f.ctrl.reaped, f.mock.stops)
	}
	if pending := f.h.pendingDeployRefreshes(); len(pending) != 1 {
		t.Fatalf("pending = %+v, want the rescue kept for the sweep", pending)
	}
	// The sweep keeps asking while the guest lives (B4) and spawns nothing.
	for i := 0; i < 3; i++ {
		f.h.SweepDeployRefreshPending(t.Context())
	}
	if f.spawns() != 0 || len(f.ctrl.reaped) != 0 || len(f.h.pendingDeployRefreshes()) != 1 {
		t.Fatalf("sweep: spawns=%d reaped=%v pending=%v", f.spawns(), f.ctrl.reaped, f.h.pendingDeployRefreshes())
	}
	// The wedge detector (evidence, not the deploy) ends the guest; the next
	// sweep boots it on the target, once.
	f.failStop(stoppedByWedged)
	f.h.SweepDeployRefreshPending(t.Context())
	f.h.SweepDeployRefreshPending(t.Context())
	if f.spawns() != 1 || len(f.h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("after the wedge stop: spawns=%d pending=%v, want one boot and the entry forgotten", f.spawns(), f.h.pendingDeployRefreshes())
	}
	if own := f.own(); own.State != VMStateActive {
		t.Fatalf("not booted: %+v", own)
	}
}

func TestDeployAdoptsALiveGuestThatWillReattachInsteadOfBootingIt(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStop("vmctl-restart")
	own := f.own()
	f.ctrl.live[own.VMID] = []int{4242}
	// Reattach works now (the route guard / health recovered): the computer is
	// adopted and takes the ordinary planned refresh, not a second process.
	_, resp := f.deploy(t, "revert-1")
	if f.spawns() > 1 {
		t.Fatalf("spawns = %d: a boot over a live guest", f.spawns())
	}
	if len(f.ctrl.reaped) != 0 {
		t.Fatalf("reaped %v", f.ctrl.reaped)
	}
	if resp.Outcome != deployRefreshRefreshed || !resp.PlannedRestart || len(f.fake.marks) != 1 {
		t.Fatalf("adopted guest: %+v marks=%v, want the planned refresh path", resp, f.fake.marks)
	}
}

func TestDeployRefundsTheRetryWhenTheRefusalSpawnedNothing(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStop(stoppedByWedged)
	f.mock.bootError = fmt.Errorf("%w: vm busy", vmmanager.ErrLiveUnmanaged)
	code, resp := f.deploy(t, "revert-1")
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred {
		t.Fatalf("transient refusal: %d %+v, want deferred", code, resp)
	}
	if own := f.own(); own.FailureWarmedAt != "" {
		t.Fatalf("a refusal that spawned nothing burned the retry: %q", own.FailureWarmedAt)
	}
	// It clears, and the sweep boots it once, without a new deploy.
	f.mock.bootError = nil
	f.h.SweepDeployRefreshPending(t.Context())
	if f.spawns() != 2 { // the refused attempt reached BootVM, the sweep's boot is the second call
		t.Fatalf("boot calls = %d, want the refused one and one success", f.spawns())
	}
	if own := f.own(); own.State != VMStateActive {
		t.Fatalf("not booted after the refusal cleared: %+v", own)
	}
}

func TestDeployDoesNotRetryAFailedRescueEveryMinute(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStop(stoppedByWedged)
	f.mock.bootError = errors.New("kernel panic: guest init failed")
	code, resp := f.deploy(t, "revert-1")
	if code < 400 || resp.Outcome != deployRefreshFailed {
		t.Fatalf("hard boot failure: %d %+v, want failed", code, resp)
	}
	before := f.spawns()
	for i := 0; i < 5; i++ {
		f.h.SweepDeployRefreshPending(t.Context())
		f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes)
		f.deploy(t, "revert-1")
	}
	if f.spawns() != before {
		t.Fatalf("a failed rescue was retried: %d spawns after the first %d", f.spawns(), before)
	}
	if pending := f.h.pendingDeployRefreshes(); len(pending) != 0 {
		t.Fatalf("a failed rescue stays pending: %+v", pending)
	}
}

func TestDeployRescueSkipsHeldProtectedComputer(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStop(stoppedByWedged)
	if err := f.reg.SetHold(f.own().ComputerID, "maintenance", "recovery"); err != nil {
		t.Fatal(err)
	}
	_, resp := f.deploy(t, "revert-1")
	if resp.Outcome != deployRefreshSkipped || !strings.Contains(resp.Reason, "held") || f.spawns() != 0 {
		t.Fatalf("held: %+v spawns=%d, want skipped held", resp, f.spawns())
	}
	if own := f.own(); len(own.DeployGrantTargets) != 0 {
		t.Fatalf("a held computer was granted a retry: %v", own.DeployGrantTargets)
	}
}

// Review findings B1-B3 on group 1 (2026-10-21), written before the fixes.
//
//   - B1: only the last target was remembered, so A, B, A re-armed A; and the
//     store compaction (Target "store-compaction", a maintenance refresh)
//     granted and rescued like a deploy, alternating with real deploys;
//   - B2: a refund restored the value it replaced even when a newer grant had
//     cleared and re-spent the retry since, erasing the newer spend;
//   - B3: a rescue of a computer stopped by vmctl-restart spent nothing, so a
//     hard boot failure left it stoppable again by the same target, and a
//     success left the allowance unspent for a later wedge.

func TestDeployGrantRemembersEveryConsumedTargetNotOnlyTheLast(t *testing.T) {
	reg, _ := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetWarmnessPolicyConfig(tierConfig())
	addOwnership(reg, &VMOwnership{VMID: "vm-paper", UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByWedged})
	for _, target := range []string{"A", "B"} {
		if !reg.GrantDeployRetry("vm-paper", target) {
			t.Fatalf("target %s did not grant", target)
		}
	}
	if reg.GrantDeployRetry("vm-paper", "A") {
		t.Fatal("A, B, A re-armed A: the grant remembers only the last target")
	}
	if reg.GrantDeployRetry("vm-paper", "B") {
		t.Fatal("B re-armed again")
	}
	// The memory is bounded, not unbounded: a long run of other targets is
	// the only way an old one is forgotten.
	for i := 0; i < 20; i++ {
		reg.GrantDeployRetry("vm-paper", fmt.Sprintf("later-%d", i))
	}
	reg.mu.RLock()
	kept := len(reg.vmByID["vm-paper"].DeployGrantTargets)
	reg.mu.RUnlock()
	if kept == 0 || kept > deployGrantTargetsKept {
		t.Fatalf("remembered %d targets, want 1..%d", kept, deployGrantTargetsKept)
	}
}

func TestStoreCompactionNeverGrantsOrRescues(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStop(stoppedByWedged)
	noop := func(string) error { return nil }
	req := deployRefreshRequest{UserID: rescueUser, DesktopID: PrimaryDesktopID, Target: storeCompactTarget}

	// A maintenance refresh of a stopped protected computer is skipped: no
	// grant, no boot.
	resp, _ := f.h.deployRefreshWith(t.Context(), req, noop)
	if resp.Outcome != deployRefreshSkipped || f.spawns() != 0 {
		t.Fatalf("compaction of a failure-stopped paper: %+v spawns=%d, want skipped and none", resp, f.spawns())
	}
	if own := f.own(); len(own.DeployGrantTargets) != 0 || own.FailureWarmedAt != "" {
		t.Fatalf("compaction granted or spent: %+v", own)
	}

	// Compactions between real deploys never re-arm a target already used.
	if _, resp := f.deploy(t, "A"); resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("deploy A: %+v", resp)
	}
	f.failStop(stoppedByWedged)
	f.h.deployRefreshWith(t.Context(), req, noop)
	f.deploy(t, "A")
	f.h.deployRefreshWith(t.Context(), req, noop)
	f.deploy(t, "A")
	if f.spawns() != 1 {
		t.Fatalf("spawns = %d, want 1: compaction alternated with deploy A and re-armed it", f.spawns())
	}
	// A compaction of an active protected computer grants nothing either.
	g := newRescueFixture(t, true)
	g.h.deployRefreshWith(t.Context(), req, noop)
	if own := g.own(); len(own.DeployGrantTargets) != 0 {
		t.Fatalf("compaction of an active computer recorded targets: %v", own.DeployGrantTargets)
	}
}

func TestStaleRefundCannotEraseANewerGrantOrSpend(t *testing.T) {
	reg, _ := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetWarmnessPolicyConfig(tierConfig())
	addOwnership(reg, &VMOwnership{VMID: "vm-paper", UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByWedged})
	failed := func() string {
		reg.mu.RLock()
		defer reg.mu.RUnlock()
		return reg.vmByID["vm-paper"].FailureWarmedAt
	}
	restop := func() {
		reg.mu.Lock()
		reg.vmByID["vm-paper"].State, reg.vmByID["vm-paper"].StoppedBy = VMStateStopped, stoppedByWedged
		reg.mu.Unlock()
	}

	// The warm sweep spends the retry; its start is about to be refused.
	sweepReceipt, ok := reg.spendFailureWarm("vm-paper")
	if !ok {
		t.Fatal("sweep could not spend")
	}
	// Meanwhile a deploy grants, spends and boots.
	if !reg.GrantDeployRetry("vm-paper", "t1") {
		t.Fatal("deploy did not grant")
	}
	if _, ok := reg.spendFailureWarm("vm-paper"); !ok {
		t.Fatal("deploy could not spend its grant")
	}
	spentByDeploy := failed()
	if spentByDeploy == "" {
		t.Fatal("deploy spend not recorded")
	}
	restop() // it wedges again under t1
	// The sweep's refusal now arrives late and refunds.
	reg.refundFailureWarm("vm-paper", sweepReceipt)
	if got := failed(); got != spentByDeploy {
		t.Fatalf("a stale refund restored FailureWarmedAt to %q, erasing the deploy's spend %q", got, spentByDeploy)
	}
	if got := warmTargetIDs(reg); len(got) != 0 {
		t.Fatalf("after the stale refund the sweep may boot under the same build: %v", got)
	}

	// A refund of the latest spend, with nothing since, still works.
	reg.GrantDeployRetry("vm-paper", "t2")
	latest, ok := reg.spendFailureWarm("vm-paper")
	if !ok {
		t.Fatal("t2 spend refused")
	}
	reg.refundFailureWarm("vm-paper", latest)
	if got := failed(); got != "" {
		t.Fatalf("a valid refund left FailureWarmedAt = %q", got)
	}
}

func TestRescueFromVMCTLRestartIsChargedAgainstTheTarget(t *testing.T) {
	// A hard boot failure: the same target must not boot it again, and the
	// sweep must not take over (the failure is a recovery failure now).
	f := newRescueFixture(t, true)
	f.failStop(stoppedByVMCTLRestart)
	f.mock.bootError = errors.New("kernel panic: guest init failed")
	if code, resp := f.deploy(t, "A"); code < 400 || resp.Outcome != deployRefreshFailed {
		t.Fatalf("hard failure: %d %+v, want failed", code, resp)
	}
	f.mock.bootError = nil
	for i := 0; i < 3; i++ {
		f.deploy(t, "A")
		f.h.SweepDeployRefreshPending(t.Context())
		f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes)
	}
	if f.spawns() != 1 {
		t.Fatalf("spawns = %d, want 1: the same target booted a computer whose rescue already failed", f.spawns())
	}
	if _, resp := f.deploy(t, "B"); resp.Outcome != deployRefreshRefreshed || f.spawns() != 2 {
		t.Fatalf("next target: %+v spawns=%d, want a boot", resp, f.spawns())
	}

	// A success leaves the allowance spent: the build that rescued it does
	// not boot it again after a later wedge.
	g := newRescueFixture(t, true)
	g.failStop(stoppedByVMCTLRestart)
	if _, resp := g.deploy(t, "A"); resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("rescue from vmctl-restart: %+v", resp)
	}
	g.failStop(stoppedByWedged)
	for i := 0; i < 3; i++ {
		g.deploy(t, "A")
		g.h.SweepDeployRefreshPending(t.Context())
		g.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes)
	}
	if g.spawns() != 1 {
		t.Fatalf("spawns = %d, want 1: the rescued build booted the wedged computer again", g.spawns())
	}
}

func TestAdoptionWithoutASpawnSpendsNothing(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStop(stoppedByVMCTLRestart)
	f.ctrl.live[f.own().VMID] = []int{4242}
	f.deploy(t, "A")
	if own := f.own(); own.FailureWarmedAt != "" {
		t.Fatalf("adopting a live guest spent the retry: %q", own.FailureWarmedAt)
	}
}

package vmctl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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
	var issued atomic.Int64
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"raw_token":"gw-token-%d"}`, issued.Add(1))
	}))
	t.Cleanup(gateway.Close)
	reg.SetGatewayURL(gateway.URL)
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

// failStop puts the computer where the wedge detector leaves it: stopped by
// a failure, with the stopped instance still tracked by the manager, as
// mgr.StopVM really leaves it (stopWedgedManaged): a rescue without a vmctl
// restart in between finds the instance and must not recover it with its old
// config.
func (f *rescueFixture) failStop(stoppedBy string) {
	f.reg.mu.Lock()
	own := f.reg.ownerships[ownershipKey(rescueUser, PrimaryDesktopID)]
	own.State, own.StoppedBy = VMStateStopped, stoppedBy
	vmID := own.VMID
	f.reg.mu.Unlock()
	f.mock.mu.Lock()
	f.mock.getVMs[vmID] = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", State: "stopped"}
	f.mock.mu.Unlock()
	delete(f.ctrl.live, vmID)
}

// failStopUntracked is the stop after a vmctl restart: the manager lost the
// instance, so a start goes through BootVM.
func (f *rescueFixture) failStopUntracked(stoppedBy string) {
	f.failStop(stoppedBy)
	f.mock.mu.Lock()
	delete(f.mock.getVMs, f.own().VMID)
	f.mock.mu.Unlock()
}

// failBoots makes every way of starting a guest fail with err (nil clears).
func (f *rescueFixture) failBoots(err error) {
	f.mock.mu.Lock()
	f.mock.bootError, f.mock.refreshError, f.mock.recoverError = err, err, err
	f.mock.mu.Unlock()
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
	want := 0 // every spawn the bound allows so far
	for i := 0; i < targets; i++ {
		target := fmt.Sprintf("build-%d", i)
		f.failStop(stoppedByWedged)
		if _, resp := f.deploy(t, target); resp.Outcome != deployRefreshRefreshed {
			t.Fatalf("target %d: %+v, want a boot", i, resp)
		}
		want++
		// The build wedges again. Every driver that can ask for a boot asks
		// many times; none may spawn under the same build.
		f.failStop(stoppedByWedged)
		for post := 0; post < 4; post++ {
			f.deploy(t, target)
			f.h.SweepDeployRefreshPending(t.Context())
			f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes)
		}
		if f.spawns() != want {
			t.Fatalf("after target %d spawns = %d, want %d (one per target)", i, f.spawns(), want)
		}
		// And a day later the backstop allows exactly one sweep retry.
		*f.now = f.now.Add(failureWarmWindow + time.Minute)
		if got := f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes); got != 1 {
			t.Fatalf("after the window: warmed %d, want 1", got)
		}
		want++
		f.failStop(stoppedByWedged)
		if got := f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes); got != 0 {
			t.Fatalf("a second sweep retry inside the window warmed %d", got)
		}
		if f.spawns() != want {
			t.Fatalf("after the backstop spawns = %d, want %d", f.spawns(), want)
		}
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
	f.failStopUntracked("vmctl-restart")
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
	f.failStopUntracked("vmctl-restart")
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
	f.failBoots(fmt.Errorf("%w: vm busy", vmmanager.ErrLiveUnmanaged))
	code, resp := f.deploy(t, "revert-1")
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred {
		t.Fatalf("transient refusal: %d %+v, want deferred", code, resp)
	}
	if own := f.own(); own.FailureWarmedAt != "" {
		t.Fatalf("a refusal that spawned nothing burned the retry: %q", own.FailureWarmedAt)
	}
	// It clears, and the sweep boots it once, without a new deploy.
	f.failBoots(nil)
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
	f.failBoots(errors.New("kernel panic: guest init failed"))
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
	if own := f.own(); len(own.DeployGrants) != 0 {
		t.Fatalf("a held computer was granted a retry: %v", own.DeployGrants)
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
	reg, now := hostCapacityRegistry(t, newProcessControlVMManager())
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
	// The memory is bounded, not unbounded: inside one window a grant past
	// the cap is refused (an old target is never forgotten early), and past
	// the window the old targets go.
	for i := 0; i < 2*deployGrantsCap; i++ {
		reg.GrantDeployRetry("vm-paper", fmt.Sprintf("later-%d", i))
	}
	kept := func() int {
		reg.mu.RLock()
		defer reg.mu.RUnlock()
		return len(reg.vmByID["vm-paper"].DeployGrants)
	}
	if kept() != deployGrantsCap {
		t.Fatalf("remembered %d targets, want the cap %d", kept(), deployGrantsCap)
	}
	if reg.GrantDeployRetry("vm-paper", "A") {
		t.Fatal("A re-armed once the cap was reached")
	}
	*now = now.Add(failureWarmWindow)
	if !reg.GrantDeployRetry("vm-paper", "after-the-window") || kept() != 1 {
		t.Fatalf("after the window: remembered %d targets, want only the new one", kept())
	}
}

func TestOfflineMaintenanceNeverGrantsOrRescues(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStop(stoppedByWedged)
	noop := func(string) error { return nil }
	req := deployRefreshRequest{UserID: rescueUser, DesktopID: PrimaryDesktopID, Target: "offline-maintenance"}

	// A maintenance refresh of a stopped protected computer is skipped: no
	// grant, no boot.
	resp, _ := f.h.deployRefreshWith(t.Context(), req, noop)
	if resp.Outcome != deployRefreshSkipped || f.spawns() != 0 {
		t.Fatalf("compaction of a failure-stopped paper: %+v spawns=%d, want skipped and none", resp, f.spawns())
	}
	if own := f.own(); len(own.DeployGrants) != 0 || own.FailureWarmedAt != "" {
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
	if own := g.own(); len(own.DeployGrants) != 0 {
		t.Fatalf("compaction of an active computer recorded targets: %v", own.DeployGrants)
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
	f.failStopUntracked(stoppedByVMCTLRestart)
	f.failBoots(errors.New("kernel panic: guest init failed"))
	if code, resp := f.deploy(t, "A"); code < 400 || resp.Outcome != deployRefreshFailed {
		t.Fatalf("hard failure: %d %+v, want failed", code, resp)
	}
	f.failBoots(nil)
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
	g.failStopUntracked(stoppedByVMCTLRestart)
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
	f.failStopUntracked(stoppedByVMCTLRestart)
	f.ctrl.live[f.own().VMID] = []int{4242}
	f.deploy(t, "A")
	if own := f.own(); own.FailureWarmedAt != "" {
		t.Fatalf("adopting a live guest spent the retry: %q", own.FailureWarmedAt)
	}
}

// Review findings A1-A3 (second reviewer, 2026-10-21), written before the
// fixes.
//
//   - A1: a spend succeeded for a computer that was no longer stopped by a
//     failure (the caller's snapshot was stale), then its refund erased a
//     spend someone else made;
//   - A2: nothing marked the computer as starting while a resume was blocked
//     in the manager, so a rescue saw "stopped", went to RecoverVM, killed the
//     healthy first boot and booted again; and a rescue acting on a stale
//     snapshot of a computer that had reattached reported "refreshed" for the
//     old image;
//   - A3: after the wedge detector's StopVM the manager still tracks the
//     stopped instance, and the rescue recovered it with the config the hung
//     boot used (old kernel, initrd, store, params) and no gateway token.

func TestSpendRefusesUnlessTheRegistryEntryIsStoppedByAFailureNow(t *testing.T) {
	reg, _ := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetWarmnessPolicyConfig(tierConfig())
	addOwnership(reg, &VMOwnership{VMID: "vm-paper", UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByWedged})
	set := func(state VMState, stoppedBy string) {
		reg.mu.Lock()
		own := reg.vmByID["vm-paper"]
		own.State, own.StoppedBy = state, stoppedBy
		reg.mu.Unlock()
	}
	warmedAt := func() string {
		reg.mu.RLock()
		defer reg.mu.RUnlock()
		return reg.vmByID["vm-paper"].FailureWarmedAt
	}
	// A rescue charges only while it holds the computer's claim (round 3).
	key := ownershipKey("paper-owner", PrimaryDesktopID)
	rescueSpend := func() (failureWarmReceipt, bool) {
		reg.mu.Lock()
		reg.lifecycle[key] = lifecycleRescue
		reg.mu.Unlock()
		defer reg.releaseLifecycle(key, lifecycleRescue)
		return reg.spendDeployRetry("vm-paper")
	}
	if _, ok := reg.spendDeployRetry("vm-paper"); ok {
		t.Fatal("a rescue spend succeeded without the rescue claim")
	}

	// The caller read "stopped by wedged"; by the time it spends, the
	// computer is running again.
	set(VMStateActive, "")
	if _, ok := reg.spendFailureWarm("vm-paper"); ok {
		t.Fatal("the sweep's spend succeeded for an active computer")
	}
	if _, ok := rescueSpend(); ok {
		t.Fatal("the rescue's spend succeeded for an active computer")
	}
	// A stop label left behind on a computer that is not stopped does not
	// qualify it.
	for _, state := range []VMState{VMStateActive, VMStateBooting, VMStateHibernated} {
		set(state, stoppedByWedged)
		if _, ok := reg.spendFailureWarm("vm-paper"); ok {
			t.Fatalf("the sweep's spend succeeded for state %s with a wedged label", state)
		}
		if _, ok := rescueSpend(); ok {
			t.Fatalf("the rescue's spend succeeded for state %s with a wedged label", state)
		}
	}
	set(VMStateStopped, "idle")
	if _, ok := rescueSpend(); ok {
		t.Fatal("a rescue spend succeeded for an idle stop")
	}
	set(VMStateHibernated, "")
	if _, ok := rescueSpend(); ok {
		t.Fatal("a rescue spend succeeded for a hibernated computer")
	}
	if warmedAt() != "" {
		t.Fatalf("a refused spend wrote FailureWarmedAt = %q", warmedAt())
	}
	// The sweep never charges a vmctl-restart stop; the rescue does.
	set(VMStateStopped, stoppedByVMCTLRestart)
	if _, ok := reg.spendFailureWarm("vm-paper"); ok {
		t.Fatal("the sweep's spend succeeded for a vmctl-restart stop")
	}
	if _, ok := rescueSpend(); !ok {
		t.Fatal("the rescue's spend refused a vmctl-restart stop")
	}
	// A failed computer is the sweep's to skip and the rescue's to charge
	// (round 3, R3-P1c).
	reg.mu.Lock()
	reg.vmByID["vm-paper"].FailureWarmedAt = ""
	reg.mu.Unlock()
	set(VMStateFailed, "")
	if _, ok := reg.spendFailureWarm("vm-paper"); ok {
		t.Fatal("the sweep's spend succeeded for a failed computer")
	}
	if _, ok := rescueSpend(); !ok {
		t.Fatal("the rescue's spend refused a failed computer")
	}
	// A refused spend leaves nothing for a stale refund to erase.
	set(VMStateStopped, stoppedByWedged)
	reg.mu.Lock()
	reg.vmByID["vm-paper"].FailureWarmedAt = ""
	reg.mu.Unlock()
	rec, ok := reg.spendFailureWarm("vm-paper")
	if !ok {
		t.Fatal("a fresh failure stop could not be charged")
	}
	set(VMStateActive, "")
	if _, ok := reg.spendFailureWarm("vm-paper"); ok {
		t.Fatal("a second spend of an active computer succeeded")
	}
	reg.refundFailureWarm("vm-paper", rec)
	if warmedAt() != "" {
		t.Fatalf("the refund of the only spend left %q", warmedAt())
	}
}

func TestRescueDefersWhileAWarmSweepBootIsInFlight(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStopUntracked(stoppedByWedged)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.mock.bootHook = func(VMManagerConfig) {
		once.Do(func() { close(started) })
		<-release
	}
	sweepDone := make(chan struct{})
	go func() {
		defer close(sweepDone)
		f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("the sweep never reached BootVM")
	}

	// The registry still says stopped while the sweep's boot is blocked in the
	// manager. A rescue now must not start a second boot or recover the first.
	type result struct {
		code int
		resp deployRefreshResponse
	}
	answered := make(chan result, 1)
	go func() {
		code, resp := f.deploy(t, "A")
		answered <- result{code, resp}
	}()
	var got result
	select {
	case got = <-answered:
	case <-time.After(3 * time.Second):
		close(release)
		<-sweepDone
		t.Fatal("the rescue blocked behind the sweep's boot instead of deferring")
	}
	if got.resp.Outcome != deployRefreshDeferred {
		close(release)
		<-sweepDone
		t.Fatalf("rescue during an in-flight start: %d %+v, want deferred", got.code, got.resp)
	}
	if n := f.spawns(); n != 1 {
		close(release)
		<-sweepDone
		t.Fatalf("spawns = %d during the in-flight start, want only the sweep's boot", n)
	}
	close(release)
	<-sweepDone
	if own := f.own(); own.State != VMStateActive {
		t.Fatalf("the sweep's boot did not complete: %+v", own)
	}
	// The pending rescue finds the computer running and takes the ordinary
	// planned refresh, not a kill-and-reboot without a marker.
	f.h.SweepDeployRefreshPending(t.Context())
	if len(f.mock.recovers) != 0 {
		t.Fatalf("recovers = %v: a healthy first boot was killed", f.mock.recovers)
	}
}

func TestRescueOnAStaleSnapshotOfAReattachedComputerTakesThePlannedRefresh(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStopUntracked(stoppedByVMCTLRestart)
	stale := f.own()
	// A reattach lands between the deploy's snapshot and the rescue.
	f.reg.mu.Lock()
	cur := f.reg.ownerships[ownershipKey(rescueUser, PrimaryDesktopID)]
	cur.State, cur.StoppedBy = VMStateActive, ""
	f.reg.mu.Unlock()
	reply := func(outcome, reason string, status int) (deployRefreshResponse, int) {
		return deployRefreshResponse{Outcome: outcome, Reason: reason}, status
	}
	resp, _, done := f.h.deployRescue(t.Context(), deployRefreshRequest{UserID: rescueUser, DesktopID: PrimaryDesktopID, Target: "A"}, stale, reply)
	if done {
		t.Fatalf("a rescue of a reattached computer answered %+v itself instead of handing it to the planned refresh", resp)
	}
	if f.spawns() != 0 {
		t.Fatalf("spawns = %d: the rescue resumed an already active computer", f.spawns())
	}
	// Through the whole deploy path the computer takes the marker refresh.
	if _, resp := f.deploy(t, "B"); resp.Outcome != deployRefreshRefreshed || !resp.PlannedRestart || len(f.fake.marks) != 1 {
		t.Fatalf("deploy of an active computer: %+v marks=%v, want the planned refresh", resp, f.fake.marks)
	}

	// A refresh already in flight defers the rescue.
	g := newRescueFixture(t, true)
	g.failStop(stoppedByWedged)
	g.reg.mu.Lock()
	g.reg.lifecycle[ownershipKey(rescueUser, PrimaryDesktopID)] = lifecycleRefresh
	g.reg.mu.Unlock()
	if _, resp := g.deploy(t, "A"); resp.Outcome != deployRefreshDeferred || g.spawns() != 0 {
		t.Fatalf("rescue during an in-flight refresh: %+v spawns=%d, want deferred and none", resp, g.spawns())
	}
}

func TestRescueOfATrackedStoppedInstanceBootsTheCurrentDeployWithAFreshToken(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStop(stoppedByWedged) // the wedge detector's StopVM keeps the instance tracked
	if _, resp := f.deploy(t, "A"); resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("rescue: %+v", resp)
	}
	f.mock.mu.Lock()
	recovers, refreshes, cfgs := len(f.mock.recovers), len(f.mock.refreshes), append([]VMManagerConfig(nil), f.mock.refreshCfgs...)
	f.mock.mu.Unlock()
	if recovers != 0 {
		t.Fatalf("recovers = %d: the rescue recovered the stopped instance with the config the hung boot used", recovers)
	}
	if refreshes != 1 {
		t.Fatalf("refreshes = %d, want one refresh onto the current deploy's boot artifacts", refreshes)
	}
	if !strings.HasPrefix(cfgs[0].GatewayToken, "gw-token-") {
		t.Fatalf("gateway token = %q, want a freshly issued one", cfgs[0].GatewayToken)
	}
	// And after a vmctl restart (nothing tracked) it boots with a fresh token too.
	g := newRescueFixture(t, true)
	g.failStopUntracked(stoppedByVMCTLRestart)
	if _, resp := g.deploy(t, "A"); resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("untracked rescue: %+v", resp)
	}
	g.mock.mu.Lock()
	boots := append([]VMManagerConfig(nil), g.mock.boots...)
	g.mock.mu.Unlock()
	if len(boots) != 1 || !strings.HasPrefix(boots[0].GatewayToken, "gw-token-") {
		t.Fatalf("untracked boots = %+v, want one with a fresh gateway token", boots)
	}
}

// Review round 3 (GPT-6.1-sol, 2026-10-21), written before the fix. The
// common cause is that no single claim covers every start path
// (docs/problems/vmctl-failed-protected-computer-waits-a-day-and-deploy-cannot-reach-it-2026-10-21.md,
// "Review round 3"):
//
//   - R3-P1a: a visit's resolve boots a stopped computer and a rescue, which
//     does not see it, boots over it; an ordinary deploy refresh does the same
//     to a visit's readiness boot of an active computer;
//   - R3-P1b: the warm sweep boots on a stale "not a failure" snapshot past
//     an allowance a rescue spent, and spends while a rescue holds the
//     computer, so the rescue answers "spent" and the deploy loses it;
//   - R3-P1c: a protected computer its own failed deploy refresh left failed
//     is never reached by the next (corrective) deploy;
//   - R3-P2: a consumed target is forgotten inside the retry window once
//     eight others were consumed after it.

// liveHookManager runs hook on every process scan, which a rescue makes after
// it claims the computer and before it charges the allowance.
type liveHookManager struct {
	*processControlVMManager
	hook func()
}

func (m *liveHookManager) LiveFirecrackerVMs() map[string][]int {
	if m.hook != nil {
		m.hook()
	}
	return m.processControlVMManager.LiveFirecrackerVMs()
}

// blockFirstBoot holds the first BootVM until release is closed.
func (f *rescueFixture) blockFirstBoot() (started, release chan struct{}) {
	started, release = make(chan struct{}), make(chan struct{})
	var first atomic.Bool
	f.mock.bootHook = func(VMManagerConfig) {
		if first.CompareAndSwap(false, true) {
			close(started)
			<-release
		}
	}
	return started, release
}

func TestRescueDefersWhileAVisitBootsTheStoppedComputer(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStopUntracked(stoppedByWedged)
	started, release := f.blockFirstBoot()
	visitDone := make(chan error, 1)
	go func() {
		_, err := f.reg.ResolveOrAssignDesktopContext(context.Background(), rescueUser, PrimaryDesktopID)
		visitDone <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("the visit never reached BootVM")
	}
	answered := make(chan deployRefreshResponse, 1)
	go func() {
		_, resp := f.deploy(t, "A")
		answered <- resp
	}()
	var got deployRefreshResponse
	select {
	case got = <-answered:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("the rescue blocked behind the visit's boot instead of deferring")
	}
	spawned := f.spawns()
	close(release)
	if err := <-visitDone; err != nil {
		t.Fatalf("visit: %v", err)
	}
	if got.Outcome != deployRefreshDeferred || spawned != 1 {
		t.Fatalf("rescue during a visit's boot: %+v spawns=%d, want deferred and only the visit's boot", got, spawned)
	}
	if own := f.own(); own.State != VMStateActive {
		t.Fatalf("the visit's boot did not publish: %+v", own)
	}
}

func TestDeployRefreshDefersWhileAVisitBootsAnActiveComputer(t *testing.T) {
	f := newRescueFixture(t, true)
	// The registry says active, the manager says stopped: a visit's readiness
	// check starts it (startExistingVM boots the tracked instance).
	f.mock.mu.Lock()
	f.mock.getVMs[f.own().VMID] = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", State: "stopped"}
	f.mock.refreshStarted, f.mock.refreshRelease = make(chan struct{}), make(chan struct{})
	started, release := f.mock.refreshStarted, f.mock.refreshRelease
	f.mock.mu.Unlock()
	visitDone := make(chan error, 1)
	go func() {
		_, err := f.reg.ResolveOrAssignDesktopContext(context.Background(), rescueUser, PrimaryDesktopID)
		visitDone <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("the visit never reached the manager")
	}
	answered := make(chan deployRefreshResponse, 1)
	go func() {
		_, resp := f.deploy(t, "A")
		answered <- resp
	}()
	var got deployRefreshResponse
	select {
	case got = <-answered:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("the deploy refresh blocked behind the visit's boot")
	}
	spawned := f.spawns()
	close(release)
	<-visitDone
	if got.Outcome != deployRefreshDeferred || spawned != 1 {
		t.Fatalf("deploy refresh during a visit's boot: %+v spawns=%d, want deferred and only the visit's boot", got, spawned)
	}
	if pending := f.h.pendingDeployRefreshes(); len(pending) != 1 {
		t.Fatalf("pending = %+v, want the deferred refresh kept for the retry sweep", pending)
	}
}

func TestSweepOnAStaleSnapshotCannotBootPastAnAllowanceARescueSpent(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStopUntracked(stoppedByVMCTLRestart)
	inGuard, release := make(chan struct{}), make(chan struct{})
	var first atomic.Bool
	guard := func(context.Context, string, string) error {
		if first.CompareAndSwap(false, true) {
			close(inGuard)
			<-release
		}
		return nil
	}
	sweepDone := make(chan struct{})
	go func() {
		defer close(sweepDone)
		f.reg.WarmAlwaysOnDesktops(context.Background(), guard)
	}()
	select {
	case <-inGuard:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("the sweep never reached its route guard")
	}
	// The sweep listed a vmctl-restart stop (no charge). Meanwhile a deploy
	// rescue spends the allowance, its boot fails hard, and the stop becomes a
	// recovery failure.
	f.failBoots(errors.New("kernel panic: guest init failed"))
	if _, resp := f.deploy(t, "A"); resp.Outcome != deployRefreshFailed {
		close(release)
		t.Fatalf("rescue: %+v, want failed", resp)
	}
	if own := f.own(); own.StoppedBy != stoppedByRecoveryFailed || own.FailureWarmedAt == "" {
		close(release)
		t.Fatalf("after the hard failure: %+v, want recovery_failed with the allowance spent", own)
	}
	f.failBoots(nil)
	close(release)
	<-sweepDone
	if n := f.spawns(); n != 1 {
		t.Fatalf("spawns = %d, want 1: the sweep booted on its stale snapshot past the spent allowance", n)
	}
}

func TestNoSpendWhileARescueHoldsTheComputer(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failStopUntracked(stoppedByWedged)
	vmID := f.own().VMID
	inClaim, release := make(chan struct{}), make(chan struct{})
	var first atomic.Bool
	f.reg.SetVMManager(&liveHookManager{processControlVMManager: f.ctrl, hook: func() {
		if first.CompareAndSwap(false, true) {
			close(inClaim)
			<-release
		}
	}})
	answered := make(chan deployRefreshResponse, 1)
	go func() {
		_, resp := f.deploy(t, "A")
		answered <- resp
	}()
	select {
	case <-inClaim:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("the rescue never scanned for a live process")
	}
	// The rescue holds the computer and has not charged yet: neither a spend
	// nor the sweep may take its allowance or boot.
	_, spent := f.reg.spendFailureWarm(vmID)
	f.reg.WarmAlwaysOnDesktops(context.Background(), allowAllRoutes)
	spawned := f.spawns()
	close(release)
	resp := <-answered
	if spent {
		t.Fatal("the sweep's spend succeeded while a rescue held the computer")
	}
	if spawned != 0 {
		t.Fatalf("spawns = %d while the rescue held the computer, want 0", spawned)
	}
	if resp.Outcome != deployRefreshRefreshed || f.spawns() != 1 {
		t.Fatalf("rescue: %+v spawns=%d, want refreshed with one boot", resp, f.spawns())
	}
}

func TestDeployRescuesAProtectedComputerItsOwnFailedRefreshLeftFailed(t *testing.T) {
	f := newRescueFixture(t, true)
	f.failBoots(errors.New("guest did not become ready"))
	if _, resp := f.deploy(t, "A"); resp.Outcome != deployRefreshFailed {
		t.Fatalf("deploy A: %+v, want failed", resp)
	}
	if own := f.own(); own.State != VMStateFailed {
		t.Fatalf("after the failed refresh: %+v, want failed", own)
	}
	f.failBoots(nil)
	ownerships, err := f.h.listOwnershipResponses(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	plan := selectDeployRefresh(ownerships)
	if len(plan.Candidates) != 1 || !plan.Candidates[0].Rescue {
		t.Fatalf("selection of a failed protected computer: %+v, want one rescue candidate", plan)
	}
	// The corrective deploy boots it, once.
	if _, resp := f.deploy(t, "B"); resp.Outcome != deployRefreshRefreshed || f.spawns() != 2 {
		t.Fatalf("corrective deploy B: %+v spawns=%d, want refreshed (A's refresh and B's rescue)", resp, f.spawns())
	}
	if own := f.own(); own.State != VMStateActive {
		t.Fatalf("after the rescue: %+v", own)
	}
	// Failed again under B: B, A, the sweep and the warmer do not boot it.
	f.reg.mu.Lock()
	f.reg.ownerships[ownershipKey(rescueUser, PrimaryDesktopID)].State = VMStateFailed
	f.reg.mu.Unlock()
	for i := 0; i < 3; i++ {
		f.deploy(t, "B")
		f.deploy(t, "A")
		f.h.SweepDeployRefreshPending(t.Context())
		f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes)
	}
	if n := f.spawns(); n != 2 {
		t.Fatalf("spawns = %d, want 2: a consumed target booted the failed computer again", n)
	}
}

func TestDeployNeverBootsOverALiveTrackedGuestOfAFailedComputer(t *testing.T) {
	f := newRescueFixture(t, true)
	f.reg.mu.Lock()
	f.reg.ownerships[ownershipKey(rescueUser, PrimaryDesktopID)].State = VMStateFailed
	f.reg.mu.Unlock()
	f.mock.mu.Lock()
	f.mock.getVMs[f.own().VMID] = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", State: "running"}
	f.mock.mu.Unlock()
	if _, resp := f.deploy(t, "A"); resp.Outcome != deployRefreshDeferred || f.spawns() != 0 {
		t.Fatalf("rescue of a failed computer whose guest still runs: %+v spawns=%d, want deferred and none", resp, f.spawns())
	}
	if own := f.own(); own.FailureWarmedAt != "" {
		t.Fatalf("a deferral spent the allowance: %q", own.FailureWarmedAt)
	}
	// A held failed computer is skipped.
	g := newRescueFixture(t, true)
	g.reg.mu.Lock()
	cur := g.reg.ownerships[ownershipKey(rescueUser, PrimaryDesktopID)]
	cur.State, cur.HoldStatus = VMStateFailed, &MaintenanceHold{}
	g.reg.mu.Unlock()
	if _, resp := g.deploy(t, "A"); resp.Outcome != deployRefreshSkipped || g.spawns() != 0 {
		t.Fatalf("held failed computer: %+v spawns=%d, want skipped and none", resp, g.spawns())
	}
}

func TestConsumedTargetIsRememberedForTheWholeRetryWindow(t *testing.T) {
	reg, now := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetWarmnessPolicyConfig(tierConfig())
	addOwnership(reg, &VMOwnership{VMID: "vm-paper", UserID: "paper-owner", ComputerID: paperComputerID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped, StoppedBy: stoppedByWedged})
	for _, target := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"} {
		if !reg.GrantDeployRetry("vm-paper", target) {
			t.Fatalf("target %s did not grant", target)
		}
		if !reg.recordFailureWarm("vm-paper") {
			t.Fatalf("target %s: the granted retry could not be spent", target)
		}
		reg.mu.Lock()
		reg.vmByID["vm-paper"].State, reg.vmByID["vm-paper"].StoppedBy = VMStateStopped, stoppedByWedged
		reg.mu.Unlock()
		*now = now.Add(time.Hour)
	}
	if reg.GrantDeployRetry("vm-paper", "A") {
		t.Fatal("A re-armed inside the retry window after eight other targets were consumed")
	}
}

// Second reviewer (Gemini 4 Argon) on bcdb74659, written before the fix:
//
//   - the deploy's grant clears the allowance before it defers; when the wedge
//     detector then stops the guest, the idle sweeper's always-on warm (which
//     runs before the retry sweep) spends that retry and recovers the stopped
//     instance with RecoverVM: the stored config of the hung build and no
//     gateway token. A stopped instance the manager still tracks must boot
//     onto the current deploy (RefreshVM) with a fresh token, from every start
//     path, not only the rescue;
//   - a stopped tracked instance holds no memory, so booting it adds a VM:
//     the rescue and the sweep skipped host memory admission for it, and
//     other wakes did not see the reservation.

func (f *rescueFixture) bootCalls() (recovers int, refreshCfgs []VMManagerConfig) {
	f.mock.mu.Lock()
	defer f.mock.mu.Unlock()
	return len(f.mock.recovers), append([]VMManagerConfig(nil), f.mock.refreshCfgs...)
}

func TestSweepAfterADeferredDeployBootsTheCurrentDeployNotTheHungConfig(t *testing.T) {
	f := newRescueFixture(t, true)
	// The rollback reaches the paper while its guest refuses connections: no
	// answer to the marker request, so the deploy defers after its grant.
	f.fake.mu.Lock()
	f.fake.outcome = guestMarkUnavailable
	f.fake.mu.Unlock()
	if _, resp := f.deploy(t, "T-good"); resp.Outcome != deployRefreshDeferred {
		t.Fatalf("deploy to an unanswering guest: %+v, want deferred", resp)
	}
	// The wedge detector stops it and the manager keeps the stopped instance;
	// the idle sweeper's always-on warm runs next.
	f.failStop(stoppedByWedged)
	if got := f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes); got != 1 {
		t.Fatalf("warmed = %d, want the one retry", got)
	}
	recovers, refreshes := f.bootCalls()
	if recovers != 0 {
		t.Fatalf("recovers = %d: the sweep booted the hung build's stored config", recovers)
	}
	if len(refreshes) != 1 || !strings.HasPrefix(refreshes[0].GatewayToken, "gw-token-") {
		t.Fatalf("refreshes = %+v, want one boot onto the current deploy with a fresh gateway token", refreshes)
	}

	// A visit's start of a stopped tracked instance boots the same way.
	g := newRescueFixture(t, true)
	g.failStop(stoppedByWedged)
	if _, err := g.reg.ResolveOrAssignDesktopContext(t.Context(), rescueUser, PrimaryDesktopID); err != nil {
		t.Fatal(err)
	}
	recovers, refreshes = g.bootCalls()
	if recovers != 0 || len(refreshes) != 1 || !strings.HasPrefix(refreshes[0].GatewayToken, "gw-token-") {
		t.Fatalf("visit start: recovers=%d refreshes=%+v, want one refresh with a fresh token", recovers, refreshes)
	}
}

func TestStartsOfAStoppedTrackedInstanceAreAdmittedForHostMemory(t *testing.T) {
	for _, path := range []string{"rescue", "sweep"} {
		f := newRescueFixture(t, true)
		f.reg.SetPressureReclaimConfig(PressureReclaimConfig{
			Mode: PressureReclaimModeActive, Signal: PressureReclaimSignalSustained, MinMemoryAvailableBytes: 4 * gib,
		})
		f.reg.SetHostAdmissionConfig(measuredAdmission())
		f.reg.setPressureSamplerForTest(func(PressureReclaimConfig) HostPressureSample {
			return HostPressureSample{MemoryTotalBytes: 32 * gib, MemoryAvailableBytes: 16 * gib, MemoryAvailablePercent: 50}
		})
		f.failStop(stoppedByWedged)
		vmID := f.own().VMID
		switch path {
		case "rescue":
			if _, resp := f.deploy(t, "A"); resp.Outcome != deployRefreshRefreshed {
				t.Fatalf("rescue: %+v", resp)
			}
		case "sweep":
			if got := f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes); got != 1 {
				t.Fatalf("sweep warmed %d", got)
			}
		}
		admitted := false
		for _, d := range f.reg.HostAdmissionSummary().Recent {
			if d.VMID == vmID {
				admitted = true
			}
		}
		if !admitted {
			t.Fatalf("%s booted a stopped tracked instance without a host memory admission", path)
		}
	}
}

package vmctl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A start that a vmctl restart interrupts
// (docs/problems/wake-during-deploy-vmctl-restart-orphans-the-vm-2026-10-22.md).
// On 2026-10-22 a wake of a hibernated test computer was mid-boot when the
// deploy restarted vmctl. The row still read hibernated, the new vmctl had
// nothing to reattach, the Firecracker process ran outside the registry for
// seven minutes, and the wedge detector, probing the hibernated
// realization's address, recorded it as the computer's own wedge.
//
// The new vmctl must either adopt the process at the address its boot was
// given, or end it and record a fate that names the restart. CLAUDE.md's
// restart rule (a crash restart never resumes work) argues for the second;
// these tests accept either and pin what neither may do.
//
// Failure modes pinned here:
//   - a start in flight at a vmctl restart leaves its process running
//     outside the registry (nothing durable to reattach);
//   - the restart adopts the process at the row's stale address (the
//     hibernated realization's, or a new computer's placeholder URL);
//   - the orphan is ended by the wedge detector and recorded as the
//     computer's failure ("wedged"), which spends an always-on retry;
//   - the fate is not durable: a reload of the ownership file forgets it;
//   - a close that could not end the process is forgotten by the next
//     restart, or the process is wedged meanwhile;
//   - the next owner act adopts the orphan instead of booting fresh;
//   - a published start, or a failed one, leaves a record behind that makes
//     a later restart close a running computer or relabel a stopped one.
//   Every start path is covered: a visit's wake of a hibernated computer
//   (the incident's cold boot, and a resume from its memory snapshot), a
//   resume, a deploy refresh of an active computer, a recovery of a failed
//   one, and a new computer's first boot.

const (
	interruptedOrphanPID = 4099850
	// staleRealizationURL is the address the row holds: the realization
	// before the one being started.
	staleRealizationURL = "http://10.200.3.2:8085"
	// spawnedRealizationURL is the address the interrupted boot's guest got
	// (vmmanager allocates a fresh subnet for every boot).
	spawnedRealizationURL = "http://10.200.9.2:8085"
)

var errVMCTLStoppedMidBoot = errors.New("vmctl stopped mid-boot")

// startInterruptedByRestart runs one start on a vmctl that a restart kills
// mid-boot: the manager spawns the guest and never returns while the test
// runs, as waitForGuestReady does for a replaying guest. It returns the VM
// being started once the spawn is under way.
func startInterruptedByRestart(t *testing.T, path string, seed *VMOwnership, tracked, snapshot bool, start func(*OwnershipRegistry) error) string {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	if seed != nil {
		addOwnership(reg, seed)
		reg.mu.Lock()
		if err := reg.writePersistenceLocked(); err != nil {
			reg.mu.Unlock()
			t.Fatalf("persist seed: %v", err)
		}
		reg.mu.Unlock()
	}
	spawned := make(chan string, 1)
	release := make(chan struct{})
	mgr := &mockVMManager{
		getVMs:         map[string]*VMInstanceInfo{},
		bootError:      errVMCTLStoppedMidBoot,
		refreshError:   errVMCTLStoppedMidBoot,
		recoverError:   errVMCTLStoppedMidBoot,
		refreshStarted: make(chan struct{}),
		refreshRelease: release,
		recoverStarted: make(chan struct{}),
		recoverRelease: release,
	}
	mgr.bootHook = func(cfg VMManagerConfig) {
		spawned <- cfg.VMID
		<-release // hang-guard: released by the test's cleanup
	}
	if tracked && seed != nil {
		mgr.getVMs[seed.VMID] = &VMInstanceInfo{HostURL: staleRealizationURL, Epoch: seed.Epoch, Healthy: true, State: "running"}
	}
	if snapshot {
		reg.SetVMManager(&blockingSnapshotVMManager{mockVMManager: mgr, spawned: spawned, release: release})
	} else {
		reg.SetVMManager(mgr)
	}

	done := make(chan error, 1)
	go func() { done <- start(reg) }()
	t.Cleanup(func() {
		close(release)
		<-done
	})
	select {
	case vmID := <-spawned:
		return vmID
	case <-mgr.refreshStarted:
		return seed.VMID
	case <-mgr.recoverStarted:
		return seed.VMID
	case err := <-done:
		t.Fatalf("start returned before its boot spawned: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the start to spawn")
	}
	return ""
}

// blockingSnapshotVMManager resumes from a memory snapshot and, like the
// boot above, never returns while the test runs.
type blockingSnapshotVMManager struct {
	*mockVMManager
	spawned chan<- string
	release <-chan struct{}
}

func (m *blockingSnapshotVMManager) ResumeVMFromSnapshot(vmID string, _ VMManagerConfig, _ func() (string, error)) (*VMInstanceInfo, error) {
	m.spawned <- vmID
	// Bounded far past any test's own run: the fixture only has to outlive
	// the restart the test simulates.
	select {
	case <-m.release:
		return nil, errVMCTLStoppedMidBoot
	case <-time.After(2 * time.Minute):
		return nil, errors.New("blockingSnapshotVMManager: never released by the test")
	}
}

// restartedVMCTL is the vmctl systemd starts after the restart: it loads the
// ownership file, and the host still runs the interrupted boot's Firecracker
// process (KillMode=process), which answers only at its own address.
func restartedVMCTL(t *testing.T, path, liveVMID string) (*OwnershipRegistry, *processControlVMManager, *time.Time) {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("reload ownership: %v", err)
	}
	mgr := newProcessControlVMManager()
	if liveVMID != "" {
		mgr.live[liveVMID] = []int{interruptedOrphanPID}
	}
	reg.SetVMManager(mgr)
	now := time.Date(2026, 10, 22, 7, 11, 1, 0, time.UTC)
	reg.clock = func() time.Time { return now }
	reg.guestNetworkProbe = func(url string) bool { return url == spawnedRealizationURL }
	return reg, mgr, &now
}

// interruptedFateIsHonest reports a stop reason that names what happened: not
// empty, not the fate the row had before the start, not a reattach still
// pending, and not a failure of the computer.
func interruptedFateIsHonest(stoppedBy, prior string) bool {
	switch stoppedBy {
	case "", prior, stoppedByVMCTLRestart, stoppedByWedged, stoppedByRecoveryFailed:
		return false
	}
	return true
}

// requireStartSettled fails unless the interrupted start was adopted at the
// address its boot was given, or closed: its process gone and the row stopped
// with an honest fate. It reports whether it was closed.
func requireStartSettled(t *testing.T, reg *OwnershipRegistry, mgr *processControlVMManager, vmID, prior, when string) bool {
	t.Helper()
	own := reg.GetOwnershipByVMID(vmID)
	if own == nil {
		t.Fatalf("%s: the ownership of %s is gone", when, vmID)
	}
	_, live := mgr.live[vmID]
	adopted := live && own.State == VMStateActive && own.ComputerURL == spawnedRealizationURL
	closed := !live && own.State == VMStateStopped && interruptedFateIsHonest(own.StoppedBy, prior)
	if !adopted && !closed {
		t.Fatalf("%s: interrupted start of %s neither adopted at %s nor closed: state=%s stopped_by=%q url=%s process_live=%t reaped=%v reattaches=%v",
			when, vmID, spawnedRealizationURL, own.State, own.StoppedBy, own.ComputerURL, live, mgr.reaped, mgr.reattaches)
	}
	return closed
}

type interruptedStartCase struct {
	name     string
	seed     func() *VMOwnership // nil: a new computer
	tracked  bool                // the old manager ran the guest being replaced
	snapshot bool                // the old manager resumes from a memory snapshot
	start    func(*OwnershipRegistry) error
}

func interruptedStartCases() []interruptedStartCase {
	seed := func(state VMState, stoppedBy string) func() *VMOwnership {
		return func() *VMOwnership {
			return &VMOwnership{VMID: "vm-c5c801f8", ComputerID: "computer-71afbf14", UserID: "batching-e2e-1022",
				DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, ComputerURL: staleRealizationURL,
				State: state, StoppedBy: stoppedBy, Epoch: 5,
				CreatedAt: time.Date(2026, 10, 22, 5, 0, 0, 0, time.UTC), LastActiveAt: time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)}
		}
	}
	return []interruptedStartCase{
		{
			name: "visit wake of a hibernated computer",
			seed: seed(VMStateHibernated, "idle"),
			start: func(reg *OwnershipRegistry) error {
				_, err := reg.ResolveOrAssignDesktopContext(context.Background(), "batching-e2e-1022", PrimaryDesktopID)
				return err
			},
		},
		{
			name:     "visit wake of a hibernated computer from its memory snapshot",
			seed:     seed(VMStateHibernated, "idle"),
			snapshot: true,
			start: func(reg *OwnershipRegistry) error {
				_, err := reg.ResolveOrAssignDesktopContext(context.Background(), "batching-e2e-1022", PrimaryDesktopID)
				return err
			},
		},
		{
			name: "resume of a stopped computer",
			seed: seed(VMStateStopped, "manual"),
			start: func(reg *OwnershipRegistry) error {
				_, err := reg.ResumeVMForDesktop("batching-e2e-1022", PrimaryDesktopID)
				return err
			},
		},
		{
			name:    "deploy refresh of an active computer",
			seed:    seed(VMStateActive, ""),
			tracked: true,
			start: func(reg *OwnershipRegistry) error {
				_, err := reg.RefreshVMForDesktop("batching-e2e-1022", PrimaryDesktopID)
				return err
			},
		},
		{
			name: "recovery of a failed computer",
			seed: seed(VMStateFailed, ""),
			start: func(reg *OwnershipRegistry) error {
				_, err := reg.RecoverVMForDesktop("batching-e2e-1022", PrimaryDesktopID)
				return err
			},
		},
		{
			name: "first boot of a new computer",
			start: func(reg *OwnershipRegistry) error {
				_, err := reg.ResolveOrAssignDesktopContext(context.Background(), "first-boot-user", PrimaryDesktopID)
				return err
			},
		},
	}
}

func TestStartInterruptedByVMCTLRestartIsAdoptedOrClosedNeverOrphaned(t *testing.T) {
	for _, tc := range interruptedStartCases() {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ownerships.json")
			var seed *VMOwnership
			prior := ""
			if tc.seed != nil {
				seed = tc.seed()
				prior = seed.StoppedBy
			}
			vmID := startInterruptedByRestart(t, path, seed, tc.tracked, tc.snapshot, tc.start)

			reg, mgr, now := restartedVMCTL(t, path, vmID)
			reg.ReattachManagedVMs(context.Background(), allowAllRoutes)
			res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
			closed := requireStartSettled(t, reg, mgr, vmID, prior, "after the restart's reattach and first reconcile")
			if closed && len(res.Unmanaged) != 0 {
				t.Fatalf("a closed start still reports unmanaged processes: %v", res.Unmanaged)
			}

			// The incident's seven minutes: the fate never turns into wedged
			// and nothing runs outside the registry.
			for minute := 1; minute <= 8; minute++ {
				*now = now.Add(time.Minute)
				res = reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
				if res.Wedged != 0 {
					t.Fatalf("minute %d: reconcile stopped the interrupted start as wedged: %+v", minute, res)
				}
				if own := reg.GetOwnershipByVMID(vmID); own == nil || own.StoppedBy == stoppedByWedged {
					t.Fatalf("minute %d: ownership = %+v, want no wedged fate", minute, own)
				}
				requireStartSettled(t, reg, mgr, vmID, prior, "later reconcile")
			}

			if !closed {
				return
			}
			// The fate is durable: the next vmctl reads the same row.
			reloaded, reloadedMgr, _ := restartedVMCTL(t, path, "")
			reloaded.ReattachManagedVMs(context.Background(), allowAllRoutes)
			requireStartSettled(t, reloaded, reloadedMgr, vmID, prior, "after a second reload")
			if len(reloadedMgr.reattaches) != 0 {
				t.Fatalf("a closed start was reattached by the next vmctl: %v", reloadedMgr.reattaches)
			}
		})
	}
}

// The designed exit after an interrupted start is the next owner act: it
// boots a fresh realization, and never adopts the orphan at the row's
// stale address.
func TestOwnerActAfterAnInterruptedWakeBootsAFreshRealization(t *testing.T) {
	tc := interruptedStartCases()[0]
	path := filepath.Join(t.TempDir(), "ownerships.json")
	seed := tc.seed()
	vmID := startInterruptedByRestart(t, path, seed, false, false, tc.start)

	reg, mgr, _ := restartedVMCTL(t, path, vmID)
	reg.ReattachManagedVMs(context.Background(), allowAllRoutes)
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)

	const freshURL = "http://10.200.11.2:8085"
	mgr.bootResponse = &VMInstanceInfo{HostURL: freshURL, Epoch: seed.Epoch + 2, Healthy: true, State: "running"}
	own, err := reg.ResolveOrAssignDesktopContext(context.Background(), seed.UserID, PrimaryDesktopID)
	if err != nil {
		t.Fatalf("owner wake after the restart: %v", err)
	}
	if len(mgr.reattaches) != 0 {
		t.Fatalf("owner wake adopted the orphan of the interrupted start at %s: reattaches=%v", own.ComputerURL, mgr.reattaches)
	}
	if len(mgr.boots) != 1 || own.State != VMStateActive || own.ComputerURL != freshURL || own.StoppedBy != "" {
		t.Fatalf("owner wake = state %s url %s stopped_by %q boots %d, want one fresh boot active at %s", own.State, own.ComputerURL, own.StoppedBy, len(mgr.boots), freshURL)
	}
	if _, live := mgr.live[vmID]; live {
		t.Fatalf("the orphan still runs beside the fresh boot")
	}
}

// A close that cannot end the process (it survives the kill) stays open: the
// row keeps its honest fate, reconcile never wedges or adopts it, and the
// next restart, which reads the file, still closes it.
func TestInterruptedStartWhoseProcessSurvivesTheCloseIsClosedByTheNextRestart(t *testing.T) {
	tc := interruptedStartCases()[0]
	path := filepath.Join(t.TempDir(), "ownerships.json")
	seed := tc.seed()
	vmID := startInterruptedByRestart(t, path, seed, false, false, tc.start)

	reg, mgr, now := restartedVMCTL(t, path, vmID)
	mgr.reapFails = true
	reg.ReattachManagedVMs(context.Background(), allowAllRoutes)
	for minute := 0; minute <= 8; minute++ {
		res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
		own := reg.GetOwnershipByVMID(vmID)
		if own == nil || own.State != VMStateStopped || !interruptedFateIsHonest(own.StoppedBy, seed.StoppedBy) {
			t.Fatalf("minute %d: ownership while the close cannot end the process = %+v", minute, own)
		}
		if res.Wedged != 0 || len(mgr.reattaches) != 0 {
			t.Fatalf("minute %d: the open close was wedged or adopted: result=%+v reattaches=%v", minute, res, mgr.reattaches)
		}
		*now = now.Add(time.Minute)
	}

	next, nextMgr, _ := restartedVMCTL(t, path, vmID)
	next.ReattachManagedVMs(context.Background(), allowAllRoutes)
	next.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if !requireStartSettled(t, next, nextMgr, vmID, seed.StoppedBy, "after the next restart") {
		t.Fatalf("the next restart adopted a start the previous one had closed")
	}
}

// A start that was published leaves nothing for the next restart: the
// running computer is reattached as before, never closed, and a computer
// hibernated after its wake stays hibernated.
func TestPublishedStartIsReattachedByTheNextRestartNotClosed(t *testing.T) {
	for _, tc := range []interruptedStartCase{interruptedStartCases()[0], interruptedStartCases()[2], interruptedStartCases()[3]} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ownerships.json")
			seed := tc.seed()
			reg := NewOwnershipRegistry("http://127.0.0.1:8085")
			if err := reg.SetPersistencePath(path); err != nil {
				t.Fatalf("SetPersistencePath: %v", err)
			}
			addOwnership(reg, seed)
			published := &VMInstanceInfo{HostURL: spawnedRealizationURL, Epoch: seed.Epoch + 1, Healthy: true, State: "running"}
			mgr := &mockVMManager{getVMs: map[string]*VMInstanceInfo{}, bootResponse: published, refreshResponse: published}
			if tc.tracked {
				mgr.getVMs[seed.VMID] = &VMInstanceInfo{HostURL: staleRealizationURL, Epoch: seed.Epoch, Healthy: true, State: "running"}
			}
			reg.SetVMManager(mgr)
			if err := tc.start(reg); err != nil {
				t.Fatalf("start: %v", err)
			}

			next, nextMgr, _ := restartedVMCTL(t, path, seed.VMID)
			if got := next.ReattachManagedVMs(context.Background(), allowAllRoutes); got != 1 {
				t.Fatalf("reattached = %d, want the published computer adopted", got)
			}
			next.ReconcileVMProcesses(context.Background(), allowAllRoutes)
			own := next.GetOwnershipByVMID(seed.VMID)
			if own == nil || own.State != VMStateActive || own.ComputerURL != spawnedRealizationURL || len(nextMgr.reaped) != 0 {
				t.Fatalf("published start after the restart = %+v reaped=%v, want reattached at %s", own, nextMgr.reaped, spawnedRealizationURL)
			}
		})
	}
	t.Run("hibernated after its wake", func(t *testing.T) {
		tc := interruptedStartCases()[0]
		path := filepath.Join(t.TempDir(), "ownerships.json")
		seed := tc.seed()
		reg := NewOwnershipRegistry("http://127.0.0.1:8085")
		if err := reg.SetPersistencePath(path); err != nil {
			t.Fatalf("SetPersistencePath: %v", err)
		}
		addOwnership(reg, seed)
		reg.SetVMManager(&mockVMManager{bootResponse: &VMInstanceInfo{HostURL: spawnedRealizationURL, Epoch: seed.Epoch + 1, Healthy: true, State: "running"}})
		if err := tc.start(reg); err != nil {
			t.Fatalf("wake: %v", err)
		}
		if err := reg.HibernateVMForDesktop(seed.UserID, PrimaryDesktopID); err != nil {
			t.Fatalf("hibernate: %v", err)
		}
		next, nextMgr, _ := restartedVMCTL(t, path, "")
		next.ReattachManagedVMs(context.Background(), allowAllRoutes)
		next.ReconcileVMProcesses(context.Background(), allowAllRoutes)
		own := next.GetOwnershipByVMID(seed.VMID)
		if own == nil || own.State != VMStateHibernated || own.StoppedBy != "idle" || len(nextMgr.reaped) != 0 {
			t.Fatalf("hibernated computer after the restart = %+v, want hibernated by idle", own)
		}
	})
}

// A start whose boot failed before the restart keeps the fate it had: the
// restart did not interrupt it, so it is not relabeled.
func TestFailedStartKeepsItsFateAcrossARestart(t *testing.T) {
	tc := interruptedStartCases()[0]
	path := filepath.Join(t.TempDir(), "ownerships.json")
	seed := tc.seed()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	addOwnership(reg, seed)
	reg.SetVMManager(&mockVMManager{bootError: errors.New("guest did not become healthy")})
	if err := tc.start(reg); err == nil {
		t.Fatal("wake with a failing boot succeeded")
	}
	next, _, _ := restartedVMCTL(t, path, "")
	next.ReattachManagedVMs(context.Background(), allowAllRoutes)
	own := next.GetOwnershipByVMID(seed.VMID)
	if own == nil || own.State != VMStateHibernated || own.StoppedBy != "idle" {
		t.Fatalf("failed wake after the restart = %+v, want hibernated by idle as before", own)
	}
}

// Review round 2 (bunch 37). Further failure modes pinned below:
//   - a boot that crashes vmctl mid-boot is closed and re-booted by every
//     sweep, forever: repeated interrupted starts are unbounded (F1);
//   - a start that drops the shield while the close cannot end the process
//     (the platform warm, a direct recover) fails the computer, and the
//     wedge detector then wedges it and spends its retry: the incident
//     again, for the platform computer (F2);
//   - the platform computer's first-ever boot leaves no row, so a restart
//     mid-boot leaves nothing to close (F3);
//   - a warm and an owner start of the platform computer both spawn, and one
//     start's publish drops the other's claim (F4);
//   - a publish write that only half landed (claim kept on the published
//     row) closes a running computer, or a claim that cannot be written
//     lets the start spawn unclaimed (lows).

// interruptStartOnDisk records, in the ownership file, a start of vmID that
// claimed and then lost vmctl to a restart: the durable state a start
// leaves when vmctl dies between its claim and its publish.
func interruptStartOnDisk(t *testing.T, reg *OwnershipRegistry, vmID string) {
	t.Helper()
	reg.mu.Lock()
	defer reg.mu.Unlock()
	own := reg.vmByID[vmID]
	if own == nil {
		t.Fatalf("no ownership for %s", vmID)
	}
	own.BootClaim = &BootClaim{Kind: bootClaimBoot, Epoch: own.Epoch + 1, ClaimedAt: "2026-10-22T07:10:00Z"}
	if err := reg.writePersistenceLocked(); err != nil {
		t.Fatalf("persist claim: %v", err)
	}
}

func alwaysOnTestComputer() *VMOwnership {
	return &VMOwnership{VMID: "vm-paid-c0ffee", ComputerID: "computer-paid-c0ffee", UserID: "always-on-e2e-1022",
		DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, WarmnessClass: WarmnessClassCriticalProtected,
		ComputerURL: staleRealizationURL, State: VMStateStopped, StoppedBy: stoppedByVMCTLRestart, Epoch: 5,
		CreatedAt: time.Date(2026, 10, 22, 5, 0, 0, 0, time.UTC), LastActiveAt: time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)}
}

func seedOwnershipFile(t *testing.T, path string, own *VMOwnership) *OwnershipRegistry {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	addOwnership(reg, own)
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if err := reg.writePersistenceLocked(); err != nil {
		t.Fatalf("persist seed: %v", err)
	}
	return reg
}

func isAlwaysOnTarget(reg *OwnershipRegistry, vmID string) bool {
	for _, target := range reg.alwaysOnWarmTargets() {
		if target.vmID == vmID {
			return true
		}
	}
	return false
}

// F1: the first close is free (the next sweep boots without spending the
// retry); the second consecutive close of the same computer is charged like
// a failure and makes it sweep-ineligible, with a fate that names the
// restart; a publish ends the run.
func TestRepeatedInterruptedStartsAreBoundedLikeAFailure(t *testing.T) {
	seed := alwaysOnTestComputer()
	const freshURL = "http://10.200.12.2:8085"

	t.Run("one close re-boots without a retry spend", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ownerships.json")
		interruptStartOnDisk(t, seedOwnershipFile(t, path, seed), seed.VMID)
		reg, mgr, _ := restartedVMCTL(t, path, seed.VMID)
		reg.ReattachManagedVMs(context.Background(), allowAllRoutes)
		requireStartSettled(t, reg, mgr, seed.VMID, seed.StoppedBy, "after the first close")
		own := reg.GetOwnershipByVMID(seed.VMID)
		if stoppedByFailure(own.StoppedBy) || own.FailureWarmedAt != "" || !isAlwaysOnTarget(reg, seed.VMID) {
			t.Fatalf("first close = stopped_by %q failure_warmed_at %q target %t, want a free re-boot", own.StoppedBy, own.FailureWarmedAt, isAlwaysOnTarget(reg, seed.VMID))
		}
		mgr.bootResponse = &VMInstanceInfo{HostURL: freshURL, Epoch: seed.Epoch + 2, Healthy: true, State: "running"}
		if got := reg.WarmAlwaysOnDesktops(context.Background(), allowAllRoutes); got != 1 {
			t.Fatalf("sweep after one close warmed %d, want 1", got)
		}
		own = reg.GetOwnershipByVMID(seed.VMID)
		if len(mgr.boots) != 1 || own.State != VMStateActive || own.FailureWarmedAt != "" {
			t.Fatalf("sweep re-boot = boots %d state %s failure_warmed_at %q, want one uncharged boot", len(mgr.boots), own.State, own.FailureWarmedAt)
		}

		// The publish ends the run: the next interrupted start is a first
		// close again, still free.
		interruptStartOnDisk(t, reg, seed.VMID)
		next, nextMgr, _ := restartedVMCTL(t, path, seed.VMID)
		next.ReattachManagedVMs(context.Background(), allowAllRoutes)
		requireStartSettled(t, next, nextMgr, seed.VMID, "", "after a close that follows a publish")
		if own := next.GetOwnershipByVMID(seed.VMID); stoppedByFailure(own.StoppedBy) || own.FailureWarmedAt != "" || !isAlwaysOnTarget(next, seed.VMID) {
			t.Fatalf("close after a publish = %+v, want a free first close", own)
		}
	})

	t.Run("two consecutive closes are sweep-ineligible", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ownerships.json")
		interruptStartOnDisk(t, seedOwnershipFile(t, path, seed), seed.VMID)
		first, _, _ := restartedVMCTL(t, path, seed.VMID)
		first.ReattachManagedVMs(context.Background(), allowAllRoutes)
		// The sweep's re-boot crashes vmctl again before it publishes.
		interruptStartOnDisk(t, first, seed.VMID)

		reg, mgr, now := restartedVMCTL(t, path, seed.VMID)
		reg.ReattachManagedVMs(context.Background(), allowAllRoutes)
		requireStartSettled(t, reg, mgr, seed.VMID, seed.StoppedBy, "after the second close")
		own := reg.GetOwnershipByVMID(seed.VMID)
		if !stoppedByFailure(own.StoppedBy) || own.FailureWarmedAt == "" {
			t.Fatalf("second consecutive close = stopped_by %q failure_warmed_at %q, want a charged failure fate", own.StoppedBy, own.FailureWarmedAt)
		}
		if isAlwaysOnTarget(reg, seed.VMID) {
			t.Fatal("a computer whose start crashed vmctl twice is still a sweep target")
		}
		mgr.bootResponse = &VMInstanceInfo{HostURL: freshURL, Epoch: seed.Epoch + 3, Healthy: true, State: "running"}
		for minute := 0; minute < 10; minute++ {
			if got := reg.WarmAlwaysOnDesktops(context.Background(), allowAllRoutes); got != 0 {
				t.Fatalf("minute %d: the sweep warmed a computer whose start crashed vmctl twice", minute)
			}
			*now = now.Add(time.Minute)
		}
		if len(mgr.boots) != 0 {
			t.Fatalf("boots = %d, want none inside the window", len(mgr.boots))
		}
		// Durable: the next vmctl reads the same bound.
		reloaded, _, _ := restartedVMCTL(t, path, "")
		if isAlwaysOnTarget(reloaded, seed.VMID) {
			t.Fatal("the bound did not survive a reload")
		}
		// The owner act still boots it.
		if _, err := reg.ResolveOrAssignDesktopContext(context.Background(), seed.UserID, PrimaryDesktopID); err != nil {
			t.Fatalf("owner wake after the bound: %v", err)
		}
		if own := reg.GetOwnershipByVMID(seed.VMID); own.State != VMStateActive || len(mgr.boots) != 1 {
			t.Fatalf("owner wake = %+v boots %d, want one boot", own, len(mgr.boots))
		}
	})
}

func platformComputerRow(state VMState, stoppedBy string) *VMOwnership {
	return &VMOwnership{VMID: UniversalWirePlatformVMID, UserID: UniversalWirePlatformOwnerID,
		DesktopID: UniversalWirePlatformDesktopID, Kind: VMKindInteractive, WarmnessClass: WarmnessClassPublicPlatform,
		ComputerURL: staleRealizationURL, State: state, StoppedBy: stoppedBy, Epoch: 1146,
		CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), LastActiveAt: time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC)}
}

// F2 (DeepSeek's repro): the close cannot end the process, and the platform
// warm runs every sweep against a manager that does not track the VM. The
// warm and a direct recover must refuse while the close is open; the row
// keeps its honest fate, never failed, never wedged, never charged.
func TestPlatformWarmAndRecoverRefuseWhileAnInterruptedStartIsOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownerships.json")
	seed := platformComputerRow(VMStateStopped, stoppedByVMCTLRestart)
	interruptStartOnDisk(t, seedOwnershipFile(t, path, seed), seed.VMID)

	reg, mgr, now := restartedVMCTL(t, path, seed.VMID)
	mgr.reapFails = true
	mgr.resumeError = errors.New("vm " + seed.VMID + " not found")
	mgr.bootError = errors.New("vm has a live untracked Firecracker process; refusing to boot over it")
	reg.ReattachManagedVMs(context.Background(), allowAllRoutes)
	for minute := 0; minute <= 8; minute++ {
		reg.WarmUniversalWirePlatformComputer(context.Background(), allowAllRoutes)
		if _, err := reg.RecoverVMForDesktop(seed.UserID, seed.DesktopID); err == nil {
			t.Fatalf("minute %d: a direct recover started the computer while its interrupted start is open", minute)
		}
		res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
		own := reg.GetOwnershipByVMID(seed.VMID)
		if own == nil || own.State != VMStateStopped || !interruptedFateIsHonest(own.StoppedBy, seed.StoppedBy) || own.FailureWarmedAt != "" {
			t.Fatalf("minute %d: ownership while the close is open = %+v", minute, own)
		}
		if res.Wedged != 0 || len(mgr.reattaches) != 0 {
			t.Fatalf("minute %d: the open close was wedged or adopted: %+v reattaches=%v", minute, res, mgr.reattaches)
		}
		*now = now.Add(time.Minute)
	}
	if len(mgr.resumes) != 0 || len(mgr.boots) != 0 || len(mgr.recovers) != 0 {
		t.Fatalf("a start ran while the close was open: resumes=%v boots=%d recovers=%v", mgr.resumes, len(mgr.boots), mgr.recovers)
	}

	// The close finishes; the next sweep boots a fresh realization.
	mgr.reapFails = false
	mgr.bootError = nil
	const freshURL = "http://10.200.13.2:8085"
	mgr.bootResponse = &VMInstanceInfo{HostURL: freshURL, Epoch: seed.Epoch + 2, Healthy: true, State: "running"}
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if got := reg.WarmUniversalWirePlatformComputer(context.Background(), allowAllRoutes); got != 1 {
		t.Fatalf("warm after the close = %d, want 1", got)
	}
	if own := reg.GetOwnershipByVMID(seed.VMID); own.State != VMStateActive || own.ComputerURL != freshURL || own.BootClaim != nil {
		t.Fatalf("platform after the close and warm = %+v, want active at %s with no claim", own, freshURL)
	}
}

// F3: the platform computer's first-ever boot is claimed by a persisted
// booting row, so a restart mid-boot is closed, and the ensure boots it
// fresh once the close is done (refusing, without failing it, while not).
func TestPlatformFirstBootInterruptedByARestartIsClosedAndEnsuredAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownerships.json")
	vmID := startInterruptedByRestart(t, path, nil, false, false, func(reg *OwnershipRegistry) error {
		return reg.EnsureUniversalWirePlatformComputer(context.Background())
	})
	if vmID != UniversalWirePlatformVMID {
		t.Fatalf("spawned %s, want the platform computer", vmID)
	}
	reg, mgr, _ := restartedVMCTL(t, path, vmID)
	mgr.reapFails = true
	reg.ReattachManagedVMs(context.Background(), allowAllRoutes)
	if own := reg.GetOwnershipByVMID(vmID); own == nil || own.State != VMStateStopped || !interruptedFateIsHonest(own.StoppedBy, "") {
		t.Fatalf("platform first boot after the restart = %+v, want a stopped row with an honest fate", own)
	}
	if err := reg.EnsureUniversalWirePlatformComputer(context.Background()); err == nil || !transientStartRefusal(err) {
		t.Fatalf("ensure while the close is open = %v, want a transient refusal", err)
	}
	if own := reg.GetOwnershipByVMID(vmID); own.State != VMStateStopped || len(mgr.boots) != 0 {
		t.Fatalf("ensure while the close is open changed the row or booted: %+v boots=%d", own, len(mgr.boots))
	}

	mgr.reapFails = false
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	const freshURL = "http://10.200.14.2:8085"
	mgr.bootResponse = &VMInstanceInfo{HostURL: freshURL, Epoch: 3, Healthy: true, State: "running"}
	if err := reg.EnsureUniversalWirePlatformComputer(context.Background()); err != nil {
		t.Fatalf("ensure after the close: %v", err)
	}
	own := reg.GetOwnershipByVMID(vmID)
	if own == nil || own.State != VMStateActive || own.ComputerURL != freshURL || len(mgr.boots) != 1 || len(mgr.reattaches) != 0 {
		t.Fatalf("ensure after the close = %+v boots=%d reattaches=%v, want one fresh boot at %s", own, len(mgr.boots), mgr.reattaches, freshURL)
	}
}

// F4: a warm and an owner start of the platform computer never both spawn,
// and a publish drops only the claim it published.
func TestPlatformWarmAndOwnerStartNeverBothSpawn(t *testing.T) {
	for _, first := range []string{"owner recover", "warm"} {
		t.Run(first+" first", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ownerships.json")
			seed := platformComputerRow(VMStateStopped, stoppedByVMCTLRestart)
			reg := seedOwnershipFile(t, path, seed)
			spawned := make(chan struct{}, 4)
			release := make(chan struct{})
			mgr := &mockVMManager{getVMs: map[string]*VMInstanceInfo{},
				bootResponse:   &VMInstanceInfo{HostURL: spawnedRealizationURL, Epoch: seed.Epoch + 1, Healthy: true, State: "running"},
				resumeResponse: &VMInstanceInfo{HostURL: spawnedRealizationURL, Epoch: seed.Epoch, Healthy: true, State: "running"},
				resumeStarted:  make(chan struct{}), resumeRelease: release}
			mgr.bootHook = func(VMManagerConfig) {
				spawned <- struct{}{}
				<-release
			}
			reg.SetVMManager(mgr)
			owner := func() error { _, err := reg.RecoverVMForDesktop(seed.UserID, seed.DesktopID); return err }
			warm := func() error { reg.WarmUniversalWirePlatformComputer(context.Background(), allowAllRoutes); return nil }
			a, b := owner, warm
			if first == "warm" {
				a, b = warm, owner
			}
			done := make(chan error, 1)
			go func() { done <- a() }()
			select {
			case <-spawned:
			case <-mgr.resumeStarted:
			case <-time.After(5 * time.Second):
				t.Fatal("the first start did not spawn")
			}
			// The second start either returns (refused or waiting is not
			// allowed to spawn) or blocks in a second spawn; either way the
			// count is read before the first is released.
			second := make(chan error, 1)
			go func() { second <- b() }()
			secondDone := false
			select {
			case <-second:
				secondDone = true
			case <-spawned:
			case <-time.After(2 * time.Second):
			}
			mgr.mu.Lock()
			starts := len(mgr.boots) + len(mgr.resumes)
			mgr.mu.Unlock()
			close(release)
			if !secondDone {
				<-second
			}
			if err := <-done; err != nil {
				t.Fatalf("first start: %v", err)
			}
			if starts != 1 {
				t.Fatalf("starts while the first was in flight = %d, want 1", starts)
			}
			if own := reg.GetOwnershipByVMID(seed.VMID); own.State != VMStateActive || own.BootClaim != nil {
				t.Fatalf("after the first start published = %+v, want active with no claim", own)
			}
		})
	}

	t.Run("a publish keeps a later start's claim", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ownerships.json")
		seed := platformComputerRow(VMStateStopped, stoppedByVMCTLRestart)
		reg := seedOwnershipFile(t, path, seed)
		later := &BootClaim{Kind: bootClaimBoot, Epoch: seed.Epoch + 50}
		mgr := &mockVMManager{getVMs: map[string]*VMInstanceInfo{},
			bootResponse: &VMInstanceInfo{HostURL: spawnedRealizationURL, Epoch: seed.Epoch + 1, Healthy: true, State: "running"}}
		mgr.bootHook = func(VMManagerConfig) {
			// Another start of the same computer claims it while this one is
			// in flight.
			reg.mu.Lock()
			reg.vmByID[seed.VMID].BootClaim = later
			reg.mu.Unlock()
		}
		reg.SetVMManager(mgr)
		if _, err := reg.RecoverVMForDesktop(seed.UserID, seed.DesktopID); err != nil {
			t.Fatalf("recover: %v", err)
		}
		if own := reg.GetOwnershipByVMID(seed.VMID); own.BootClaim == nil || own.BootClaim.Epoch != later.Epoch {
			t.Fatalf("publish at epoch %d dropped the later start's claim: %+v", seed.Epoch+1, own.BootClaim)
		}
	})
}

// publishedIn: a claim that outlived its publish (the publish landed, the
// claim was not cleared) never ends the running computer; a claim of a later
// realization than the row's does.
func TestStaleClaimOnAPublishedRowIsDroppedNotClosed(t *testing.T) {
	for _, tc := range []struct {
		name       string
		claimEpoch int64
		closed     bool
	}{
		{"claim at the published epoch", 5, false},
		{"claim of an earlier epoch", 4, false},
		{"claim of a later epoch", 6, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ownerships.json")
			seed := alwaysOnTestComputer()
			seed.State, seed.StoppedBy, seed.ComputerURL = VMStateActive, "", spawnedRealizationURL
			seed.BootClaim = &BootClaim{Kind: bootClaimBoot, Epoch: tc.claimEpoch}
			seedOwnershipFile(t, path, seed)
			reg, mgr, _ := restartedVMCTL(t, path, seed.VMID)
			reg.ReattachManagedVMs(context.Background(), allowAllRoutes)
			own := reg.GetOwnershipByVMID(seed.VMID)
			_, live := mgr.live[seed.VMID]
			if tc.closed {
				if live || own.State != VMStateStopped || own.StoppedBy != stoppedByStartInterrupted {
					t.Fatalf("later claim on an active row = %+v live=%t, want closed", own, live)
				}
				return
			}
			if !live || own.State != VMStateActive || own.BootClaim != nil || len(mgr.reaped) != 0 {
				t.Fatalf("stale claim on a published row = %+v live=%t reaped=%v, want reattached and the claim dropped", own, live, mgr.reaped)
			}
		})
	}
}

// A start whose claim cannot be written is refused before anything spawns.
func TestStartIsRefusedWhenItsClaimCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ownerships.json")
	seed := alwaysOnTestComputer()
	seed.StoppedBy = "manual"
	reg := seedOwnershipFile(t, path, seed)
	mgr := &mockVMManager{getVMs: map[string]*VMInstanceInfo{},
		bootResponse: &VMInstanceInfo{HostURL: spawnedRealizationURL, Epoch: seed.Epoch + 1, Healthy: true, State: "running"}}
	reg.SetVMManager(mgr)
	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	reg.mu.Lock()
	reg.persistencePath = filepath.Join(blocker, "ownerships.json")
	reg.mu.Unlock()
	if _, err := reg.ResumeVMForDesktop(seed.UserID, seed.DesktopID); err == nil {
		t.Fatal("a start whose claim could not be written succeeded")
	}
	if len(mgr.boots) != 0 {
		t.Fatalf("boots = %d, want none: the start spawned without a durable claim", len(mgr.boots))
	}
	if own := reg.GetOwnershipByVMID(seed.VMID); own.BootClaim != nil || own.State != VMStateStopped {
		t.Fatalf("refused start left %+v, want the stopped row without a claim", own)
	}
}

// Review round 3 (E1, E3). The escape from the repeated-interruption bound
// is a deploy of a new target: it re-arms the charged retry and the deploy
// rescue boots the computer once. A critical_protected computer always had
// that route; the public_platform computer had none (it is not an
// always-on boot class, so a stopped one was never a rescue candidate) and
// stayed down for the whole window.
func TestDeployReArmsAndBootsAComputerWhoseStartsKeptCrashingVMCTL(t *testing.T) {
	for _, tc := range []struct {
		name      string
		protected bool
		class     WarmnessClass
	}{
		{"critical_protected", true, ""},
		{"public_platform", false, WarmnessClassPublicPlatform},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRescueFixture(t, tc.protected)
			f.failStopUntracked(stoppedByStartInterruptedRepeatedly)
			f.reg.mu.Lock()
			own := f.reg.ownerships[ownershipKey(rescueUser, PrimaryDesktopID)]
			if tc.class != "" {
				own.WarmnessClass = tc.class
			}
			own.StartInterruptions = 2
			own.FailureWarmedAt = f.now.UTC().Format(time.RFC3339Nano)
			vmID := own.VMID
			f.reg.mu.Unlock()

			// Inside the window no automatic start boots it.
			f.reg.WarmAlwaysOnDesktops(t.Context(), allowAllRoutes)
			if f.spawns() != 0 {
				t.Fatalf("the sweep booted a computer inside its charged window: spawns=%d", f.spawns())
			}
			listed, err := f.h.listOwnershipResponses(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			rescue := false
			for _, c := range selectDeployRefresh(listed).Candidates {
				if c.VMID == vmID && c.Rescue {
					rescue = true
				}
			}
			if !rescue {
				t.Fatalf("%s computer whose starts kept crashing vmctl is not a deploy rescue candidate", tc.name)
			}
			if _, resp := f.deploy(t, "build-r3"); resp.Outcome != deployRefreshRefreshed {
				t.Fatalf("deploy of a new target = %+v, want the rescue to boot it", resp)
			}
			got := f.own()
			if f.spawns() != 1 || got.State != VMStateActive || got.StoppedBy != "" || got.StartInterruptions != 0 {
				t.Fatalf("after the deploy rescue: spawns=%d own=%+v, want one boot, active, the run reset", f.spawns(), got)
			}
		})
	}
}

// The platform class reaches the deploy rescue only for the repeated
// fate: its other stops have their own escape (the platform warm and
// ensure), and a deploy does not take them over.
func TestDeployRescueReachesAPlatformComputerOnlyForRepeatedInterruptions(t *testing.T) {
	platform := string(WarmnessClassPublicPlatform)
	ownerships := []ownershipResponse{
		{VMID: "vm-repeated", UserID: "a", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: platform, StoppedBy: stoppedByStartInterruptedRepeatedly},
		{VMID: "vm-restart", UserID: "b", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: platform, StoppedBy: stoppedByVMCTLRestart},
		{VMID: "vm-interrupted", UserID: "c", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: platform, StoppedBy: stoppedByStartInterrupted},
		{VMID: "vm-wedged", UserID: "d", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive, WarmnessClass: platform, StoppedBy: stoppedByWedged},
		{VMID: "vm-failed", UserID: "e", DesktopID: "primary", State: "failed", Kind: VMKindInteractive, WarmnessClass: platform},
	}
	rescue := map[string]bool{}
	for _, c := range selectDeployRefresh(ownerships).Candidates {
		if c.Rescue {
			rescue[c.VMID] = true
		}
	}
	if !rescue["vm-repeated"] {
		t.Errorf("a platform computer whose starts kept crashing vmctl is not a rescue candidate: %v", rescue)
	}
	for _, vmID := range []string{"vm-restart", "vm-interrupted", "vm-wedged", "vm-failed"} {
		if rescue[vmID] {
			t.Errorf("%s: a deploy rescue took over a platform stop the platform warm or ensure owns", vmID)
		}
	}
}

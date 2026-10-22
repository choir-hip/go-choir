package vmctl

import (
	"context"
	"errors"
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
//   (the incident), a resume, a deploy refresh of an active computer, a
//   recovery of a failed one, and a new computer's first boot.

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
func startInterruptedByRestart(t *testing.T, path string, seed *VMOwnership, tracked bool, start func(*OwnershipRegistry) error) string {
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
	reg.SetVMManager(mgr)

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
	name    string
	seed    func() *VMOwnership // nil: a new computer
	tracked bool                // the old manager ran the guest being replaced
	start   func(*OwnershipRegistry) error
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
			vmID := startInterruptedByRestart(t, path, seed, tc.tracked, tc.start)

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
	vmID := startInterruptedByRestart(t, path, seed, false, tc.start)

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
	vmID := startInterruptedByRestart(t, path, seed, false, tc.start)

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
	for _, tc := range interruptedStartCases()[:3] {
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

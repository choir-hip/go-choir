package vmctl

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Review round 3 (bunch86) of
// docs/problems/vm-state-retention-skips-test-computers-and-quarantines-2026-10-22.md,
// findings R3-1 to R3-6. Ways it could still fail, written before the
// fixes:
//   - the orphan rule's destroy-time recheck is dropped and recovery state
//     that appears after the plan is destroyed (R3-1);
//   - a dir the orphan or ephemeral rule keeps is not reported with its
//     reasons and bytes (R3-2, R3-5);
//   - the live ephemeral rule destroys a dir holding an unfinished,
//     unreadable or journal-less quarantine, a staging image or an
//     unfinished journal, at plan or destroy time, or without the
//     lifecycle claim (R3-3);
//   - a quarantine whose recovery finished minutes ago goes with its dir
//     (R3-6).

// requireQuarantineCtime skips a test that needs a quarantine to age out:
// off Linux there is no ctime, so the age is zero by design and nothing
// ages out (retention_stat_other.go, fail closed).
func requireQuarantineCtime(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("quarantine age is read from the inode ctime, Linux only; off Linux the age is zero (fail closed) and nothing ages out")
	}
}

func keptStateDir(plan RetentionPrunePlan, vmID string) *RetentionKeptStateDir {
	for i := range plan.KeptStateDirs {
		if plan.KeptStateDirs[i].VMID == vmID {
			return &plan.KeptStateDirs[i]
		}
	}
	return nil
}

func TestOrphanRuleRechecksRecoveryStateAtDestroyTime(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeOff, RetentionPruneModeOff)
	f.reg.mu.Lock()
	f.reg.retentionPrune.OrphanMinAge = time.Nanosecond
	f.reg.mu.Unlock()
	if err := os.MkdirAll(filepath.Join(f.stateDir, "vm-late-orphan"), 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	cand := plannedCandidate(t, f.reg.RetentionPrunePlan(), "vm-late-orphan")
	f.quarantine(t, "vm-late-orphan", 1, "late", "staging", "")
	if f.reg.destroyRetentionCandidate(cand) || containsString(f.mgr.destroys, "vm-late-orphan") {
		t.Fatal("a quarantine that appeared after the plan did not stop the orphan removal")
	}
}

func TestKeptOrphanDirsAreReportedWithReasonsAndBytes(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeOff, RetentionPruneModeOff)
	f.reg.mu.Lock()
	f.reg.retentionPrune.OrphanMinAge = time.Nanosecond
	f.reg.mu.Unlock()
	f.quarantineNaming(t, "candidate-fleet-e15cb89f", 1, "orph1", "done", "active", f.own[rtOwnerUser].ComputerID)
	journalOnly := filepath.Join(f.stateDir, "vm-journal-only")
	if err := os.MkdirAll(journalOnly, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(journalOnly, "rec-1-x.journal"), []byte(`{"phase":"done","status":"active"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	plan := f.reg.RetentionPrunePlan()
	for vmID, reason := range map[string]string{"candidate-fleet-e15cb89f": "holds_quarantine", "vm-journal-only": "holds_recovery_journal"} {
		k := keptStateDir(plan, vmID)
		if k == nil {
			t.Fatalf("%s kept but not reported: %+v", vmID, plan.KeptStateDirs)
		}
		if k.Rule != "orphan_state_dir" || !hasReason(k.Reasons, reason) || k.SizeBytes <= 0 || k.Release == "" {
			t.Errorf("%s = %+v, want rule orphan_state_dir, reason %s, bytes and a release", vmID, k, reason)
		}
	}
	if plan.Inventory.KeptStateDirs != 2 || plan.Inventory.KeptStateDirBytes <= 0 {
		t.Fatalf("inventory = %+v", plan.Inventory)
	}
}

// ephemeralFixture: tracer-user on example.com, stopped two days, primary
// interactive: the live ephemeral rule's candidate.
func ephemeralFixture(t *testing.T) (*OwnershipRegistry, *mockVMManager, *VMOwnership, string) {
	t.Helper()
	reg, mgr, _, tracer := protectedRetentionRegistry(t)
	stateDir := reg.retentionPruneSnapshot().StateDir
	if err := os.MkdirAll(filepath.Join(stateDir, tracer.VMID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, tracer.VMID, "data.img"), make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	return reg, mgr, tracer, stateDir
}

func writeRecoveryFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEphemeralRuleKeepsADirHoldingRecoveryState(t *testing.T) {
	cases := map[string]map[string]string{
		"pre-swap quarantine":     {"data.img.quarantine-1-a": "x", "rec-1-a.journal": `{"phase":"staging"}`},
		"journal-less quarantine": {"data.img.quarantine-1-a": "x"},
		"failed recovery":         {"data.img.quarantine-1-a": "x", "rec-1-a.journal": `{"phase":"verified","status":"failed"}`},
		"unreadable journal":      {"data.img.quarantine-1-a": "x", "rec-1-a.journal": `{not json`},
		"staging image":           {"data.img.staging-1-a": "x"},
		"unfinished journal":      {"rec-1-a.journal": `{"phase":"fenced"}`},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			reg, mgr, tracer, stateDir := ephemeralFixture(t)
			writeRecoveryFiles(t, filepath.Join(stateDir, tracer.VMID), files)
			plan := reg.RetentionPrunePlan()
			for _, c := range plan.Candidates {
				if c.VMID == tracer.VMID {
					t.Fatalf("planned for destruction: %+v", c)
				}
			}
			k := keptStateDir(plan, tracer.VMID)
			if k == nil || k.Rule != "ephemeral_test_primary" || !hasReason(k.Reasons, "holds_unfinished_recovery") {
				t.Fatalf("kept dir report = %+v", k)
			}
			reg.PruneRetention(context.Background(), allowLifecycleRoute)
			if containsString(mgr.destroys, tracer.VMID) || reg.GetOwnership("tracer-user") == nil {
				t.Fatalf("destroyed: %v", mgr.destroys)
			}
		})
	}
	// A finished, old recovery does not pin an ephemeral computer.
	reg, mgr, tracer, stateDir := ephemeralFixture(t)
	requireQuarantineCtime(t)
	reg.setRetentionClockForTest(func() time.Time { return time.Now().Add(30 * 24 * time.Hour) })
	reg.mu.Lock()
	reg.vmByID[tracer.VMID].LastActiveAt = time.Now().Add(-48 * time.Hour)
	reg.mu.Unlock()
	writeRecoveryFiles(t, filepath.Join(stateDir, tracer.VMID), map[string]string{"data.img.quarantine-1-a": "x", "rec-1-a.journal": `{"phase":"done","status":"active"}`})
	reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if !containsString(mgr.destroys, tracer.VMID) {
		t.Fatalf("an old finished recovery pinned the ephemeral computer: %v", mgr.destroys)
	}
}

func TestEphemeralRuleRechecksRecoveryStateUnderTheClaimAtDestroyTime(t *testing.T) {
	reg, mgr, tracer, stateDir := ephemeralFixture(t)
	cand := plannedCandidate(t, reg.RetentionPrunePlan(), tracer.VMID)
	writeRecoveryFiles(t, filepath.Join(stateDir, tracer.VMID), map[string]string{"data.img.staging-1-a": "x"})
	if reg.destroyRetentionCandidate(cand) || containsString(mgr.destroys, tracer.VMID) {
		t.Fatal("a staging image that appeared after the plan did not stop the ephemeral removal")
	}
	if err := os.Remove(filepath.Join(stateDir, tracer.VMID, "data.img.staging-1-a")); err != nil {
		t.Fatal(err)
	}
	// Another path's lifecycle claim stops it, and a visit during the
	// destroy is refused by the retention claim.
	key := ownershipKey("tracer-user", PrimaryDesktopID)
	reg.mu.Lock()
	reg.lifecycle[key] = lifecycleResume
	reg.mu.Unlock()
	if reg.destroyRetentionCandidate(cand) {
		t.Fatal("removed while a resume held the claim")
	}
	reg.mu.Lock()
	delete(reg.lifecycle, key)
	reg.mu.Unlock()
	visit := &visitDuringDestroy{mockVMManager: mgr, reg: reg, user: "tracer-user"}
	reg.SetVMManager(visit)
	if !reg.destroyRetentionCandidate(cand) {
		t.Fatal("removal did not run (setup)")
	}
	if len(visit.errors) != 1 || visit.errors[0] == nil {
		t.Fatalf("a visit during the ephemeral removal = %v, want refused", visit.errors)
	}
}

func TestTestIdleKeepsADirWithAFreshlyFinishedQuarantine(t *testing.T) {
	requireQuarantineCtime(t)
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeOff)
	f.now = time.Now().Add(time.Hour)
	f.reg.mu.Lock()
	f.reg.vmByID[f.vm(rtTesterA)].LastActiveAt = time.Now().Add(-10 * 24 * time.Hour)
	f.reg.mu.Unlock()
	f.quarantine(t, f.vm(rtTesterA), 1, "fresh", "done", "active")
	e := testClassEntry(f.reg.RetentionPrunePlan(), f.vm(rtTesterA))
	if e == nil || e.Verdict != "keep" || !hasReason(e.Reasons, "holds_young_quarantine") {
		t.Fatalf("tester-a = %+v, want keep holds_young_quarantine", e)
	}
	f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if containsString(f.mgr.destroys, f.vm(rtTesterA)) {
		t.Fatal("destroyed a dir whose quarantine finished an hour ago")
	}
}

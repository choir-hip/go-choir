package vmctl

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Review round 2 (bunch76) of
// docs/problems/vm-state-retention-skips-test-computers-and-quarantines-2026-10-22.md,
// findings R2-1, R2-5 and R2-7. Ways it could still fail, written before
// the fixes:
//   - the package does not build off Linux (a Linux-only stat field);
//   - a non-primary or non-interactive desktop of a test account is removed;
//   - a test computer whose dir holds an unfinished recovery's quarantine,
//     or that was stopped to recover, is removed (DestroyVMState removes the
//     whole dir);
//   - the lifecycle claim is not taken, or another holder's claim does not
//     stop the removal;
//   - a manifest that cannot be written does not stop a removal;
//   - a quarantine's verdict is not re-read at removal time;
//   - a quarantine's age is read from its last write (mtime), not the rename;
//   - a minimum age below a day is accepted.

func TestRetentionSourcesUseNoLinuxOnlyStatFields(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || strings.HasSuffix(file, "_linux.go") {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "//go:build linux") {
			continue
		}
		for _, field := range []string{".Ctim.", ".Ctimespec", ".Atim.", ".Mtim."} {
			if strings.Contains(string(body), field) {
				t.Errorf("%s reads syscall.Stat_t%s outside a linux-only file; it does not build on darwin", file, field)
			}
		}
	}
}

func TestTestIdleTakesOnlyThePrimaryInteractiveDesktop(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeOff)
	branch, err := f.reg.ResolveOrAssignDesktop(rtTesterA, "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	f.reg.mu.Lock()
	own := f.reg.vmByID[branch.VMID]
	own.State = VMStateHibernated
	own.LastActiveAt = f.now.Add(-10 * 24 * time.Hour)
	worker := f.reg.vmByID[f.vm(rtTesterB)]
	worker.Kind = VMKind("worker")
	f.reg.mu.Unlock()
	plan := f.reg.RetentionPrunePlan()
	if e := testClassEntry(plan, branch.VMID); e == nil || e.Verdict != "keep" || !hasReason(e.Reasons, "not_primary_desktop") {
		t.Fatalf("branch desktop = %+v, want keep not_primary_desktop", e)
	}
	if e := testClassEntry(plan, f.vm(rtTesterB)); e == nil || e.Verdict != "keep" || !hasReason(e.Reasons, "not_interactive") {
		t.Fatalf("worker kind = %+v, want keep not_interactive", e)
	}
	f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if containsString(f.mgr.destroys, branch.VMID) || containsString(f.mgr.destroys, f.vm(rtTesterB)) {
		t.Fatalf("destroyed %v", f.mgr.destroys)
	}
}

func TestTestIdleKeepsADirHoldingAnUnfinishedRecovery(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeOff)
	f.quarantine(t, f.vm(rtTesterA), 1, "pre1", "staging", "")
	f.quarantine(t, f.vm(rtTesterB), 1, "nojournal", "", "")
	plan := f.reg.RetentionPrunePlan()
	for _, user := range []string{rtTesterA, rtTesterB} {
		if e := testClassEntry(plan, f.vm(user)); e == nil || e.Verdict != "keep" || !hasReason(e.Reasons, "holds_unfinished_recovery") {
			t.Fatalf("%s = %+v, want keep holds_unfinished_recovery", user, e)
		}
	}
	f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if len(f.mgr.destroys) != 0 {
		t.Fatalf("destroyed a dir holding a possibly unique quarantine: %v", f.mgr.destroys)
	}
}

func TestTestIdleRechecksTheDirAndRecoveryAtDestroyTime(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeOff)
	plan := f.reg.RetentionPrunePlan()
	candA, candB := plannedCandidate(t, plan, f.vm(rtTesterA)), plannedCandidate(t, plan, f.vm(rtTesterB))
	f.quarantine(t, f.vm(rtTesterA), 1, "late", "swapped", "")
	if f.reg.destroyRetentionCandidate(candA) {
		t.Fatal("a quarantine that appeared after the plan did not stop the removal")
	}
	f.reg.mu.Lock()
	f.reg.vmByID[f.vm(rtTesterB)].StoppedBy = "recover_current"
	f.reg.mu.Unlock()
	if f.reg.destroyRetentionCandidate(candB) {
		t.Fatal("a computer stopped to recover was removed")
	}
	f.reg.mu.Lock()
	f.reg.vmByID[f.vm(rtTesterB)].StoppedBy = ""
	f.reg.vmByID[f.vm(rtTesterB)].LastActiveAt = time.Time{}
	f.reg.mu.Unlock()
	if f.reg.destroyRetentionCandidate(candB) {
		t.Fatal("a computer with no recorded activity counted as idle")
	}
	if len(f.mgr.destroys) != 0 {
		t.Fatalf("destroyed %v", f.mgr.destroys)
	}
}

func plannedCandidate(t *testing.T, plan RetentionPrunePlan, vmID string) RetentionPruneCandidate {
	t.Helper()
	for _, c := range plan.Candidates {
		if c.VMID == vmID {
			return c
		}
	}
	t.Fatalf("%s not planned: %+v", vmID, plan.Candidates)
	return RetentionPruneCandidate{}
}

// visitDuringDestroy resolves the computer while its state is being destroyed.
type visitDuringDestroy struct {
	*mockVMManager
	reg    *OwnershipRegistry
	user   string
	errors []error
}

func (m *visitDuringDestroy) DestroyVMState(vmID string) error {
	_, err := m.reg.ResolveOrAssign(m.user)
	m.errors = append(m.errors, err)
	return m.mockVMManager.DestroyVMState(vmID)
}

func TestTestIdleRemovalHoldsTheLifecycleClaim(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeOff)
	mgr := &visitDuringDestroy{mockVMManager: f.mgr, reg: f.reg, user: rtTesterA}
	f.reg.SetVMManager(mgr)
	cand := plannedCandidate(t, f.reg.RetentionPrunePlan(), f.vm(rtTesterA))
	if !f.reg.destroyRetentionCandidate(cand) {
		t.Fatal("removal did not run (setup)")
	}
	if len(mgr.errors) != 1 || mgr.errors[0] == nil || !strings.Contains(mgr.errors[0].Error(), string(lifecycleRetention)) {
		t.Fatalf("a visit during the removal = %v, want refused by the retention claim", mgr.errors)
	}
}

func TestTestIdleRemovalYieldsToAnotherLifecycleHolder(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeOff)
	cand := plannedCandidate(t, f.reg.RetentionPrunePlan(), f.vm(rtTesterA))
	key := ownershipKey(rtTesterA, PrimaryDesktopID)
	f.reg.mu.Lock()
	f.reg.lifecycle[key] = lifecycleResume
	f.reg.mu.Unlock()
	if f.reg.destroyRetentionCandidate(cand) || len(f.mgr.destroys) != 0 {
		t.Fatalf("removed while a resume held the claim: %v", f.mgr.destroys)
	}
	f.reg.mu.Lock()
	holder := f.reg.lifecycle[key]
	f.reg.mu.Unlock()
	if holder != lifecycleResume {
		t.Fatalf("the resume's claim was replaced by %q", holder)
	}
}

func TestNoRemovalWhenTheManifestCannotBeWritten(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeActive)
	q := f.quarantine(t, f.vm(rtTesterRecent), 1, "done1", "done", "active")
	plan := f.reg.RetentionPrunePlan()
	f.reg.mu.Lock()
	f.reg.retentionPrune.ManifestPath = filepath.Join(f.stateDir, "missing-dir", "manifest.jsonl")
	f.reg.mu.Unlock()
	result := f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if len(f.mgr.destroys) != 0 || result.Deleted != 0 || f.reg.GetOwnership(rtTesterA) == nil {
		t.Fatalf("test computer removed without a manifest line: %v", f.mgr.destroys)
	}
	if e := quarantineEntry(plan, q); e == nil || e.Verdict != "will_remove" {
		t.Fatalf("setup: quarantine = %+v", e)
	}
	if _, err := os.Stat(filepath.Join(f.stateDir, f.vm(rtTesterRecent), q)); err != nil || result.QuarantinesRemoved != 0 {
		t.Fatalf("quarantine removed without a manifest line: %v", err)
	}
}

func TestQuarantineRemovalRereadsEveryCheck(t *testing.T) {
	type change struct {
		name  string
		apply func(f *retentionFixture, dir, q string)
	}
	changes := []change{
		{"owner held", func(f *retentionFixture, dir, q string) {
			f.reg.mu.Lock()
			f.reg.ownerships[ownershipKey(rtTesterRecent, PrimaryDesktopID)].HoldStatus = &MaintenanceHold{Reason: "recovery"}
			f.reg.mu.Unlock()
		}},
		{"selector added", func(f *retentionFixture, dir, q string) {
			f.reg.mu.Lock()
			f.reg.retentionPrune.SelectorNamed = append(f.reg.retentionPrune.SelectorNamed, f.own[rtTesterRecent].ComputerID)
			f.reg.mu.Unlock()
		}},
		{"owner turned paid", func(f *retentionFixture, dir, q string) {
			cfg := f.reg.warmnessPolicySnapshot()
			cfg.PaidUserIDs[rtTesterRecent] = true
			f.reg.SetWarmnessPolicyConfig(cfg)
		}},
		{"journal now failed", func(f *retentionFixture, dir, q string) {
			body := `{"recovery_id":"rec-1-done1","phase":"verified","status":"failed"}`
			if err := os.WriteFile(filepath.Join(dir, "rec-1-done1.journal"), []byte(body), 0o600); err != nil {
				panic(err)
			}
		}},
		{"journal gone", func(f *retentionFixture, dir, q string) {
			if err := os.Remove(filepath.Join(dir, "rec-1-done1.journal")); err != nil {
				panic(err)
			}
		}},
		{"min age raised", func(f *retentionFixture, dir, q string) {
			f.reg.mu.Lock()
			f.reg.retentionPrune.QuarantineMinAge = 90 * 24 * time.Hour
			f.reg.mu.Unlock()
		}},
		{"mode switched off", func(f *retentionFixture, dir, q string) {
			f.reg.mu.Lock()
			f.reg.retentionPrune.QuarantineMode = RetentionPruneModeDryRun
			f.reg.mu.Unlock()
		}},
	}
	for _, c := range changes {
		t.Run(c.name, func(t *testing.T) {
			f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeOff, RetentionPruneModeActive)
			dir := filepath.Join(f.stateDir, f.vm(rtTesterRecent))
			q := f.quarantine(t, f.vm(rtTesterRecent), 1, "done1", "done", "active")
			entry := quarantineEntry(f.reg.RetentionPrunePlan(), q)
			if entry == nil || entry.Verdict != "will_remove" {
				t.Fatalf("setup: %+v", entry)
			}
			c.apply(f, dir, q)
			if _, ok := f.reg.removeRetentionQuarantine(*entry); ok {
				t.Fatal("removed although the check changed after the plan")
			}
			if _, err := os.Stat(filepath.Join(dir, q)); err != nil {
				t.Fatalf("quarantine gone: %v", err)
			}
		})
	}
}

func TestQuarantineAgeIsFromTheRenameNotTheLastWrite(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeOff, RetentionPruneModeActive)
	dir := filepath.Join(f.stateDir, f.vm(rtTesterRecent))
	q := f.quarantine(t, f.vm(rtTesterRecent), 1, "done1", "done", "active")
	old := time.Now().Add(-60 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, q), old, old); err != nil { // mtime 60 days back; ctime is now
		t.Fatal(err)
	}
	f.now = time.Now().Add(time.Hour)
	e := quarantineEntry(f.reg.RetentionPrunePlan(), q)
	if e == nil || e.Verdict != "keep" || !hasReason(e.Reasons, "age_below_min_age") {
		t.Fatalf("a quarantine renamed an hour ago but last written 60 days ago = %+v, want keep", e)
	}
	if _, ok := f.reg.removeRetentionQuarantine(RetentionQuarantineEntry{VMID: e.VMID, Name: e.Name, Path: e.Path, Verdict: "will_remove"}); ok {
		t.Fatal("removed by mtime")
	}
}

func TestRetentionMinimumAgesHaveADayFloor(t *testing.T) {
	cfg := normalizeRetentionPruneConfig(RetentionPruneConfig{TestIdleMinAge: time.Second, QuarantineMinAge: time.Minute})
	if cfg.TestIdleMinAge < 24*time.Hour || cfg.QuarantineMinAge < 24*time.Hour {
		t.Fatalf("min ages = %s, %s; want at least 24h", cfg.TestIdleMinAge, cfg.QuarantineMinAge)
	}
}

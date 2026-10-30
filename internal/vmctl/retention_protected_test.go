package vmctl

import (
	"context"
	"testing"
	"time"
)

// docs/archive/problems/retention-prune-would-destroy-the-paper-2026-10-16.md: the
// paper's account is on an ephemeral email domain, so retention prune treated
// the critical_protected platform computer as a disposable test computer.
// Ways it could fail, written before the code:
//   - R1 a protected computer on an ephemeral account, stopped past the
//     minimum age, is planned or destroyed;
//   - R2 protection is checked in the plan but not at destroy time, so a
//     class change between plan and destroy slips through;
//   - R3 the guard over-reaches and an unprotected ephemeral computer is no
//     longer pruned.

func protectedRetentionRegistry(t *testing.T) (*OwnershipRegistry, *mockVMManager, *VMOwnership, *VMOwnership) {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{
		Mode:                  RetentionPruneModeActive,
		StateDir:              t.TempDir(),
		EphemeralEmailDomains: []string{"example.com"},
		EphemeralMinAge:       time.Hour,
		MaxDeletes:            10,
		MaxBytes:              1024 * 1024 * 1024,
	})
	reg.setRetentionUserEmailsForTest(map[string]string{
		"paper-user":  "paper@example.com",
		"tracer-user": "tracer@example.com",
	})
	paper, err := reg.ResolveOrAssign("paper-user")
	if err != nil {
		t.Fatalf("resolve paper: %v", err)
	}
	tracer, err := reg.ResolveOrAssign("tracer-user")
	if err != nil {
		t.Fatalf("resolve tracer: %v", err)
	}
	old := time.Now().Add(-48 * time.Hour)
	reg.mu.Lock()
	for _, user := range []string{"paper-user", "tracer-user"} {
		own := reg.ownerships[ownershipKey(user, PrimaryDesktopID)]
		own.State = VMStateStopped
		own.LastActiveAt = old
	}
	reg.mu.Unlock()
	cfg := DefaultWarmnessPolicyConfig()
	cfg.PlatformComputerIDs[paper.ComputerID] = true
	reg.SetWarmnessPolicyConfig(cfg)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	return reg, mgr, paper, tracer
}

func TestRetentionNeverPlansOrDestroysAProtectedComputer(t *testing.T) {
	reg, mgr, paper, tracer := protectedRetentionRegistry(t)
	if paper.ComputerID == "" {
		t.Fatal("the paper needs a computer id to be a platform computer")
	}
	plan := reg.RetentionPrunePlan()
	for _, c := range plan.Candidates {
		if c.VMID == paper.VMID { // R1
			t.Fatalf("protected paper planned for deletion: %+v", c)
		}
	}
	result := reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if containsString(mgr.destroys, paper.VMID) || reg.GetOwnership("paper-user") == nil { // R1
		t.Fatalf("protected paper destroyed: %v", mgr.destroys)
	}
	if !containsString(mgr.destroys, tracer.VMID) || result.Deleted != 1 { // R3
		t.Fatalf("unprotected tracer must still be pruned: destroys=%v deleted=%d", mgr.destroys, result.Deleted)
	}
}

func TestRetentionRechecksProtectionAtDestroyTime(t *testing.T) {
	reg, mgr, paper, _ := protectedRetentionRegistry(t)
	// R2: a plan made before the computer became protected must not carry
	// it to deletion.
	reg.SetWarmnessPolicyConfig(DefaultWarmnessPolicyConfig())
	stale := reg.RetentionPrunePlan()
	var candidate *RetentionPruneCandidate
	for i := range stale.Candidates {
		if stale.Candidates[i].VMID == paper.VMID {
			candidate = &stale.Candidates[i]
		}
	}
	if candidate == nil {
		t.Fatal("unprotected, the paper should be a candidate (setup)")
	}
	cfg := DefaultWarmnessPolicyConfig()
	cfg.PlatformComputerIDs[paper.ComputerID] = true
	reg.SetWarmnessPolicyConfig(cfg)
	if reg.destroyRetentionCandidate(*candidate) || containsString(mgr.destroys, paper.VMID) {
		t.Fatalf("destroy-time check let a protected computer through: %v", mgr.destroys)
	}
}

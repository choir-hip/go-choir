package vmctl

// A pending account deletion is a promise that nothing is destroyed before the
// purge (seat 2, review of lane account-delete, B1). The retention sweeper is
// a second, older destroyer of VM state; these tests run the real sweeper
// (PruneRetention) against a real auth database holding a real deletion
// record.
//
// Failure list (written before the fix):
//  R1 the ephemeral-domain rule destroys the state of an account that has a
//     deletion record (pending or purging);
//  R2 the test-class idle rule does;
//  R3 the quarantine age-out rule removes a data image copy of such an account;
//  R4 the same sweeper still destroys an identical account with no record, or
//     one whose deletion was cancelled (the rule must not be switched off);
//  R5 an unreadable auth database makes the sweeper destroy anything (fail
//     closed for an account that might be deleting).

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
)

type retentionDeletionEnv struct {
	reg   *OwnershipRegistry
	mgr   *mockVMManager
	store *auth.Store
	user  string
	own   *VMOwnership
	dir   string
}

func newRetentionDeletionEnv(t *testing.T, mutate func(*RetentionPruneConfig)) *retentionDeletionEnv {
	t.Helper()
	dir := t.TempDir()
	store, err := auth.OpenStore(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	u, err := store.CreateUser("ret-user", "ret@example.com")
	if err != nil {
		t.Fatal(err)
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	stateDir := filepath.Join(dir, "state")
	cfg := RetentionPruneConfig{
		Mode: RetentionPruneModeActive, StateDir: stateDir, AuthDBPath: filepath.Join(dir, "auth.db"),
		EphemeralEmailDomains: []string{"example.com"}, EphemeralMinAge: time.Hour,
		MaxDeletes: 10, MaxBytes: 1 << 30,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	reg.SetRetentionPruneConfig(cfg)
	own, err := reg.ResolveOrAssign(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(stateDir, own.VMID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, own.VMID, "data.img"), make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	reg.mu.Lock()
	o := reg.ownerships[ownershipKey(u.ID, PrimaryDesktopID)]
	o.State = VMStateStopped
	o.LastActiveAt = time.Now().Add(-48 * time.Hour)
	reg.mu.Unlock()
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	return &retentionDeletionEnv{reg: reg, mgr: mgr, store: store, user: u.ID, own: own, dir: dir}
}

func (e *retentionDeletionEnv) prune() RetentionPruneResult {
	return e.reg.PruneRetention(context.Background(), allowLifecycleRoute)
}

func TestRetentionRemovesAnEphemeralAccountWithNoDeletionRecord(t *testing.T) {
	e := newRetentionDeletionEnv(t, nil)
	e.prune()
	if !containsString(e.mgr.destroys, e.own.VMID) {
		t.Fatalf("control: the ephemeral rule no longer destroys an idle example.com computer: %v", e.mgr.destroys)
	}
}

func TestRetentionNeverDestroysAComputerOfAPendingOrPurgingDeletion(t *testing.T) { // R1
	for _, purging := range []bool{false, true} {
		e := newRetentionDeletionEnv(t, nil)
		at := time.Now().UTC().Add(-30 * time.Hour) // past the 24 hour mark seat 2 named
		if purging {
			at = time.Now().UTC().Add(-100 * time.Hour)
		}
		if _, _, err := e.store.RequestAccountDeletion(e.user, at); err != nil {
			t.Fatal(err)
		}
		if purging {
			if _, err := e.store.BeginPurge(e.user, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
		}
		plan := e.reg.RetentionPrunePlan()
		for _, c := range plan.Candidates {
			if c.VMID == e.own.VMID {
				t.Fatalf("purging=%v: planned for destruction: %+v", purging, c)
			}
		}
		// Even a stale plan is stopped where the state goes.
		e.prune()
		if containsString(e.mgr.destroys, e.own.VMID) || e.reg.GetOwnership(e.user) == nil {
			t.Fatalf("purging=%v: destroyed by the retention sweeper: %v", purging, e.mgr.destroys)
		}
	}
}

func TestRetentionDestroyTimeRecheckStopsAStalePlan(t *testing.T) { // R1, the race
	e := newRetentionDeletionEnv(t, nil)
	cand := plannedCandidate(t, e.reg.RetentionPrunePlan(), e.own.VMID)
	if _, _, err := e.store.RequestAccountDeletion(e.user, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if e.reg.destroyRetentionCandidate(cand) || containsString(e.mgr.destroys, e.own.VMID) {
		t.Fatal("a plan made before the request destroyed a pending account's computer")
	}
}

func TestRetentionRemovesTheComputerAgainOnceTheDeletionIsCancelled(t *testing.T) { // R4
	e := newRetentionDeletionEnv(t, nil)
	if _, _, err := e.store.RequestAccountDeletion(e.user, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	e.prune()
	if containsString(e.mgr.destroys, e.own.VMID) {
		t.Fatal("destroyed while pending")
	}
	tok, err := e.store.OfferDeletionCancel(e.user, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.CancelAccountDeletion(tok, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	e.prune()
	if !containsString(e.mgr.destroys, e.own.VMID) {
		t.Fatal("a cancelled deletion leaves the account exempt from retention for good")
	}
}

func TestTestClassIdleRuleNeverDestroysAPendingAccountsComputer(t *testing.T) { // R2
	e := newRetentionDeletionEnv(t, func(c *RetentionPruneConfig) {
		c.EphemeralEmailDomains = nil
		c.TestIdleMode = RetentionPruneModeActive
		c.TestIdleMinAge = 24 * time.Hour
	})
	cfg := DefaultWarmnessPolicyConfig()
	cfg.TestUserIDs = map[string]bool{e.user: true}
	e.reg.SetWarmnessPolicyConfig(cfg)
	if _, _, err := e.store.RequestAccountDeletion(e.user, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	e.prune()
	if containsString(e.mgr.destroys, e.own.VMID) || e.reg.GetOwnership(e.user) == nil {
		t.Fatalf("the test-class rule destroyed a pending account's computer: %v", e.mgr.destroys)
	}
	// Control: with no record the same rule does take it.
	c := newRetentionDeletionEnv(t, func(c *RetentionPruneConfig) {
		c.EphemeralEmailDomains = nil
		c.TestIdleMode = RetentionPruneModeActive
		c.TestIdleMinAge = 24 * time.Hour
	})
	cfg2 := DefaultWarmnessPolicyConfig()
	cfg2.TestUserIDs = map[string]bool{c.user: true}
	c.reg.SetWarmnessPolicyConfig(cfg2)
	c.prune()
	if !containsString(c.mgr.destroys, c.own.VMID) {
		t.Fatalf("control: the test-class rule no longer runs here: %v", c.mgr.destroys)
	}
}

func TestQuarantineAgeOutKeepsAPendingAccountsImage(t *testing.T) { // R3
	requireQuarantineCtime(t)
	for _, pending := range []bool{false, true} {
		e := newRetentionDeletionEnv(t, func(c *RetentionPruneConfig) {
			c.EphemeralEmailDomains = nil
			c.QuarantineMode = RetentionPruneModeActive
			c.QuarantineMinAge = 24 * time.Hour
		})
		cfg := DefaultWarmnessPolicyConfig()
		cfg.TestUserIDs = map[string]bool{e.user: true} // a test-class owner: the rule applies to it
		e.reg.SetWarmnessPolicyConfig(cfg)
		e.reg.setRetentionClockForTest(func() time.Time { return time.Now().Add(30 * 24 * time.Hour) })
		vmDir := filepath.Join(e.reg.retentionPruneSnapshot().StateDir, e.own.VMID)
		writeRecoveryFiles(t, vmDir, map[string]string{"data.img.quarantine-1-a": "x", "rec-1-a.journal": `{"phase":"done","status":"active","computer_id":"` + e.own.ComputerID + `"}`})
		if pending {
			if _, _, err := e.store.RequestAccountDeletion(e.user, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
		}
		e.prune()
		_, err := os.Stat(filepath.Join(vmDir, "data.img.quarantine-1-a"))
		if pending && err != nil {
			t.Fatalf("a pending account's quarantined data image was removed: %v", err)
		}
		if !pending && err == nil {
			t.Fatal("control: the age-out rule no longer removes this image, so the test proves nothing")
		}
	}
}

func TestRetentionFailsClosedWhenTheDeletionRecordsCannotBeRead(t *testing.T) { // R5
	e := newRetentionDeletionEnv(t, nil)
	bad := withDeletionRecords(RetentionPruneConfig{AuthDBPath: filepath.Join(e.dir, "does-not-exist.db")})
	if !bad.accountDeleting("anyone") {
		t.Fatal("an unreadable deletion table must count as every account deleting (fail closed)")
	}
	if withDeletionRecords(RetentionPruneConfig{}).accountDeleting(e.user) {
		t.Fatal("no auth database configured knows no deletion")
	}
	// The real sweeper, pointed at a file that is not a database, destroys nothing.
	garbage := filepath.Join(e.dir, "garbage.db")
	if err := os.WriteFile(garbage, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.reg.mu.Lock()
	e.reg.retentionPrune.AuthDBPath = garbage
	e.reg.retentionPrune.EphemeralUserIDPrefixes = []string{"ret-"}
	e.reg.mu.Unlock()
	e.prune()
	if containsString(e.mgr.destroys, e.own.VMID) {
		t.Fatalf("destroyed with the deletion records unreadable: %v", e.mgr.destroys)
	}
}

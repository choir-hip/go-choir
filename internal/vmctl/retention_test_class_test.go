package vmctl

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// docs/problems/vm-state-retention-skips-test-computers-and-quarantines-2026-10-22.md.
// Ways the test-class idle rule and the quarantine age-out could fail,
// written before the code:
//   - T1 the paper (a platform computer on a test account) is planned or removed;
//   - T2 the owner's (paid) computer, or a non-test account's primary
//     computer, is planned or removed;
//   - T3 a selector-named test computer is planned or removed; class: and *
//     tokens do not name a computer;
//   - T4 an active, held or recently active test computer is planned;
//   - T5 observe mode deletes anything; the shadow plan deletes anything;
//   - T6 a class change between plan and destroy slips through;
//   - T7 a pre-swap, journal-less, failed, young, protected or selected
//     quarantine is removed;
//   - T8 a removal leaves no manifest line, or the line lacks what, when,
//     why or size;
//   - T9 the report omits a kept computer or quarantine, or its reasons;
//   - T10 with both new switches off, the plan is today's plan.

const (
	rtPaperUser    = "paper-user"
	rtOwnerUser    = "owner-user"
	rtRealUser     = "real-user"
	rtTesterA      = "tester-a"      // test by account (the ephemeral tier)
	rtTesterB      = "tester-b"      // test by computer id (computer-tiers test_computers)
	rtTesterSel    = "tester-sel"    // test, named by a VM_*_ONLY selector
	rtTesterActive = "tester-active" // test, running
	rtTesterRecent = "tester-recent" // test, stopped yesterday
	rtTesterHeld   = "tester-held"   // test, under a maintenance hold
)

type retentionFixture struct {
	reg      *OwnershipRegistry
	mgr      *mockVMManager
	stateDir string
	now      time.Time
	own      map[string]*VMOwnership
}

func newRetentionFixture(t *testing.T, mode, testIdleMode, quarantineMode string) *retentionFixture {
	t.Helper()
	f := &retentionFixture{
		reg:      NewOwnershipRegistry("http://127.0.0.1:8085"),
		mgr:      &mockVMManager{},
		stateDir: t.TempDir(),
		now:      time.Now().Add(30 * 24 * time.Hour),
		own:      map[string]*VMOwnership{},
	}
	f.reg.setRetentionClockForTest(func() time.Time { return f.now })
	users := []string{rtPaperUser, rtOwnerUser, rtRealUser, rtTesterA, rtTesterB, rtTesterSel, rtTesterActive, rtTesterRecent, rtTesterHeld}
	for _, user := range users {
		own, err := f.reg.ResolveOrAssign(user)
		if err != nil {
			t.Fatalf("resolve %s: %v", user, err)
		}
		if own.ComputerID == "" {
			t.Fatalf("%s has no computer id (setup)", user)
		}
		f.own[user] = own
		if err := os.MkdirAll(filepath.Join(f.stateDir, own.VMID), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.stateDir, own.VMID, "data.img"), make([]byte, 64*1024), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	idle := f.now.Add(-10 * 24 * time.Hour)
	f.reg.mu.Lock()
	for _, user := range users {
		own := f.reg.ownerships[ownershipKey(user, PrimaryDesktopID)]
		own.State = VMStateHibernated
		own.LastActiveAt = idle
	}
	f.reg.ownerships[ownershipKey(rtTesterActive, PrimaryDesktopID)].State = VMStateActive
	f.reg.ownerships[ownershipKey(rtTesterRecent, PrimaryDesktopID)].LastActiveAt = f.now.Add(-24 * time.Hour)
	f.reg.ownerships[ownershipKey(rtTesterHeld, PrimaryDesktopID)].HoldStatus = &MaintenanceHold{Reason: "maintenance"}
	f.reg.mu.Unlock()

	f.reg.SetRetentionPruneConfig(RetentionPruneConfig{
		Mode:             mode,
		StateDir:         f.stateDir,
		TestIdleMode:     testIdleMode,
		TestIdleMinAge:   7 * 24 * time.Hour,
		QuarantineMode:   quarantineMode,
		QuarantineMinAge: 7 * 24 * time.Hour,
		SelectorNamed:    []string{f.own[rtTesterSel].ComputerID},
		MaxDeletes:       100,
		MaxBytes:         1 << 40,
	})
	cfg := DefaultWarmnessPolicyConfig()
	cfg.PlatformComputerIDs[f.own[rtPaperUser].ComputerID] = true
	cfg.PaidUserIDs[rtOwnerUser] = true
	cfg.TestUserIDs = map[string]bool{
		rtPaperUser: true, rtOwnerUser: true, rtTesterA: true, rtTesterSel: true,
		rtTesterActive: true, rtTesterRecent: true, rtTesterHeld: true,
	}
	cfg.TestComputerIDs = map[string]bool{f.own[rtTesterB].ComputerID: true}
	f.reg.SetWarmnessPolicyConfig(cfg)
	f.reg.SetVMManager(f.mgr)
	return f
}

func (f *retentionFixture) vm(user string) string { return f.own[user].VMID }

// quarantine writes a quarantined image and its journal into a VM dir and
// backdates nothing: the age is the fixture clock against the file's ctime.
func (f *retentionFixture) quarantine(t *testing.T, vmID string, generation int, op, phase, status string) string {
	t.Helper()
	computerID := "computer-of-" + vmID
	for _, own := range f.own {
		if own.VMID == vmID {
			computerID = own.ComputerID
		}
	}
	return f.quarantineNaming(t, vmID, generation, op, phase, status, computerID)
}

// quarantineNaming writes a quarantine whose journal names computerID.
func (f *retentionFixture) quarantineNaming(t *testing.T, vmID string, generation int, op, phase, status, computerID string) string {
	t.Helper()
	dir := filepath.Join(f.stateDir, vmID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "data.img.quarantine-" + strconv.Itoa(generation) + "-" + op
	if err := os.WriteFile(filepath.Join(dir, name), make([]byte, 128*1024), 0o600); err != nil {
		t.Fatal(err)
	}
	if phase != "" {
		journal := map[string]any{
			"recovery_id":    "rec-" + strconv.Itoa(generation) + "-" + op,
			"phase":          phase,
			"status":         status,
			"computer_id":    computerID,
			"vm_id":          vmID,
			"canonical_head": "head-" + op,
			"fencing_token":  map[string]any{"secret": "never-in-a-report"},
		}
		body, _ := json.Marshal(journal)
		if err := os.WriteFile(filepath.Join(dir, "rec-"+strconv.Itoa(generation)+"-"+op+".journal"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

func testClassEntry(plan RetentionPrunePlan, vmID string) *RetentionTestClassEntry {
	for i := range plan.TestClass {
		if plan.TestClass[i].VMID == vmID {
			return &plan.TestClass[i]
		}
	}
	return nil
}

func quarantineEntry(plan RetentionPrunePlan, name string) *RetentionQuarantineEntry {
	for i := range plan.Quarantines {
		if plan.Quarantines[i].Name == name {
			return &plan.Quarantines[i]
		}
	}
	return nil
}

func hasReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}

func TestTestIdleObservePlansOnlyIdleUnselectedTestComputersAndDeletesNothing(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeDryRun, RetentionPruneModeOff)
	plan := f.reg.RetentionPrunePlan()

	want := map[string]string{
		rtTesterA: "would_remove", rtTesterB: "would_remove",
		rtTesterSel: "keep", rtTesterActive: "keep", rtTesterRecent: "keep", rtTesterHeld: "keep",
	}
	for user, verdict := range want {
		e := testClassEntry(plan, f.vm(user))
		if e == nil { // T9
			t.Fatalf("%s missing from the test-class report: %+v", user, plan.TestClass)
		}
		if e.Verdict != verdict {
			t.Errorf("%s verdict = %s (%v), want %s", user, e.Verdict, e.Reasons, verdict)
		}
		if e.SizeBytes <= 0 || e.ComputerID == "" || e.State == "" {
			t.Errorf("%s entry lacks bytes, computer or state: %+v", user, e)
		}
		if verdict == "keep" && len(e.Reasons) == 0 { // T9
			t.Errorf("%s kept without a reason", user)
		}
	}
	for user, reason := range map[string]string{
		rtTesterSel: "selector_named", rtTesterActive: "state_active",
		rtTesterRecent: "idle_below_min_age", rtTesterHeld: "held",
	} {
		if e := testClassEntry(plan, f.vm(user)); e == nil || !hasReason(e.Reasons, reason) {
			t.Errorf("%s reasons = %+v, want %s", user, e, reason)
		}
	}
	for _, user := range []string{rtPaperUser, rtOwnerUser, rtRealUser} { // T1, T2
		if e := testClassEntry(plan, f.vm(user)); e != nil {
			t.Errorf("%s (not test class) in the test-class report: %+v", user, e)
		}
	}
	for _, c := range plan.Candidates { // T5: observe plans no deletion
		if c.Reason == "test_class_idle" {
			t.Fatalf("observe mode produced a deletion candidate: %+v", c)
		}
	}
	if plan.Inventory.TestClassComputers != 6 || plan.Inventory.TestClassWouldRemove != 2 || plan.Inventory.TestClassWouldRemoveBytes <= 0 {
		t.Fatalf("inventory = %+v", plan.Inventory)
	}
	result := f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if len(f.mgr.destroys) != 0 || result.Deleted != 0 { // T5
		t.Fatalf("observe deleted: %v", f.mgr.destroys)
	}
}

func TestTestIdleEnforceRemovesThroughTheRetentionPathAndNeverAProtectedComputer(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeOff)
	plan := f.reg.RetentionPrunePlan()
	if e := testClassEntry(plan, f.vm(rtTesterA)); e == nil || e.Verdict != "will_remove" {
		t.Fatalf("tester-a = %+v, want will_remove", e)
	}
	result := f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	got := map[string]bool{}
	for _, vmID := range f.mgr.destroys {
		got[vmID] = true
	}
	if !got[f.vm(rtTesterA)] || !got[f.vm(rtTesterB)] || len(got) != 2 || result.Deleted != 2 {
		t.Fatalf("destroys = %v (deleted %d), want tester-a and tester-b only", f.mgr.destroys, result.Deleted)
	}
	for _, user := range []string{rtTesterA, rtTesterB} {
		if f.reg.GetOwnership(user) != nil {
			t.Errorf("%s ownership kept after removal", user)
		}
	}
	for _, user := range []string{rtPaperUser, rtOwnerUser, rtRealUser, rtTesterSel, rtTesterActive, rtTesterRecent, rtTesterHeld} {
		if f.reg.GetOwnership(user) == nil { // T1 T2 T3 T4
			t.Errorf("%s removed", user)
		}
	}
	lines := readManifest(t, f.stateDir)
	if !manifestHas(lines, "test_class_idle", f.vm(rtTesterA), "removed") { // T8
		t.Fatalf("no manifest line for tester-a: %v", lines)
	}
}

func TestTestIdleRechecksAtDestroyTime(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeOff)
	plan := f.reg.RetentionPrunePlan()
	var candidate *RetentionPruneCandidate
	for i := range plan.Candidates {
		if plan.Candidates[i].VMID == f.vm(rtTesterA) {
			candidate = &plan.Candidates[i]
		}
	}
	if candidate == nil || candidate.Reason != "test_class_idle" {
		t.Fatalf("tester-a not planned (setup): %+v", plan.Candidates)
	}
	// T6: the account turns paid between plan and destroy.
	cfg := f.reg.warmnessPolicySnapshot()
	cfg.PaidUserIDs[rtTesterA] = true
	f.reg.SetWarmnessPolicyConfig(cfg)
	if f.reg.destroyRetentionCandidate(*candidate) || containsString(f.mgr.destroys, f.vm(rtTesterA)) {
		t.Fatal("a computer promoted out of the test class was destroyed")
	}
	// And a selector naming it after the plan.
	cfg.PaidUserIDs[rtTesterA] = false
	f.reg.SetWarmnessPolicyConfig(cfg)
	f.reg.mu.Lock()
	f.reg.retentionPrune.SelectorNamed = append(f.reg.retentionPrune.SelectorNamed, f.vm(rtTesterA))
	f.reg.mu.Unlock()
	if f.reg.destroyRetentionCandidate(*candidate) {
		t.Fatal("a computer named by a selector after the plan was destroyed")
	}
}

func TestShadowPlanReportsButNeverEnforces(t *testing.T) {
	requireQuarantineCtime(t)
	f := newRetentionFixture(t, RetentionPruneModeOff, RetentionPruneModeOff, RetentionPruneModeOff)
	f.reg.SetRetentionShadowPruneConfig(RetentionPruneConfig{
		Mode:             RetentionPruneModeActive,
		StateDir:         f.stateDir,
		TestIdleMode:     RetentionPruneModeActive,
		QuarantineMode:   RetentionPruneModeActive,
		TestIdleMinAge:   7 * 24 * time.Hour,
		QuarantineMinAge: 7 * 24 * time.Hour,
	})
	name := f.quarantine(t, f.vm(rtTesterRecent), 1, "aa", "done", "active")
	plan := f.reg.RetentionShadowPlan()
	if e := testClassEntry(plan, f.vm(rtTesterA)); e == nil || e.Verdict != "would_remove" { // T5
		t.Fatalf("shadow tester-a = %+v, want would_remove", e)
	}
	if e := quarantineEntry(plan, name); e == nil || e.Verdict != "would_remove" {
		t.Fatalf("shadow quarantine = %+v, want would_remove", e)
	}
	f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if len(f.mgr.destroys) != 0 {
		t.Fatalf("shadow config deleted: %v", f.mgr.destroys)
	}
	if _, err := os.Stat(filepath.Join(f.stateDir, f.vm(rtTesterRecent), name)); err != nil {
		t.Fatalf("shadow config removed a quarantine: %v", err)
	}
}

func TestQuarantineAgeOutKeepsEverythingThatMayBeTheOnlyCopy(t *testing.T) {
	requireQuarantineCtime(t)
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeOff, RetentionPruneModeDryRun)
	real := f.vm(rtTesterRecent) // a test computer's dir: the only class whose quarantines age out
	done := f.quarantine(t, real, 1, "done1", "done", "active")
	preSwap := f.quarantine(t, real, 2, "pre1", "staging", "")
	swapped := f.quarantine(t, real, 5, "swap1", "swapped", "")
	noJournal := f.quarantine(t, real, 3, "nojournal", "", "")
	failed := f.quarantine(t, real, 4, "failed1", "verified", "failed")
	paperQ := f.quarantine(t, f.vm(rtPaperUser), 1, "paper1", "done", "active")
	selQ := f.quarantine(t, f.vm(rtTesterSel), 1, "sel1", "done", "active")
	// R2-2: an ownerless candidate-fleet dir holding the owner's done
	// recovery (the 10-07 pair), and one naming the paper.
	ownerComputer := f.own[rtOwnerUser].ComputerID
	orphanQ := f.quarantineNaming(t, "candidate-fleet-e15cb89f", 1, "orph1", "done", "active", ownerComputer)
	orphanPaperQ := f.quarantineNaming(t, "candidate-fleet-e15cb89f", 2, "orph2", "done", "active", f.own[rtPaperUser].ComputerID)
	// R2-2: a test computer's dir whose journal names the paper or the
	// owner's computer (a moved realization) is kept.
	misPaper := f.quarantineNaming(t, real, 6, "mispaper", "done", "active", f.own[rtPaperUser].ComputerID)
	misOwner := f.quarantineNaming(t, real, 7, "misowner", "done", "active", stableComputerID(rtOwnerUser, PrimaryDesktopID, ""))
	misOther := f.quarantineNaming(t, real, 8, "misother", "done", "active", "computer-someone-else-0000")
	// R2-3: a real (non-test) account's done quarantine is kept.
	realQ := f.quarantine(t, f.vm(rtRealUser), 1, "real1", "done", "active")
	ownerQ := f.quarantine(t, f.vm(rtOwnerUser), 1, "own1", "done", "active")
	// Sonnet F8: names that do not parse and non-regular files are listed, kept.
	if err := os.WriteFile(filepath.Join(f.stateDir, real, "data.img.quarantine-old"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(f.stateDir, real, "data.img.quarantine-9-adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	// R2-2: an operator-saved dir is not VM state and is never scanned.
	saved := f.quarantine(t, "saved-by-operator", 1, "saved1", "done", "active")

	plan := f.reg.RetentionPrunePlan()
	cases := map[string]struct{ verdict, reason string }{
		done:                         {"would_remove", ""},
		orphanQ:                      {"keep", "no_ownership"},
		orphanPaperQ:                 {"keep", "no_ownership"},
		misPaper:                     {"keep", "journal_computer_protected"},
		misOwner:                     {"keep", "journal_computer_protected"},
		misOther:                     {"keep", "journal_computer_mismatch"},
		realQ:                        {"keep", "owner_not_test_class"},
		ownerQ:                       {"keep", "owner_protected_class"},
		preSwap:                      {"keep", "journal_phase_staging"},
		swapped:                      {"keep", "journal_phase_swapped"},
		noJournal:                    {"keep", "journal_missing"},
		failed:                       {"keep", "recovery_failed"},
		paperQ:                       {"keep", "owner_protected_class"},
		selQ:                         {"keep", "selector_named"},
		"data.img.quarantine-old":    {"keep", "unparsed_name"},
		"data.img.quarantine-9-adir": {"keep", "not_regular"},
	}
	if e := quarantineEntry(plan, saved); e != nil {
		t.Errorf("an operator-saved dir was scanned: %+v", e)
	}
	for name, want := range cases {
		e := quarantineEntry(plan, name)
		if e == nil { // T9
			t.Fatalf("%s missing from the quarantine report: %+v", name, plan.Quarantines)
		}
		if e.Verdict != want.verdict || (want.reason != "" && !hasReason(e.Reasons, want.reason)) { // T7
			t.Errorf("%s = %s %v, want %s %s", name, e.Verdict, e.Reasons, want.verdict, want.reason)
		}
		if want.verdict != "keep" && (e.SizeBytes <= 0 || e.QuarantinedAt == "" || e.VMID == "") {
			t.Errorf("%s entry lacks size, time or vm: %+v", name, e)
		}
		if e.VMID == "" {
			t.Errorf("%s entry lacks its vm: %+v", name, e)
		}
	}
	if e := quarantineEntry(plan, done); e.RecoveryID != "rec-1-done1" || e.JournalPhase != "done" || e.CanonicalHead != "head-done1" {
		t.Errorf("why fields = %+v", e)
	}
	body, _ := json.Marshal(plan)
	if strings.Contains(string(body), "never-in-a-report") {
		t.Fatal("the plan carries the journal's fencing token")
	}

	// T7: young quarantines are kept.
	f.now = time.Now()
	young := f.reg.RetentionPrunePlan()
	if e := quarantineEntry(young, done); e == nil || e.Verdict != "keep" || !hasReason(e.Reasons, "age_below_min_age") {
		t.Fatalf("young quarantine = %+v", e)
	}
	f.now = time.Now().Add(30 * 24 * time.Hour)

	f.reg.PruneRetention(context.Background(), allowLifecycleRoute) // T5: observe
	if _, err := os.Stat(filepath.Join(f.stateDir, real, done)); err != nil {
		t.Fatalf("observe removed a quarantine: %v", err)
	}
}

func TestQuarantineAgeOutEnforceRemovesAndWritesTheManifest(t *testing.T) {
	requireQuarantineCtime(t)
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeOff, RetentionPruneModeActive)
	real := f.vm(rtTesterRecent)
	done := f.quarantine(t, real, 1, "done1", "done", "active")
	preSwap := f.quarantine(t, real, 2, "pre1", "staging", "")
	saved := f.quarantine(t, "saved-by-operator", 1, "saved1", "done", "active")
	orphan := f.quarantineNaming(t, "candidate-fleet-e15cb89f", 1, "orph1", "done", "active", f.own[rtOwnerUser].ComputerID)
	plan := f.reg.RetentionPrunePlan()
	if e := quarantineEntry(plan, done); e == nil || e.Verdict != "will_remove" {
		t.Fatalf("done quarantine = %+v, want will_remove", e)
	}
	if plan.Decision != "will_prune" {
		t.Fatalf("decision = %s, want will_prune with only a quarantine to remove", plan.Decision)
	}
	result := f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if _, err := os.Stat(filepath.Join(f.stateDir, real, done)); !os.IsNotExist(err) {
		t.Fatalf("aged-out quarantine still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.stateDir, real, preSwap)); err != nil { // T7
		t.Fatalf("pre-swap quarantine removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.stateDir, real, "data.img")); err != nil {
		t.Fatalf("the live data image was touched: %v", err)
	}
	for _, p := range []string{filepath.Join("saved-by-operator", saved), filepath.Join("candidate-fleet-e15cb89f", orphan)} { // R2-2
		if _, err := os.Stat(filepath.Join(f.stateDir, p)); err != nil {
			t.Fatalf("%s removed: %v", p, err)
		}
	}
	if result.QuarantinesRemoved != 1 || result.QuarantineBytesRemoved <= 0 || len(f.mgr.destroys) != 0 {
		t.Fatalf("result = %+v destroys=%v", result, f.mgr.destroys)
	}
	lines := readManifest(t, f.stateDir) // T8
	var removed map[string]any
	for _, l := range lines {
		if l["kind"] == "quarantine_age_out" && l["event"] == "removed" {
			removed = l
		}
	}
	if removed == nil {
		t.Fatalf("no removed line: %v", lines)
	}
	for _, key := range []string{"path", "vm_id", "quarantined_at", "removed_at", "size_bytes", "recovery_id", "journal_phase", "why"} {
		if v, ok := removed[key]; !ok || v == "" || v == nil {
			t.Errorf("manifest line lacks %s: %v", key, removed)
		}
	}
	if !manifestHas(lines, "quarantine_age_out", real, "removing") {
		t.Errorf("no intent line before the unlink: %v", lines)
	}
}

func TestBothSwitchesOffLeaveThePlanAsToday(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeOff, RetentionPruneModeOff)
	f.quarantine(t, f.vm(rtTesterRecent), 1, "done1", "done", "active")
	plan := f.reg.RetentionPrunePlan()
	if len(plan.TestClass) != 0 || len(plan.Quarantines) != 0 || len(plan.Candidates) != 0 { // T10
		t.Fatalf("switches off, yet the plan reports or plans: test=%d quarantines=%d candidates=%+v", len(plan.TestClass), len(plan.Quarantines), plan.Candidates)
	}
	f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if len(f.mgr.destroys) != 0 {
		t.Fatalf("switches off, yet destroyed %v", f.mgr.destroys)
	}
	if _, err := os.Stat(filepath.Join(f.stateDir, retentionManifestName)); !os.IsNotExist(err) {
		t.Fatalf("switches off, yet a manifest was written: %v", err)
	}
}

func TestSelectorNamedComputersFromEnviron(t *testing.T) {
	got := RetentionSelectorNamedFromEnviron([]string{
		"VM_GUEST_RECLAIM_ONLY=computer-aaa, computer-bbb",
		"VM_MEMORY_HOTPLUG_ONLY=class:test",
		"VM_SNAPSHOT_DIFF_ONLY=",
		"VM_FIRECRACKER_NEXT_ONLY=*",
		"VM_CAPSULE_TUNNEL_ENFORCE_ONLY=vm-123\ncomputer-aaa",
		"VMCTL_PORT=8083",
		"NOT_VM_X_ONLY=computer-zzz",
		"VM_ONLY=computer-yyy",
		"CHOIR_CLAUDE_DELEGATE_COMPUTERS=computer-delegate",
		"CHOIR_VISUAL_CAPTURE_COMPUTERS=computer-visual",
		"VMCTL_JAILER_ONLY=computer-jailer vm-jailer",
		"CHOIR_CAPSULE_MIRROR_COMPUTERS=computer-mirror",
		"CHOIR_CAPSULE_MIRRORS_ONLY=computer-mirror2",
	})
	want := []string{"computer-aaa", "computer-bbb", "computer-delegate", "computer-jailer", "computer-mirror", "computer-mirror2", "computer-visual", "vm-123", "vm-jailer"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("selector-named = %v, want %v (T3: class: and * name no computer)", got, want)
	}
}

func readManifest(t *testing.T, stateDir string) []map[string]any {
	t.Helper()
	file, err := os.Open(filepath.Join(stateDir, retentionManifestName))
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer file.Close()
	var lines []map[string]any
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("manifest line %q: %v", scanner.Text(), err)
		}
		lines = append(lines, line)
	}
	return lines
}

func manifestHas(lines []map[string]any, kind, vmID, event string) bool {
	for _, l := range lines {
		if l["kind"] == kind && l["vm_id"] == vmID && l["event"] == event {
			return true
		}
	}
	return false
}

func TestTestComputerIDsClassifyButNeverDemoteAPlatformOrPaidComputer(t *testing.T) {
	cfg := DefaultWarmnessPolicyConfig()
	cfg.PlatformComputerIDs["computer-paper"] = true
	cfg.PaidUserIDs["owner"] = true
	cfg.TestComputerIDs = map[string]bool{"computer-paper": true, "computer-owner": true, "computer-t": true}
	if got := warmnessClassForOwnership(&VMOwnership{UserID: "someone", ComputerID: "computer-t"}, cfg); got != WarmnessClassTest {
		t.Fatalf("listed test computer = %s, want test", got)
	}
	if got := warmnessClassForOwnership(&VMOwnership{UserID: "someone", ComputerID: "computer-paper"}, cfg); got != WarmnessClassCriticalProtected {
		t.Fatalf("platform computer also listed as test = %s", got)
	}
	if got := warmnessClassForOwnership(&VMOwnership{UserID: "owner", ComputerID: "computer-owner"}, cfg); got != WarmnessClassPremiumAlwaysOn {
		t.Fatalf("paid computer also listed as test = %s", got)
	}
}

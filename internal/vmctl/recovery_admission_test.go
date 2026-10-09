package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// recoveryEvidenceFake serves the canonical head and advertised watermark
// endpoints vmctl reads before admitting a realization start.
type recoveryEvidenceFake struct {
	mu                sync.Mutex
	headSequence      uint64
	headStatus        int
	watermarkSequence uint64
	baseRef           string
	watermarkStatus   int
	headCalls         int
	watermarkCalls    int
	escrowed          bool
}

func (f *recoveryEvidenceFake) setEscrowed(escrowed bool) {
	f.mu.Lock()
	f.escrowed = escrowed
	f.mu.Unlock()
}

func (f *recoveryEvidenceFake) setHead(sequence uint64) {
	f.mu.Lock()
	f.headSequence = sequence
	f.mu.Unlock()
}

func (f *recoveryEvidenceFake) setWatermark(sequence uint64, baseRef string) {
	f.mu.Lock()
	f.watermarkSequence = sequence
	f.baseRef = baseRef
	f.mu.Unlock()
}

func (f *recoveryEvidenceFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/internal/computers/events/head":
		f.mu.Lock()
		sequence, status := f.headSequence, f.headStatus
		f.headCalls++
		f.mu.Unlock()
		if status != 0 {
			http.Error(w, "head unavailable", status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id":          r.URL.Query().Get("computer_id"),
			"sequence":             sequence,
			"canonical_event_head": strings.Repeat("a", 64),
		})
	case "/internal/computers/files/watermark":
		f.mu.Lock()
		sequence, baseRef, status := f.watermarkSequence, f.baseRef, f.watermarkStatus
		f.watermarkCalls++
		f.mu.Unlock()
		if status != 0 {
			http.Error(w, "watermark unavailable", status)
			return
		}
		if sequence == 0 || strings.TrimSpace(baseRef) == "" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"watermark_sequence": sequence,
			"base_ref":           baseRef,
		})
	case "/internal/computers/keys/escrow/status":
		f.mu.Lock()
		escrowed := f.escrowed
		f.mu.Unlock()
		escrows := []any{}
		if escrowed {
			escrows = append(escrows, map[string]any{"protector": "custodian", "key_digest": strings.Repeat("c", 64)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"escrows": escrows})
	default:
		http.NotFound(w, r)
	}
}

// recoveryJobsFake serves the platform checkpoint job endpoint.
type recoveryJobsFake struct {
	mu                sync.Mutex
	status            string
	watermarkSequence uint64
	baseRef           string
	posts             []map[string]any
}

func (f *recoveryJobsFake) postCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.posts)
}

func (f *recoveryJobsFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/internal/computers/projection-base/jobs" {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		http.Error(w, "internal caller required", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodPost:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.posts = append(f.posts, body)
		status, sequence, baseRef := f.status, f.watermarkSequence, f.baseRef
		f.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id":        body["computer_id"],
			"status":             status,
			"reason":             body["reason"],
			"watermark_sequence": sequence,
			"base_ref":           baseRef,
		})
	case http.MethodGet:
		f.mu.Lock()
		status := f.status
		f.mu.Unlock()
		if status == "" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": r.URL.Query().Get("computer_id"),
			"status":      status,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// testGuestBootRefusalError mirrors vmmanager.GuestBootRefusedError's
// structural surface (GuestBootRefusal) without importing vmmanager.
type testGuestBootRefusalError struct {
	kind      string
	reason    string
	local     uint64
	watermark uint64
	target    uint64
	empty     bool
	chain     bool
}

func (e *testGuestBootRefusalError) Error() string {
	return fmt.Sprintf("guest boot refused (%s): %s", e.kind, e.reason)
}

func (e *testGuestBootRefusalError) GuestBootRefusal() (string, string, uint64, uint64, uint64, bool, bool) {
	return e.kind, e.reason, e.local, e.watermark, e.target, e.empty, e.chain
}

func writeRecoveryConditions(t *testing.T, path string, conditions ...*RecoveryCondition) {
	t.Helper()
	payload := recoveryConditionsFile{SchemaVersion: recoveryConditionsSchemaVersion, Conditions: conditions}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal recovery conditions: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write recovery conditions: %v", err)
	}
}

func newRecoveryAdmissionRegistry(t *testing.T, statePath, corpusdURL, jobsURL string) *OwnershipRegistry {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.ConfigureRecoveryAdmission(RecoveryAdmissionConfig{
		StatePath:  statePath,
		CorpusdURL: corpusdURL,
		JobsURL:    jobsURL,
	}); err != nil {
		t.Fatalf("configure recovery admission: %v", err)
	}
	t.Cleanup(reg.recoveryAdmissionHandle().Wait)
	return reg
}

func seedRecoveryOwnership(reg *OwnershipRegistry, userID, computerID string, state VMState) *VMOwnership {
	own := &VMOwnership{
		VMID:       "vm-" + computerID,
		ComputerID: computerID,
		UserID:     userID,
		DesktopID:  PrimaryDesktopID,
		State:      state,
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(userID, PrimaryDesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()
	return own
}

func requireRecoveryRefusal(t *testing.T, err error) *RecoveryRefusal {
	t.Helper()
	if err == nil {
		t.Fatal("expected a typed recovery refusal, got nil error")
	}
	var refusal *RecoveryRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want *RecoveryRefusal", err)
	}
	return refusal
}

func waitForRecoveryTest(t *testing.T, probe func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if probe() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not observed before deadline")
}

func TestRecoveryAdmissionGuestRefusalStopsRetriesAcrossRestart(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headSequence: 572151, watermarkSequence: 148431, baseRef: "base-stale"}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobs := &recoveryJobsFake{status: "running"}
	jobsServer := httptest.NewServer(jobs)
	t.Cleanup(jobsServer.Close)
	statePath := filepath.Join(t.TempDir(), "recovery-conditions.json")

	reg := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, jobsServer.URL)
	mgr := &mockVMManager{bootError: &testGuestBootRefusalError{
		kind:      "tail_excess",
		reason:    "recovery tail 423720 events exceeds 10000; publish a fresher base (local=0 W=148431 H=572151)",
		local:     0,
		watermark: 148431,
		target:    572151,
		empty:     true,
		chain:     true,
	}}
	reg.SetVMManager(mgr)
	seedRecoveryOwnership(reg, "user-blocked", "computer-blocked", VMStateStopped)

	_, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-blocked", PrimaryDesktopID)
	refusal := requireRecoveryRefusal(t, err)
	if refusal.Kind != RecoveryRefusalTailExcess {
		t.Fatalf("first refusal kind = %s, want %s", refusal.Kind, RecoveryRefusalTailExcess)
	}
	if refusal.RetryAfterSeconds != recoveryRetryAfterSeconds {
		t.Fatalf("retry_after_seconds = %d, want %d", refusal.RetryAfterSeconds, recoveryRetryAfterSeconds)
	}
	if len(mgr.boots) != 1 {
		t.Fatalf("first resolve must attempt exactly one guest boot, got %d", len(mgr.boots))
	}
	waitForRecoveryTest(t, func() bool { return jobs.postCount() == 1 })
	if posted, _ := jobs.posts[0]["computer_id"].(string); posted != "computer-blocked" {
		t.Fatalf("job POST computer_id = %q, want computer-blocked", posted)
	}

	// Second resolve must refuse from the durable condition without any boot.
	_, err = reg.ResolveOrAssignDesktopContext(context.Background(), "user-blocked", PrimaryDesktopID)
	refusal = requireRecoveryRefusal(t, err)
	if refusal.Kind != RecoveryRefusalTailExcess {
		t.Fatalf("second refusal kind = %s, want %s", refusal.Kind, RecoveryRefusalTailExcess)
	}
	if len(mgr.boots) != 1 {
		t.Fatalf("blind warmness retry booted a durably refused computer, boots=%d", len(mgr.boots))
	}
	time.Sleep(50 * time.Millisecond)
	if jobs.postCount() != 1 {
		t.Fatalf("duplicate job POSTs while job is non-terminal: %d", jobs.postCount())
	}

	// A vmctl restart reloads the condition durably and still never boots.
	restarted := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, jobsServer.URL)
	restartedMgr := &mockVMManager{}
	restarted.SetVMManager(restartedMgr)
	seedRecoveryOwnership(restarted, "user-blocked", "computer-blocked", VMStateStopped)
	if _, ok := restarted.RecoveryConditionFor("computer-blocked"); !ok {
		t.Fatal("durable recovery condition did not survive restart")
	}
	_, err = restarted.ResolveOrAssignDesktopContext(context.Background(), "user-blocked", PrimaryDesktopID)
	requireRecoveryRefusal(t, err)
	if len(restartedMgr.boots) != 0 {
		t.Fatalf("restarted vmctl booted a durably refused computer, boots=%d", len(restartedMgr.boots))
	}
}

func TestRecoveryAdmissionChangedWatermarkClearsBlockedStart(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headSequence: 572151, watermarkSequence: 571000, baseRef: "base-fresh"}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobs := &recoveryJobsFake{status: "running"}
	jobsServer := httptest.NewServer(jobs)
	t.Cleanup(jobsServer.Close)
	statePath := filepath.Join(t.TempDir(), "recovery-conditions.json")
	writeRecoveryConditions(t, statePath, &RecoveryCondition{
		ComputerID: "computer-blocked",
		OwnerID:    "user-blocked",
		Kind:       RecoveryRefusalTailExcess,
		Reason:     "recovery tail 423720 events exceeds 10000",
		Witness: RecoveryInputWitness{
			LocalSequence:     0,
			WatermarkSequence: 148431,
			TargetSequence:    572151,
			BaseRef:           "base-stale",
			EmptyStore:        true,
			ChainExists:       true,
			HasLocalWitness:   true,
		},
		ObservedAt: time.Now().UTC().Add(-time.Hour),
	})

	reg := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, jobsServer.URL)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	seedRecoveryOwnership(reg, "user-blocked", "computer-blocked", VMStateStopped)

	own, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-blocked", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("fresh watermark must clear the refusal and boot: %v", err)
	}
	if own == nil || own.State != VMStateActive {
		t.Fatalf("resolve ownership = %+v, want active", own)
	}
	if len(mgr.boots) != 1 {
		t.Fatalf("cleared condition must boot exactly once, got %d", len(mgr.boots))
	}
	if _, ok := reg.RecoveryConditionFor("computer-blocked"); ok {
		t.Fatal("condition must clear when fresh recovery inputs allow the plan")
	}

	reloaded := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, jobsServer.URL)
	if _, ok := reloaded.RecoveryConditionFor("computer-blocked"); ok {
		t.Fatal("cleared condition must stay cleared across restart")
	}
}

func TestRecoveryAdmissionRetainedResumeWithStaleWatermarkStaysAllowed(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headSequence: 572151, watermarkSequence: 148431, baseRef: "base-stale"}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobsServer := httptest.NewServer(&recoveryJobsFake{status: "running"})
	t.Cleanup(jobsServer.Close)
	statePath := filepath.Join(t.TempDir(), "recovery-conditions.json")
	writeRecoveryConditions(t, statePath, &RecoveryCondition{
		ComputerID: "computer-retained",
		OwnerID:    "user-retained",
		Kind:       RecoveryRefusalTailExcess,
		Reason:     "stale refusal recorded before the retained store advanced",
		Witness: RecoveryInputWitness{
			LocalSequence:     571000,
			WatermarkSequence: 148431,
			TargetSequence:    572151,
			BaseRef:           "base-stale",
			EmptyStore:        false,
			ChainExists:       true,
			HasLocalWitness:   true,
		},
		ObservedAt: time.Now().UTC().Add(-time.Hour),
	})

	reg := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, jobsServer.URL)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	seedRecoveryOwnership(reg, "user-retained", "computer-retained", VMStateStopped)

	own, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-retained", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("healthy retained head with stale W must resume, got refusal: %v", err)
	}
	if own == nil || own.State != VMStateActive {
		t.Fatalf("resolve ownership = %+v, want active", own)
	}
	if len(mgr.boots) != 1 {
		t.Fatalf("allowed retained resume must boot exactly once, got %d", len(mgr.boots))
	}
}

func TestRecoveryAdmissionUnknownLocalRetainedStartIsNotRefused(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headSequence: 572151, watermarkSequence: 148431, baseRef: "base-stale"}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobs := &recoveryJobsFake{status: "running"}
	jobsServer := httptest.NewServer(jobs)
	t.Cleanup(jobsServer.Close)

	reg := newRecoveryAdmissionRegistry(t, filepath.Join(t.TempDir(), "recovery-conditions.json"), corpusdServer.URL, jobsServer.URL)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	seedRecoveryOwnership(reg, "user-unknown", "computer-unknown", VMStateStopped)

	own, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-unknown", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("stale W alone must not strand a retained store with unknown local state: %v", err)
	}
	if own == nil || len(mgr.boots) != 1 {
		t.Fatalf("unknown local state must boot (guest planner decides), boots=%d own=%+v", len(mgr.boots), own)
	}
	if jobs.postCount() != 0 {
		t.Fatalf("no refusal means no repair job, posts=%d", jobs.postCount())
	}
}

func TestRecoveryAdmissionFreshInstallRefusesOvercapBeforeBoot(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headSequence: 572151, watermarkSequence: 148431, baseRef: "base-stale"}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobs := &recoveryJobsFake{status: "running"}
	jobsServer := httptest.NewServer(jobs)
	t.Cleanup(jobsServer.Close)

	reg := newRecoveryAdmissionRegistry(t, filepath.Join(t.TempDir(), "recovery-conditions.json"), corpusdServer.URL, jobsServer.URL)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)

	_, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-fresh", PrimaryDesktopID)
	refusal := requireRecoveryRefusal(t, err)
	if refusal.Kind != RecoveryRefusalTailExcess {
		t.Fatalf("fresh install refusal kind = %s, want %s", refusal.Kind, RecoveryRefusalTailExcess)
	}
	if len(mgr.boots) != 0 {
		t.Fatalf("deterministic over-cap fresh install must not launch a VM, boots=%d", len(mgr.boots))
	}
	if len(mgr.reservedEpochs) != 0 {
		t.Fatalf("refused start must not reserve a boot epoch, reserved=%v", mgr.reservedEpochs)
	}
	waitForRecoveryTest(t, func() bool { return jobs.postCount() == 1 })

	// Head growth alone must neither boot nor re-enqueue the blocked job.
	corpusd.setHead(572999)
	_, err = reg.ResolveOrAssignDesktopContext(context.Background(), "user-fresh", PrimaryDesktopID)
	requireRecoveryRefusal(t, err)
	if len(mgr.boots) != 0 {
		t.Fatalf("blocked fresh install must not boot, boots=%d", len(mgr.boots))
	}
	time.Sleep(50 * time.Millisecond)
	if jobs.postCount() != 1 {
		t.Fatalf("head growth alone must not requeue the repair job, posts=%d", jobs.postCount())
	}

	// A published fresher base clears the recovery refusal, but a fresh
	// realization of an existing chain still has no privacy key source: the
	// start is refused before boot instead of crash-looping on the missing key
	// (docs/problems/fresh-realization-missing-privacy-key-blind-boot-2026-10-08.md).
	corpusd.setWatermark(571000, "base-fresh")
	_, err = reg.ResolveOrAssignDesktopContext(context.Background(), "user-fresh", PrimaryDesktopID)
	refusal = requireRecoveryRefusal(t, err)
	if refusal.Kind != RecoveryRefusalPrivacyKeyUnavailable {
		t.Fatalf("repaired fresh realization refusal kind = %s, want %s", refusal.Kind, RecoveryRefusalPrivacyKeyUnavailable)
	}
	if len(mgr.boots) != 0 {
		t.Fatalf("key-unsatisfiable fresh realization must not boot, boots=%d", len(mgr.boots))
	}
	// The base condition was replaced in place by the key condition, keeping
	// the fresh-realization witness (no clear-then-store window).
	condition, ok := reg.RecoveryConditionFor(stableComputerID("user-fresh", PrimaryDesktopID, ""))
	if !ok || condition.Kind != RecoveryRefusalPrivacyKeyUnavailable || !condition.Witness.FreshRealization {
		t.Fatalf("replaced condition = %+v (%t), want fresh privacy_key_unavailable", condition, ok)
	}
}

func TestRecoveryAdmissionFreshRealizationOfExistingChainRefusesMissingPrivacyKey(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headSequence: 2, watermarkSequence: 2, baseRef: "base-current"}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobs := &recoveryJobsFake{status: "succeeded"}
	jobsServer := httptest.NewServer(jobs)
	t.Cleanup(jobsServer.Close)
	statePath := filepath.Join(t.TempDir(), "recovery-conditions.json")

	reg := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, jobsServer.URL)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)

	for attempt := 0; attempt < 2; attempt++ {
		_, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-keyless", PrimaryDesktopID)
		refusal := requireRecoveryRefusal(t, err)
		if refusal.Kind != RecoveryRefusalPrivacyKeyUnavailable {
			t.Fatalf("attempt %d: kind = %s, want %s", attempt, refusal.Kind, RecoveryRefusalPrivacyKeyUnavailable)
		}
		if refusal.Witness.TargetSequence != 2 || !refusal.Witness.ChainExists || !refusal.Witness.EmptyStore {
			t.Fatalf("attempt %d: witness = %+v, want empty store over existing chain at 2", attempt, refusal.Witness)
		}
		if refusal.RetryAfterSeconds != privacyKeyRetryAfterSeconds {
			t.Fatalf("attempt %d: retry after = %d, want %d", attempt, refusal.RetryAfterSeconds, privacyKeyRetryAfterSeconds)
		}
		if !strings.Contains(refusal.Reason, "privacy key") {
			t.Fatalf("attempt %d: reason must name the missing key: %q", attempt, refusal.Reason)
		}
	}
	if len(mgr.boots) != 0 || len(mgr.reservedEpochs) != 0 {
		t.Fatalf("refused start must not boot or reserve an epoch, boots=%d reserved=%v", len(mgr.boots), mgr.reservedEpochs)
	}
	// A checkpoint job cannot supply a key, and recovery-input changes must
	// not reopen the refusal.
	time.Sleep(50 * time.Millisecond)
	if jobs.postCount() != 0 {
		t.Fatalf("key refusal must not enqueue a checkpoint job, posts=%d", jobs.postCount())
	}
	computerID := stableComputerID("user-keyless", PrimaryDesktopID, "")
	if condition, ok := reg.RecoveryConditionFor(computerID); !ok || condition.Kind != RecoveryRefusalPrivacyKeyUnavailable {
		t.Fatalf("key refusal must persist a durable condition: %+v (%t)", condition, ok)
	}
	corpusd.setHead(3)
	corpusd.setWatermark(3, "base-newer")
	_, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-keyless", PrimaryDesktopID)
	if refusal := requireRecoveryRefusal(t, err); refusal.Kind != RecoveryRefusalPrivacyKeyUnavailable {
		t.Fatalf("watermark change reopened a key refusal: kind=%s", refusal.Kind)
	}

	// Durable across a vmctl restart.
	restarted := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, jobsServer.URL)
	if condition, ok := restarted.RecoveryConditionFor(computerID); !ok || condition.Kind != RecoveryRefusalPrivacyKeyUnavailable {
		t.Fatalf("key refusal must survive a vmctl restart: %+v (%t)", condition, ok)
	}
	if len(mgr.boots) != 0 {
		t.Fatalf("key-refused computer booted, boots=%d", len(mgr.boots))
	}
}

func TestRecoveryAdmissionGenesisStartIsNotRefusedForMissingKey(t *testing.T) {
	// No canonical chain: the guest creates the key before minting genesis.
	corpusd := &recoveryEvidenceFake{}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobsServer := httptest.NewServer(&recoveryJobsFake{status: "running"})
	t.Cleanup(jobsServer.Close)

	for _, head := range []int{http.StatusNotFound, 0} {
		corpusd.mu.Lock()
		corpusd.headStatus = head
		corpusd.mu.Unlock()
		reg := newRecoveryAdmissionRegistry(t, filepath.Join(t.TempDir(), "recovery-conditions.json"), corpusdServer.URL, jobsServer.URL)
		mgr := &mockVMManager{}
		reg.SetVMManager(mgr)
		if _, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-genesis", PrimaryDesktopID); err != nil {
			t.Fatalf("head status %d: genesis start must not be refused: %v", head, err)
		}
		if len(mgr.boots) != 1 {
			t.Fatalf("head status %d: genesis start must boot once, boots=%d", head, len(mgr.boots))
		}
	}
}

func TestRecoveryAdmissionGuestStartupFailuresArePrompt(t *testing.T) {
	corpusdServer := httptest.NewServer(&recoveryEvidenceFake{headSequence: 9, watermarkSequence: 9, baseRef: "base"})
	t.Cleanup(corpusdServer.Close)
	jobs := &recoveryJobsFake{status: "running"}
	jobsServer := httptest.NewServer(jobs)
	t.Cleanup(jobsServer.Close)
	reg := newRecoveryAdmissionRegistry(t, filepath.Join(t.TempDir(), "recovery-conditions.json"), corpusdServer.URL, jobsServer.URL)
	own := seedRecoveryOwnership(reg, "user-startup", "computer-startup", VMStateStopped)

	for kind, want := range map[string]RecoveryRefusalKind{
		"startup_failed":   RecoveryRefusalGuestStartupFailed,
		"some_future_kind": RecoveryRefusalGuestRefused,
	} {
		guestErr := &testGuestBootRefusalError{kind: kind, reason: "autoputer: open runtime store: disk full", target: 9, chain: true}
		refusal := requireRecoveryRefusal(t, reg.noteRecoveryStartFailure(own, guestErr))
		if refusal.Kind != want {
			t.Fatalf("guest kind %q mapped to %s, want %s", kind, refusal.Kind, want)
		}
		if !strings.Contains(refusal.Reason, "disk full") {
			t.Fatalf("guest kind %q lost its reason: %q", kind, refusal.Reason)
		}
		if condition, ok := reg.RecoveryConditionFor("computer-startup"); ok {
			t.Fatalf("guest kind %q must not persist a durable condition: %+v", kind, condition)
		}
	}

	// A guest-measured missing key is deterministic: durable, but no
	// checkpoint job can repair it.
	guestErr := &testGuestBootRefusalError{kind: "privacy_key_unavailable", reason: "privacy keyring: load guest key: no such file", target: 9, chain: true}
	refusal := requireRecoveryRefusal(t, reg.noteRecoveryStartFailure(own, guestErr))
	if refusal.Kind != RecoveryRefusalPrivacyKeyUnavailable {
		t.Fatalf("guest key refusal kind = %s", refusal.Kind)
	}
	if condition, ok := reg.RecoveryConditionFor("computer-startup"); !ok || condition.Kind != RecoveryRefusalPrivacyKeyUnavailable {
		t.Fatalf("guest key refusal must be durable: %+v (%t)", condition, ok)
	}
	time.Sleep(50 * time.Millisecond)
	if jobs.postCount() != 0 {
		t.Fatalf("startup and key failures must not enqueue checkpoint jobs, posts=%d", jobs.postCount())
	}
}

func TestRecoveryAdmissionMetadataUnavailableIsDistinctFromTailExcess(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headStatus: http.StatusInternalServerError}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobsServer := httptest.NewServer(&recoveryJobsFake{status: "running"})
	t.Cleanup(jobsServer.Close)
	statePath := filepath.Join(t.TempDir(), "recovery-conditions.json")
	writeRecoveryConditions(t, statePath, &RecoveryCondition{
		ComputerID: "computer-blocked",
		OwnerID:    "user-blocked",
		Kind:       RecoveryRefusalTailExcess,
		Reason:     "recovery tail 423720 events exceeds 10000",
		Witness: RecoveryInputWitness{
			LocalSequence:     0,
			WatermarkSequence: 148431,
			TargetSequence:    572151,
			BaseRef:           "base-stale",
			EmptyStore:        true,
			ChainExists:       true,
			HasLocalWitness:   true,
		},
		ObservedAt: time.Now().UTC().Add(-time.Hour),
	})

	reg := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, jobsServer.URL)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	seedRecoveryOwnership(reg, "user-blocked", "computer-blocked", VMStateStopped)

	_, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-blocked", PrimaryDesktopID)
	refusal := requireRecoveryRefusal(t, err)
	if refusal.Kind != RecoveryRefusalMetadataUnavailable {
		t.Fatalf("metadata failure kind = %s, want %s", refusal.Kind, RecoveryRefusalMetadataUnavailable)
	}
	if len(mgr.boots) != 0 {
		t.Fatalf("durably blocked computer must not boot during metadata outage, boots=%d", len(mgr.boots))
	}
	condition, ok := reg.RecoveryConditionFor("computer-blocked")
	if !ok || condition.Kind != RecoveryRefusalTailExcess {
		t.Fatalf("metadata outage must not rewrite the durable condition: %+v (%t)", condition, ok)
	}

	// Without a durable condition, unverifiable metadata must not refuse an
	// unknown local state: the guest planner decides.
	freshReg := newRecoveryAdmissionRegistry(t, filepath.Join(t.TempDir(), "recovery-conditions.json"), corpusdServer.URL, jobsServer.URL)
	freshMgr := &mockVMManager{}
	freshReg.SetVMManager(freshMgr)
	if _, err := freshReg.ResolveOrAssignDesktopContext(context.Background(), "user-open", PrimaryDesktopID); err != nil {
		t.Fatalf("unverifiable metadata must not refuse a fresh computer: %v", err)
	}
	if len(freshMgr.boots) != 1 {
		t.Fatalf("fresh computer must boot and let the guest planner decide, boots=%d", len(freshMgr.boots))
	}
}

func TestRecoveryAdmissionMaintenanceRecoveryBypassesBreaker(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headSequence: 572151, watermarkSequence: 148431, baseRef: "base-stale"}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobsServer := httptest.NewServer(&recoveryJobsFake{status: "running"})
	t.Cleanup(jobsServer.Close)
	statePath := filepath.Join(t.TempDir(), "recovery-conditions.json")
	writeRecoveryConditions(t, statePath, &RecoveryCondition{
		ComputerID: "computer-held01",
		OwnerID:    "user-held",
		Kind:       RecoveryRefusalTailExcess,
		Reason:     "recovery tail 423720 events exceeds 10000",
		Witness: RecoveryInputWitness{
			LocalSequence:     0,
			WatermarkSequence: 148431,
			TargetSequence:    572151,
			BaseRef:           "base-stale",
			EmptyStore:        true,
			ChainExists:       true,
			HasLocalWitness:   true,
		},
		ObservedAt: time.Now().UTC().Add(-time.Hour),
	})

	reg := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, jobsServer.URL)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	own := seedRecoveryOwnership(reg, "user-held", "computer-held01", VMStateStopped)
	own.HoldStatus = &MaintenanceHold{Reason: "maintenance", HeldBy: "recovery"}

	recovered, err := reg.RecoverVMForDesktopMaintenance("user-held", PrimaryDesktopID, true)
	if err != nil {
		t.Fatalf("authorized maintenance recovery must bypass the automatic breaker: %v", err)
	}
	if recovered == nil || len(mgr.boots) != 1 {
		t.Fatalf("maintenance recovery boots=%d own=%+v", len(mgr.boots), recovered)
	}
	if _, ok := reg.RecoveryConditionFor("computer-held01"); ok {
		t.Fatal("a successful boot clears the stale durable condition")
	}
}

// TestRecoveryAdmissionEscrowIsAKeySource: with a custodian escrow the fresh
// realization receives its key (SH slice 3), so it is admitted; a durable
// privacy_key_unavailable condition reopens once escrow exists.
func TestRecoveryAdmissionEscrowIsAKeySource(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headSequence: 2, watermarkSequence: 2, baseRef: "base-current"}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	jobsServer := httptest.NewServer(&recoveryJobsFake{status: "succeeded"})
	t.Cleanup(jobsServer.Close)
	reg := newRecoveryAdmissionRegistry(t, filepath.Join(t.TempDir(), "recovery-conditions.json"), corpusdServer.URL, jobsServer.URL)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)

	_, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-escrow", PrimaryDesktopID)
	if refusal := requireRecoveryRefusal(t, err); refusal.Kind != RecoveryRefusalPrivacyKeyUnavailable {
		t.Fatalf("unescrowed fresh realization kind = %s", refusal.Kind)
	}
	corpusd.setEscrowed(true)
	if _, err := reg.ResolveOrAssignDesktopContext(context.Background(), "user-escrow", PrimaryDesktopID); err != nil {
		t.Fatalf("escrow must reopen the key condition: %v", err)
	}
	if len(mgr.boots) != 1 {
		t.Fatalf("reopened realization must boot once, boots=%d", len(mgr.boots))
	}
	if condition, ok := reg.RecoveryConditionFor(stableComputerID("user-escrow", PrimaryDesktopID, "")); ok {
		t.Fatalf("admitted start must clear the condition: %+v", condition)
	}

	escrowedReg := newRecoveryAdmissionRegistry(t, filepath.Join(t.TempDir(), "recovery-conditions.json"), corpusdServer.URL, jobsServer.URL)
	escrowedMgr := &mockVMManager{}
	escrowedReg.SetVMManager(escrowedMgr)
	if _, err := escrowedReg.ResolveOrAssignDesktopContext(context.Background(), "user-escrowed-fresh", PrimaryDesktopID); err != nil {
		t.Fatalf("escrowed fresh realization must be admitted: %v", err)
	}
	if len(escrowedMgr.boots) != 1 {
		t.Fatalf("escrowed fresh realization must boot once, boots=%d", len(escrowedMgr.boots))
	}
}

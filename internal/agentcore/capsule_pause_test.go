package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Sandbox pause and resume (docs/design/sandbox-pause-resume-2026-11-09.md,
// r3): the runtime-side failure modes, written first. The executor is a
// fake with the executor's contract (the real one is tested in
// internal/capsule on Linux).
//
// Ways this can fail:
//   - the gate counts a committed pause, so a busy computer never updates
//     (mode 15), or it excludes only one half, so the executing operation
//     still refuses;
//   - the gate admits an uncommitted, expired or mid-drain pause (modes 5, 15);
//   - the ack and the pre-ack TTL unpause race, leaving a marker over a
//     thawed capsule (mode 21), or the TTL unpauses after the ack and turns
//     a planned restart into a crash close (mode 19);
//   - a lost DELETE leaves the capsule frozen for ever (mode 22);
//   - the host backs off instead of asking each minute (holds_new_work);
//   - a reconcile entry revokes a paused capsule (mode 16) or a spared one
//     at a planned boot;
//   - a crash boot resumes a pause (modes 2, 8, 12), or a restore-reason
//     marker does (mode 20);
//   - a bad record, a spawn failure or any resume error leaves the
//     assignment open (modes 7, 25);
//   - a second wake resumes twice (mode 9), a queued admission races the
//     resume (mode 10), a deadline wake cancels during the pause (mode 11),
//     or a run's wake runs a turn against an absent capsule (mode 23).

type fakePauseCapsule struct {
	absentAssignmentCapsule
	mu      sync.Mutex
	has     map[string]bool
	look    map[string]capsule.PauseLookup
	seeds   map[string]capsule.PausedSeed
	pauses  map[string]int // PauseCapsule calls
	gate    chan struct{}  // PauseCapsule waits on it when non-nil
	pauseEr error
	ackErr  error
	ackFail map[string]error
	prepEr  error
	spawnEr error
	mintEr  error

	spawned    []capsule.SpawnSpec
	unpaused   []string
	discarded  []string
	deleteKeep []map[string]bool
	completed  []string
	order      []string
	prepGate   chan struct{}
	ttl        time.Duration
}

func newFakePause() *fakePauseCapsule {
	return &fakePauseCapsule{has: map[string]bool{}, look: map[string]capsule.PauseLookup{}, seeds: map[string]capsule.PausedSeed{}, pauses: map[string]int{}}
}

func (f *fakePauseCapsule) ForceDestroy(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.has, id)
	delete(f.look, id)
	return nil
}
func (f *fakePauseCapsule) HasCapsule(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.has[id]
}
func (f *fakePauseCapsule) Spawn(_ context.Context, spec capsule.SpawnSpec) (*capsule.Capsule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.order = append(f.order, "spawn")
	if f.spawnEr != nil {
		return nil, f.spawnEr
	}
	f.spawned = append(f.spawned, spec)
	f.has[spec.CapsuleID] = true
	return &capsule.Capsule{ID: spec.CapsuleID, State: capsule.StateActive}, nil
}
func (f *fakePauseCapsule) MintCapabilityHandle(string, capsule.AgentRole, string, string, time.Duration, string) (*capsule.Capability, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.order = append(f.order, "mint")
	if f.mintEr != nil {
		return nil, f.mintEr
	}
	return &capsule.Capability{}, nil
}

func (f *fakePauseCapsule) PauseCapsule(_ context.Context, req capsule.PauseRequest) (capsule.PauseRecord, error) {
	f.mu.Lock()
	f.pauses[req.CapsuleID]++
	gate, err := f.gate, f.pauseEr
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	if err != nil {
		return capsule.PauseRecord{}, err
	}
	now := time.Now().UTC()
	ttl := req.TTL
	if ttl <= 0 {
		ttl = time.Minute
	}
	rec := capsule.PauseRecord{Version: capsule.PauseRecordVersion, AssignmentID: req.AssignmentID, Attempt: req.Attempt, CapsuleID: req.CapsuleID,
		SubjectDigest: req.SubjectDigest, ReleaseCommit: req.ReleaseCommit, PausedAt: now, ExpiresAt: now.Add(ttl), UpperDigest: "sha256:u", UpperFiles: 1, UpperBytes: 4}
	f.mu.Lock()
	f.look[req.CapsuleID] = capsule.PauseLookup{Where: capsule.PauseLive, Record: rec}
	f.mu.Unlock()
	return rec, nil
}
func (f *fakePauseCapsule) UnpauseCapsule(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unpaused = append(f.unpaused, id)
	delete(f.look, id)
	return nil
}
func (f *fakePauseCapsule) AckPause(id string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ackErr != nil {
		return f.ackErr
	}
	if err := f.ackFail[id]; err != nil {
		return err
	}
	l, ok := f.look[id]
	if !ok {
		return errors.New("no record")
	}
	if !l.Record.Acked && !now.Before(l.Record.ExpiresAt) {
		return errors.New("expired")
	}
	l.Record.Acked = true
	f.look[id] = l
	return nil
}
func (f *fakePauseCapsule) IsPaused(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.look[id].Where == capsule.PauseLive
}
func (f *fakePauseCapsule) LookupPause(id string) capsule.PauseLookup {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.look[id]
}
func (f *fakePauseCapsule) SparedPausedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for id, l := range f.look {
		if l.Where == capsule.PauseSpared {
			ids = append(ids, id)
		}
	}
	return ids
}
func (f *fakePauseCapsule) PrepareResume(_ context.Context, want capsule.PauseRecord) (capsule.PausedSeed, error) {
	f.mu.Lock()
	gate := f.prepGate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.order = append(f.order, "prepare")
	if f.prepEr != nil {
		return capsule.PausedSeed{}, f.prepEr
	}
	l := f.look[want.CapsuleID]
	if l.Record.AssignmentID != want.AssignmentID || l.Record.SubjectDigest != want.SubjectDigest {
		return capsule.PausedSeed{}, errors.New("record does not match the binding")
	}
	seed := capsule.PausedSeed{Record: l.Record, UpperDir: "/state/.paused/" + want.CapsuleID + "/upper", Dir: "/state/.paused/" + want.CapsuleID}
	l.Where = capsule.PauseConsumed
	f.look[want.CapsuleID] = l
	f.seeds[want.CapsuleID] = seed
	return seed, nil
}
func (f *fakePauseCapsule) CompleteResume(seed capsule.PausedSeed) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completed = append(f.completed, seed.Record.CapsuleID)
	return nil
}
func (f *fakePauseCapsule) DiscardPause(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.discarded = append(f.discarded, id)
	delete(f.look, id)
	return nil
}
func (f *fakePauseCapsule) DeleteSparedPauses(keep map[string]bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := map[string]bool{}
	for k, v := range keep {
		cp[k] = v
	}
	f.deleteKeep = append(f.deleteKeep, cp)
	return nil
}

// pauseFixture is a runtime with one bound implementation assignment.
type pauseFixture struct {
	rt         *Runtime
	s          *store.Store
	fake       *fakePauseCapsule
	assignment types.EngineeringAssignment
	capsuleID  string
	handler    *APIHandler
}

// secondSeed seeds a second document's authority (its own trajectory and
// assignment id) for the tests that need two live capsules.
func (f *pauseFixture) secondSeed(t *testing.T, key string) store.EngineeringAssignmentSeed {
	t.Helper()
	seed, err := store.SeedEngineeringAssignmentAuthorityForDocument(f.s, "owner-pause", f.rt.TextureComputerID(), key, 1)
	if err != nil {
		t.Fatal(err)
	}
	return seed
}

func newPauseFixture(t *testing.T) *pauseFixture {
	t.Helper()
	rt, s := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	ctx := context.Background()
	fake := newFakePause()
	rt.assignmentRuntime = fake
	rt.pauseTune = capsulePauseTuning{ttl: time.Minute, watchdog: time.Hour, unpauseRetry: 10 * time.Millisecond, resumeRetry: time.Millisecond}
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-pause", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-pause")
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, store.SeedEngineeringBindRequest(open, "run:"+open.AssignmentID, "opaque-pause")); err != nil {
		t.Fatal(err)
	}
	assignment, err := s.GetEngineeringAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, open.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	fake.has[assignment.Binding.CapsuleID] = true
	return &pauseFixture{rt: rt, s: s, fake: fake, assignment: assignment, capsuleID: assignment.Binding.CapsuleID, handler: &APIHandler{rt: rt}}
}

func (f *pauseFixture) mark(t *testing.T, query string) (int, string) {
	t.Helper()
	return postPlannedRestartURL(t, f.handler, "/internal/runtime/planned-restart"+query, `{"target":"761d94c5"}`)
}

func (f *pauseFixture) markerExists() bool {
	_, err := os.Stat(PlannedRestartMarkerPath(f.rt.cfg.StorePath))
	return err == nil
}

func (f *pauseFixture) unpausedCount() int {
	f.fake.mu.Lock()
	defer f.fake.mu.Unlock()
	return len(f.fake.unpaused)
}

func (f *pauseFixture) deleteMark(t *testing.T) {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/internal/runtime/planned-restart", nil)
	req.RemoteAddr = "192.0.2.1:4000"
	rec := httptest.NewRecorder()
	f.handler.HandleInternalPlannedRestart(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func (f *pauseFixture) reload(t *testing.T) types.EngineeringAssignment {
	t.Helper()
	a, err := f.s.GetEngineeringAssignment(context.Background(), f.assignment.Binding.OwnerID, f.assignment.Binding.ComputerID, f.assignment.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func pauseEventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// Modes 5 and 15, and the holds_new_work cadence: phase 1 answers 409
// "pausing" and starts the pause; phase 2 acks in milliseconds and writes the
// marker; the pausing refusals tell the host to keep asking each minute.
func TestPauseMarkPhaseOneThenPhaseTwoAcks(t *testing.T) {
	f := newPauseFixture(t)
	code, body := f.mark(t, "?allow_busy=1")
	if code != http.StatusConflict || !strings.Contains(body, `"reason":"pausing"`) || !strings.Contains(body, `"holds_new_work":true`) {
		t.Fatalf("phase 1: status=%d body=%s, want 409 pausing with holds_new_work", code, body)
	}
	if f.markerExists() {
		t.Fatal("phase 1 wrote the marker while a pause was incomplete")
	}
	f.rt.waitCapsulePauses()
	if !f.fake.IsPaused(f.capsuleID) {
		t.Fatal("the pause did not commit")
	}
	started := time.Now()
	code, body = f.mark(t, "?allow_busy=1")
	if code != http.StatusOK {
		t.Fatalf("phase 2: status=%d body=%s, want 200", code, body)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("phase 2 took %s, want milliseconds", time.Since(started))
	}
	if !f.markerExists() {
		t.Fatal("phase 2 answered 200 without a durable marker")
	}
	if l := f.fake.LookupPause(f.capsuleID); !l.Record.Acked {
		t.Fatalf("the record was not acked: %+v", l)
	}
	// A repeat over an already paused and acked capsule is idempotent.
	if code, body = f.mark(t, "?allow_busy=1"); code != http.StatusOK {
		t.Fatalf("repeat mark over a paused capsule: status=%d body=%s", code, body)
	}
	if f.fake.pauses[f.capsuleID] != 1 {
		t.Fatalf("PauseCapsule called %d times, want 1", f.fake.pauses[f.capsuleID])
	}
}

// Mode 5: a repeat mark while pausing is still 409 pausing, and joins the
// pause in progress (single flight).
func TestPauseMarkWhilePausingStaysPausingAndJoins(t *testing.T) {
	f := newPauseFixture(t)
	f.fake.gate = make(chan struct{})
	for i := 0; i < 3; i++ {
		code, body := f.mark(t, "?allow_busy=1")
		if code != http.StatusConflict || !strings.Contains(body, `"reason":"pausing"`) || !strings.Contains(body, `"holds_new_work":true`) {
			t.Fatalf("mark %d while pausing: status=%d body=%s", i, code, body)
		}
	}
	if f.markerExists() {
		t.Fatal("a marker was written while a pause was incomplete")
	}
	close(f.fake.gate)
	f.rt.waitCapsulePauses()
	if f.fake.pauses[f.capsuleID] != 1 {
		t.Fatalf("PauseCapsule called %d times for three marks, want 1 (single flight)", f.fake.pauses[f.capsuleID])
	}
}

// The scope (r3): only an allow_busy mark pauses; an ordinary computer's
// sweep still waits for idle, and nothing is paused.
func TestPauseNotStartedWithoutAllowBusy(t *testing.T) {
	f := newPauseFixture(t)
	code, body := f.mark(t, "")
	if code != http.StatusConflict || !strings.Contains(body, `"live_capsules":1`) || strings.Contains(body, "pausing") {
		t.Fatalf("status=%d body=%s, want today's busy refusal", code, body)
	}
	f.rt.waitCapsulePauses()
	if f.fake.pauses[f.capsuleID] != 0 || f.markerExists() {
		t.Fatalf("a pause started without allow_busy (calls=%d marker=%v)", f.fake.pauses[f.capsuleID], f.markerExists())
	}
}

// Mode 13: the executor refuses (a claude token in the upper): the pause is
// abandoned, nothing is paused, the mark refuses as today.
func TestPauseAbandonedKeepsTheRefusal(t *testing.T) {
	f := newPauseFixture(t)
	f.fake.pauseEr = fmt.Errorf("%w: token in the upper", capsule.ErrPauseRefused)
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", code, body)
	}
	f.rt.waitCapsulePauses()
	f.fake.pauseEr = nil
	code, body := f.mark(t, "?allow_busy=1")
	// The abandoned pause is gone, so the second mark starts a fresh one.
	if code != http.StatusConflict || f.markerExists() {
		t.Fatalf("after an abandoned pause: status=%d body=%s marker=%v", code, body, f.markerExists())
	}
	if f.fake.IsPaused(f.capsuleID) && f.fake.pauses[f.capsuleID] == 1 {
		t.Fatal("the abandoned pause left the capsule paused")
	}
}

// A pause starts only while the assignment is active and bound as an
// implementation slot with no fate saga: otherwise the mark refuses as today.
func TestPauseRefusedForIneligibleAssignments(t *testing.T) {
	f := newPauseFixture(t)
	a := f.assignment
	a.Binding.Kind = types.EngineeringAssignmentVerification
	if f.rt.capsulePauseEligible(a) {
		t.Fatal("a verification-slot capsule is pausable")
	}
	a = f.assignment
	a.CapsuleDisposition = types.EngineeringCapsuleFrozen
	if f.rt.capsulePauseEligible(a) {
		t.Fatal("a frozen capsule is pausable")
	}
	f.fake.mu.Lock()
	f.fake.has[f.capsuleID] = false
	f.fake.mu.Unlock()
	if f.rt.capsulePauseEligible(f.assignment) {
		t.Fatal("a capsule the executor does not hold is pausable")
	}
}

// Mode 3: the host's refresh fails before the ack: the TTL unpauses, the run
// continues, no restart happened.
func TestPauseTTLUnpausesWhenNotAcked(t *testing.T) {
	f := newPauseFixture(t)
	f.rt.pauseTune.ttl = 60 * time.Millisecond
	if code, _ := f.mark(t, "?allow_busy=1"); code != http.StatusConflict {
		t.Fatal("phase 1")
	}
	f.rt.waitCapsulePauses()
	pauseEventually(t, "the TTL unpause", func() bool { return f.unpausedCount() == 1 })
	if f.fake.IsPaused(f.capsuleID) || f.markerExists() {
		t.Fatal("still paused or marked after the TTL")
	}
	// The gate no longer excludes it: a late mark restarts the pause, not a restart.
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusConflict {
		t.Fatalf("after the TTL: status=%d body=%s", code, body)
	}
}

// Mode 4: the host clears its marker (a failed refresh) while paused: unpause.
func TestPauseMarkDeleteUnpauses(t *testing.T) {
	f := newPauseFixture(t)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusOK {
		t.Fatalf("ack: %d %s", code, body)
	}
	f.deleteMark(t)
	if f.fake.IsPaused(f.capsuleID) || f.markerExists() {
		t.Fatalf("after the DELETE: paused=%v marker=%v", f.fake.IsPaused(f.capsuleID), f.markerExists())
	}
}

// Mode 19: after the ack the TTL expires before the kill: no unpause; the
// planned boot resumes.
func TestPauseTTLDoesNotUnpauseAfterTheAck(t *testing.T) {
	f := newPauseFixture(t)
	f.rt.pauseTune.ttl = 80 * time.Millisecond
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusOK {
		t.Fatalf("ack: %d %s", code, body)
	}
	time.Sleep(300 * time.Millisecond)
	if !f.fake.IsPaused(f.capsuleID) || f.unpausedCount() != 0 {
		t.Fatalf("the TTL unpaused an acked pause (paused=%v unpauses=%d)", f.fake.IsPaused(f.capsuleID), f.unpausedCount())
	}
}

// Mode 21: the marker write and the pre-ack TTL unpause race: exactly one
// wins; never a marker over a thawed capsule.
func TestPauseAckAndTTLRaceNeverLeavesAMarkerOverAThawedCapsule(t *testing.T) {
	for i := 0; i < 25; i++ {
		f := newPauseFixture(t)
		f.rt.pauseTune.ttl = time.Duration(1+i%7) * time.Millisecond
		f.mark(t, "?allow_busy=1")
		f.rt.waitCapsulePauses()
		time.Sleep(time.Duration(i%5) * time.Millisecond)
		code, _ := f.mark(t, "?allow_busy=1")
		time.Sleep(20 * time.Millisecond)
		paused := f.fake.IsPaused(f.capsuleID)
		if code == http.StatusOK && !paused {
			t.Fatalf("iteration %d: the ack won (200, marker=%v) but the capsule is thawed (unpauses=%d)", i, f.markerExists(), f.unpausedCount())
		}
		if f.markerExists() && !paused {
			t.Fatalf("iteration %d: a marker over a thawed capsule", i)
		}
		if code != http.StatusOK && f.markerExists() {
			t.Fatalf("iteration %d: a refused mark left a marker", i)
		}
	}
}

// Mode 22: after the ack the refresh fails and the DELETE is lost: the
// watchdog unpauses when the marker is gone or older than 15 minutes, and a
// fresh marker keeps the capsule frozen.
func TestPauseWatchdogUnpausesWhenTheMarkerIsGoneOrStale(t *testing.T) {
	f := newPauseFixture(t)
	f.rt.pauseTune.watchdog = 15 * time.Millisecond
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusOK {
		t.Fatalf("ack: %d %s", code, body)
	}
	time.Sleep(120 * time.Millisecond)
	if !f.fake.IsPaused(f.capsuleID) {
		t.Fatal("the watchdog unpaused under a fresh marker")
	}
	// The marker ages out (the kill never came).
	raw, err := os.ReadFile(PlannedRestartMarkerPath(f.rt.cfg.StorePath))
	if err != nil {
		t.Fatal(err)
	}
	var marker PlannedRestart
	if err := json.Unmarshal(raw, &marker); err != nil {
		t.Fatal(err)
	}
	marker.RequestedAt = time.Now().Add(-plannedRestartMaxAge - time.Minute)
	raw, _ = json.Marshal(marker)
	if err := os.WriteFile(PlannedRestartMarkerPath(f.rt.cfg.StorePath), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	pauseEventually(t, "the watchdog unpause after the marker aged out", func() bool { return !f.fake.IsPaused(f.capsuleID) })
}

func TestPauseWatchdogUnpausesWhenTheMarkerIsRemoved(t *testing.T) {
	f := newPauseFixture(t)
	f.rt.pauseTune.watchdog = 15 * time.Millisecond
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	f.mark(t, "?allow_busy=1")
	if err := ClearPlannedRestartMarker(f.rt.cfg.StorePath); err != nil { // the DELETE was lost, then the marker went
		t.Fatal(err)
	}
	pauseEventually(t, "the watchdog unpause", func() bool { return !f.fake.IsPaused(f.capsuleID) })
}

// Mode 15, both halves: an executing operation without a record refuses, and
// a committed pause leaves both the capsule and its executing operation out
// of the gate; /health keeps counting the paused capsule as live.
func TestPauseGateExcludesBothHalvesOfACommittedPause(t *testing.T) {
	f := newPauseFixture(t)
	ctx := context.Background()
	operations, err := selfdev.NewStore(f.s, f.s)
	if err != nil {
		t.Fatal(err)
	}
	f.rt.selfdevOperations = operations
	now := time.Now().UTC()
	insert := func(id, key string) {
		if _, err := f.s.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
			id, f.rt.TextureComputerID(), key, strings.Repeat("d", 64), "trajectory-"+id, strings.Repeat("a", 64),
			"artifact:sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateExecuting, now, now); err != nil {
			t.Fatal(err)
		}
	}
	insert("op-bound", store.EngineeringFreezeRevisionIdempotencyKey(f.assignment.AssignmentID, 1))
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	// Both halves exclude: the mark acks.
	if code, body := f.mark(t, "?allow_busy=1"); code != http.StatusOK {
		t.Fatalf("committed pause with its executing operation: status=%d body=%s, want 200", code, body)
	}
	// /health, the host's idle and crash-loop fallbacks keep counting it live.
	if n, err := f.rt.liveCapsuleCount(ctx); err != nil || n != 1 {
		t.Fatalf("liveCapsuleCount = %d, %v, want 1 (a paused capsule is live to /health)", n, err)
	}
	// An executing operation without a record refuses phase 2: pause again
	// with only the bound operation, then another operation starts executing.
	f.deleteMark(t)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	insert("op-foreign", store.EngineeringFreezeRevisionIdempotencyKey("some-other-assignment", 1))
	code, body := f.mark(t, "?allow_busy=1")
	if code != http.StatusConflict || !strings.Contains(body, `"selfdev_active_operations":1`) || f.markerExists() {
		t.Fatalf("an executing operation without a record: status=%d body=%s marker=%v, want 409", code, body, f.markerExists())
	}
	// And an operation in another busy state (an apply) is never excluded.
	if _, err := f.s.DB().ExecContext(ctx, `UPDATE self_development_operations SET state=? WHERE operation_id='op-bound'`, selfdev.StateMaterializing); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.DB().ExecContext(ctx, `DELETE FROM self_development_operations WHERE operation_id='op-foreign'`); err != nil {
		t.Fatal(err)
	}
	if code, body = f.mark(t, "?allow_busy=1"); code != http.StatusConflict || f.markerExists() {
		t.Fatalf("an apply in flight on a paused assignment: status=%d body=%s marker=%v, want 409", code, body, f.markerExists())
	}
}

// Mode 15: with one capsule still pausing, phase 2 refuses.
func TestPauseGateRefusesWhileAnotherCapsuleIsStillPausing(t *testing.T) {
	f := newPauseFixture(t)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	// A second live capsule appears (another assignment) with its pause held.
	other := f.openSecondAssignment(t)
	f.fake.gate = make(chan struct{})
	code, body := f.mark(t, "?allow_busy=1")
	if code != http.StatusConflict || !strings.Contains(body, `"reason":"pausing"`) || f.markerExists() {
		t.Fatalf("one capsule still pausing: status=%d body=%s marker=%v", code, body, f.markerExists())
	}
	close(f.fake.gate)
	f.rt.waitCapsulePauses()
	if code, body = f.mark(t, "?allow_busy=1"); code != http.StatusOK {
		t.Fatalf("both paused: status=%d body=%s", code, body)
	}
	if !f.fake.IsPaused(other.Binding.CapsuleID) || !f.fake.IsPaused(f.capsuleID) {
		t.Fatal("not every capsule is paused at the ack")
	}
}

func (f *pauseFixture) openSecondAssignment(t *testing.T) types.EngineeringAssignment {
	t.Helper()
	ctx := context.Background()
	seed := f.secondSeed(t, "second")
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-pause-2")
	if _, err := f.s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.BindEngineeringAssignment(ctx, store.SeedEngineeringBindRequest(open, "run:"+open.AssignmentID, "opaque-pause-2")); err != nil {
		t.Fatal(err)
	}
	a, err := f.s.GetEngineeringAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, open.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	f.fake.mu.Lock()
	f.fake.has[a.Binding.CapsuleID] = true
	f.fake.mu.Unlock()
	return a
}

// Failure mode 15's refusal bodies, for the host's cadence (r3): the
// "paused, awaiting ack" refusal (an expired or refused ack) also carries
// holds_new_work, so vmctl keeps asking each minute, under the pre-ack TTL.
func TestPauseAwaitingAckRefusalHoldsNewWork(t *testing.T) {
	f := newPauseFixture(t)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	f.fake.ackErr = errors.New("expired")
	code, body := f.mark(t, "?allow_busy=1")
	if code != http.StatusConflict || !strings.Contains(body, `"reason":"paused_awaiting_ack"`) || !strings.Contains(body, `"holds_new_work":true`) || f.markerExists() {
		t.Fatalf("a refused ack: status=%d body=%s marker=%v", code, body, f.markerExists())
	}
}

// ---- reconcile and resume

func (f *pauseFixture) plannedBoot(reason string) {
	f.rt.bootRestart = PlannedRestart{Reason: reason, Target: "761d94c5", RequestedAt: time.Now()}
	f.rt.bootRestartPlanned = true
}

func (f *pauseFixture) spare(t *testing.T) {
	t.Helper()
	f.fake.mu.Lock()
	defer f.fake.mu.Unlock()
	f.fake.has[f.capsuleID] = false
	f.fake.look[f.capsuleID] = capsule.PauseLookup{Where: capsule.PauseSpared, Record: capsule.PauseRecord{
		Version: capsule.PauseRecordVersion, AssignmentID: f.assignment.AssignmentID, Attempt: 1, CapsuleID: f.capsuleID,
		SubjectDigest: f.assignment.Binding.SubjectDigest, Acked: true, ReleaseCommit: "old-release", UpperFiles: 3, UpperBytes: 30,
		Spec: capsule.SpawnSpec{CapsuleID: f.capsuleID, OwnerRunID: f.assignment.BoundRunID, SizeClass: capsule.SizeClassBuild},
	}}
}

// An in-memory capsule stub for the source preflight of the resume.
type fixedPreflight struct {
	ref, digest string
	err         error
}

func (p fixedPreflight) PreflightSourceSnapshot(context.Context, string) (capsule.SourcePreflight, error) {
	return capsule.SourcePreflight{ArtifactRef: p.ref, SubjectDigest: p.digest}, p.err
}
func (p fixedPreflight) PreflightSourceCommit(context.Context, string) (capsule.SourcePreflight, error) {
	return capsule.SourcePreflight{ArtifactRef: p.ref, SubjectDigest: p.digest}, p.err
}

func (f *pauseFixture) matchingPreflight() fixedPreflight {
	return fixedPreflight{ref: f.assignment.Binding.SourceArtifactRef, digest: strings.TrimPrefix(f.assignment.Binding.SubjectDigest, "sha256:")}
}

// Mode 16: a lifecycle work-item wake or a run end runs the reconcile during
// the pause; the paused capsule (frozen, not active, so unusable) is kept,
// nothing is revoked or armed.
func TestReconcileKeepsAPausedCapsuleOnEveryEntry(t *testing.T) {
	f := newPauseFixture(t)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	a := f.assignment
	for name, entry := range map[string]func() error{
		"trajectory reconcile (lifecycle wake, run end)": func() error {
			return f.rt.ReconcileEngineeringAssignmentsForTrajectory(context.Background(), a.Binding.OwnerID, a.Binding.ComputerID, a.Binding.TrajectoryID)
		},
		"boot sweep": func() error {
			f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())
			return nil
		},
	} {
		if err := entry(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := f.reload(t)
		if got.Disposition.Terminal() || got.CapsuleDisposition != types.EngineeringCapsuleActive {
			t.Fatalf("%s revoked a paused capsule: %+v", name, got)
		}
	}
}

// Modes 2, 8, 12, 25: a markerless boot never resumes: it closes the
// assignment "interrupted by a restart" and deletes the directory.
func TestCrashBootClosesAndDeletesAPausedCapsule(t *testing.T) {
	for _, where := range []capsule.PauseWhere{capsule.PauseSpared, capsule.PauseResidual} {
		t.Run(string(where), func(t *testing.T) {
			f := newPauseFixture(t)
			f.spare(t)
			l := f.fake.look[f.capsuleID]
			l.Where = where
			f.fake.look[f.capsuleID] = l
			f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())
			f.rt.waitPausedResumes()
			got := f.reload(t)
			if !got.Disposition.Terminal() || got.CapsuleDisposition != types.EngineeringCapsuleRevoked {
				t.Fatalf("a crash boot left the paused assignment open: %+v", got)
			}
			if len(f.fake.discarded) == 0 || f.fake.discarded[0] != f.capsuleID {
				t.Fatalf("the paused directory was not deleted: %v", f.fake.discarded)
			}
			if len(f.fake.spawned) != 0 {
				t.Fatal("a crash boot spawned a resumed capsule")
			}
			if len(f.fake.deleteKeep) == 0 || len(f.fake.deleteKeep[len(f.fake.deleteKeep)-1]) != 0 {
				t.Fatalf("the spared sweep kept something after a crash: %v", f.fake.deleteKeep)
			}
		})
	}
}

// Mode 20: a restore-reason marker never resumes a pause (even a same-boot
// residual one); the ordinary planned path closes the absent capsule.
func TestRestoreMarkerBootDoesNotResumeAPause(t *testing.T) {
	f := newPauseFixture(t)
	f.spare(t)
	f.plannedBoot(PlannedRestartSelfDevelopmentRestore)
	f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())
	f.rt.waitPausedResumes()
	if len(f.fake.spawned) != 0 {
		t.Fatal("a restore-reason boot resumed a paused capsule")
	}
	if got := f.reload(t); !got.Disposition.Terminal() {
		t.Fatalf("the assignment stays open over a deleted directory: %+v", got)
	}
	if len(f.fake.discarded) == 0 {
		t.Fatal("the directory was not deleted")
	}
}

// Mode 6, 9, 18: a planned boot with a valid record resumes exactly once as
// an ordinary Spawn with the same capsule id and the seeded upper; the run's
// first turn carries the resume context; a second wake is a no-op.
func TestPlannedBootResumesOnceWithASeededSpawn(t *testing.T) {
	f := newPauseFixture(t)
	f.spare(t)
	f.rt.sourcePreflight = f.matchingPreflight()
	f.plannedBoot(PlannedRestartPlatformRefresh)
	f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())
	f.rt.waitPausedResumes()
	if len(f.fake.spawned) != 1 {
		t.Fatalf("spawns = %d, want 1 (order %v)", len(f.fake.spawned), f.fake.order)
	}
	spec := f.fake.spawned[0]
	if spec.CapsuleID != f.capsuleID || spec.SeedUpper != "/state/.paused/"+f.capsuleID+"/upper" {
		t.Fatalf("spec = %+v, want the same capsule id and the preserved upper as the seed", spec)
	}
	if spec.SourceArtifactRef != f.assignment.Binding.SourceArtifactRef || spec.ExpectedSubjectDigest == "" || spec.SizeClass != capsule.SizeClassBuild {
		t.Fatalf("the spawn did not carry the recorded spec and the staged source: %+v", spec)
	}
	if got := f.reload(t); got.Disposition.Terminal() || got.Disposition != types.EngineeringAssignmentBound || got.CapsuleDisposition != types.EngineeringCapsuleActive {
		t.Fatalf("the resumed assignment = %+v, want still bound and active", got)
	}
	if got := strings.Join(f.fake.order, ","); got != "prepare,spawn,mint" {
		t.Fatalf("order = %s, want prepare (validate and teardown), spawn, mint", got)
	}
	if len(f.fake.completed) != 1 {
		t.Fatalf("complete resume calls = %d", len(f.fake.completed))
	}
	// The resume context reaches the run's first turn, once.
	run := &types.RunRecord{Metadata: map[string]any{"assignment_id": f.assignment.AssignmentID}}
	note := f.rt.takePauseResumeNote(run)
	for _, want := range []string{"paused for a platform update", "kernel state are gone", "rerun them", "/computer/files", "/tmp is empty after the resume", "$HOME and $GOCACHE"} {
		if !strings.Contains(note, want) {
			t.Fatalf("the resume note lacks %q: %s", want, note)
		}
	}
	if again := f.rt.takePauseResumeNote(run); again != "" {
		t.Fatalf("the note was given twice: %s", again)
	}
	// A second resume wake in the same boot: the record is taken, no-op.
	f.rt.armPausedResume(f.assignment)
	f.rt.waitPausedResumes()
	if len(f.fake.spawned) != 1 {
		t.Fatalf("a second wake spawned again (%d)", len(f.fake.spawned))
	}
	if f.rt.reconcilePausedAssignment(context.Background(), f.reload(t), false) {
		t.Fatal("a resumed capsule is still held by the pause guard")
	}
}

// Mode 7 and 25: every resume error closes the assignment "interrupted by a
// restart" and deletes the directory.
func TestEveryResumeErrorClosesTheAssignment(t *testing.T) {
	cases := map[string]func(f *pauseFixture){
		"record mismatch":  func(f *pauseFixture) { f.fake.prepEr = errors.New("record mismatch (digest, counts)") },
		"teardown failure": func(f *pauseFixture) { f.fake.prepEr = errors.New("resume: residual root stayed mounted") },
		"spawn failure":    func(f *pauseFixture) { f.fake.spawnEr = errors.New("mount: invalid argument") },
		"mint failure":     func(f *pauseFixture) { f.fake.mintEr = errors.New("mint refused") },
		"subject unavailable": func(f *pauseFixture) {
			f.rt.sourcePreflight = fixedPreflight{err: errors.New("commit not in the new release repo")}
		},
		"subject drifted": func(f *pauseFixture) {
			f.rt.sourcePreflight = fixedPreflight{ref: "capsule-source-git:commit:other", digest: "other"}
		},
		"binding mismatch": func(f *pauseFixture) {
			f.fake.mu.Lock()
			l := f.fake.look[f.capsuleID]
			l.Record.SubjectDigest = "sha256:somewhere-else"
			f.fake.look[f.capsuleID] = l
			f.fake.mu.Unlock()
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPauseFixture(t)
			f.spare(t)
			f.rt.sourcePreflight = f.matchingPreflight()
			setup(f)
			f.plannedBoot(PlannedRestartPlatformRefresh)
			f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())
			f.rt.waitPausedResumes()
			got := f.reload(t)
			if !got.Disposition.Terminal() {
				t.Fatalf("a failed resume left the assignment open: %+v", got)
			}
			if len(f.fake.discarded) == 0 {
				t.Fatal("a failed resume left the directory")
			}
			if len(f.fake.completed) != 0 {
				t.Fatal("a failed resume completed")
			}
		})
	}
}

// A zero remaining deadline closes before any work.
func TestResumeWithAnExpiredDeadlineClosesBeforeAnyWork(t *testing.T) {
	f := newPauseFixture(t)
	f.spare(t)
	f.rt.sourcePreflight = f.matchingPreflight()
	f.rt.pauseTune.now = func() time.Time { return time.Now().Add(100 * 24 * time.Hour) }
	f.plannedBoot(PlannedRestartPlatformRefresh)
	f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())
	f.rt.waitPausedResumes()
	if !f.reload(t).Disposition.Terminal() || len(f.fake.order) != 0 {
		t.Fatalf("an expired deadline did work: order=%v", f.fake.order)
	}
}

// Mode 10: a queued engineering request at a planned boot is admitted only
// after the paused assignments are resolved.
func TestQueuedAdmissionWaitsForThePausedResume(t *testing.T) {
	f := newPauseFixture(t)
	f.spare(t)
	f.rt.sourcePreflight = f.matchingPreflight()
	f.fake.prepGate = make(chan struct{})
	f.plannedBoot(PlannedRestartPlatformRefresh)
	var scheduled []string
	var mu sync.Mutex
	f.rt.scheduleActor = func(_ context.Context, _, _, _, kind, _, _, _ string, _ time.Time) error {
		mu.Lock()
		scheduled = append(scheduled, kind)
		mu.Unlock()
		return nil
	}
	// A queued request opened before the boot: open and unbound.
	ctx := context.Background()
	seed := f.secondSeed(t, "queued")
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-queued")
	if _, err := f.s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	f.rt.bootedAt = time.Now().Add(time.Minute)
	f.rt.kernelMode = true
	f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(ctx)
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	early := len(scheduled)
	mu.Unlock()
	if early != 0 {
		t.Fatalf("a queued request was admitted (%d wakes) while a paused assignment was unresolved", early)
	}
	close(f.fake.prepGate)
	f.rt.waitPausedResumes()
	pauseEventually(t, "the queued request's wake after the resume", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(scheduled) > 0
	})
	if len(f.fake.spawned) != 1 {
		t.Fatalf("spawns = %d", len(f.fake.spawned))
	}
}

// Mode 11: a bound-deadline wake during the pause defers to the resume.
func TestDeadlineWakeDefersToThePause(t *testing.T) {
	f := newPauseFixture(t)
	f.spare(t)
	content, _ := json.Marshal(assignedEngineeringFateDeadline{AssignmentID: f.assignment.AssignmentID, Attempt: 1, Stage: assignedEngineeringBoundDeadlineStage})
	a := f.assignment
	t.Setenv("CHOIR_ASSIGNMENT_DEADLINE", "1ns")
	time.Sleep(2 * time.Millisecond)
	if err := f.rt.HandleAssignedEngineeringFateDeadline(context.Background(), a.Binding.OwnerID, a.Binding.ComputerID, a.Binding.ParentAgentID, string(content)); err != nil {
		t.Fatal(err)
	}
	if got := f.reload(t); got.Disposition.Terminal() {
		t.Fatalf("a deadline wake cancelled an assignment with a pause record: %+v", got)
	}
}

// Mode 23: a channel_message or coagent_result wake for the paused run at a
// planned boot defers until the resume completes or closes. Every activation
// path ends in ExecuteActivationSyncChecked.
func TestRunActivationDefersWhileItsCapsuleIsPausedOrResuming(t *testing.T) {
	f := newPauseFixture(t)
	run := &types.RunRecord{
		RunID: f.assignment.BoundRunID, OwnerID: f.assignment.Binding.OwnerID, ComputerID: f.assignment.Binding.ComputerID,
		AgentProfile: "engineering", State: types.RunPending,
		Metadata: map[string]any{"assignment_id": f.assignment.AssignmentID},
	}
	if fromStore, err := f.s.GetLifecycleRun(context.Background(), run.OwnerID, run.ComputerID, run.RunID); err == nil {
		run = &fromStore
	}
	if !assignedEngineeringRun(run) {
		t.Skip("the seeded run is not an assigned engineering run in this tree")
	}
	for name, where := range map[string]capsule.PauseWhere{"spared (planned boot, resume pending)": capsule.PauseSpared, "residual (same-boot)": capsule.PauseResidual, "live pause": capsule.PauseLive} {
		f.fake.mu.Lock()
		f.fake.look[f.capsuleID] = capsule.PauseLookup{Where: where, Record: capsule.PauseRecord{AssignmentID: f.assignment.AssignmentID}}
		f.fake.mu.Unlock()
		if err := f.rt.ExecuteActivationSyncChecked(context.Background(), run); !errors.Is(err, ErrActivationOccurrenceMustRemainUnprocessed) {
			t.Fatalf("%s: activation = %v, want ErrActivationOccurrenceMustRemainUnprocessed", name, err)
		}
	}
	// In-flight resume: the record is already consumed but the saga runs.
	f.fake.mu.Lock()
	f.fake.look[f.capsuleID] = capsule.PauseLookup{Where: capsule.PauseConsumed}
	f.fake.mu.Unlock()
	f.rt.pauses.mu.Lock()
	f.rt.pauses.resuming = map[string]bool{f.capsuleID: true}
	f.rt.pauses.mu.Unlock()
	if err := f.rt.pausedActivationDeferral(context.Background(), run); !errors.Is(err, ErrActivationOccurrenceMustRemainUnprocessed) {
		t.Fatalf("a resume in flight: %v", err)
	}
	// Completed or closed: the gate lifts.
	f.rt.pauses.mu.Lock()
	f.rt.pauses.resuming = nil
	f.rt.pauses.mu.Unlock()
	if err := f.rt.pausedActivationDeferral(context.Background(), run); err != nil {
		t.Fatalf("after the resume: %v", err)
	}
	f.fake.mu.Lock()
	delete(f.fake.look, f.capsuleID)
	f.fake.mu.Unlock()
	if err := f.rt.pausedActivationDeferral(context.Background(), run); err != nil {
		t.Fatalf("after the close: %v", err)
	}
}

// Decision 1: capsulePauseState fails closed. An unreadable record reads as
// live (not paused), so the gate never excludes it.
func TestCapsulePauseStateFailsClosedOnAnError(t *testing.T) {
	f := newPauseFixture(t)
	f.fake.mu.Lock()
	f.fake.look[f.capsuleID] = capsule.PauseLookup{Err: errors.New("unreadable")}
	f.fake.mu.Unlock()
	if got := f.rt.capsulePauseState(f.assignment); got.Where != capsule.PauseNone {
		t.Fatalf("an unreadable record reads as %q, want live", got.Where)
	}
	if set := f.rt.committedPausedAssignments([]types.EngineeringAssignment{f.assignment}, time.Now()); len(set) != 0 {
		t.Fatalf("the gate excluded an unreadable record: %v", set)
	}
}

// Expired and uncommitted records are not paused for phase 2.
func TestGateOnlyExcludesCommittedUnexpiredRecords(t *testing.T) {
	f := newPauseFixture(t)
	now := time.Now()
	set := func(rec capsule.PauseRecord) map[string]bool {
		rec.AssignmentID = f.assignment.AssignmentID
		f.fake.mu.Lock()
		f.fake.look[f.capsuleID] = capsule.PauseLookup{Where: capsule.PauseLive, Record: rec}
		f.fake.mu.Unlock()
		return f.rt.committedPausedAssignments([]types.EngineeringAssignment{f.assignment}, now)
	}
	if len(set(capsule.PauseRecord{ExpiresAt: now.Add(time.Minute)})) != 1 {
		t.Fatal("a committed unexpired record is not excluded")
	}
	if len(set(capsule.PauseRecord{ExpiresAt: now.Add(-time.Second)})) != 0 {
		t.Fatal("an expired unacked record is excluded")
	}
	if len(set(capsule.PauseRecord{ExpiresAt: now.Add(-time.Hour), Acked: true})) != 1 {
		t.Fatal("an acked record is bounded by the marker, not its pre-ack expiry")
	}
	f.fake.mu.Lock()
	f.fake.look[f.capsuleID] = capsule.PauseLookup{}
	f.fake.mu.Unlock()
	if len(f.rt.committedPausedAssignments([]types.EngineeringAssignment{f.assignment}, now)) != 0 {
		t.Fatal("no record is excluded")
	}
}

// Mode 20 (spare decision): only a fresh planned platform-update marker, of
// this binary's epoch, spares paused directories; evaluated at the same
// instant the consume uses.
func TestPlannedRestartSparesPausedCapsulesIsReasonAware(t *testing.T) {
	dir := t.TempDir()
	store := dir + "/state.db"
	now := time.Now().UTC()
	if PlannedRestartSparesPausedCapsules(store, now) {
		t.Fatal("no marker spares")
	}
	for name, tc := range map[string]struct {
		marker PlannedRestart
		want   bool
	}{
		"platform update": {PlannedRestart{Reason: PlannedRestartPlatformRefresh, RequestedAt: now.Add(-time.Minute)}, true},
		"restore":         {PlannedRestart{Reason: PlannedRestartSelfDevelopmentRestore, RequestedAt: now.Add(-time.Minute)}, false},
		"stale platform":  {PlannedRestart{Reason: PlannedRestartPlatformRefresh, RequestedAt: now.Add(-plannedRestartMaxAge - time.Minute)}, false},
		"future-dated":    {PlannedRestart{Reason: PlannedRestartPlatformRefresh, RequestedAt: now.Add(time.Hour)}, false},
		"other reason":    {PlannedRestart{Reason: "something else", RequestedAt: now.Add(-time.Minute)}, false},
	} {
		if err := WritePlannedRestartMarker(store, tc.marker); err != nil {
			t.Fatal(err)
		}
		if got := PlannedRestartSparesPausedCapsules(store, now); got != tc.want {
			t.Errorf("%s: spares = %v, want %v", name, got, tc.want)
		}
		// The check is non-consuming.
		if _, err := os.Stat(PlannedRestartMarkerPath(store)); err != nil {
			t.Fatalf("%s: the check consumed the marker: %v", name, err)
		}
	}
	// A work-state epoch of another binary is a crash boot.
	raw, _ := json.Marshal(PlannedRestart{Reason: PlannedRestartPlatformRefresh, RequestedAt: now.Add(-time.Minute), WorkStateEpoch: CurrentWorkStateEpoch + 1})
	if err := os.WriteFile(PlannedRestartMarkerPath(store), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if PlannedRestartSparesPausedCapsules(store, now) {
		t.Fatal("a marker of another work-state epoch spares")
	}
}

// ForceDestroy (owner cancel, a terminal trajectory) clears the pause entry
// and its TTL, so no unpause retries run against a destroyed capsule.
func TestRevokeClearsThePauseEntry(t *testing.T) {
	f := newPauseFixture(t)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	f.rt.dropCapsulePause(f.capsuleID)
	f.rt.pauses.mu.Lock()
	n := len(f.rt.pauses.entries)
	f.rt.pauses.mu.Unlock()
	if n != 0 {
		t.Fatalf("the pause entry survived a revoke (%d)", n)
	}
	time.Sleep(50 * time.Millisecond)
	if f.unpausedCount() != 0 {
		t.Fatal("a dropped entry unpaused a destroyed capsule")
	}
}

// A computer with a pause surface and nothing to pause marks as it always did,
// allow_busy or not: the ack re-check must not turn an idle mark into a refusal.
func TestIdleMarkWithAPauseSurfaceStillWritesTheMarker(t *testing.T) {
	for _, query := range []string{"", "?allow_busy=1"} {
		rt, _ := testRuntime(t)
		t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
		_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
		rt.pauseRuntime = newFakePause()
		handler := &APIHandler{rt: rt}
		if code, body := postPlannedRestartURL(t, handler, "/internal/runtime/planned-restart"+query, `{"target":"761d94c5"}`); code != http.StatusOK {
			t.Fatalf("idle mark %q: status=%d body=%s, want 200", query, code, body)
		}
		if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); err != nil {
			t.Fatalf("idle mark %q: no marker: %v", query, err)
		}
	}
}

// B1: a later entry's refused ack leaves no marker; the capsule acked earlier
// in the same call must not stay frozen for ever (the TTL path refuses once
// acked). The watchdog starts with the ack and unpauses it.
func TestPauseRefusedLaterAckDoesNotStrandTheEarlierAckedCapsule(t *testing.T) {
	f := newPauseFixture(t)
	f.rt.pauseTune.watchdog = 15 * time.Millisecond
	other := f.openSecondAssignment(t)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	ids := []string{f.capsuleID, other.Binding.CapsuleID}
	if ids[0] > ids[1] {
		ids[0], ids[1] = ids[1], ids[0]
	}
	f.fake.mu.Lock()
	f.fake.ackFail = map[string]error{ids[1]: errors.New("expired between the clock read and the ack")}
	f.fake.mu.Unlock()
	code, body := f.mark(t, "?allow_busy=1")
	if code != http.StatusConflict || !strings.Contains(body, `"reason":"paused_awaiting_ack"`) || f.markerExists() {
		t.Fatalf("refused later ack: status=%d body=%s marker=%v", code, body, f.markerExists())
	}
	pauseEventually(t, "the earlier-acked capsule to unpause with no marker pending", func() bool { return !f.fake.IsPaused(ids[0]) })
}

// N4: the exclusion is for allow_busy marks only: an ordinary mark over a
// committed pause refuses as today and writes no marker.
func TestPauseExclusionIsAllowBusyOnly(t *testing.T) {
	f := newPauseFixture(t)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	code, body := f.mark(t, "")
	if code != http.StatusConflict || !strings.Contains(body, `"live_capsules":1`) || f.markerExists() {
		t.Fatalf("ordinary mark over a committed pause: status=%d body=%s marker=%v", code, body, f.markerExists())
	}
}

// N2: a pause that has not committed is held by the ack and refuses it.
func TestPauseAckRefusesWhileAnEntryHasNotCommitted(t *testing.T) {
	f := newPauseFixture(t)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	other := f.openSecondAssignment(t)
	f.fake.gate = make(chan struct{})
	f.rt.startCapsulePause(other)
	refusal, err := f.rt.ackPausedCapsules(context.Background(), true, func() error { return nil })
	if err != nil || refusal == "" {
		t.Fatalf("ack with a pause in progress = %q, %v, want a refusal", refusal, err)
	}
	close(f.fake.gate)
	f.rt.waitCapsulePauses()
}

// N3: an abandoned pause re-arms the bound deadline a deferral may have eaten.
func TestAbandonedPauseRearmsTheDeadline(t *testing.T) {
	f := newPauseFixture(t)
	var mu sync.Mutex
	var kinds []string
	f.rt.kernelMode = true
	f.rt.scheduleActor = func(_ context.Context, _, _, _, kind, _, _, _ string, _ time.Time) error {
		mu.Lock()
		kinds = append(kinds, kind)
		mu.Unlock()
		return nil
	}
	f.fake.pauseEr = fmt.Errorf("%w: token", capsule.ErrPauseRefused)
	f.mark(t, "?allow_busy=1")
	f.rt.waitCapsulePauses()
	mu.Lock()
	defer mu.Unlock()
	for _, k := range kinds {
		if k == assignedEngineeringFateDeadlineUpdateKind {
			return
		}
	}
	t.Fatalf("no deadline wake armed after an abandoned pause (scheduled %v)", kinds)
}

// N1: the activation gate reads the run's own assignment attempt.
func TestPausedActivationDeferralUsesTheRunsAttempt(t *testing.T) {
	f := newPauseFixture(t)
	f.fake.mu.Lock()
	f.fake.look[f.capsuleID] = capsule.PauseLookup{Where: capsule.PauseSpared, Record: capsule.PauseRecord{AssignmentID: f.assignment.AssignmentID}}
	f.fake.mu.Unlock()
	run := &types.RunRecord{
		OwnerID: f.assignment.Binding.OwnerID, ComputerID: f.assignment.Binding.ComputerID, AgentProfile: "engineering",
		Metadata: map[string]any{"assignment_id": f.assignment.AssignmentID, "assignment_attempt": 2},
	}
	if !assignedEngineeringRun(run) {
		t.Skip("not an assigned engineering run in this tree")
	}
	if err := f.rt.pausedActivationDeferral(context.Background(), run); err != nil {
		t.Fatalf("a run of attempt 2 was gated on attempt 1's assignment: %v", err)
	}
	run.Metadata["assignment_attempt"] = 1
	if err := f.rt.pausedActivationDeferral(context.Background(), run); !errors.Is(err, ErrActivationOccurrenceMustRemainUnprocessed) {
		t.Fatalf("attempt 1 = %v, want deferral", err)
	}
}

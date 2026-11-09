package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Pause and resume of a live engineering sandbox across a planned restart
// (docs/design/sandbox-pause-resume-2026-11-09.md, r3). The executor does
// the capsule work (internal/capsule/pause_linux.go); this file is the
// runtime's side: the two-phase mark, the pause table, the TTL and the
// marker-bounded watchdog, and the gate's exclusion of a committed pause.
// The resume and the boot reconcile are in capsule_pause_resume.go.

// assignmentPauseRuntime is the executor surface the pause and resume
// drive. *capsule.Executor implements it; a test substitutes it
// (Runtime.pauseRuntime) to drive the machinery without a guest kernel.
type assignmentPauseRuntime interface {
	PauseCapsule(context.Context, capsule.PauseRequest) (capsule.PauseRecord, error)
	UnpauseCapsule(context.Context, string) error
	AckPause(string, time.Time) error
	IsPaused(string) bool
	LookupPause(string) capsule.PauseLookup
	SparedPausedIDs() []string
	PrepareResume(context.Context, capsule.PauseRecord) (capsule.PausedSeed, error)
	CompleteResume(capsule.PausedSeed) error
	DiscardPause(context.Context, string) error
	DeleteSparedPauses(map[string]bool) error
}

const (
	capsulePauseDrainDefault    = 10 * time.Minute
	capsulePauseTTLDefault      = 10 * time.Minute
	capsulePauseWatchdogDefault = time.Minute
	capsulePauseRetryDefault    = 5 * time.Second
	// capsulePauseRefusalPausing and ...Awaiting are the reasons the mark
	// refusals carry (both hold new work: the host keeps asking each minute).
	capsulePauseRefusalPausing  = "pausing"
	capsulePauseRefusalAwaiting = "paused_awaiting_ack"
)

// capsulePauseTuning holds the pause's bounds; the zero value is the
// default. Tests shorten them.
type capsulePauseTuning struct {
	drain, ttl, watchdog, unpauseRetry, resumeRetry time.Duration
	now                                             func() time.Time
}

func (t capsulePauseTuning) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

func orDuration(v, d time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return d
}

// capsulePauseEntry is one capsule this process is pausing or holds paused.
// mu is the per-capsule mutex of the r3 atomic ack: the gate check, the ack
// in the record, the marker write and the sync all happen under it, and the
// pre-ack TTL path takes the same mutex and refuses once acked is set.
type capsulePauseEntry struct {
	mu         sync.Mutex
	capsuleID  string
	assignment types.EngineeringAssignment
	committed  bool
	acked      bool
	gone       bool
	timer      *time.Timer
	watching   bool
}

type capsulePauseTable struct {
	mu      sync.Mutex
	entries map[string]*capsulePauseEntry
	// resuming is the capsules whose resume is in flight (or armed) this
	// boot; resumeNotes is the run-context text per assignment, taken once.
	resuming    map[string]bool
	resumeNotes map[string]string
	wg          sync.WaitGroup
	resumeWG    sync.WaitGroup
}

func (t *capsulePauseTable) get(capsuleID string) *capsulePauseEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.entries[capsuleID]
}

func (t *capsulePauseTable) snapshot() []*capsulePauseEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*capsulePauseEntry, 0, len(t.entries))
	for _, e := range t.entries {
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b *capsulePauseEntry) int { return strings.Compare(a.capsuleID, b.capsuleID) })
	return out
}

func (t *capsulePauseTable) remove(e *capsulePauseEntry) {
	t.mu.Lock()
	if t.entries[e.capsuleID] == e {
		delete(t.entries, e.capsuleID)
	}
	t.mu.Unlock()
}

// capsulePauser is the pause surface of the capsule authority, or nil when
// it has none (no executor, a test fake without pause support).
func (rt *Runtime) capsulePauser() assignmentPauseRuntime {
	if rt == nil {
		return nil
	}
	if rt.pauseRuntime != nil {
		return rt.pauseRuntime
	}
	if p, ok := rt.assignedCapsule().(assignmentPauseRuntime); ok {
		return p
	}
	return nil
}

// capsulePauseRecordFor reads the pause record state of an assignment's
// capsule. It fails closed: an IO or parse error reads as not paused
// (live), never as paused (design Decision 1).
func (rt *Runtime) capsulePauseState(assignment types.EngineeringAssignment) capsule.PauseLookup {
	p := rt.capsulePauser()
	capsuleID := strings.TrimSpace(assignment.Binding.CapsuleID)
	if p == nil || capsuleID == "" {
		return capsule.PauseLookup{}
	}
	look := p.LookupPause(capsuleID)
	if look.Err != nil {
		log.Printf("runtime: pause record of capsule %s unreadable, reading it as live: %v", capsuleID, look.Err)
		return capsule.PauseLookup{}
	}
	if look.Where != capsule.PauseConsumed && look.Record.AssignmentID != "" && look.Record.AssignmentID != assignment.AssignmentID {
		return capsule.PauseLookup{}
	}
	return look
}

// capsulePausedOrResuming reports whether the assignment's capsule is held by
// the pause machinery: paused in this process, a record waiting for the
// boot's resume, or a resume in flight. The reconcile entries and the run's
// activation consult it.
func (rt *Runtime) capsulePausedOrResuming(assignment types.EngineeringAssignment) bool {
	if rt == nil || rt.capsulePauser() == nil {
		return false
	}
	capsuleID := strings.TrimSpace(assignment.Binding.CapsuleID)
	rt.pauses.mu.Lock()
	resuming := rt.pauses.resuming[capsuleID]
	_, pausing := rt.pauses.entries[capsuleID]
	rt.pauses.mu.Unlock()
	if resuming || pausing {
		return true
	}
	switch rt.capsulePauseState(assignment).Where {
	case capsule.PauseLive, capsule.PauseSpared, capsule.PauseResidual:
		return true
	}
	return false
}

// committedPausedAssignments is the set of live assignments whose capsule
// carries a committed, unexpired pause record (acked records never expire
// before the marker's own bound). The restart gate's exclusion.
func (rt *Runtime) committedPausedAssignments(assignments []types.EngineeringAssignment, now time.Time) map[string]bool {
	p := rt.capsulePauser()
	if p == nil {
		return nil
	}
	out := map[string]bool{}
	for _, a := range assignments {
		if !plannedRestartLiveCapsule(a) {
			continue
		}
		look := rt.capsulePauseState(a)
		if look.Where != capsule.PauseLive || look.Record.AssignmentID != a.AssignmentID {
			continue
		}
		if look.Record.Acked || now.Before(look.Record.ExpiresAt) {
			out[a.AssignmentID] = true
		}
	}
	return out
}

// capsulePauseEligible reports whether a live assignment's capsule may be
// paused: bound, active, an implementation slot, no fate saga in flight and a
// capsule this executor holds (Decision 3).
func (rt *Runtime) capsulePauseEligible(a types.EngineeringAssignment) bool {
	if a.Disposition != types.EngineeringAssignmentBound || a.CapsuleDisposition != types.EngineeringCapsuleActive ||
		strings.TrimSpace(a.BoundRunID) == "" || strings.TrimSpace(a.Binding.CapsuleID) == "" ||
		a.Binding.Kind != types.EngineeringAssignmentImplementation || assignedEngineeringFatePending(a) {
		return false
	}
	exec := rt.assignedCapsule()
	return exec != nil && exec.HasCapsule(a.Binding.CapsuleID)
}

// plannedRestartGate counts the work a deploy-refresh mark would cut, with
// the two halves of the gate excluding committed pauses (Decision 1), and for
// an allow_busy mark starts the pause of every live capsule when pausing is
// the only thing between the gate and a restart (Decision 2, phase 1). It
// reports pausing when pauses are running or committed-but-not-acked
// capsules remain.
func (rt *Runtime) plannedRestartGate(ctx context.Context, allowBusy bool) (work plannedRestartWork, pausing bool, err error) {
	if rt.capsulePauser() == nil || !rt.liveCapsulesCountable() {
		work, err = rt.plannedRestartBusy(ctx, true)
		return work, false, err
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, computerID)
	if err != nil {
		return plannedRestartWork{}, false, fmt.Errorf("cannot read engineering assignments: %w", err)
	}
	now := rt.pauseTune.clock()
	committed := rt.committedPausedAssignments(assignments, now)
	work, err = rt.plannedRestartBusyExcluding(ctx, true, committed)
	if err != nil || !work.unsafe() || !allowBusy {
		return work, false, err
	}
	if work.SnapshotInFlight || work.SelfdevAwaitingDecision > 0 {
		return work, false, nil
	}
	var candidates []types.EngineeringAssignment
	excluded := map[string]bool{}
	for id := range committed {
		excluded[id] = true
	}
	for _, a := range assignments {
		if !plannedRestartLiveCapsule(a) || committed[a.AssignmentID] {
			continue
		}
		if !rt.capsulePauseEligible(a) {
			return work, false, nil
		}
		candidates = append(candidates, a)
		excluded[a.AssignmentID] = true
	}
	if len(candidates) == 0 {
		return work, false, nil
	}
	after, err := rt.plannedRestartBusyExcluding(ctx, true, excluded)
	if err != nil || after.unsafe() {
		return work, false, err
	}
	for _, a := range candidates {
		rt.startCapsulePause(a)
	}
	return work, true, nil
}

// startCapsulePause begins the background pause of one capsule. It is
// single-flight: a second phase-1 mark joins the pause in progress.
func (rt *Runtime) startCapsulePause(a types.EngineeringAssignment) {
	p := rt.capsulePauser()
	capsuleID := strings.TrimSpace(a.Binding.CapsuleID)
	rt.pauses.mu.Lock()
	if rt.pauses.entries == nil {
		rt.pauses.entries = map[string]*capsulePauseEntry{}
	}
	if _, joined := rt.pauses.entries[capsuleID]; joined {
		rt.pauses.mu.Unlock()
		return
	}
	entry := &capsulePauseEntry{capsuleID: capsuleID, assignment: a}
	rt.pauses.entries[capsuleID] = entry
	rt.pauses.wg.Add(1)
	rt.pauses.mu.Unlock()
	tune := rt.pauseTune
	go func() {
		defer rt.pauses.wg.Done()
		req := capsule.PauseRequest{
			CapsuleID: capsuleID, AssignmentID: a.AssignmentID, Attempt: a.Binding.Attempt,
			SubjectDigest: a.Binding.SubjectDigest, ReleaseCommit: rt.buildCommit(),
			DrainTimeout: orDuration(tune.drain, capsulePauseDrainDefault), TTL: orDuration(tune.ttl, capsulePauseTTLDefault),
		}
		log.Printf("runtime: pausing capsule %s of assignment %s for a platform update", capsuleID, a.AssignmentID)
		rec, err := p.PauseCapsule(context.Background(), req)
		entry.mu.Lock()
		defer entry.mu.Unlock()
		if err != nil {
			// The executor unwound: the capsule is ordinary again and the
			// mark refuses as today (token in the upper, drain timeout).
			entry.gone = true
			rt.pauses.remove(entry)
			log.Printf("runtime: pause of capsule %s abandoned: %v", capsuleID, err)
			return
		}
		entry.committed = true
		wait := time.Until(rec.ExpiresAt)
		if tune.now != nil {
			wait = rec.ExpiresAt.Sub(tune.clock())
		}
		entry.timer = time.AfterFunc(max(wait, 0), func() { rt.capsulePauseExpire(entry) })
		log.Printf("runtime: capsule %s paused (upper %d files, %d bytes; drain %dms freeze %dms digest %dms)", capsuleID, rec.UpperFiles, rec.UpperBytes, rec.DrainMS, rec.FreezeMS, rec.DigestMS)
	}()
}

// waitCapsulePauses waits for pauses in progress (tests, the abandon path).
func (rt *Runtime) waitCapsulePauses() { rt.pauses.wg.Wait() }

// capsulePauseExpire is the pre-ack TTL: it takes the entry's mutex, the one
// the atomic ack holds, and refuses once the record is acked.
func (rt *Runtime) capsulePauseExpire(entry *capsulePauseEntry) {
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.gone || entry.acked {
		return
	}
	log.Printf("runtime: pause of capsule %s was not acknowledged within its ttl; unpausing", entry.capsuleID)
	rt.unpauseEntryLocked(entry)
}

// unpauseEntryLocked thaws the capsule, removes the record and lifts the
// block; on failure it retries until it succeeds. The caller holds entry.mu.
func (rt *Runtime) unpauseEntryLocked(entry *capsulePauseEntry) bool {
	if entry.gone {
		return true
	}
	p := rt.capsulePauser()
	if p == nil {
		return false
	}
	if err := p.UnpauseCapsule(context.Background(), entry.capsuleID); err != nil {
		retry := orDuration(rt.pauseTune.unpauseRetry, capsulePauseRetryDefault)
		log.Printf("runtime: unpause of capsule %s failed, retrying in %s: %v", entry.capsuleID, retry, err)
		time.AfterFunc(retry, func() {
			entry.mu.Lock()
			defer entry.mu.Unlock()
			rt.unpauseEntryLocked(entry)
		})
		return false
	}
	entry.gone, entry.acked = true, false
	if entry.timer != nil {
		entry.timer.Stop()
	}
	rt.pauses.remove(entry)
	// A deadline wake that deferred to the pause is lost with it: arm it again.
	rt.armAssignedEngineeringDeadline(entry.assignment)
	return true
}

// unpauseAllCapsules unpauses every capsule this process holds paused: the
// host cleared its marker (a failed refresh), so no restart is coming.
func (rt *Runtime) unpauseAllCapsules() {
	for _, entry := range rt.pauses.snapshot() {
		entry.mu.Lock()
		if entry.committed || entry.acked {
			rt.unpauseEntryLocked(entry)
		}
		entry.mu.Unlock()
	}
}

// ackPausedCapsules runs write, the marker write and the filesystem sync,
// under every committed pause's mutex after re-checking the gate and
// setting acked in each record (r3: one mutex covers the gate check, the
// ack, the marker write and the sync). A refusal reason is returned when the
// gate no longer holds or an ack was refused (the TTL path will unpause
// it). With no committed pause it only runs write.
func (rt *Runtime) ackPausedCapsules(ctx context.Context, allowBusy bool, write func() error) (refusal string, err error) {
	var held []*capsulePauseEntry
	for _, entry := range rt.pauses.snapshot() {
		entry.mu.Lock()
		if entry.gone || !entry.committed {
			entry.mu.Unlock()
			continue
		}
		held = append(held, entry)
	}
	defer func() {
		for _, entry := range held {
			entry.mu.Unlock()
		}
	}()
	p := rt.capsulePauser()
	if p == nil || !rt.liveCapsulesCountable() {
		return "", write()
	}
	// The gate is re-checked under the locks even with nothing held: the
	// phase-2 gate read a committed record before the locks, and a pre-ack
	// TTL unpause that won the mutex since has thawed the capsule (r3: never
	// a marker over a thawed capsule).
	assignments, lerr := rt.store.ListEngineeringAssignmentsForComputer(ctx, strings.TrimSpace(rt.TextureComputerID()))
	if lerr != nil {
		return "", fmt.Errorf("cannot read engineering assignments: %w", lerr)
	}
	now := rt.pauseTune.clock()
	committed := rt.committedPausedAssignments(assignments, now)
	work, werr := rt.plannedRestartBusyExcluding(ctx, true, committed)
	if werr != nil {
		return "", werr
	}
	if work.unsafe() {
		return capsulePauseRefusalPausing, nil
	}
	for _, entry := range held {
		if !committed[entry.assignment.AssignmentID] && !entry.acked {
			return capsulePauseRefusalAwaiting, nil
		}
	}
	for _, entry := range held {
		if entry.acked {
			continue
		}
		if aerr := p.AckPause(entry.capsuleID, now); aerr != nil {
			log.Printf("runtime: ack of capsule %s pause refused: %v", entry.capsuleID, aerr)
			return capsulePauseRefusalAwaiting, nil
		}
		entry.acked = true
	}
	werr = write()
	for _, entry := range held {
		rt.watchPausedCapsule(entry)
	}
	return "", werr
}

// watchPausedCapsule starts the marker-bounded watchdog of an acked pause:
// each minute it reads the non-consuming marker check and unpauses when the
// marker is gone or older than plannedRestartMaxAge. A fresh marker keeps
// the capsule frozen. The caller holds entry.mu.
func (rt *Runtime) watchPausedCapsule(entry *capsulePauseEntry) {
	if entry.watching || !entry.acked {
		return
	}
	entry.watching = true
	every := orDuration(rt.pauseTune.watchdog, capsulePauseWatchdogDefault)
	storePath := rt.cfg.StorePath
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for range ticker.C {
			entry.mu.Lock()
			if entry.gone {
				entry.mu.Unlock()
				return
			}
			if !plannedRestartMarkerPending(storePath, rt.pauseTune.clock()) {
				log.Printf("runtime: the restart marker for paused capsule %s is gone or stale; unpausing", entry.capsuleID)
				rt.unpauseEntryLocked(entry)
			}
			gone := entry.gone
			entry.mu.Unlock()
			if gone {
				return
			}
		}
	}()
}

// plannedRestartBusyExcluding is plannedRestartBusy with the committed
// pauses left out of both halves of the gate: their capsules leave
// LiveCapsules and their executing operations leave SelfdevActive. An
// operation in any other busy state (accepted, materializing, rollback
// pending) still counts. Used by the deploy-refresh mark only.
func (rt *Runtime) plannedRestartBusyExcluding(ctx context.Context, countCapsules bool, excluded map[string]bool) (plannedRestartWork, error) {
	if len(excluded) == 0 {
		return rt.plannedRestartBusy(ctx, countCapsules)
	}
	work, err := rt.plannedRestartBusyWith(ctx, countCapsules, excluded)
	return work, err
}

// pausedExecutingOperation reports an operation the exclusion leaves out of
// SelfdevActive: executing, and the freeze operation of an excluded assignment.
func pausedExecutingOperation(op selfdev.Operation, excluded map[string]bool) bool {
	if op.State != selfdev.StateExecuting || len(excluded) == 0 {
		return false
	}
	id, ok := freezeKeyAssignment(op.IdempotencyKey)
	return ok && excluded[id]
}

var errCapsulePauseUnavailable = errors.New("capsule pause is unavailable")

// dropCapsulePause forgets the pause entry of a capsule that is being
// destroyed (ForceDestroy clears the executor's side).
func (rt *Runtime) dropCapsulePause(capsuleID string) {
	if rt == nil {
		return
	}
	rt.pauses.mu.Lock()
	entry := rt.pauses.entries[strings.TrimSpace(capsuleID)]
	rt.pauses.mu.Unlock()
	if entry == nil {
		return
	}
	entry.mu.Lock()
	entry.gone = true
	if entry.timer != nil {
		entry.timer.Stop()
	}
	entry.mu.Unlock()
	rt.pauses.remove(entry)
}

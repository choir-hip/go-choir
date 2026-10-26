package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Engineering admission: one live engineering assignment per computer in the
// initial sequential phase, FIFO among open requests, a retryable refusal with
// the work left pending (docs/choir-doctrine.md, the management admission
// ledger; docs/computer-ontology.md; SDC ARCH-10 launch half, launch item
// L3-08; docs/problems/engineering-admission-supersedes-live-assignment-2026-10-21.md).
//
// The request is the assignment row the Cast's open commit wrote. Its arrival
// order is (created_at, assignment id), both durable on that row, so there is
// no queue table and no second authority: every attempt re-derives the order
// and the slot from the rows. A request that has to wait stays open and
// unbound; its spawn wake is re-armed with a retry count (the wake's update id
// is derived from its content, so a retry with the same content would dedup
// into the consumed row) and a growing delay. Admission never cancels live
// work: a stuck live assignment ends at its own deadline or by management's
// explicit Cancel.

// EngineeringAdmissionMailbox is the mailbox admission's own wakes go to:
// the backoff retry, the nudge when an assignment ends, and the planned-boot
// resume. Admission needs nothing from management's actor lane (it takes
// engineeringAssignmentOpenMu and writes assignment rows under their
// lifecycle CAS), and management is always in an activation when an
// assignment ends, reading its report, so a wake on management's mailbox
// waited for that run
// (docs/problems/fp4-nudge-waits-behind-management-activation-2026-10-22.md).
// The open-time spawn wake, minted by the store in the open commit, goes
// here too (store.EngineeringAdmissionMailbox); the handler still accepts a
// wake on the casting agent's mailbox, minted before that move.
const EngineeringAdmissionMailbox = store.EngineeringAdmissionMailbox

// ErrEngineeringAdmissionBusy is the retryable refusal: the request waits.
var ErrEngineeringAdmissionBusy = errors.New("engineering admission busy")

// errEngineeringRequestExpired settles a spawn wake whose request waited past
// the assignment deadline: the request is cancelled, nothing is spawned.
var errEngineeringRequestExpired = errors.New("engineering request expired before admission")

// engineeringAdmissionBusy names the request that must wait and what it waits
// behind. It matches ErrEngineeringAdmissionBusy under errors.Is.
type engineeringAdmissionBusy struct {
	AssignmentID string
	BlockedBy    string
	// Why is "live" (another assignment holds a capsule) or "queued_ahead"
	// (an older open request goes first).
	Why string
}

func (e *engineeringAdmissionBusy) Error() string {
	return fmt.Sprintf("engineering admission busy: assignment %s waits behind %s (%s); the request stays open and retries",
		e.AssignmentID, e.BlockedBy, e.Why)
}

func (e *engineeringAdmissionBusy) Is(target error) bool {
	return target == ErrEngineeringAdmissionBusy
}

const (
	engineeringAdmissionRetryBase = 10 * time.Second
	engineeringAdmissionRetryMax  = time.Minute
)

// engineeringAdmissionRetryDelay is the wait before retry number retry (1 is
// the first): 10s, doubling, capped at one minute. Each attempt re-reads every
// assignment object of the computer, so the poll backs off.
func engineeringAdmissionRetryDelay(retry int) time.Duration {
	delay := engineeringAdmissionRetryBase
	for i := 1; i < retry && delay < engineeringAdmissionRetryMax; i++ {
		delay *= 2
	}
	if delay > engineeringAdmissionRetryMax {
		delay = engineeringAdmissionRetryMax
	}
	return delay
}

// engineeringAssignmentHoldsSlot reports whether the assignment occupies the
// computer's one engineering slot: it is not terminal and it holds or is
// tearing down a capsule (bound, or any capsule fate short of unbound and
// revoked). A terminal assignment never holds the slot; its leaked capsule is
// reclaimed separately.
func engineeringAssignmentHoldsSlot(a types.EngineeringAssignment) bool {
	if a.Disposition.Terminal() {
		return false
	}
	if a.Disposition == types.EngineeringAssignmentBound {
		return true
	}
	switch a.CapsuleDisposition {
	case types.EngineeringCapsuleActive, types.EngineeringCapsuleFreezeRequested, types.EngineeringCapsuleFrozen, types.EngineeringCapsuleRevokeRequested:
		return true
	}
	return false
}

// engineeringRequestExpired reports whether a request has waited past the
// assignment deadline: "assignment deadlines fail rather than hang".
func engineeringRequestExpired(a types.EngineeringAssignment, now time.Time) bool {
	return !a.CreatedAt.IsZero() && now.After(a.CreatedAt.Add(assignedEngineeringDeadline()))
}

// engineeringAdmissionNow is the clock admission decides expiry by.
func (rt *Runtime) engineeringAdmissionNow() time.Time {
	if rt.admissionNow != nil {
		return rt.admissionNow().UTC()
	}
	return time.Now().UTC()
}

// engineeringRequestQueuedAhead reports whether other is an open, unbound,
// unexpired request that arrived before this one.
func engineeringRequestQueuedAhead(other, this types.EngineeringAssignment, now time.Time) bool {
	if other.Disposition != types.EngineeringAssignmentOpen || other.CapsuleDisposition != types.EngineeringCapsuleUnbound ||
		strings.TrimSpace(other.BoundRunID) != "" || engineeringRequestExpired(other, now) {
		return false
	}
	if !other.CreatedAt.Equal(this.CreatedAt) {
		return other.CreatedAt.Before(this.CreatedAt)
	}
	return other.AssignmentID < this.AssignmentID
}

func sameEngineeringAssignment(a, b types.EngineeringAssignment) bool {
	return a.AssignmentID == b.AssignmentID && a.Binding.Attempt == b.Binding.Attempt
}

// admitEngineeringAssignment is the one admission function of the spawn saga.
// nil admits the request; a *engineeringAdmissionBusy refuses it, retryably.
// The caller holds engineeringAssignmentOpenMu, so no other saga binds a
// capsule while the rows are read. It writes only the cancel of this
// request when it has expired. Last it asks the carry whether the request
// must wait for its trajectory's pending candidate (carryHoldFor).
func (rt *Runtime) admitEngineeringAssignment(ctx context.Context, assignment types.EngineeringAssignment) error {
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, assignment.Binding.ComputerID)
	if err != nil {
		return fmt.Errorf("engineering admission: list assignments: %w", err)
	}
	now := rt.engineeringAdmissionNow()
	if assignment.Disposition == types.EngineeringAssignmentOpen && engineeringRequestExpired(assignment, now) {
		// The saga expires an overdue request before it asks for admission,
		// but the deadline can pass between the two clock readings. The
		// caller reads errEngineeringRequestExpired as "cancelled" and
		// consumes the wake, so the request is cancelled here too (review
		// round 2, R1).
		if err := rt.expireQueuedEngineeringRequest(ctx, assignment); err != nil {
			return fmt.Errorf("expire queued assignment %s: %w", assignment.AssignmentID, err)
		}
		return errEngineeringRequestExpired
	}
	// The slot first: a live assignment outranks any queue position.
	for _, other := range assignments {
		if sameEngineeringAssignment(other, assignment) || !engineeringAssignmentHoldsSlot(other) {
			continue
		}
		return &engineeringAdmissionBusy{AssignmentID: assignment.AssignmentID, BlockedBy: other.AssignmentID, Why: "live"}
	}
	for _, other := range assignments {
		if sameEngineeringAssignment(other, assignment) || !engineeringRequestQueuedAhead(other, assignment, now) {
			continue
		}
		return &engineeringAdmissionBusy{AssignmentID: assignment.AssignmentID, BlockedBy: other.AssignmentID, Why: "queued_ahead"}
	}
	// The carry hold (SD-03, carried_patch.go): an implementation request
	// whose trajectory's newest work still has an open self-development
	// operation waits for that decision, so it builds on the settled
	// candidate instead of starting from the base. A refusal, never a wait
	// under the mutex: nothing that settles an operation goes through here.
	held, err := rt.carryHoldFor(ctx, assignment)
	if err != nil {
		return &engineeringAdmissionBusy{AssignmentID: assignment.AssignmentID,
			BlockedBy: fmt.Sprintf("its trajectory's newest candidate, which could not be read (%v)", err), Why: "carry_unreadable"}
	}
	if held != nil {
		return &engineeringAdmissionBusy{AssignmentID: assignment.AssignmentID,
			BlockedBy: fmt.Sprintf("%s (assignment %s), whose self-development operation %s is still %s", held.CandidateID, held.AssignmentID, held.OperationID, held.OperationState),
			Why:       "candidate_pending"}
	}
	return nil
}

// deferEngineeringAdmission consumes a spawn wake whose request has to wait:
// it re-arms the wake with the next retry count. It returns nil when the wake
// is settled, and an error (so the dispatcher redelivers the original wake)
// when the re-arm could not be made. An overdue request never gets here: it
// is expired before admission (expireOverdueEngineeringRequests).
func (rt *Runtime) deferEngineeringAdmission(ctx context.Context, assignment types.EngineeringAssignment, wake delegatedAssignmentSpawnDeadline, objective string, busy *engineeringAdmissionBusy) error {
	binding := assignment.Binding
	if wake.Nudge != "" {
		// A one-shot nudge: the request's own retry chain still runs.
		log.Printf("runtime: engineering admission: nudge for %s found the slot busy behind %s (%s)", assignment.AssignmentID, busy.BlockedBy, busy.Why)
		return nil
	}
	retry := wake.Retry + 1
	if !rt.kernelMode || rt.scheduleActor == nil {
		return fmt.Errorf("no wake available to retry the request: %w", busy)
	}
	content, err := json.Marshal(delegatedAssignmentSpawnDeadline{
		AssignmentID: assignment.AssignmentID, Attempt: binding.Attempt, Objective: objective, CandidateID: wake.CandidateID, Retry: retry, Boot: wake.Boot,
	})
	if err != nil {
		return err
	}
	delay := engineeringAdmissionRetryDelay(retry)
	if err := rt.scheduleActor(ctx, binding.OwnerID, binding.ComputerID, EngineeringAdmissionMailbox,
		delegatedAssignmentSpawnDeadlineUpdateKind, string(content), binding.TrajectoryID, "", time.Now().UTC().Add(delay)); err != nil {
		return fmt.Errorf("re-arm the admission retry for %s: %w", assignment.AssignmentID, err)
	}
	log.Printf("runtime: engineering admission: assignment %s waits behind %s (%s); retry %d in %s",
		assignment.AssignmentID, busy.BlockedBy, busy.Why, retry, delay)
	return nil
}

// plannedBootKeepsQueuedRequest reports whether an open, unbound assignment
// opened before this boot is a queued request to keep rather than an
// interrupted spawn strand to close (review finding B1). Only a planned boot
// keeps it, and only where a wake can resume it (kernel mode) and no capsule
// was spawned for it. A crash boot never reaches here: it closes every open
// assignment interrupted_by_restart. The rule holds for the whole process
// life, so a later reconcile of the trajectory keeps the request too.
func (rt *Runtime) plannedBootKeepsQueuedRequest(a types.EngineeringAssignment, exec assignmentCapsuleRuntime) bool {
	return rt.bootRestartPlanned && rt.kernelMode && rt.scheduleActor != nil &&
		a.Disposition == types.EngineeringAssignmentOpen && a.CapsuleDisposition == types.EngineeringCapsuleUnbound &&
		strings.TrimSpace(a.BoundRunID) == "" && exec != nil && !exec.HasCapsule(a.Binding.CapsuleID)
}

// resumeQueuedEngineeringRequestsAtPlannedBoot arms the one resume a planned
// restart gives each queued request it kept: a spawn wake due now whose
// content carries this boot's identity, so it never dedups into a consumed
// pre-boot wake. A pre-boot retry wake that also survives runs as well; both
// are ordinary admission attempts under the saga mutex, so the second is a
// no-op or a refusal. If the resume crashes the computer, the next boot has no
// marker and closes the request: one resume per planned restart, no loop. An
// overdue request is expired by the wake, before admission.
func (rt *Runtime) resumeQueuedEngineeringRequestsAtPlannedBoot(ctx context.Context, computerID string) {
	exec := rt.assignedCapsule()
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, computerID)
	if err != nil {
		log.Printf("runtime: planned boot: list queued engineering requests: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, a := range assignments {
		if !a.CreatedAt.Before(rt.bootedAt) || !rt.plannedBootKeepsQueuedRequest(a, exec) {
			continue
		}
		content, err := json.Marshal(delegatedAssignmentSpawnDeadline{AssignmentID: a.AssignmentID, Attempt: a.Binding.Attempt, Boot: rt.bootedAt.UnixNano()})
		if err != nil {
			continue
		}
		if err := rt.scheduleActor(ctx, a.Binding.OwnerID, a.Binding.ComputerID, EngineeringAdmissionMailbox,
			delegatedAssignmentSpawnDeadlineUpdateKind, string(content), a.Binding.TrajectoryID, "", now); err != nil {
			log.Printf("runtime: planned boot: resume queued engineering request %s: %v", a.AssignmentID, err)
			continue
		}
		log.Printf("runtime: planned boot: queued engineering request %s kept and resumed", a.AssignmentID)
	}
}

// expireOverdueEngineeringRequests closes every queued request of the computer
// that has waited past the assignment deadline, this one included, before
// admission is asked (review finding B2): expiry does not depend on the slot
// being busy, and an expired request is never left open while a younger one
// is admitted. The caller holds engineeringAssignmentOpenMu, so no saga is
// inside its spawn window. It returns errEngineeringRequestExpired when this
// request expired, and an error when this request's expiry could not be
// recorded. Another request's failed expiry is logged; it does not block
// admission, because an expired request is out of the queue either way.
func (rt *Runtime) expireOverdueEngineeringRequests(ctx context.Context, assignment types.EngineeringAssignment) error {
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, assignment.Binding.ComputerID)
	if err != nil {
		return fmt.Errorf("engineering admission: list assignments: %w", err)
	}
	now := rt.engineeringAdmissionNow()
	selfExpired := false
	for _, a := range assignments {
		if a.Disposition != types.EngineeringAssignmentOpen || a.CapsuleDisposition != types.EngineeringCapsuleUnbound ||
			strings.TrimSpace(a.BoundRunID) != "" || !engineeringRequestExpired(a, now) {
			continue
		}
		self := sameEngineeringAssignment(a, assignment)
		if err := rt.expireQueuedEngineeringRequest(ctx, a); err != nil {
			if self {
				return fmt.Errorf("expire queued assignment %s: %w", a.AssignmentID, err)
			}
			log.Printf("runtime: engineering admission: expire overdue request %s: %v", a.AssignmentID, err)
			continue
		}
		log.Printf("runtime: engineering admission: assignment %s expired in the queue", a.AssignmentID)
		selfExpired = selfExpired || self
	}
	if selfExpired {
		return errEngineeringRequestExpired
	}
	return nil
}

// expireQueuedEngineeringRequest cancels a queued request that waited past
// the assignment deadline, through the same cancel as management's Cancel.
func (rt *Runtime) expireQueuedEngineeringRequest(ctx context.Context, a types.EngineeringAssignment) error {
	parent := types.RunRecord{
		RunID: a.Binding.ParentRunID, OwnerID: a.Binding.OwnerID, ComputerID: a.Binding.ComputerID, AgentID: a.Binding.ParentAgentID,
	}
	reason := fmt.Sprintf("queued request expired: waited past the assignment deadline (%s) without admission; the request is cancelled",
		assignedEngineeringDeadline())
	_, err := rt.cancelAssignedEngineering(ctx, parent, a.AssignmentID, a.Binding.Attempt, reason)
	return err
}

// wakeNextQueuedEngineeringRequest asks the first queued request of the
// computer, in arrival order, to try admission now, after freed (an
// assignment that just turned terminal) left the slot; otherwise it would wait
// for its next backoff retry. The wake is a one-shot (Nudge) with an identity
// per freed assignment. Admission still decides under the saga mutex, so a
// nudge is only ever early, never wrong. A pre-boot request after a crash boot
// is never woken: the crash rule closes it instead.
func (rt *Runtime) wakeNextQueuedEngineeringRequest(ctx context.Context, freed types.EngineeringAssignment) {
	if rt == nil || rt.store == nil || !rt.kernelMode || rt.scheduleActor == nil || !freed.Disposition.Terminal() {
		return
	}
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, freed.Binding.ComputerID)
	if err != nil {
		log.Printf("runtime: engineering admission: list requests after %s ended: %v", freed.AssignmentID, err)
		return
	}
	now := rt.engineeringAdmissionNow()
	var next *types.EngineeringAssignment
	for i := range assignments {
		a := assignments[i]
		if sameEngineeringAssignment(a, freed) || a.Disposition != types.EngineeringAssignmentOpen ||
			a.CapsuleDisposition != types.EngineeringCapsuleUnbound || strings.TrimSpace(a.BoundRunID) != "" {
			continue
		}
		if !rt.bootRestartPlanned && a.CreatedAt.Before(rt.bootedAt) {
			continue
		}
		if engineeringRequestExpired(a, now) {
			// An expired head is out of the queue; the first live request
			// behind it is the one to wake (review round 2, R4).
			continue
		}
		if next == nil || engineeringRequestQueuedAhead(a, *next, now) {
			next = &assignments[i]
		}
	}
	if next == nil {
		return
	}
	content, err := json.Marshal(delegatedAssignmentSpawnDeadline{AssignmentID: next.AssignmentID, Attempt: next.Binding.Attempt, Nudge: freed.AssignmentID})
	if err != nil {
		return
	}
	if err := rt.scheduleActor(ctx, next.Binding.OwnerID, next.Binding.ComputerID, EngineeringAdmissionMailbox,
		delegatedAssignmentSpawnDeadlineUpdateKind, string(content), next.Binding.TrajectoryID, "", now); err != nil {
		log.Printf("runtime: engineering admission: wake %s after %s ended: %v", next.AssignmentID, freed.AssignmentID, err)
	}
}

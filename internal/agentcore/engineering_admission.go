package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

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

// ErrEngineeringAdmissionBusy is the retryable refusal: the request waits.
var ErrEngineeringAdmissionBusy = errors.New("engineering admission busy")

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
// capsule while the rows are read. It reads and writes nothing else.
func (rt *Runtime) admitEngineeringAssignment(ctx context.Context, assignment types.EngineeringAssignment) error {
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, assignment.Binding.ComputerID)
	if err != nil {
		return fmt.Errorf("engineering admission: list assignments: %w", err)
	}
	now := time.Now().UTC()
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
	return nil
}

// deferEngineeringAdmission consumes a spawn wake whose request has to wait:
// it expires the request when it has waited past the deadline, otherwise it
// re-arms the wake with the next retry count. It returns nil when the wake is
// settled either way, and an error (so the dispatcher redelivers the original
// wake) when neither the cancel nor the re-arm could be made.
func (rt *Runtime) deferEngineeringAdmission(ctx context.Context, assignment types.EngineeringAssignment, wake delegatedAssignmentSpawnDeadline, objective string, busy *engineeringAdmissionBusy) error {
	binding := assignment.Binding
	if engineeringRequestExpired(assignment, time.Now().UTC()) {
		parent := types.RunRecord{
			RunID: binding.ParentRunID, OwnerID: binding.OwnerID, ComputerID: binding.ComputerID, AgentID: binding.ParentAgentID,
		}
		reason := fmt.Sprintf("queued request expired: waited past the assignment deadline (%s) behind %s; request remains pending and retryable",
			assignedEngineeringDeadline(), busy.BlockedBy)
		if _, err := rt.cancelAssignedEngineering(ctx, parent, assignment.AssignmentID, binding.Attempt, reason); err != nil {
			return fmt.Errorf("expire queued assignment %s: %w", assignment.AssignmentID, err)
		}
		log.Printf("runtime: engineering admission: assignment %s expired in the queue behind %s (%s)", assignment.AssignmentID, busy.BlockedBy, busy.Why)
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
	if err := rt.scheduleActor(ctx, binding.OwnerID, binding.ComputerID, binding.ParentAgentID,
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
		if err := rt.scheduleActor(ctx, a.Binding.OwnerID, a.Binding.ComputerID, a.Binding.ParentAgentID,
			delegatedAssignmentSpawnDeadlineUpdateKind, string(content), a.Binding.TrajectoryID, "", now); err != nil {
			log.Printf("runtime: planned boot: resume queued engineering request %s: %v", a.AssignmentID, err)
			continue
		}
		log.Printf("runtime: planned boot: queued engineering request %s kept and resumed", a.AssignmentID)
	}
}

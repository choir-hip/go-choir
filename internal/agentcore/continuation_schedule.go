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

const (
	activationBudgetDeadlineUpdateKind         = "activation_budget_deadline"
	assignedEngineeringFateDeadlineUpdateKind  = "assigned_engineering_fate_deadline"
	delegatedAssignmentSpawnDeadlineUpdateKind = "delegated_assignment_spawn_deadline"
	freshMintManagementDeadlineUpdateKind      = "fresh_mint_management_resume_deadline"
	reactivatedManagementDeadlineUpdateKind    = "reactivated_management_resume_deadline"
	wireReconcilerPublishDeadlineUpdateKind    = "wire_reconciler_publish_deadline"
)

type assignedEngineeringFateDeadline struct {
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
}

// delegatedAssignmentSpawnDeadline is the durable payload that carries a
// delegated cast's spawn inputs to the post-commit wake. The binding stores
// only digests and work-item IDs — not the objective text the bound run needs
// as its prompt — so the saga inputs travel on the wake.
type delegatedAssignmentSpawnDeadline struct {
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
	Objective    string `json:"objective"`
	CandidateID  string `json:"candidate_id,omitempty"`
}

// armDelegatedCastSpawn schedules the durable wake that resumes a delegated
// cast's spawn/bind/activate saga after the cell commits. The commit path
// (commitActIntent) performs only the durable open; running the saga inside
// the reducer would hold the cell lock across capsule spawn (consensus
// precondition). The wake is near-immediate — the saga is latency-sensitive —
// and resumable: re-delivery re-derives the deterministic binding and replays.
func (rt *Runtime) armDelegatedCastSpawn(assignment types.EngineeringAssignment, objective, candidateID string) {
	if rt == nil || rt.store == nil {
		return
	}
	binding := assignment.Binding
	// Only a committed-but-unbound delegated open needs the deferred saga; a
	// bound/terminal or non-delegated assignment is already resolved.
	if binding.CastAuthority != types.EngineeringCastAuthorityDelegated ||
		assignment.Disposition == types.EngineeringAssignmentBound ||
		assignment.Disposition.Terminal() {
		return
	}
	payload, err := json.Marshal(delegatedAssignmentSpawnDeadline{
		AssignmentID: assignment.AssignmentID, Attempt: binding.Attempt,
		Objective: objective, CandidateID: candidateID,
	})
	if err != nil {
		log.Printf("runtime: encode delegated spawn deadline for %s: %v", assignment.AssignmentID, err)
		return
	}
	rt.scheduleContinuation(context.Background(), binding.OwnerID, binding.ComputerID, binding.ParentAgentID,
		delegatedAssignmentSpawnDeadlineUpdateKind, string(payload), binding.TrajectoryID, "", time.Now().UTC())
}

// HandleDelegatedAssignmentSpawnDeadline re-drives a delegated cast's
// spawn/bind/activate saga from its committed-but-unbound open. The saga
// re-derives the exact digests the binding committed (resumeDelegatedCast
// Assignment verifies the subject/capability digests fail-closed), so a
// replayed or duplicated wake is a safe no-op. A bound, terminal, or
// non-delegated assignment returns without re-running.
func (rt *Runtime) HandleDelegatedAssignmentSpawnDeadline(ctx context.Context, ownerID, computerID, agentID, content string) error {
	if rt == nil || rt.store == nil {
		return nil
	}
	var deadline delegatedAssignmentSpawnDeadline
	if err := json.Unmarshal([]byte(content), &deadline); err != nil {
		return fmt.Errorf("decode delegated assignment spawn deadline: %w", err)
	}
	deadline.AssignmentID = strings.TrimSpace(deadline.AssignmentID)
	if deadline.AssignmentID == "" || deadline.Attempt == 0 {
		return fmt.Errorf("delegated spawn deadline requires assignment_id and attempt")
	}
	assignment, err := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, deadline.AssignmentID, deadline.Attempt)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	binding := assignment.Binding
	if binding.CastAuthority != types.EngineeringCastAuthorityDelegated || binding.ParentAgentID != agentID ||
		assignment.Disposition == types.EngineeringAssignmentBound || assignment.Disposition.Terminal() {
		return nil
	}
	req := DelegatedCastRequest{
		Objective:           deadline.Objective,
		Kind:                binding.Kind,
		CandidateID:         deadline.CandidateID,
		CommitmentControlID: binding.ParentControlID,
		CasterAgentID:       binding.ParentAgentID,
	}
	if _, resumeErr := rt.resumeDelegatedCastAssignment(ctx, assignment, req); resumeErr != nil {
		return fmt.Errorf("delegated spawn saga for %s: %w", deadline.AssignmentID, resumeErr)
	}
	return nil
}

func encodeAssignedEngineeringFateDeadline(assignmentID string, attempt uint64) (string, error) {
	content, err := json.Marshal(assignedEngineeringFateDeadline{AssignmentID: assignmentID, Attempt: attempt})
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func decodeAssignedEngineeringFateDeadline(content string) (assignedEngineeringFateDeadline, error) {
	var deadline assignedEngineeringFateDeadline
	if err := json.Unmarshal([]byte(content), &deadline); err != nil {
		return assignedEngineeringFateDeadline{}, err
	}
	deadline.AssignmentID = strings.TrimSpace(deadline.AssignmentID)
	if deadline.AssignmentID == "" || deadline.Attempt == 0 {
		return assignedEngineeringFateDeadline{}, fmt.Errorf("assignment_id and attempt are required")
	}
	return deadline, nil
}

// armAssignedEngineeringDeadline makes the I26 bound-assignment deadline
// derivable from the assignment's durable creation receipt. Delegated casts
// arm it when their durable open lands; every bind path arms it again, so an
// open that takes longer than the deadline to bind still receives an immediate
// expiry wake once it becomes live. The handler is deliberately the predicate
// authority: an unbound, terminal, or pending-fate assignment is a no-op.
func (rt *Runtime) armAssignedEngineeringDeadline(assignment types.EngineeringAssignment) {
	if rt == nil || assignment.Disposition.Terminal() || assignment.CreatedAt.IsZero() {
		return
	}
	content, err := encodeAssignedEngineeringFateDeadline(assignment.AssignmentID, assignment.Binding.Attempt)
	if err != nil {
		log.Printf("runtime: encode assignment %s deadline: %v", assignment.AssignmentID, err)
		return
	}
	rt.scheduleContinuation(context.Background(), assignment.Binding.OwnerID, assignment.Binding.ComputerID,
		assignment.Binding.ParentAgentID, assignedEngineeringFateDeadlineUpdateKind, content,
		assignment.Binding.TrajectoryID, "", assignment.CreatedAt.Add(assignedEngineeringDeadline()))
}

// HandleAssignedEngineeringFateDeadline either resumes a still-pending fate
// saga or fails closed a bound, active assignment whose durable deadline has
// elapsed. Both paths re-read durable state, so duplicate delivery is a no-op.
func (rt *Runtime) HandleAssignedEngineeringFateDeadline(ctx context.Context, ownerID, computerID, agentID, content string) error {
	deadline, err := decodeAssignedEngineeringFateDeadline(content)
	if err != nil {
		return fmt.Errorf("decode assigned Engineering fate deadline: %w", err)
	}
	assignment, err := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, deadline.AssignmentID, deadline.Attempt)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if assignment.Binding.ParentAgentID != agentID {
		return nil
	}
	if assignedEngineeringFatePending(assignment) {
		return rt.resumeStrandedFateAssignmentIfPending(ctx, ownerID, computerID, deadline.AssignmentID, deadline.Attempt)
	}
	if assignment.Disposition != types.EngineeringAssignmentBound ||
		assignment.CapsuleDisposition != types.EngineeringCapsuleActive ||
		strings.TrimSpace(assignment.BoundRunID) == "" ||
		assignment.CreatedAt.IsZero() ||
		time.Now().UTC().Before(assignment.CreatedAt.Add(assignedEngineeringDeadline())) {
		return nil
	}
	parent := types.RunRecord{
		RunID: assignment.Binding.ParentRunID, OwnerID: assignment.Binding.OwnerID,
		ComputerID: assignment.Binding.ComputerID, AgentID: assignment.Binding.ParentAgentID,
	}
	result, err := rt.cancelAssignedEngineering(ctx, parent, assignment.AssignmentID, assignment.Binding.Attempt,
		fmt.Sprintf("assignment deadline expired after %s; request remains pending and retryable", assignedEngineeringDeadline()))
	if err != nil {
		return fmt.Errorf("cancel expired assignment %s: %w", assignment.AssignmentID, err)
	}
	if !result.Replay {
		log.Printf("runtime: assignment %s failed closed at deadline (attempt %d); request stays pending",
			assignment.AssignmentID, assignment.Binding.Attempt)
	}
	return nil
}

// scheduleContinuation mints the kernel's durable backup for a process-local
// continuation. Legacy delivery still owns the live timer until WithKernelMode
// is the write-fence cutover, so a scheduling failure never disables that timer.
func (rt *Runtime) scheduleContinuation(ctx context.Context, ownerID, computerID, agentID, kind, content, trajectoryID, fromAgentID string, notBefore time.Time) {
	if rt == nil || !rt.kernelMode || rt.scheduleActor == nil || notBefore.IsZero() {
		return
	}
	if err := rt.scheduleActor(ctx, ownerID, computerID, agentID, kind, content, trajectoryID, fromAgentID, notBefore.UTC()); err != nil {
		log.Printf("runtime: schedule continuation kind=%s target=%s: %v", kind, agentID, err)
	}
}

// HandleActivationBudgetDeadline is the durable counterpart to the activation
// context timer. A terminal run (including one cancelled by the surviving
// timer) is already resolved and is therefore an idempotent no-op.
func (rt *Runtime) HandleActivationBudgetDeadline(ctx context.Context, ownerID, computerID, agentID, runID string) error {
	rec, matched, err := rt.scheduledRunMatches(ctx, ownerID, computerID, agentID, runID)
	if err != nil || !matched || rec.State.Terminal() {
		return err
	}
	if err := rt.terminalizeRun(ctx, rec.RunID, rec.OwnerID, "activation budget exceeded: progress deadline reached"); err != nil && !strings.Contains(err.Error(), "cannot cancel") {
		return err
	}
	return nil
}

// HandleFreshMintManagementResumeDeadline re-drives only the exact durable
// run that armed the watchdog; a changed, missing, or terminal run is a no-op.
func (rt *Runtime) HandleFreshMintManagementResumeDeadline(ctx context.Context, ownerID, computerID, agentID, runID string) error {
	rec, matched, err := rt.scheduledRunMatches(ctx, ownerID, computerID, agentID, runID)
	if err != nil || !matched || rec.State.Terminal() {
		return err
	}
	_, err = rt.redriveStrandedFreshMintManagement(ctx, ownerID, runID)
	return err
}

// HandleReactivatedManagementResumeDeadline applies the existing fail-closed
// predicate. Repeated delivery observes the terminal state and does not apply it twice.
func (rt *Runtime) HandleReactivatedManagementResumeDeadline(ctx context.Context, ownerID, computerID, agentID, runID string) error {
	rec, matched, err := rt.scheduledRunMatches(ctx, ownerID, computerID, agentID, runID)
	if err != nil || !matched || rec.State.Terminal() {
		return err
	}
	_, err = rt.failExpiredReactivatedManagementResume(ctx, ownerID, runID, time.Now().UTC())
	return err
}

func (rt *Runtime) scheduledRunMatches(ctx context.Context, ownerID, computerID, agentID, runID string) (types.RunRecord, bool, error) {
	if rt == nil || rt.store == nil {
		return types.RunRecord{}, false, fmt.Errorf("scheduled continuation: store unavailable")
	}
	rec, err := rt.store.GetRunByOwner(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(runID))
	if errors.Is(err, store.ErrNotFound) {
		return types.RunRecord{}, false, nil
	}
	if err != nil {
		return types.RunRecord{}, false, err
	}
	return rec, rec.OwnerID == strings.TrimSpace(ownerID) && rec.ComputerID == strings.TrimSpace(computerID) && rec.AgentID == strings.TrimSpace(agentID), nil
}

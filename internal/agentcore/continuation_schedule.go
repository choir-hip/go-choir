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
	activationBudgetDeadlineUpdateKind        = "activation_budget_deadline"
	assignedEngineeringFateDeadlineUpdateKind = "assigned_engineering_fate_deadline"
	freshMintManagementDeadlineUpdateKind     = "fresh_mint_management_resume_deadline"
	reactivatedManagementDeadlineUpdateKind   = "reactivated_management_resume_deadline"
	wireReconcilerPublishDeadlineUpdateKind   = "wire_reconciler_publish_deadline"
)

type assignedEngineeringFateDeadline struct {
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
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

// HandleAssignedEngineeringFateDeadline resumes a still-pending fate saga.
// The saga's durable predicate makes a second delivery a no-op.
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
	return rt.resumeStrandedFateAssignmentIfPending(ctx, ownerID, computerID, deadline.AssignmentID, deadline.Attempt)
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

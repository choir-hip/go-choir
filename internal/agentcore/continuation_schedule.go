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
	cellTerminalDeadlineUpdateKind             = "cell_terminal_deadline"
	assignedEngineeringFateDeadlineUpdateKind  = "assigned_engineering_fate_deadline"
	delegatedAssignmentSpawnDeadlineUpdateKind = "delegated_assignment_spawn_deadline"
	freshMintManagementDeadlineUpdateKind      = "fresh_mint_management_resume_deadline"
	reactivatedManagementDeadlineUpdateKind    = "reactivated_management_resume_deadline"
	selfdevMaterializationRetryKind            = "selfdev_materialization_retry"
)

type assignedEngineeringFateDeadline struct {
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
	// Stage is "bound" on the bound-assignment deadline armed at bind. The
	// wake's update id is derived from its content, so without it the bind
	// arm would dedup into an earlier {assignment_id, attempt} wake (an
	// open-time arm or a fate watchdog) that may already be consumed.
	Stage string `json:"stage,omitempty"`
}

const assignedEngineeringBoundDeadlineStage = "bound"

// delegatedAssignmentSpawnDeadline is the durable spawn obligation minted
// atomically inside the assignment Open commit (actorWakeOutboxFromObject).
// Objective/candidate are optional overrides for legacy payloads; the handler
// derives them from the assigned work item and binding.
type delegatedAssignmentSpawnDeadline struct {
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
	Objective    string `json:"objective"`
	CandidateID  string `json:"candidate_id,omitempty"`
	// Retry counts the admission retries of a request that had to wait for the
	// engineering slot (engineering_admission.go). Zero is the wake the open
	// commit minted. It is part of the content because a wake's update id is
	// derived from its content: a retry with unchanged content would dedup
	// into the consumed row.
	Retry int `json:"retry,omitempty"`
	// Boot is set (the boot's start, unix nanoseconds) on the one resume a
	// planned boot arms for a queued request, and carried by its retries, so
	// that chain has ids of its own (engineering_admission.go).
	Boot int64 `json:"boot,omitempty"`
	// Nudge names the assignment whose end freed the slot, on the wake that
	// asks the next queued request to try now. It is a one-shot: a nudge that
	// finds the slot busy again arms nothing, because the request's own retry
	// chain still runs.
	Nudge string `json:"nudge,omitempty"`
}

// cellTerminalDeadline is the complete reduction identity needed to terminate
// a cell after its worker/process is gone. The durable actor wake is the timer;
// no resident process is required to author the timeout fate.
type cellTerminalDeadline struct {
	RunID                 string `json:"run_id"`
	OwnerID               string `json:"owner_id"`
	ComputerID            string `json:"computer_id"`
	AgentID               string `json:"agent_id"`
	ChannelID             string `json:"channel_id"`
	CellID                string `json:"cell_id"`
	Cursor                uint64 `json:"cursor"`
	ActivationUpdatedUnix int64  `json:"activation_updated_unix_nano"`
	TrajectoryID          string `json:"trajectory_id,omitempty"`
}

func encodeCellTerminalDeadline(rec *types.RunRecord, scope ReductionScope) (string, error) {
	if rec == nil {
		return "", fmt.Errorf("cell terminal deadline: run is required")
	}
	content, err := json.Marshal(cellTerminalDeadline{
		RunID: rec.RunID, OwnerID: rec.OwnerID, ComputerID: rec.ComputerID,
		AgentID: rec.AgentID, ChannelID: scope.ChannelID, CellID: stableCellID(scope),
		Cursor: scope.Cursor, ActivationUpdatedUnix: rec.UpdatedAt.UTC().UnixNano(),
		TrajectoryID: trajectoryIDForRun(rec),
	})
	return string(content), err
}

func decodeCellTerminalDeadline(content string) (cellTerminalDeadline, error) {
	var deadline cellTerminalDeadline
	if err := json.Unmarshal([]byte(content), &deadline); err != nil {
		return cellTerminalDeadline{}, err
	}
	deadline.RunID = strings.TrimSpace(deadline.RunID)
	deadline.OwnerID = strings.TrimSpace(deadline.OwnerID)
	deadline.ComputerID = strings.TrimSpace(deadline.ComputerID)
	deadline.AgentID = strings.TrimSpace(deadline.AgentID)
	deadline.ChannelID = strings.TrimSpace(deadline.ChannelID)
	deadline.CellID = strings.TrimSpace(deadline.CellID)
	if deadline.RunID == "" || deadline.OwnerID == "" || deadline.ComputerID == "" ||
		deadline.AgentID == "" || deadline.CellID == "" || deadline.ActivationUpdatedUnix == 0 {
		return cellTerminalDeadline{}, fmt.Errorf("run, owner, computer, agent, cell identity, and activation timestamp are required")
	}
	return deadline, nil
}

// armCellTerminalDeadline mints a timeout wake at cell dispatch. It shares the
// activation's hard deadline, so a cell cannot outlive its activation and a
// restart still has the exact cell identity to close.
func (rt *Runtime) armCellTerminalDeadline(ctx context.Context, reduction *rlmCallReduction) {
	if rt == nil || reduction == nil || !reduction.active || reduction.rec == nil {
		return
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return
	}
	content, err := encodeCellTerminalDeadline(reduction.rec, reduction.scope)
	if err != nil {
		log.Printf("runtime: encode cell terminal deadline: %v", err)
		return
	}
	rt.scheduleContinuation(context.WithoutCancel(ctx), reduction.rec.OwnerID, reduction.rec.ComputerID,
		reduction.rec.AgentID, cellTerminalDeadlineUpdateKind, content,
		trajectoryIDForRun(reduction.rec), "", deadline)
}

// HandleCellTerminalDeadline records timeout for an unfinished dispatched
// cell. The fate entry's deterministic identity makes duplicate delivery and a
// late worker result safe. It only cancels the same activation generation that
// armed the deadline; a runtime-restarted run still records the timeout but
// remains available for the re-wake to resume.
func (rt *Runtime) HandleCellTerminalDeadline(ctx context.Context, ownerID, computerID, agentID, content string) error {
	deadline, err := decodeCellTerminalDeadline(content)
	if err != nil {
		return fmt.Errorf("decode cell terminal deadline: %w", err)
	}
	if deadline.OwnerID != strings.TrimSpace(ownerID) || deadline.ComputerID != strings.TrimSpace(computerID) ||
		deadline.AgentID != strings.TrimSpace(agentID) {
		return nil
	}
	rec, matched, err := rt.scheduledRunMatches(ctx, ownerID, computerID, agentID, deadline.RunID)
	if err != nil || !matched || rec.State.Terminal() {
		return err
	}
	scope := ReductionScope{
		FromAgentID: deadline.AgentID, ChannelID: deadline.ChannelID, RunID: deadline.RunID,
		OwnerID: deadline.OwnerID, ComputerID: deadline.ComputerID, CellID: deadline.CellID, Cursor: deadline.Cursor,
	}
	if existing, found, err := CellFate(ctx, rt.store, scope.OwnerID, scope.RunID, scope.CellID); err != nil {
		return fmt.Errorf("load cell deadline fate: %w", err)
	} else if found {
		_ = existing
		return nil
	}
	if err := RecordCellFate(context.WithoutCancel(ctx), rt.store, scope, cellFateTimeout, "cell terminal deadline exceeded"); err != nil {
		return err
	}
	if rec.State.Active() && rec.UpdatedAt.UTC().UnixNano() == deadline.ActivationUpdatedUnix {
		if err := rt.terminalizeRun(context.WithoutCancel(ctx), rec.RunID, rec.OwnerID, "cell terminal deadline exceeded"); err != nil &&
			!strings.Contains(err.Error(), "cannot cancel") {
			return err
		}
	}
	return nil
}

// HandleDelegatedAssignmentSpawnDeadline re-drives an assignment's
// spawn/bind/activate saga from its committed-but-unbound open — shared by
// owner and delegated casts since the wake is minted inside the Open commit.
// The saga re-derives the exact digests the binding committed, so a replayed
// or duplicated wake is a safe no-op. A bound, terminal, or non-open
// assignment returns without re-running.
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
	// The wake comes on admission's own mailbox (the open commit's spawn
	// wake, retries, nudges, boot resumes) or, for a wake minted before the
	// open-time wake moved there, on the casting agent's.
	fromAdmission := agentID == EngineeringAdmissionMailbox
	if (binding.ParentAgentID != agentID && !fromAdmission) ||
		assignment.Disposition == types.EngineeringAssignmentBound || assignment.Disposition.Terminal() ||
		strings.TrimSpace(assignment.BoundRunID) != "" {
		return nil
	}
	// The saga owns engineeringAssignmentOpenMu for the whole spawn→bind
	// window — the wake handler must take the same lock and re-read so a
	// trajectory reconcile can't reap a live saga's pre-bind window, and a
	// committed bind/cancel is observed before external effects.
	rt.engineeringAssignmentOpenMu.Lock()
	defer rt.engineeringAssignmentOpenMu.Unlock()
	assignment, err = rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, deadline.AssignmentID, deadline.Attempt)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	binding = assignment.Binding
	if (binding.ParentAgentID != agentID && !fromAdmission) || assignment.Disposition != types.EngineeringAssignmentOpen ||
		strings.TrimSpace(assignment.BoundRunID) != "" {
		return nil
	}
	objective := strings.TrimSpace(deadline.Objective)
	if objective == "" {
		var objErr error
		if objective, objErr = rt.assignmentObjective(ctx, assignment); objErr != nil {
			return objErr
		}
	}
	if _, resumeErr := rt.resumeDelegatedCastAssignment(ctx, assignment, assignmentSpawnRequest{Objective: objective}); resumeErr != nil {
		if errors.Is(resumeErr, errEngineeringRequestExpired) {
			// The request waited past its deadline and is now cancelled.
			return nil
		}
		// Admission refused the request because the slot is busy or an older
		// request goes first: the work stays pending, the wake is consumed
		// and a retry is armed (or the request expires). Nothing is cancelled.
		var busy *engineeringAdmissionBusy
		if errors.As(resumeErr, &busy) {
			return rt.deferEngineeringAdmission(ctx, assignment, deadline, objective, busy)
		}
		return fmt.Errorf("spawn saga for %s: %w", deadline.AssignmentID, resumeErr)
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

// assignedEngineeringBoundAt is when the assignment's capsule was granted to
// its run: the start of execution, from which the bound deadline runs. Time a
// request spent queued before admission is not execution time (review finding
// B3). It reads the grant attestation the bind committed; a row bound without
// one falls back to its bound run's creation (the bind commit writes both),
// and only then to the assignment's creation.
func (rt *Runtime) assignedEngineeringBoundAt(ctx context.Context, assignment types.EngineeringAssignment) time.Time {
	if att := assignment.GrantPolicyAttestation; att != nil && !att.GrantedAt.IsZero() {
		return att.GrantedAt.UTC()
	}
	if runID := strings.TrimSpace(assignment.BoundRunID); runID != "" && rt.store != nil {
		if run, err := rt.store.GetLifecycleRun(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, runID); err == nil && !run.CreatedAt.IsZero() {
			return run.CreatedAt.UTC()
		}
	}
	return assignment.CreatedAt.UTC()
}

// armAssignedEngineeringDeadline arms the I26 bound-assignment deadline when
// an assignment binds: a wake due at bind plus the deadline, with an identity
// of its own (Stage). An open arms nothing: a queued request's own deadline is
// its spawn wake's expiry (engineering_admission.go). The handler is the
// predicate authority: an unbound, terminal, or pending-fate assignment is a
// no-op.
func (rt *Runtime) armAssignedEngineeringDeadline(assignment types.EngineeringAssignment) {
	if rt == nil || assignment.Disposition.Terminal() || strings.TrimSpace(assignment.BoundRunID) == "" {
		return
	}
	content, err := json.Marshal(assignedEngineeringFateDeadline{
		AssignmentID: assignment.AssignmentID, Attempt: assignment.Binding.Attempt, Stage: assignedEngineeringBoundDeadlineStage,
	})
	if err != nil {
		log.Printf("runtime: encode assignment %s deadline: %v", assignment.AssignmentID, err)
		return
	}
	ctx := context.Background()
	rt.scheduleContinuation(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID,
		assignment.Binding.ParentAgentID, assignedEngineeringFateDeadlineUpdateKind, string(content),
		assignment.Binding.TrajectoryID, "", rt.assignedEngineeringBoundAt(ctx, assignment).Add(assignedEngineeringDeadline()))
}

// HandleAssignedEngineeringFateDeadline either resumes a still-pending fate
// saga or fails closed a bound, active assignment whose durable deadline,
// counted from bind, has elapsed. Both paths re-read durable state, so
// duplicate delivery is a no-op. A wake without the bound stage (an open-time
// arm from before this change, or a watchdog) that finds the run not yet due
// re-arms the bound deadline, so a row bound by an older binary keeps one.
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
		return fateWakeReadRetry("the fate wake's read", deadline.AssignmentID, err)
	}
	if assignment.Binding.ParentAgentID != agentID {
		return nil
	}
	if assignedEngineeringFatePending(assignment) {
		return rt.resumeStrandedFateAssignmentIfPending(ctx, ownerID, computerID, deadline.AssignmentID, deadline.Attempt)
	}
	if assignment.Disposition != types.EngineeringAssignmentBound ||
		assignment.CapsuleDisposition != types.EngineeringCapsuleActive ||
		strings.TrimSpace(assignment.BoundRunID) == "" {
		return nil
	}
	if time.Now().UTC().Before(rt.assignedEngineeringBoundAt(ctx, assignment).Add(assignedEngineeringDeadline())) {
		if deadline.Stage != assignedEngineeringBoundDeadlineStage {
			rt.armAssignedEngineeringDeadline(assignment)
		}
		return nil
	}
	parent := types.RunRecord{
		RunID: assignment.Binding.ParentRunID, OwnerID: assignment.Binding.OwnerID,
		ComputerID: assignment.Binding.ComputerID, AgentID: assignment.Binding.ParentAgentID,
	}
	result, err := rt.cancelAssignedEngineering(ctx, parent, assignment.AssignmentID, assignment.Binding.Attempt,
		fmt.Sprintf("assignment deadline expired after %s from bind; the assignment is cancelled", assignedEngineeringDeadline()))
	if err != nil {
		return fmt.Errorf("cancel expired assignment %s: %w", assignment.AssignmentID, err)
	}
	if !result.Replay {
		log.Printf("runtime: assignment %s failed closed at deadline (attempt %d)",
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

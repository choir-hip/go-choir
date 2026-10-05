package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
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
	engineeringProgressDeadlineUpdateKind      = "engineering_progress_overdue_deadline"
	freshMintManagementDeadlineUpdateKind      = "fresh_mint_management_resume_deadline"
	reactivatedManagementDeadlineUpdateKind    = "reactivated_management_resume_deadline"
	selfdevMaterializationRetryKind            = "selfdev_materialization_retry"
)

type assignedEngineeringFateDeadline struct {
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
}

// delegatedAssignmentSpawnDeadline is the durable spawn obligation minted
// atomically inside the assignment Open commit (actorWakeOutboxFromObject).
// Objective/candidate are optional overrides for legacy payloads; the handler
// derives them from the assigned work item and binding.
type delegatedAssignmentSpawnDeadline struct {
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
	Objective    string `json:"objective"`
	CandidateID  string `json:"candidate_id,omitempty"`
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
	if binding.ParentAgentID != agentID ||
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
	if binding.ParentAgentID != agentID || assignment.Disposition != types.EngineeringAssignmentOpen ||
		strings.TrimSpace(assignment.BoundRunID) != "" {
		return nil
	}
	objective, objErr := rt.assignmentSpawnResumeObjective(ctx, assignment, deadline.Objective)
	if objErr != nil {
		return objErr
	}
	candidateID := strings.TrimSpace(deadline.CandidateID)
	if candidateID == "" {
		candidateID = strings.TrimSpace(binding.SourceCandidateID)
	}
	if binding.CastAuthority == types.EngineeringCastAuthorityDelegated {
		req := DelegatedCastRequest{
			Objective: objective, Kind: binding.Kind, CandidateID: candidateID,
			CommitmentControlID: binding.ParentControlID, CasterAgentID: binding.ParentAgentID,
		}
		if _, resumeErr := rt.resumeDelegatedCastAssignment(ctx, assignment, req); resumeErr != nil {
			return fmt.Errorf("delegated spawn saga for %s: %w", deadline.AssignmentID, resumeErr)
		}
		return nil
	}
	// Owner cast: the spawn obligation rode on the open commit's outbox wake;
	// re-drive the same document-bound resume the desk path uses.
	if _, resumeErr := rt.resumeAssignedEngineeringForDocument(ctx, assignment, OpenDocumentAssignmentRequest{
		Objective: objective, Kind: binding.Kind, CandidateID: candidateID,
	}); resumeErr != nil {
		return fmt.Errorf("owner spawn saga for %s: %w", deadline.AssignmentID, resumeErr)
	}
	return nil
}

// assignmentSpawnResumeObjective resolves the committed objective text for a
// spawn wake: the assigned work item is the durable carrier (Open writes it in
// the same transaction), so the wake itself needs only assignment identity.
func (rt *Runtime) assignmentSpawnResumeObjective(ctx context.Context, assignment types.EngineeringAssignment, wakeObjective string) (string, error) {
	if objective := strings.TrimSpace(wakeObjective); objective != "" {
		return objective, nil
	}
	work, err := rt.store.GetLifecycleWorkItem(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.AssignedWorkItemID)
	if err != nil {
		return "", fmt.Errorf("load assigned work item for spawn resume: %w", err)
	}
	return strings.TrimSpace(work.Objective), nil
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

// HandleEngineeringProgressOverdueDeadline evaluates one derivable
// progress-overdue wake: re-reads the assignment, skips when fresher progress
// superseded the silence anchor or the assignment left the bound/active
// state, and commits exactly one observation packet to the supervisor when
// the window elapsed in silence. The packet rides the producer-report surface
// — Texture for document casts, persistent Management for delegated casts —
// so the supervisor observes "wedged" identically to observing a report.
func (rt *Runtime) HandleEngineeringProgressOverdueDeadline(ctx context.Context, ownerID, computerID, agentID, content string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("runtime store unavailable")
	}
	var payload struct {
		AssignmentID         string `json:"assignment_id"`
		Attempt              uint64 `json:"attempt"`
		SilenceSinceUnixNano int64  `json:"silence_since_unix_nano"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &payload); err != nil {
		return fmt.Errorf("decode engineering progress deadline: %w", err)
	}
	assignment, err := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, payload.AssignmentID, payload.Attempt)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if assignment.Binding.ParentAgentID != agentID ||
		assignment.Disposition != types.EngineeringAssignmentBound ||
		assignment.PendingProposal != nil ||
		assignment.CapsuleDisposition != types.EngineeringCapsuleActive ||
		strings.TrimSpace(assignment.BoundRunID) == "" {
		return nil
	}
	silenceSince := time.Unix(0, payload.SilenceSinceUnixNano).UTC()
	if assignment.UpdatedAt.After(silenceSince) || time.Now().UTC().Before(silenceSince.Add(engineeringProgressReviewWindow())) {
		return nil
	}
	req := types.EmitEngineeringProgressObservationRequest{
		CommandID: fmt.Sprintf("progress-overdue:%s:%d:%d", payload.AssignmentID, payload.Attempt, payload.SilenceSinceUnixNano),
		OwnerID:   ownerID, ComputerID: computerID,
		AssignmentID: payload.AssignmentID, Attempt: payload.Attempt,
		SilenceSinceUnixNano: payload.SilenceSinceUnixNano,
	}
	req.CommandDigest, err = store.ComputeEmitEngineeringProgressObservationDigest(req)
	if err != nil {
		return err
	}
	result, err := rt.store.EmitEngineeringProgressObservation(ctx, req)
	if err != nil {
		return err
	}
	if !result.Replay && result.Update != nil {
		rt.wakeUpdatedCoagent(ctx, *result.Update)
	}
	return nil
}

// engineeringProgressReviewWindow returns the bound-assignment silence window.
// CHOIR_PROGRESS_DEADLINE (a Go duration) overrides the default for staging
// probes and focused tests; it must be constant for a process's lifetime so
// the outbox's derived not-before and the fire-time predicate agree.
func engineeringProgressReviewWindow() time.Duration {
	if v := strings.TrimSpace(os.Getenv("CHOIR_PROGRESS_DEADLINE")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return store.EngineeringProgressReviewWindow
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

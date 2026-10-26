package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// trajectoryKindForRun derives the trajectory kind from the spawn surface.
// v1 keeps the smallest honest set; the settlement-rule edge (frame_lock,
// portfolio M1) says these classifications are reviewed after the first real
// settlement cycle, not defended.
func trajectoryKindForRun(rec *types.RunRecord) types.TrajectoryKind {
	profile := agentProfileForRun(rec)
	switch profile {
	case frozenProcessorProfile:
		return types.TrajectoryKindPublication
	case agentprofile.Conductor, agentprofile.Texture, agentprofile.Email:
		return types.TrajectoryKindDocument
	default:
		return types.TrajectoryKindTask
	}
}

// runRootSubjectRef is the subject ref every run-root row carries: the run
// that minted it. A run-root row's rule requires it, so the rule can validate.
const runRootSubjectRef = "root_loop_id"

// defaultSettlementRuleForKind is the per-kind settlement rule minted as
// data. A run-root row (every kind but publication) settles when its root run
// completes with nothing open (settleRunRootTrajectory); its rule used to
// require no subject refs, which no evaluator accepts, so it never settled
// (docs/problems/run-root-trajectories-never-settle-2026-10-26.md).
func defaultSettlementRuleForKind(kind types.TrajectoryKind) types.SettlementRule {
	switch kind {
	case types.TrajectoryKindPublication:
		return types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true,
			RequiredSubjectRefs: []string{"publish_ref", "edition_ref"}}
	default:
		return types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true,
			RequiredSubjectRefs: []string{runRootSubjectRef}}
	}
}

// stampAndMintTrajectory copies the metadata trajectory_id onto the run
// record's column field and durably mints the trajectory record if this is
// the first run on the trajectory. Minting is additive and best-effort: a
// mint failure is logged, never fails the spawn (M1 invariant: zero
// behavior change).
func (rt *Runtime) stampAndMintTrajectory(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || rec == nil {
		return
	}
	trajectoryID := trajectoryIDForRun(rec)
	if trajectoryID == "" {
		return
	}
	rec.TrajectoryID = trajectoryID
	if rt.store == nil {
		return
	}
	kind := trajectoryKindForRun(rec)
	subjectRefs := map[string]string{}
	if channelID := strings.TrimSpace(rec.ChannelID); channelID != "" {
		subjectRefs["channel_id"] = channelID
	}
	if rec.RunID == trajectoryID || strings.TrimSpace(rec.RequestedByRunID) == "" {
		subjectRefs["root_loop_id"] = rec.RunID
	}
	if processorKey := metadataStringValue(rec.Metadata, runMetadataProcessorKey); processorKey != "" {
		subjectRefs["processor_key"] = processorKey
	}
	if _, err := rt.store.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        rec.OwnerID,
		Kind:           kind,
		SubjectRefs:    subjectRefs,
		Status:         types.TrajectoryLive,
		SettlementRule: defaultSettlementRuleForKind(kind),
	}); err != nil {
		log.Printf("runtime: mint trajectory %s for run %s: %v", trajectoryID, rec.RunID, err)
	}
}

// TrajectoryObligations answers "what is this trajectory waiting on?" from
// durable state alone: the trajectory record, its open work items, and the
// settlement rule evaluated as data.
type TrajectoryObligations struct {
	Trajectory      types.TrajectoryRecord `json:"trajectory"`
	OpenWorkItems   []types.WorkItemRecord `json:"open_work_items"`
	PendingUpdates  int                    `json:"pending_updates"`
	SettlementReady bool                   `json:"settlement_ready"`
	WaitingOn       []string               `json:"waiting_on"`
	// RunRoot is set for a run-root row: what its root run is doing and what
	// that means for the row.
	RunRoot *RunRootVerdict `json:"run_root,omitempty"`
}

// RunRootVerdict reads a run-root trajectory row truthfully: it is the
// registry row of one root run, not a unit of work.
type RunRootVerdict struct {
	RunID    string `json:"run_id"`
	RunState string `json:"run_state"`
	Verdict  string `json:"verdict"`
}

// isRunRootRecord is true for a registry row minted for a root run: not a
// publication, not a lifecycle row, and carrying a root_loop_id that is the
// trajectory's own id (a run that joined someone else's trajectory also stamps
// root_loop_id, with its own run id).
func isRunRootRecord(rec types.TrajectoryRecord) bool {
	root := strings.TrimSpace(rec.SubjectRefs[runRootSubjectRef])
	return rec.Kind != types.TrajectoryKindPublication && rec.LifecycleVersion == 0 &&
		root != "" && root == strings.TrimSpace(rec.TrajectoryID)
}

// TrajectoryObligations loads the trajectory and evaluates its settlement
// rule against open work items and subject refs. It never mutates state;
// M5 is the first consumer that acts on the verdict.
func (rt *Runtime) TrajectoryObligations(ctx context.Context, ownerID, trajectoryID string) (TrajectoryObligations, error) {
	if rt == nil || rt.store == nil {
		return TrajectoryObligations{}, fmt.Errorf("trajectory obligations: runtime store is unavailable")
	}
	trajectory, err := rt.store.GetTrajectory(ctx, ownerID, trajectoryID)
	if err != nil {
		return TrajectoryObligations{}, err
	}
	open, err := rt.store.ListWorkItemsByTrajectory(ctx, ownerID, trajectoryID, true)
	if err != nil {
		return TrajectoryObligations{}, err
	}
	pendingUpdates, err := rt.store.CountPendingWorkerUpdatesByTrajectory(ctx, ownerID, trajectoryID)
	if err != nil {
		return TrajectoryObligations{}, err
	}
	evaluated := trajectory
	var runRoot *RunRootVerdict
	if isRunRootRecord(trajectory) {
		// Rows minted before the rule required root_loop_id carry an empty
		// list that can never validate; read them under the rule they should
		// have had. Nothing rewrites the stored row.
		if len(evaluated.SettlementRule.RequiredSubjectRefs) == 0 {
			evaluated.SettlementRule.RequiredSubjectRefs = []string{runRootSubjectRef}
		}
		runRoot = &RunRootVerdict{RunID: strings.TrimSpace(trajectory.SubjectRefs[runRootSubjectRef])}
	}
	ready, waitingOn := EvaluateTrajectorySettlement(evaluated, len(open))
	if pendingUpdates > 0 {
		ready = false
		waitingOn = append(waitingOn, fmt.Sprintf("%d pending update_coagent update(s)", pendingUpdates))
	}
	if runRoot != nil {
		// After the pending-update check, so the verdict text agrees with it.
		rt.describeRunRoot(ctx, ownerID, trajectory, runRoot, &ready, &waitingOn)
	}
	return TrajectoryObligations{
		Trajectory:      trajectory,
		OpenWorkItems:   open,
		PendingUpdates:  pendingUpdates,
		SettlementReady: ready,
		WaitingOn:       waitingOn,
		RunRoot:         runRoot,
	}, nil
}

// describeRunRoot fills a run-root verdict from the root run's stored state.
// A row is not ready while its root run has not completed.
func (rt *Runtime) describeRunRoot(ctx context.Context, ownerID string, trajectory types.TrajectoryRecord, verdict *RunRootVerdict, ready *bool, waitingOn *[]string) {
	run, err := rt.getRunForComputer(ctx, ownerID, verdict.RunID)
	switch {
	case err != nil:
		verdict.RunState = "unknown"
		verdict.Verdict = "run-root registry row; its root run could not be read"
		*ready = false
		*waitingOn = append(*waitingOn, fmt.Sprintf("root run %s could not be read", verdict.RunID))
	case run.State == types.RunCompleted:
		verdict.RunState = string(run.State)
		switch {
		case trajectory.Status != types.TrajectoryLive:
			verdict.Verdict = fmt.Sprintf("run-root registry row; its root run completed and the row is %s", trajectory.Status)
		case *ready:
			verdict.Verdict = "run-root registry row; its root run completed and nothing is open, so it is ready to settle (a row minted before settle-on-completion stays live until cancelled)"
		default:
			verdict.Verdict = "run-root registry row; its root run completed but work is still open on it"
		}
	default:
		verdict.RunState = string(run.State)
		verdict.Verdict = fmt.Sprintf("run-root registry row; its root run is %s", run.State)
		*ready = false
		*waitingOn = append(*waitingOn, fmt.Sprintf("root run %s is %s", verdict.RunID, run.State))
	}
}

// settleRunRootTrajectory settles a run-root row once its root run has
// completed, nothing is open on the trajectory and no run on it is active.
// A child run or a work item that still holds the trajectory keeps it live.
// Best effort: a failure is logged and never fails the run.
func (rt *Runtime) settleRunRootTrajectory(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || rt.store == nil || rec == nil || rec.State != types.RunCompleted {
		return
	}
	trajectoryID := trajectoryIDForRun(rec)
	if trajectoryID == "" || rec.RunID != trajectoryID || strings.TrimSpace(rec.RequestedByRunID) != "" {
		return
	}
	obligations, err := rt.TrajectoryObligations(ctx, rec.OwnerID, trajectoryID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("runtime: settle run-root trajectory %s: %v", trajectoryID, err)
		}
		return
	}
	if obligations.RunRoot == nil || obligations.Trajectory.Status != types.TrajectoryLive || !obligations.SettlementReady {
		return
	}
	// The store re-checks open work, pending updates and non-terminal runs
	// (passivated included) under the trajectory lock before it settles.
	if _, err := rt.store.SettleTrajectoryIfIdle(ctx, rec.OwnerID, trajectoryID); err != nil &&
		!errors.Is(err, store.ErrLifecycleAuthorityRequired) {
		log.Printf("runtime: settle run-root trajectory %s: %v", trajectoryID, err)
	}
}

// EvaluateTrajectorySettlement evaluates a settlement rule as pure data:
// no store access, no side effects. It returns whether the trajectory may
// settle and, when it may not, what it is waiting on.
func EvaluateTrajectorySettlement(rec types.TrajectoryRecord, openWorkItems int) (bool, []string) {
	if rec.Status != types.TrajectoryLive {
		return false, []string{fmt.Sprintf("trajectory status is %s, not live", rec.Status)}
	}
	if rec.SettlementRule.Version != types.LifecycleReducerVersion {
		return false, []string{fmt.Sprintf("invalid settlement rule version %q", rec.SettlementRule.Version)}
	}
	if !rec.SettlementRule.RequireNoOpenWorkItems {
		return false, []string{"invalid settlement rule: no-open-work predicate is required"}
	}
	var waitingOn []string
	if rec.SettlementRule.RequireNoOpenWorkItems && openWorkItems > 0 {
		waitingOn = append(waitingOn, fmt.Sprintf("%d open work item(s)", openWorkItems))
	}
	if len(rec.SettlementRule.RequiredSubjectRefs) == 0 {
		return false, []string{"invalid settlement rule: required subject refs are required"}
	}
	seenRefs := make(map[string]struct{}, len(rec.SettlementRule.RequiredSubjectRefs))
	for _, rawRef := range rec.SettlementRule.RequiredSubjectRefs {
		ref := strings.TrimSpace(rawRef)
		if ref == "" {
			waitingOn = append(waitingOn, "invalid empty required subject ref")
			continue
		}
		if _, duplicate := seenRefs[ref]; duplicate {
			waitingOn = append(waitingOn, fmt.Sprintf("duplicate required subject ref %q", ref))
			continue
		}
		seenRefs[ref] = struct{}{}
		if strings.TrimSpace(rec.SubjectRefs[ref]) == "" {
			waitingOn = append(waitingOn, fmt.Sprintf("missing subject ref %q", ref))
		}
	}
	return len(waitingOn) == 0, waitingOn
}

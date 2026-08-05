package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// trajectoryKindForRun derives the trajectory kind from the spawn surface.
// v1 keeps the smallest honest set; the settlement-rule edge (frame_lock,
// portfolio M1) says these classifications are reviewed after the first real
// settlement cycle, not defended.
func trajectoryKindForRun(rec *types.RunRecord) types.TrajectoryKind {
	switch agentprofile.Canonical(agentProfileForRun(rec)) {
	case agentprofile.Processor:
		return types.TrajectoryKindPublication
	case agentprofile.Conductor, agentprofile.Texture, agentprofile.Email:
		return types.TrajectoryKindDocument
	default:
		return types.TrajectoryKindTask
	}
}

// defaultSettlementRuleForKind is the per-kind settlement rule minted as
// data. Nothing in M1 acts on the verdict; M5 wires reconciliation to it.
func defaultSettlementRuleForKind(kind types.TrajectoryKind) types.SettlementRule {
	switch kind {
	case types.TrajectoryKindPublication:
		return types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true,
			RequiredSubjectRefs: []string{"publish_ref", "edition_ref"}}
	default:
		return types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true}
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
	ready, waitingOn := EvaluateTrajectorySettlement(trajectory, len(open))
	if pendingUpdates > 0 {
		ready = false
		waitingOn = append(waitingOn, fmt.Sprintf("%d pending update_coagent update(s)", pendingUpdates))
	}
	return TrajectoryObligations{
		Trajectory:      trajectory,
		OpenWorkItems:   open,
		PendingUpdates:  pendingUpdates,
		SettlementReady: ready,
		WaitingOn:       waitingOn,
	}, nil
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

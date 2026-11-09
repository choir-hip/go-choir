package agentcore

import (
	"context"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// selfdevSettledTrajectoryCause is the visible fate of a candidate whose
// trajectory settled before anyone decided it.
const selfdevSettledTrajectoryCause = "trajectory settled without a decision"

// closeSettledTrajectoryCandidates gives every undecided candidate (frozen or
// verified) of a trajectory that just settled a terminal fate, through the
// same close the assignment fate uses (failSelfdevOperation). Nothing can
// verify or decide it any more, and while it is open vmctl defers every deploy
// refresh of the computer
// (docs/problems/orphan-frozen-candidate-holds-deploys-2026-11-09.md).
// awaiting_approval is the owner's decision and stays open. This closes work;
// it never resumes any, so it needs no planned-restart marker.
func (rt *Runtime) closeSettledTrajectoryCandidates(ctx context.Context, computerID, trajectoryID string) {
	computerID, trajectoryID = strings.TrimSpace(computerID), strings.TrimSpace(trajectoryID)
	if rt == nil || rt.selfdevOperations == nil || computerID == "" || trajectoryID == "" {
		return
	}
	open, err := rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateFrozen, selfdev.StateVerified)
	if err != nil {
		log.Printf("selfdev: close candidates of settled trajectory %s: list: %v", trajectoryID, err)
		return
	}
	for _, operation := range open {
		// A clean-rebase attempt names its prior's trajectory but is driven by
		// its own path (clean_rebase.go); it is not this trajectory's work.
		if operation.TrajectoryID != trajectoryID || isCleanRebaseOperation(operation.OperationID) {
			continue
		}
		log.Printf("selfdev: trajectory %s settled; closing undecided candidate %s from %s", trajectoryID, operation.OperationID, operation.State)
		rt.failSelfdevOperationFrom(ctx, operation, selfdevSettledTrajectoryCause, selfdev.StateFrozen, selfdev.StateVerified)
	}
}

// closeSettledTrajectoryCandidatesAtBoot is the catch-up for a settle whose
// observer never ran (a crash between the settle commit and the close): at
// every boot, planned or crash, each frozen or verified candidate whose
// trajectory is already settled or cancelled gets the same fate. It closes
// work and resumes none. The owner comes from the computer's assignments.
func (rt *Runtime) closeSettledTrajectoryCandidatesAtBoot(ctx context.Context, computerID string) {
	if rt == nil || rt.selfdevOperations == nil || rt.store == nil {
		return
	}
	open, err := rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateFrozen, selfdev.StateVerified)
	if err != nil || len(open) == 0 {
		if err != nil {
			log.Printf("selfdev: boot: list undecided candidates: %v", err)
		}
		return
	}
	owners := map[string]string{}
	if assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, computerID); err != nil {
		log.Printf("selfdev: boot: list assignments for undecided candidates: %v", err)
	} else {
		for _, a := range assignments {
			owners[a.Binding.TrajectoryID] = a.Binding.OwnerID
		}
	}
	for _, operation := range open {
		if isCleanRebaseOperation(operation.OperationID) {
			continue
		}
		ownerID := owners[operation.TrajectoryID]
		if ownerID == "" {
			ownerID = strings.TrimSpace(rt.selfdevRouteOwnerID)
		}
		if ownerID == "" {
			continue
		}
		trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, operation.TrajectoryID)
		if err != nil || (trajectory.Status != types.TrajectorySettled && trajectory.Status != types.TrajectoryCancelled) {
			continue
		}
		log.Printf("selfdev: boot: trajectory %s is %s; closing undecided candidate %s from %s", operation.TrajectoryID, trajectory.Status, operation.OperationID, operation.State)
		rt.failSelfdevOperationFrom(ctx, operation, selfdevSettledTrajectoryCause, selfdev.StateFrozen, selfdev.StateVerified)
	}
}

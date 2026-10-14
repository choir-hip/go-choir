package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// openFreezeCandidate opens the promotion candidate for an engineering
// assignment that freezes a change with no candidate yet (Gate 2 track D:
// self-development is not a special operation; any request whose capsule
// result would change the computer becomes a candidate). The candidate is
// keyed by the assignment, bound to the assignment's trajectory, pins the
// current heads, and opens only when the computer's signed mode authorizes
// proposals. Verification, decision, materialization and rollback key off
// the operation unchanged.
func openFreezeCandidate(ctx context.Context, operations *selfdev.Store, authorizeProposal func(context.Context) error, computerID, trajectoryID, assignmentID, objective string) (selfdev.Operation, error) {
	computerID, trajectoryID, assignmentID = strings.TrimSpace(computerID), strings.TrimSpace(trajectoryID), strings.TrimSpace(assignmentID)
	if operations == nil || authorizeProposal == nil || computerID == "" || trajectoryID == "" || assignmentID == "" {
		return selfdev.Operation{}, fmt.Errorf("self-development candidate: incomplete freeze identity")
	}
	// C1e D2: the operation is the assignment's, resolved by its idempotency
	// key. An existing one is returned before the mode is consulted, so a
	// re-freeze after the mode is switched off still reaches its own
	// operation; a retry on the same trajectory opens its own.
	idempotencyKey := store.EngineeringFreezeIdempotencyKey(assignmentID)
	if existing, found, err := operations.GetByIdempotency(ctx, computerID, idempotencyKey); err != nil {
		return selfdev.Operation{}, fmt.Errorf("self-development candidate: %w", err)
	} else if found {
		return advanceFreezeCandidate(ctx, operations, existing)
	}
	if err := authorizeProposal(ctx); err != nil {
		return selfdev.Operation{}, fmt.Errorf("self-development candidate: the computer's mode does not authorize a proposal: %w", err)
	}
	promptRef, err := computerevent.ArtifactRefFromDigest(computerevent.DigestBytes([]byte(strings.TrimSpace(objective))))
	if err != nil {
		return selfdev.Operation{}, err
	}
	operation, err := operations.Start(ctx, selfdev.StartRequest{
		ComputerID: computerID, IdempotencyKey: idempotencyKey, PromptArtifactRef: promptRef.String(),
		OperationID: store.EngineeringFreezeOperationID(computerID, assignmentID), TrajectoryID: trajectoryID,
	})
	if err != nil {
		return selfdev.Operation{}, fmt.Errorf("self-development candidate: open: %w", err)
	}
	return advanceFreezeCandidate(ctx, operations, operation)
}

// pickTrajectoryOperation is the explicit pick a trajectory-wide caller makes
// among a trajectory's operations (C1e D2: each assignment opens its own, so
// a trajectory can carry several). It names the newest operation still in
// flight, or else the newest. ops is in creation order (ListByTrajectory).
func pickTrajectoryOperation(ops []selfdev.Operation) (selfdev.Operation, bool) {
	for i := len(ops) - 1; i >= 0; i-- {
		switch ops[i].State {
		case selfdev.StateFailed, selfdev.StateRejected, selfdev.StateRolledBack, selfdev.StateApplied:
			continue
		}
		return ops[i], true
	}
	if len(ops) == 0 {
		return selfdev.Operation{}, false
	}
	return ops[len(ops)-1], true
}

func advanceFreezeCandidate(ctx context.Context, operations *selfdev.Store, operation selfdev.Operation) (selfdev.Operation, error) {
	if operation.State != selfdev.StateRequested {
		return operation, nil
	}
	return operations.Transition(ctx, operation.ComputerID, operation.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil)
}

// selfDevelopmentProposalAuthorized reports whether the computer's current
// signed mode authorizes opening a proposal. Off and audit-only do not.
func (rt *Runtime) selfDevelopmentProposalAuthorized(ctx context.Context) error {
	if rt == nil || rt.selfdevControl == nil {
		return fmt.Errorf("self-development mode authority unavailable")
	}
	mode, err := rt.selfdevControl.SelfDevelopmentMode(ctx)
	if err != nil {
		return err
	}
	switch strings.TrimSpace(mode.Mode) {
	case platform.SelfDevelopmentModeProposeOnly, platform.SelfDevelopmentModeAcceptOnce, platform.SelfDevelopmentModeQualifiedConsensus:
		return nil
	default:
		return fmt.Errorf("mode is %q", mode.Mode)
	}
}

package agentcore

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Settlement past the judgment (types/commitment_rules.go, R18, R19):
// owner_approves and approval_minutes settle on the owner's act on the
// candidate, and approved_within_rounds on the request's Freezes and its
// approval, across every round cast for the same trajectory. Both run where
// every owner act passes (notifyTextureOfOwnerAct), best effort (R6).
//
// Failure modes, written before the code:
//
//	S1 A consensus or platform decision is counted as the owner's own.
//	   -> the settlement names the decision actor and marks it delegated
//	      when the actor is not the computer's owner.
//	S2 The approval time disagrees with the round receipt.
//	   -> the approve act's record time when there is one, else the act's,
//	      as recordRoundReceipt reads it.
//	S3 approved_within_rounds stays open forever on a request that ends.
//	   -> an abandon act settles it "no"; a request left without an act
//	      stays open (named residual).
//	S4 A Freeze refused and retried counts twice.
//	   -> Freezes are the trajectory's operations that froze a bundle, one
//	      per operation (a reused engineer's revision is its own operation).

// ownerActAssignmentID is the assignment an engineering operation belongs
// to (its freeze-open idempotency key), "" for any other operation.
func ownerActAssignmentID(operation selfdev.Operation) string {
	id, ok := strings.CutPrefix(strings.TrimSpace(operation.IdempotencyKey), "freeze-open:")
	if !ok {
		return ""
	}
	return strings.TrimSpace(id)
}

// settleOwnerActPrecommits settles what an owner act on an engineering
// candidate decides.
func (rt *Runtime) settleOwnerActPrecommits(ctx context.Context, ownerID string, operation selfdev.Operation, act string) {
	if rt == nil || rt.store == nil {
		return
	}
	ownerID = firstNonEmpty(strings.TrimSpace(ownerID), strings.TrimSpace(rt.selfdevRouteOwnerID))
	computerID := firstNonEmpty(strings.TrimSpace(rt.TextureComputerID()), strings.TrimSpace(operation.ComputerID))
	assignmentID := ownerActAssignmentID(operation)
	if ownerID == "" || computerID == "" || assignmentID == "" {
		return
	}
	if act == types.OwnerActFail && !operationApproved(operation) {
		return // a failure that no approval preceded decides nothing here
	}
	ctx = context.WithoutCancel(ctx)
	now := time.Now().UTC()
	decisionAt := now
	if act == types.OwnerActApply || act == types.OwnerActFail {
		if approve, _ := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.OwnerActRecordID(operation.OperationID, types.OwnerActApprove)); approve != nil {
			if t := parseRoundTime(approve.Provenance.CommittedAt); !t.IsZero() {
				decisionAt = t
			}
		}
	}
	actor := strings.TrimSpace(operation.DecisionActor)
	obs := types.EngineeringOwnerActObservation{Act: act, OperationID: operation.OperationID, DecisionAt: decisionAt,
		Actor: actor, Delegated: actor != "" && actor != ownerID}
	rt.settleRulePrecommits(ctx, ownerID, computerID, assignmentID, "owner act "+act, now, func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnOwnerAct(rule, obs)
	})
	rt.settleRequestPrecommits(ctx, ownerID, computerID, operation.TrajectoryID, act == types.OwnerActAbandon)
}

// operationApproved reports an operation whose candidate an approval
// decided (any state past the decision but a rejection).
func operationApproved(op selfdev.Operation) bool {
	return strings.TrimSpace(op.DecisionEvent) != "" && op.State != selfdev.StateRejected
}

// settleRequestPrecommits settles every open approved_within_rounds claim
// of the trajectory's rounds (R19).
func (rt *Runtime) settleRequestPrecommits(ctx context.Context, ownerID, computerID, trajectoryID string, abandoned bool) {
	trajectoryID = strings.TrimSpace(trajectoryID)
	if rt == nil || rt.store == nil || rt.selfdevOperations == nil || trajectoryID == "" {
		return
	}
	ops, err := rt.selfdevOperations.ListByTrajectory(ctx, computerID, trajectoryID)
	if err != nil {
		log.Printf("runtime: trajectory %s approved_within_rounds: %v", trajectoryID, err)
		return
	}
	frozen := make([]selfdev.Operation, 0, len(ops))
	for _, op := range ops {
		if strings.TrimSpace(op.BundleDigest) != "" {
			frozen = append(frozen, op)
		}
	}
	sort.SliceStable(frozen, func(i, j int) bool { return frozen[i].CreatedAt < frozen[j].CreatedAt })
	assignments, err := rt.store.ListEngineeringAssignments(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		log.Printf("runtime: trajectory %s approved_within_rounds: %v", trajectoryID, err)
		return
	}
	now := time.Now().UTC()
	for _, assignment := range assignments {
		rt.settleRulePrecommits(ctx, ownerID, computerID, assignment.AssignmentID, "request", now, func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
			if rule.Kind != types.PrecommitRuleApprovedWithinRounds {
				return types.RuleSettlement{}, false
			}
			since := parseRoundTime(rule.Since)
			obs := types.EngineeringRequestObservation{At: now, Abandoned: abandoned}
			for _, op := range frozen {
				if created := parseRoundTime(op.CreatedAt); !since.IsZero() && created.Before(since) {
					continue
				}
				obs.FreezesSince++
				if operationApproved(op) && !obs.Approved {
					obs.Approved, obs.FreezesToApproval, obs.ApprovedOperationID = true, obs.FreezesSince, op.OperationID
				}
			}
			return types.SettleRuleOnRequest(rule, obs)
		})
	}
}

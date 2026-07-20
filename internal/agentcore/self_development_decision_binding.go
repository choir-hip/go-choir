package agentcore

import (
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

type verifiedSelfDevelopmentDecision struct {
	NextState         string
	Actor             string
	ModeReceiptDigest string
}

func verifyFinalizedSelfDevelopmentDecision(operation selfdev.Operation, transition computerevent.DurableEvent) (verifiedSelfDevelopmentDecision, error) {
	event := transition.Request.Event
	eventDigest, err := event.Digest()
	if err != nil || eventDigest != transition.Request.EventDigest {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: event digest mismatch")
	}
	nextState := selfdev.StateRejected
	if event.EventKind == computerevent.EventEffectAccepted {
		nextState = selfdev.StateAccepted
	} else if event.EventKind != computerevent.EventEffectRejected {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: event kind mismatch")
	}
	actor := strings.TrimPrefix(event.AuthorityRef, "external-owner:")
	if actor == "" || actor == event.AuthorityRef {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: owner authority mismatch")
	}
	if operation.ComputerID != event.ComputerID || operation.OperationID != event.ParentEventID ||
		operation.TrajectoryID == "" || operation.TrajectoryID != event.TrajectoryID ||
		operation.CapsuleID == "" || operation.CapsuleID != event.CapsuleID ||
		operation.BundleDigest == "" || operation.BundleDigest != event.ProposedEffectRef {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: operation identity mismatch")
	}
	if event.SchemaVersion != computerevent.SchemaVersionV1 || event.ActorProfile != agentprofile.Super ||
		event.PrivacyClass != "owner" || event.ReducerVersion != computerevent.ReducerVersionV1 ||
		!computerevent.IsSHA256(event.RequestCommitment) || !computerevent.IsSHA256(event.DecisionRef) {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: event authority contract mismatch")
	}
	if len(event.VerifierRefs) != 1 || !selfDevelopmentContainsString(operation.VerifierRefs, event.VerifierRefs[0]) {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: verifier join mismatch")
	}
	if transition.Receipt.ReceiptKind != "EventHeadReceipt" || transition.Receipt.ReceiptID == "" ||
		transition.Receipt.KindFields["event_digest"] != eventDigest {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: receipt join mismatch")
	}
	modeReceiptDigest := ""
	if len(event.InputArtifactRefs) != 1 {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: mode receipt cardinality mismatch")
	}
	{
		modeReceiptRef, err := computerevent.ParseArtifactRef(event.InputArtifactRefs[0])
		if err != nil {
			return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: mode receipt artifact reference mismatch")
		}
		modeReceiptDigest = modeReceiptRef.Digest().String()
	}
	if operation.State != selfdev.StateAwaitingApproval && !selfDevelopmentDecisionStateDescends(operation.State, nextState) {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: operation state is not a legal decision descendant")
	}
	if operation.DecisionEvent != "" && operation.DecisionEvent != eventDigest {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: operation event projection mismatch")
	}
	if operation.DecisionReceipt != "" && operation.DecisionReceipt != transition.Receipt.ReceiptID {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: operation receipt projection mismatch")
	}
	if operation.DecisionActor != "" && operation.DecisionActor != actor {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: operation actor projection mismatch")
	}
	if operation.State != selfdev.StateAwaitingApproval &&
		(operation.DesiredHead != transition.Request.Next.DesiredEventHead || operation.EffectiveHead != transition.Request.Next.EffectiveEventHead) {
		return verifiedSelfDevelopmentDecision{}, fmt.Errorf("decision binding: operation head projection mismatch")
	}
	return verifiedSelfDevelopmentDecision{NextState: nextState, Actor: actor, ModeReceiptDigest: modeReceiptDigest}, nil
}

func selfDevelopmentDecisionStateDescends(state, decisionState string) bool {
	if decisionState == selfdev.StateRejected {
		return state == selfdev.StateRejected
	}
	switch state {
	case selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateApplied,
		selfdev.StateFailed, selfdev.StateDegraded, selfdev.StateRollbackPending, selfdev.StateRolledBack:
		return true
	default:
		return false
	}
}

package computerevent

import (
	"errors"
	"testing"
)

// package_applied discharges the pending transition without moving the
// platform state's commitment; package_reverted moves no head
// (docs/design/package-operation-decision-2026-11-06.md 2.5).
func TestReducerPackageAppliedAndRevertedLeaveThePlatformStateAlone(t *testing.T) {
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	head, err := Reduce(nil, genesis, TransitionInput{TargetStateCommitment: testDigestA})
	if err != nil {
		t.Fatal(err)
	}
	accepted := testEvent(t, &head, EventEffectAccepted)
	accepted.ProposedEffectRef, accepted.DecisionRef, accepted.VerifierRefs = "artifact:bundle", "decision:consensus", []string{"artifact:verifier"}
	acceptedHead, err := Reduce(&head, accepted, TransitionInput{TargetStateCommitment: testDigestB})
	if err != nil {
		t.Fatal(err)
	}

	// An applied package must name the pending decision and the unchanged
	// effective commitment.
	wrongDecision := testEvent(t, &acceptedHead, EventPackageApplied)
	wrongDecision.DecisionRef, wrongDecision.ResultingEffectiveCommitment = "decision:other", testDigestA
	if _, err := Reduce(&acceptedHead, wrongDecision, TransitionInput{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("package_applied naming another decision: %v", err)
	}
	movedState := testEvent(t, &acceptedHead, EventPackageApplied)
	movedState.DecisionRef, movedState.ResultingEffectiveCommitment = acceptedHead.PendingTransitionRef, testDigestB
	if _, err := Reduce(&acceptedHead, movedState, TransitionInput{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("package_applied that moves the platform commitment: %v", err)
	}
	applied := testEvent(t, &acceptedHead, EventPackageApplied)
	applied.DecisionRef, applied.ResultingEffectiveCommitment = acceptedHead.PendingTransitionRef, testDigestA
	appliedHead, err := Reduce(&acceptedHead, applied, TransitionInput{})
	if err != nil {
		t.Fatal(err)
	}
	appliedDigest, _ := applied.Digest()
	if appliedHead.PendingTransitionRef != "" || appliedHead.EffectiveStateCommitment != testDigestA || appliedHead.DesiredStateCommitment != testDigestA ||
		appliedHead.EffectiveEventHead != appliedDigest || appliedHead.DesiredEventHead != appliedDigest {
		t.Fatalf("package_applied projection = %+v", appliedHead)
	}
	if _, err := Reduce(&head, func() Event {
		e := testEvent(t, &head, EventPackageApplied)
		e.DecisionRef, e.ResultingEffectiveCommitment = "decision:consensus", testDigestA
		return e
	}(), TransitionInput{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("package_applied with nothing pending: %v", err)
	}

	reverted := testEvent(t, &appliedHead, EventPackageReverted)
	if _, err := Reduce(&appliedHead, reverted, TransitionInput{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("package_reverted naming no applied event: %v", err)
	}
	reverted = testEvent(t, &appliedHead, EventPackageReverted)
	reverted.DecisionRef = appliedDigest
	revertedHead, err := Reduce(&appliedHead, reverted, TransitionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if revertedHead.DesiredEventHead != appliedHead.DesiredEventHead || revertedHead.EffectiveEventHead != appliedHead.EffectiveEventHead ||
		revertedHead.EffectiveStateCommitment != appliedHead.EffectiveStateCommitment || revertedHead.PendingTransitionRef != "" {
		t.Fatalf("package_reverted moved a head: before=%+v after=%+v", appliedHead, revertedHead)
	}
}

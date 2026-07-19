package computerevent

import (
	"crypto/sha256"
	"encoding/hex"
)

// ComputePinIntentCommitment binds the complete immutable event intent and
// transition input before any platform PinReceipt exists. Payload PinReceipts
// bind this value; the final append request then binds their receipt digests.
// This directed graph avoids a receipt-digest/request-commitment hash cycle.
func ComputePinIntentCommitment(event Event, input TransitionInput) (string, error) {
	intent := requestIntent(event, input)
	canonical, err := CanonicalJSON(intent)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

// ComputeRequestCommitment binds immutable intent, pin intent, and the exact
// ordered payload PinReceipt digests. Sequence and previous_head remain
// excluded so causal-only events may follow the appender's declared rebase path.
func ComputeRequestCommitment(event Event, input TransitionInput, pinIntentCommitment string, payloadPinReceiptDigests []string) (string, error) {
	intent := map[string]any{
		"event_intent":                requestIntent(event, input),
		"pin_intent_commitment":       pinIntentCommitment,
		"payload_pin_receipt_digests": nonNilStrings(payloadPinReceiptDigests),
	}
	canonical, err := CanonicalJSON(intent)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func requestIntent(event Event, input TransitionInput) map[string]any {
	return map[string]any{
		"schema_version":                       event.SchemaVersion,
		"event_id":                             event.EventID,
		"computer_id":                          event.ComputerID,
		"event_kind":                           event.EventKind,
		"occurred_at":                          event.OccurredAt,
		"idempotency_key":                      event.IdempotencyKey,
		"trajectory_id":                        event.TrajectoryID,
		"parent_event_id":                      event.ParentEventID,
		"capsule_id":                           event.CapsuleID,
		"actor_profile":                        event.ActorProfile,
		"authority_ref":                        event.AuthorityRef,
		"model_policy_refs":                    nonNilStrings(event.ModelPolicyRefs),
		"input_artifact_refs":                  nonNilStrings(event.InputArtifactRefs),
		"output_artifact_refs":                 nonNilStrings(event.OutputArtifactRefs),
		"payload_commitment":                   event.PayloadCommitment,
		"privacy_class":                        event.PrivacyClass,
		"proposed_effect_ref":                  event.ProposedEffectRef,
		"decision_ref":                         event.DecisionRef,
		"verifier_refs":                        nonNilStrings(event.VerifierRefs),
		"reducer_version":                      event.ReducerVersion,
		"expected_desired_event_head":          event.ExpectedDesiredEventHead,
		"expected_effective_event_head":        event.ExpectedEffectiveEventHead,
		"expected_pending_transition_ref":      event.ExpectedPendingTransitionRef,
		"expected_desired_state_commitment":    event.ExpectedDesiredStateCommitment,
		"expected_effective_state_commitment":  event.ExpectedEffectiveStateCommitment,
		"resulting_effective_state_commitment": event.ResultingEffectiveCommitment,
		"transition_input":                     input,
	}
}

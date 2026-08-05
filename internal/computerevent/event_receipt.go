package computerevent

import (
	"bytes"
	"context"
	"fmt"
)

type EventHeadReceiptVerifier struct {
	Keys KeyResolver
}

func (v EventHeadReceiptVerifier) VerifyEventHeadReceipt(_ context.Context, receipt Receipt, request CASRequest) error {
	if v.Keys == nil {
		return fmt.Errorf("event head receipt: key resolver is required")
	}
	if receipt.ReceiptKind != "EventHeadReceipt" || receipt.Issuer != "corpusd" {
		return fmt.Errorf("event head receipt: wrong kind or issuer")
	}
	if err := receipt.Verify(v.Keys); err != nil {
		return err
	}
	expected := map[string]any{
		"computer_id":                request.Event.ComputerID,
		"previous_head":              request.Event.PreviousHead,
		"event_digest":               request.EventDigest,
		"sequence":                   request.Event.Sequence,
		"event_kind":                 request.Event.EventKind,
		"request_commitment":         request.Event.RequestCommitment,
		"pin_receipt_digests":        append([]string{request.EventPinReceiptDigest}, request.PayloadPinReceiptDigests...),
		"desired_event_head":         request.Next.DesiredEventHead,
		"effective_event_head":       request.Next.EffectiveEventHead,
		"pending_transition_ref":     request.Next.PendingTransitionRef,
		"desired_state_commitment":   request.Next.DesiredStateCommitment,
		"effective_state_commitment": request.Next.EffectiveStateCommitment,
	}
	if err := receipt.RequireKindFields(
		"computer_id", "previous_head", "event_digest", "sequence", "event_kind",
		"request_commitment", "pin_receipt_digests", "desired_event_head",
		"effective_event_head", "pending_transition_ref", "desired_state_commitment",
		"effective_state_commitment",
	); err != nil {
		return err
	}
	for name, expectedValue := range expected {
		actualValue, ok := receipt.KindFields[name]
		if !ok {
			return fmt.Errorf("event head receipt: missing %s", name)
		}
		expectedJSON, err := CanonicalJSON(expectedValue)
		if err != nil {
			return err
		}
		actualJSON, err := CanonicalJSON(actualValue)
		if err != nil {
			return err
		}
		if !bytes.Equal(expectedJSON, actualJSON) {
			return fmt.Errorf("event head receipt: %s mismatch", name)
		}
	}
	return nil
}

package agentcore

import (
	"context"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// ReconcilePendingLifecycleReceipts appends lifecycle-observed events for any
// pending platform lifecycle receipts (start/stop/restart/credential events).
// It runs ONLY after the replay reached the canonical head so the finish is an
// intact witness chain (B9) and never re-enters the replay path. Boot runs
// it after replay (autoputer); a snapshot resume runs it after installing a
// delivered envelope (hibernate.go), so the consumption is appended as
// key_revoked the same way in both. Moved unchanged from autoputer/run.go.
func ReconcilePendingLifecycleReceipts(appender *computerevent.ComputerEventAppender, credentials *selfdev.GuestCredentials, computerID string, ctx context.Context) error {
	if appender == nil || credentials == nil {
		return nil
	}
	for _, lifecycleReceipt := range credentials.PendingLifecycleReceipts() {
		payload, payloadErr := lifecycleReceipt.CanonicalBytes()
		computerField, _ := lifecycleReceipt.KindFields["computer_id"].(string)
		actionField, _ := lifecycleReceipt.KindFields["action"].(string)
		if payloadErr != nil || lifecycleReceipt.ReceiptKind != "LifecycleReceipt" || lifecycleReceipt.Verify(credentials.KeyResolver()) != nil ||
			computerField != computerID || (actionField != "start" && actionField != "stop" && actionField != "restart" && actionField != "credential_envelope_consumed") {
			return fmt.Errorf("autoputer: pending lifecycle receipt binding refused")
		}
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return eventErr
		}
		eventKind := computerevent.EventLifecycleObserved
		authorityRef := "platform-control:lifecycle"
		if actionField == "credential_envelope_consumed" {
			eventKind = computerevent.EventKeyRevoked
			authorityRef = "platform-control:credential-revocation"
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: eventKind, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: "lifecycle-observed:" + lifecycleReceipt.ReceiptID,
			ActorProfile:   agentprofile.Management, AuthorityRef: authorityRef,
			PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, _, appendErr := appender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, "application/vnd.choir.lifecycle-receipt+json", "public"); appendErr != nil {
			return appendErr
		}
		if actionField == "credential_envelope_consumed" {
			if transitionErr := credentials.CompletePostRevocation(lifecycleReceipt.ReceiptID); transitionErr != nil {
				return transitionErr
			}
		} else if acknowledgeErr := credentials.AcknowledgePendingLifecycleReceipt(lifecycleReceipt.ReceiptID); acknowledgeErr != nil {
			return acknowledgeErr
		}
	}
	return nil
}

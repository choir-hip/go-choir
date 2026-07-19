package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func (s *Store) FinalizedDecisionForOperation(ctx context.Context, computerID, operationID, trajectoryID, capsuleID string) (computerevent.DurableEvent, bool, error) {
	if s == nil || s.db == nil || strings.TrimSpace(computerID) == "" || strings.TrimSpace(operationID) == "" || strings.TrimSpace(trajectoryID) == "" {
		return computerevent.DurableEvent{}, false, fmt.Errorf("computer event recovery: complete operation identity is required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT event_json, event_digest, next_desired_event_head, next_effective_event_head, COALESCE(next_pending_transition_ref, ''), next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, COALESCE(event_head_receipt_json, '') FROM computer_event_index WHERE computer_id=? AND status='finalized' AND event_kind IN (?, ?) ORDER BY sequence DESC`, computerID, computerevent.EventEffectAccepted, computerevent.EventEffectRejected)
	if err != nil {
		return computerevent.DurableEvent{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var record computerevent.DurableEvent
		var rawEvent string
		var rawReceipt string
		if err := rows.Scan(&rawEvent, &record.Request.EventDigest, &record.Request.Next.DesiredEventHead, &record.Request.Next.EffectiveEventHead,
			&record.Request.Next.PendingTransitionRef, &record.Request.Next.DesiredStateCommitment, &record.Request.Next.EffectiveStateCommitment,
			&record.Request.Next.ReducerVersion, &record.Request.Next.CredentialRevocationEpoch, &rawReceipt); err != nil {
			return computerevent.DurableEvent{}, false, err
		}
		if err := json.Unmarshal([]byte(rawEvent), &record.Request.Event); err != nil {
			return computerevent.DurableEvent{}, false, err
		}
		if record.Request.Event.ParentEventID != operationID || record.Request.Event.TrajectoryID != trajectoryID || record.Request.Event.CapsuleID != capsuleID {
			continue
		}
		if rawReceipt == "" || json.Unmarshal([]byte(rawReceipt), &record.Receipt) != nil {
			return computerevent.DurableEvent{}, false, fmt.Errorf("computer event recovery: finalized decision receipt unavailable")
		}
		record.Request.Next.ComputerID = computerID
		record.Request.Next.Sequence = record.Request.Event.Sequence
		record.Request.Next.CanonicalEventHead = record.Request.EventDigest
		return record, true, nil
	}
	return computerevent.DurableEvent{}, false, rows.Err()
}

package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func (s *Store) FinalizedDecisionForOperation(ctx context.Context, computerID, operationID, trajectoryID, capsuleID string) (computerevent.DurableEvent, bool, error) {
	if s == nil || s.db == nil || strings.TrimSpace(computerID) == "" || strings.TrimSpace(operationID) == "" || strings.TrimSpace(trajectoryID) == "" {
		return computerevent.DurableEvent{}, false, fmt.Errorf("computer event recovery: complete operation identity is required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT event_digest, sequence, next_desired_event_head, next_effective_event_head, COALESCE(next_pending_transition_ref, ''), next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch FROM computer_event_index WHERE computer_id=? AND status='finalized' AND event_kind IN (?, ?) ORDER BY sequence DESC`, computerID, computerevent.EventEffectAccepted, computerevent.EventEffectRejected)
	if err != nil {
		return computerevent.DurableEvent{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var record computerevent.DurableEvent
		var sequence uint64
		if err := rows.Scan(&record.Request.EventDigest, &sequence, &record.Request.Next.DesiredEventHead, &record.Request.Next.EffectiveEventHead,
			&record.Request.Next.PendingTransitionRef, &record.Request.Next.DesiredStateCommitment, &record.Request.Next.EffectiveStateCommitment,
			&record.Request.Next.ReducerVersion, &record.Request.Next.CredentialRevocationEpoch); err != nil {
			return computerevent.DurableEvent{}, false, err
		}
		// The guest keeps no event or receipt bodies: read this one from the
		// recent-event cache or the tape, newest first, until the decision
		// for the operation is found.
		body, err := s.finalizedEventBody(ctx, computerID, record.Request.EventDigest, sequence)
		if err != nil {
			return computerevent.DurableEvent{}, false, err
		}
		record.Request.Event = body.Request.Event
		record.Receipt = body.Receipt
		if record.Request.Event.ParentEventID != operationID || record.Request.Event.TrajectoryID != trajectoryID || record.Request.Event.CapsuleID != capsuleID {
			continue
		}
		record.Request.Next.ComputerID = computerID
		record.Request.Next.Sequence = record.Request.Event.Sequence
		record.Request.Next.CanonicalEventHead = record.Request.EventDigest
		return record, true, nil
	}
	return computerevent.DurableEvent{}, false, rows.Err()
}

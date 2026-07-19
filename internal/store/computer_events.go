package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func (s *Store) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("computer event projection: nil store")
	}
	var head computerevent.Head
	err := s.db.QueryRowContext(ctx, `SELECT computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, COALESCE(pending_transition_ref, ''), desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch FROM computer_event_projection_heads WHERE computer_id=?`, computerID).Scan(&head.ComputerID, &head.Sequence, &head.CanonicalEventHead, &head.DesiredEventHead, &head.EffectiveEventHead, &head.PendingTransitionRef, &head.DesiredStateCommitment, &head.EffectiveStateCommitment, &head.ReducerVersion, &head.CredentialRevocationEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("computer event projection: read head: %w", err)
	}
	return &head, nil
}

func (s *Store) Prepare(ctx context.Context, request computerevent.CASRequest) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("computer event projection: nil store")
	}
	event := request.Event
	if computed, err := event.Digest(); err != nil || computed != request.EventDigest || request.EventArtifactDigest != request.EventDigest {
		return fmt.Errorf("computer event projection: event digest mismatch")
	}
	eventJSON, err := event.CanonicalBytes()
	if err != nil {
		return err
	}
	pinsJSON, err := json.Marshal(request.PayloadPinReceiptDigests)
	if err != nil {
		return err
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	var existingDigest, existingCommitment, status string
	err = s.db.QueryRowContext(ctx, `SELECT event_digest, request_commitment, status FROM computer_event_index WHERE computer_id=? AND idempotency_key=?`, event.ComputerID, event.IdempotencyKey).Scan(&existingDigest, &existingCommitment, &status)
	if err == nil {
		if existingDigest != request.EventDigest || existingCommitment != event.RequestCommitment {
			return fmt.Errorf("computer event projection: idempotency commitment changed")
		}
		if status == "prepared" || status == "finalized" {
			return nil
		}
		return fmt.Errorf("computer event projection: invalid durable status %q", status)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("computer event projection: check idempotency: %w", err)
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_json, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_pending_transition_ref, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, target_state_commitment, restored_prior_effective, prepared_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'prepared', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, request.EventDigest, event.ComputerID, event.Sequence, event.PreviousHead, event.EventKind, string(eventJSON), request.EventArtifactDigest, request.EventPinReceiptDigest, string(pinsJSON), event.RequestCommitment, event.IdempotencyKey, request.Next.DesiredEventHead, request.Next.EffectiveEventHead, nullableEventString(request.Next.PendingTransitionRef), request.Next.DesiredStateCommitment, request.Next.EffectiveStateCommitment, request.Next.ReducerVersion, request.Next.CredentialRevocationEpoch, nullableEventString(request.Input.TargetStateCommitment), request.Input.RestoredPriorEffective, now)
	if err != nil {
		return fmt.Errorf("computer event projection: prepare: %w", err)
	}
	s.markDoltHistoryDirty()
	return nil
}

func (s *Store) Finalize(ctx context.Context, computerID, eventDigest string, receipt computerevent.Receipt) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("computer event projection: nil store")
	}
	receiptEventDigest, _ := receipt.KindFields["event_digest"].(string)
	if receipt.ReceiptKind != "EventHeadReceipt" || receiptEventDigest != eventDigest {
		return fmt.Errorf("computer event projection: receipt does not bind event")
	}
	receiptJSON, err := receipt.CanonicalBytes()
	if err != nil {
		return err
	}
	receiptDigest := computerevent.DigestBytes(receiptJSON)
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sequence uint64
	var previousHead, status, desiredHead, effectiveHead, desiredCommitment, effectiveCommitment string
	var pending sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT sequence, previous_head, status, next_desired_event_head, next_effective_event_head, next_pending_transition_ref, next_desired_state_commitment, next_effective_state_commitment FROM computer_event_index WHERE computer_id=? AND event_digest=? FOR UPDATE`, computerID, eventDigest).Scan(&sequence, &previousHead, &status, &desiredHead, &effectiveHead, &pending, &desiredCommitment, &effectiveCommitment)
	if err != nil {
		return fmt.Errorf("computer event projection: load prepared event: %w", err)
	}
	if status == "finalized" {
		return nil
	}
	if status != "prepared" {
		return fmt.Errorf("computer event projection: cannot finalize status %q", status)
	}
	var currentSequence uint64
	var currentHead string
	err = tx.QueryRowContext(ctx, `SELECT sequence, canonical_event_head FROM computer_event_projection_heads WHERE computer_id=? FOR UPDATE`, computerID).Scan(&currentSequence, &currentHead)
	if errors.Is(err, sql.ErrNoRows) {
		if sequence != 1 || previousHead != computerevent.ZeroHead {
			return computerevent.ErrProjectionMismatch
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, pending_transition_ref, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`, computerID, sequence, eventDigest, desiredHead, effectiveHead, nullableEventString(pending.String), desiredCommitment, effectiveCommitment, computerevent.ReducerVersionV1, time.Now().UTC())
	} else if err == nil {
		if currentSequence+1 != sequence || currentHead != previousHead {
			return computerevent.ErrProjectionMismatch
		}
		_, err = tx.ExecContext(ctx, `UPDATE computer_event_projection_heads SET sequence=?, canonical_event_head=?, desired_event_head=?, effective_event_head=?, pending_transition_ref=?, desired_state_commitment=?, effective_state_commitment=?, updated_at=? WHERE computer_id=? AND sequence=? AND canonical_event_head=?`, sequence, eventDigest, desiredHead, effectiveHead, nullableEventString(pending.String), desiredCommitment, effectiveCommitment, time.Now().UTC(), computerID, currentSequence, currentHead)
	}
	if err != nil {
		return fmt.Errorf("computer event projection: update head: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE computer_event_index SET status='finalized', event_head_receipt_json=?, event_head_receipt_digest=?, finalized_at=? WHERE computer_id=? AND event_digest=? AND status='prepared'`, string(receiptJSON), receiptDigest, time.Now().UTC(), computerID, eventDigest)
	if err != nil {
		return fmt.Errorf("computer event projection: finalize event: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return fmt.Errorf("computer event projection: finalize CAS lost")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.markDoltHistoryDirty()
	return s.commitDoltCheckpoint(ctx, "finalize computer event "+eventDigest)
}

func (s *Store) DiscardPrepared(ctx context.Context, computerID, eventDigest string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("computer event projection: nil store")
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM computer_event_index WHERE computer_id=? AND event_digest=? AND status='prepared'`, computerID, eventDigest)
	if err != nil {
		return fmt.Errorf("computer event projection: discard prepared: %w", err)
	}
	s.markDoltHistoryDirty()
	return s.commitDoltCheckpoint(ctx, "discard prepared computer event "+eventDigest)
}

func (s *Store) Prepared(ctx context.Context, computerID string) ([]computerevent.CASRequest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_json, event_digest, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, next_desired_event_head, next_effective_event_head, COALESCE(next_pending_transition_ref, ''), next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, COALESCE(target_state_commitment, ''), restored_prior_effective FROM computer_event_index WHERE computer_id=? AND status='prepared' ORDER BY sequence`, computerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []computerevent.CASRequest
	for rows.Next() {
		var request computerevent.CASRequest
		var rawEvent, rawPins string
		if err := rows.Scan(&rawEvent, &request.EventDigest, &request.EventArtifactDigest, &request.EventPinReceiptDigest, &rawPins, &request.Next.DesiredEventHead, &request.Next.EffectiveEventHead, &request.Next.PendingTransitionRef, &request.Next.DesiredStateCommitment, &request.Next.EffectiveStateCommitment, &request.Next.ReducerVersion, &request.Next.CredentialRevocationEpoch, &request.Input.TargetStateCommitment, &request.Input.RestoredPriorEffective); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rawEvent), &request.Event); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rawPins), &request.PayloadPinReceiptDigests); err != nil {
			return nil, err
		}
		request.Next.ComputerID = request.Event.ComputerID
		request.PinIntentCommitment, err = computerevent.ComputePinIntentCommitment(request.Event, request.Input)
		if err != nil {
			return nil, err
		}
		request.Next.Sequence = request.Event.Sequence
		request.Next.CanonicalEventHead = request.EventDigest
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

// EventByIdempotency returns the durable embedded event projection for recovery
// of a controller action that was interrupted after the canonical append.
func (s *Store) EventByIdempotency(ctx context.Context, computerID, idempotencyKey string) (computerevent.Event, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: nil store")
	}
	var raw, status string
	err := s.db.QueryRowContext(ctx, `SELECT event_json, status FROM computer_event_index WHERE computer_id=? AND idempotency_key=?`, computerID, idempotencyKey).Scan(&raw, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Event{}, false, nil
	}
	if err != nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: read idempotent event: %w", err)
	}
	if status != "finalized" {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: event is %s", status)
	}
	var event computerevent.Event
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: decode event: %w", err)
	}
	return event, true, nil
}

func (s *Store) EventByDigest(ctx context.Context, computerID, eventDigest string) (computerevent.Event, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT event_json FROM computer_event_index WHERE computer_id=? AND event_digest=? AND status='finalized'`, strings.TrimSpace(computerID), strings.TrimSpace(eventDigest)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Event{}, false, nil
	}
	if err != nil {
		return computerevent.Event{}, false, err
	}
	var event computerevent.Event
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return computerevent.Event{}, false, err
	}
	return event, true, nil
}

func (s *Store) EventReceiptByIdempotency(ctx context.Context, computerID, idempotencyKey string) (computerevent.Receipt, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.Receipt{}, false, fmt.Errorf("computer event projection: nil store")
	}
	var raw, status string
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(event_head_receipt_json, ''), status FROM computer_event_index WHERE computer_id=? AND idempotency_key=?`, computerID, idempotencyKey).Scan(&raw, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Receipt{}, false, nil
	}
	if err != nil || status != "finalized" || raw == "" {
		return computerevent.Receipt{}, false, fmt.Errorf("computer event projection: finalized receipt unavailable")
	}
	var receipt computerevent.Receipt
	if err := json.Unmarshal([]byte(raw), &receipt); err != nil {
		return computerevent.Receipt{}, false, err
	}
	return receipt, true, nil
}

func nullableEventString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

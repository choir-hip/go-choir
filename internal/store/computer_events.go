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
	occurredAt, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if err != nil || occurredAt.Location() != time.UTC {
		return fmt.Errorf("computer event projection: invalid event occurrence time")
	}
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
	supervisionJSON := ""
	if request.SupervisionTransaction != nil {
		if err := computerevent.ValidateSupervisionEventBinding(event, *request.SupervisionTransaction); err != nil {
			return err
		}
		canonical, err := request.SupervisionTransaction.CanonicalBytes()
		if err != nil {
			return err
		}
		supervisionJSON = string(canonical)
	} else if event.EventKind == computerevent.EventSupervisionTransaction {
		return fmt.Errorf("computer event projection: supervision transaction is missing")
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	var existingDigest, existingCommitment, status string
	err = s.db.QueryRowContext(ctx, `SELECT event_digest, request_commitment, status FROM computer_event_index WHERE computer_id=? AND idempotency_key=?`, event.ComputerID, event.IdempotencyKey).Scan(&existingDigest, &existingCommitment, &status)
	if err == nil {
		if existingDigest != request.EventDigest || existingCommitment != event.RequestCommitment {
			return fmt.Errorf("computer event projection: idempotency commitment changed")
		}
		if request.SupervisionTransaction != nil {
			var commandDigest, commandStatus, commandEventDigest string
			if err := s.db.QueryRowContext(ctx, `SELECT command_digest, status, COALESCE(event_digest, '') FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, event.ComputerID, request.SupervisionTransaction.CommandID).Scan(&commandDigest, &commandStatus, &commandEventDigest); err != nil || commandDigest != request.SupervisionTransaction.CommandDigest || commandEventDigest != request.EventDigest || (status == "prepared" && commandStatus != "prepared") || (status == "finalized" && commandStatus != "finalized") {
				return computerevent.ErrNeedsProjectionRepair
			}
		}
		if status == "prepared" || status == "finalized" {
			return nil
		}
		return fmt.Errorf("computer event projection: invalid durable status %q", status)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("computer event projection: check idempotency: %w", err)
	}
	if request.SupervisionTransaction != nil {
		preflight, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return fmt.Errorf("computer event projection: begin supervision preflight: %w", err)
		}
		preflightErr := s.applySupervisionProjectionTx(ctx, preflight, event.Sequence, event.PreviousHead, request.EventDigest, occurredAt, *request.SupervisionTransaction)
		rollbackErr := preflight.Rollback()
		if preflightErr != nil {
			return preflightErr
		}
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return fmt.Errorf("computer event projection: rollback supervision preflight: %w", rollbackErr)
		}
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("computer event projection: begin prepare: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_json, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, supervision_transaction_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_pending_transition_ref, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, target_state_commitment, restored_prior_effective, prepared_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'prepared', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, request.EventDigest, event.ComputerID, event.Sequence, event.PreviousHead, event.EventKind, string(eventJSON), request.EventArtifactDigest, request.EventPinReceiptDigest, string(pinsJSON), supervisionJSON, event.RequestCommitment, event.IdempotencyKey, request.Next.DesiredEventHead, request.Next.EffectiveEventHead, nullableEventString(request.Next.PendingTransitionRef), request.Next.DesiredStateCommitment, request.Next.EffectiveStateCommitment, request.Next.ReducerVersion, request.Next.CredentialRevocationEpoch, nullableEventString(request.Input.TargetStateCommitment), request.Input.RestoredPriorEffective, now)
	if err != nil {
		return fmt.Errorf("computer event projection: prepare: %w", err)
	}
	if request.SupervisionTransaction != nil {
		result, err := tx.ExecContext(ctx, `INSERT INTO computer_supervision_commands (computer_id, command_id, command_digest, status, created_at, updated_at) VALUES (?, ?, ?, 'reserved', ?, ?) ON DUPLICATE KEY UPDATE updated_at=updated_at`, event.ComputerID, request.SupervisionTransaction.CommandID, request.SupervisionTransaction.CommandDigest, now, now)
		if err != nil {
			return fmt.Errorf("computer event projection: reserve prepared supervision command: %w", err)
		}
		_ = result
		result, err = tx.ExecContext(ctx, `UPDATE computer_supervision_commands SET status='prepared', event_digest=?, updated_at=? WHERE computer_id=? AND command_id=? AND command_digest=? AND status IN ('reserved', 'frozen', 'pinned')`, request.EventDigest, now, event.ComputerID, request.SupervisionTransaction.CommandID, request.SupervisionTransaction.CommandDigest)
		if err != nil {
			return fmt.Errorf("computer event projection: bind prepared supervision command: %w", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			return fmt.Errorf("%w: supervision command reservation changed", computerevent.ErrNeedsProjectionRepair)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("computer event projection: commit prepare: %w", err)
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
	var sequence, nextReducerVersion, nextCredentialRevocationEpoch uint64
	var previousHead, status, desiredHead, effectiveHead, desiredCommitment, effectiveCommitment, rawSupervision, rawEvent string
	var pending sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT event_json, sequence, previous_head, status, next_desired_event_head, next_effective_event_head, next_pending_transition_ref, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, supervision_transaction_json FROM computer_event_index WHERE computer_id=? AND event_digest=? FOR UPDATE`, computerID, eventDigest).Scan(&rawEvent, &sequence, &previousHead, &status, &desiredHead, &effectiveHead, &pending, &desiredCommitment, &effectiveCommitment, &nextReducerVersion, &nextCredentialRevocationEpoch, &rawSupervision)
	if err != nil {
		return fmt.Errorf("computer event projection: load prepared event: %w", err)
	}
	if status == "finalized" {
		return nil
	}
	var event computerevent.Event
	if err := json.Unmarshal([]byte(rawEvent), &event); err != nil {
		return fmt.Errorf("computer event projection: decode prepared event: %w", err)
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if err != nil || occurredAt.Location() != time.UTC {
		return fmt.Errorf("computer event projection: invalid prepared event occurrence time")
	}
	if status != "prepared" {
		return fmt.Errorf("computer event projection: cannot finalize status %q", status)
	}
	if rawSupervision != "" {
		var supervision computerevent.SupervisionTransaction
		if err := json.Unmarshal([]byte(rawSupervision), &supervision); err != nil {
			return fmt.Errorf("computer event projection: decode supervision transaction: %w", err)
		}
		if err := s.applySupervisionProjectionTx(ctx, tx, sequence, previousHead, eventDigest, occurredAt, supervision); err != nil {
			return err
		}
	}
	var currentSequence uint64
	var currentHead string
	err = tx.QueryRowContext(ctx, `SELECT sequence, canonical_event_head FROM computer_event_projection_heads WHERE computer_id=? FOR UPDATE`, computerID).Scan(&currentSequence, &currentHead)
	if errors.Is(err, sql.ErrNoRows) {
		if sequence != 1 || previousHead != computerevent.ZeroHead {
			return computerevent.ErrProjectionMismatch
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, pending_transition_ref, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, computerID, sequence, eventDigest, desiredHead, effectiveHead, nullableEventString(pending.String), desiredCommitment, effectiveCommitment, nextReducerVersion, nextCredentialRevocationEpoch, occurredAt)
	} else if err == nil {
		if currentSequence+1 != sequence || currentHead != previousHead {
			return computerevent.ErrProjectionMismatch
		}
		_, err = tx.ExecContext(ctx, `UPDATE computer_event_projection_heads SET sequence=?, canonical_event_head=?, desired_event_head=?, effective_event_head=?, pending_transition_ref=?, desired_state_commitment=?, effective_state_commitment=?, reducer_version=?, credential_revocation_epoch=?, updated_at=? WHERE computer_id=? AND sequence=? AND canonical_event_head=?`, sequence, eventDigest, desiredHead, effectiveHead, nullableEventString(pending.String), desiredCommitment, effectiveCommitment, nextReducerVersion, nextCredentialRevocationEpoch, occurredAt, computerID, currentSequence, currentHead)
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
	if rawSupervision != "" {
		result, err = tx.ExecContext(ctx, `UPDATE computer_supervision_commands SET status='finalized', artifact_digest=?, event_head_receipt_json=?, updated_at=? WHERE computer_id=? AND event_digest=? AND status='prepared'`, event.DecisionRef, string(receiptJSON), time.Now().UTC(), computerID, eventDigest)
		if err != nil {
			return fmt.Errorf("computer event projection: finalize supervision command: %w", err)
		}
		if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
			return fmt.Errorf("computer event projection: finalize supervision command CAS lost")
		}
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
	if _, err := s.db.ExecContext(ctx, `UPDATE computer_supervision_commands SET status='pinned', event_digest=NULL, updated_at=? WHERE computer_id=? AND event_digest=? AND status='prepared'`, time.Now().UTC(), computerID, eventDigest); err != nil {
		return fmt.Errorf("computer event projection: release supervision reservation: %w", err)
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM computer_event_index WHERE computer_id=? AND event_digest=? AND status='prepared'`, computerID, eventDigest)
	if err != nil {
		return fmt.Errorf("computer event projection: discard prepared: %w", err)
	}
	s.markDoltHistoryDirty()
	return s.commitDoltCheckpoint(ctx, "discard prepared computer event "+eventDigest)
}

func (s *Store) Prepared(ctx context.Context, computerID string) ([]computerevent.CASRequest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_json, event_digest, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, supervision_transaction_json, next_desired_event_head, next_effective_event_head, COALESCE(next_pending_transition_ref, ''), next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, COALESCE(target_state_commitment, ''), restored_prior_effective FROM computer_event_index WHERE computer_id=? AND status='prepared' ORDER BY sequence`, computerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []computerevent.CASRequest
	for rows.Next() {
		var request computerevent.CASRequest
		var rawEvent, rawPins, rawSupervision string
		if err := rows.Scan(&rawEvent, &request.EventDigest, &request.EventArtifactDigest, &request.EventPinReceiptDigest, &rawPins, &rawSupervision, &request.Next.DesiredEventHead, &request.Next.EffectiveEventHead, &request.Next.PendingTransitionRef, &request.Next.DesiredStateCommitment, &request.Next.EffectiveStateCommitment, &request.Next.ReducerVersion, &request.Next.CredentialRevocationEpoch, &request.Input.TargetStateCommitment, &request.Input.RestoredPriorEffective); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rawEvent), &request.Event); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rawPins), &request.PayloadPinReceiptDigests); err != nil {
			return nil, err
		}
		if rawSupervision != "" {
			request.SupervisionTransaction = &computerevent.SupervisionTransaction{}
			if err := json.Unmarshal([]byte(rawSupervision), request.SupervisionTransaction); err != nil {
				return nil, err
			}
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

// SupervisionCommand resolves a finalized canonical command before payload
// encryption. The artifact digest is the event's private payload commitment;
// it is returned only after validating the stored event/transaction join.
func (s *Store) SupervisionCommand(ctx context.Context, computerID, commandID string) (computerevent.Receipt, string, string, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.Receipt{}, "", "", false, fmt.Errorf("computer event projection: nil store")
	}
	var rawEvent, rawTransaction, rawReceipt, status string
	err := s.db.QueryRowContext(ctx, `SELECT event_json, supervision_transaction_json, COALESCE(event_head_receipt_json, ''), status FROM computer_event_index WHERE computer_id=? AND idempotency_key=?`, strings.TrimSpace(computerID), strings.TrimSpace(commandID)).Scan(&rawEvent, &rawTransaction, &rawReceipt, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Receipt{}, "", "", false, nil
	}
	if err != nil {
		return computerevent.Receipt{}, "", "", false, fmt.Errorf("computer event projection: lookup supervision command: %w", err)
	}
	if status != "finalized" {
		return computerevent.Receipt{}, "", "", false, fmt.Errorf("%w: command %q is %s", computerevent.ErrNeedsProjectionRepair, commandID, status)
	}
	var event computerevent.Event
	var transaction computerevent.SupervisionTransaction
	var receipt computerevent.Receipt
	if err := json.Unmarshal([]byte(rawEvent), &event); err != nil {
		return computerevent.Receipt{}, "", "", false, fmt.Errorf("computer event projection: decode command event: %w", err)
	}
	if err := json.Unmarshal([]byte(rawTransaction), &transaction); err != nil {
		return computerevent.Receipt{}, "", "", false, fmt.Errorf("computer event projection: decode command transaction: %w", err)
	}
	if err := json.Unmarshal([]byte(rawReceipt), &receipt); err != nil {
		return computerevent.Receipt{}, "", "", false, fmt.Errorf("computer event projection: decode command receipt: %w", err)
	}
	if err := computerevent.ValidateSupervisionEventBinding(event, transaction); err != nil {
		return computerevent.Receipt{}, "", "", false, fmt.Errorf("computer event projection: invalid stored command: %w", err)
	}
	if event.DecisionRef == "" || !computerevent.IsSHA256(event.DecisionRef) {
		return computerevent.Receipt{}, "", "", false, fmt.Errorf("computer event projection: stored command payload artifact missing")
	}
	return receipt, event.DecisionRef, transaction.CommandDigest, true, nil
}

// FinalizedSupervisionTransaction returns the exact accepted transaction for
// idempotent recovery without recreating private payloads.
func (s *Store) FinalizedSupervisionTransaction(ctx context.Context, computerID, commandID string) (computerevent.SupervisionTransaction, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.SupervisionTransaction{}, false, fmt.Errorf("computer event projection: nil store")
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT supervision_transaction_json FROM computer_event_index WHERE computer_id=? AND idempotency_key=? AND status='finalized'`, strings.TrimSpace(computerID), strings.TrimSpace(commandID)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.SupervisionTransaction{}, false, nil
	}
	if err != nil {
		return computerevent.SupervisionTransaction{}, false, fmt.Errorf("computer event projection: lookup finalized supervision transaction: %w", err)
	}
	var transaction computerevent.SupervisionTransaction
	if err := json.Unmarshal([]byte(raw), &transaction); err != nil {
		return computerevent.SupervisionTransaction{}, false, fmt.Errorf("computer event projection: decode finalized supervision transaction: %w", err)
	}
	if err := transaction.Validate(); err != nil {
		return computerevent.SupervisionTransaction{}, false, fmt.Errorf("%w: invalid finalized supervision transaction: %v", computerevent.ErrNeedsProjectionRepair, err)
	}
	return transaction, true, nil
}

// ReserveSupervisionCommand records an exact command digest before the caller
// creates event entropy or private ciphertext. A later finalized append returns
// its original receipt and artifact; a conflicting digest never replaces it.
func (s *Store) ReserveSupervisionCommand(ctx context.Context, computerID, commandID, commandDigest string) (computerevent.Receipt, string, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.Receipt{}, "", false, fmt.Errorf("computer event projection: nil store")
	}
	if strings.TrimSpace(computerID) == "" || strings.TrimSpace(commandID) == "" || !computerevent.IsSHA256(commandDigest) {
		return computerevent.Receipt{}, "", false, fmt.Errorf("computer event projection: invalid supervision reservation")
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_supervision_commands (computer_id, command_id, command_digest, status, created_at, updated_at) VALUES (?, ?, ?, 'reserved', ?, ?) ON DUPLICATE KEY UPDATE updated_at=updated_at`, computerID, commandID, commandDigest, now, now); err != nil {
		return computerevent.Receipt{}, "", false, fmt.Errorf("computer event projection: reserve supervision command: %w", err)
	}
	var storedDigest, status, artifactDigest, rawReceipt string
	err := s.db.QueryRowContext(ctx, `SELECT command_digest, status, COALESCE(artifact_digest, ''), COALESCE(event_head_receipt_json, '') FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, computerID, commandID).Scan(&storedDigest, &status, &artifactDigest, &rawReceipt)
	if err != nil {
		return computerevent.Receipt{}, "", false, fmt.Errorf("computer event projection: load supervision reservation: %w", err)
	}

	if storedDigest != commandDigest {
		return computerevent.Receipt{}, "", false, fmt.Errorf("%w: command digest changed", computerevent.ErrSupervisionIdempotencyConflict)
	}
	switch status {
	case "reserved", "input_frozen", "frozen", "pinned":
		return computerevent.Receipt{}, "", false, nil
	case "finalized":
		var receipt computerevent.Receipt
		if artifactDigest == "" || rawReceipt == "" || json.Unmarshal([]byte(rawReceipt), &receipt) != nil {
			return computerevent.Receipt{}, "", false, computerevent.ErrNeedsProjectionRepair
		}
		return receipt, artifactDigest, true, nil
	default:
		return computerevent.Receipt{}, "", false, fmt.Errorf("computer event projection: invalid supervision reservation status %q", status)
	}
}

// PendingSupervisionReservation reports an admitted command that has not yet
// become a finalized canonical event. Callers must recover its frozen plan or
// fail closed; they must not rebuild time-bearing command inputs.
func (s *Store) PendingSupervisionReservation(ctx context.Context, computerID, commandID string) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("computer event projection: nil store")
	}
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, strings.TrimSpace(computerID), strings.TrimSpace(commandID)).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("computer event projection: lookup supervision reservation: %w", err)
	}
	switch status {
	case "reserved", "input_frozen", "frozen", "pinned":
		return true, nil
	case "finalized":
		return false, nil
	default:
		return false, fmt.Errorf("computer event projection: invalid supervision reservation status %q", status)
	}
}

// ReserveFrozenSupervisionInputs atomically binds the logical command to its
// encrypted private input plan before any external artifact pin is attempted.
func (s *Store) ReserveFrozenSupervisionInputs(ctx context.Context, computerID, commandID, commandDigest string, plan computerevent.FrozenSupervisionPlan) (computerevent.Receipt, string, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.Receipt{}, "", false, fmt.Errorf("computer event projection: nil store")
	}
	if err := plan.ValidatePrivateInputs(computerID, commandID, commandDigest); err != nil {
		return computerevent.Receipt{}, "", false, fmt.Errorf("computer event projection: invalid frozen supervision inputs: %w", err)
	}
	canonical, err := computerevent.CanonicalJSON(plan)
	if err != nil {
		return computerevent.Receipt{}, "", false, err
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_supervision_commands (computer_id, command_id, command_digest, status, event_head_receipt_json, created_at, updated_at) VALUES (?, ?, ?, 'input_frozen', ?, ?, ?) ON DUPLICATE KEY UPDATE updated_at=updated_at`, computerID, commandID, commandDigest, string(canonical), now, now); err != nil {
		return computerevent.Receipt{}, "", false, fmt.Errorf("computer event projection: reserve frozen supervision inputs: %w", err)
	}
	var storedDigest, status, artifactDigest, raw string
	if err := s.db.QueryRowContext(ctx, `SELECT command_digest, status, COALESCE(artifact_digest, ''), COALESCE(event_head_receipt_json, '') FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, computerID, commandID).Scan(&storedDigest, &status, &artifactDigest, &raw); err != nil {
		return computerevent.Receipt{}, "", false, err
	}
	if storedDigest != commandDigest {
		return computerevent.Receipt{}, "", false, computerevent.ErrSupervisionIdempotencyConflict
	}
	if status == "finalized" {
		var receipt computerevent.Receipt
		if artifactDigest == "" || json.Unmarshal([]byte(raw), &receipt) != nil {
			return computerevent.Receipt{}, "", false, computerevent.ErrNeedsProjectionRepair
		}
		return receipt, artifactDigest, true, nil
	}
	if status == "frozen" || status == "pinned" {
		return computerevent.Receipt{}, "", false, nil
	}
	if status != "input_frozen" {
		return computerevent.Receipt{}, "", false, computerevent.ErrNeedsProjectionRepair
	}
	var stored computerevent.FrozenSupervisionPlan
	if json.Unmarshal([]byte(raw), &stored) != nil || stored.ValidatePrivateInputs(computerID, commandID, commandDigest) != nil {
		return computerevent.Receipt{}, "", false, computerevent.ErrNeedsProjectionRepair
	}
	storedCanonical, err := computerevent.CanonicalJSON(stored)
	if err != nil || string(storedCanonical) != string(canonical) {
		return computerevent.Receipt{}, "", false, computerevent.ErrSupervisionIdempotencyConflict
	}
	return computerevent.Receipt{}, "", false, nil
}

// FrozenSupervisionPlan returns an immutable pre-pin plan. The reservation
// receipt column intentionally carries this private recovery record only until
// finalization, when it is atomically replaced by the canonical head receipt.
func (s *Store) FrozenSupervisionPlan(ctx context.Context, computerID, commandID string) (computerevent.FrozenSupervisionPlan, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.FrozenSupervisionPlan{}, false, fmt.Errorf("computer event projection: nil store")
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	var commandDigest, status, raw string
	err := s.db.QueryRowContext(ctx, `SELECT command_digest, status, COALESCE(event_head_receipt_json, '') FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, computerID, commandID).Scan(&commandDigest, &status, &raw)
	if errors.Is(err, sql.ErrNoRows) || status == "reserved" {
		return computerevent.FrozenSupervisionPlan{}, false, nil
	}
	if err != nil {
		return computerevent.FrozenSupervisionPlan{}, false, fmt.Errorf("computer event projection: load frozen supervision plan: %w", err)
	}
	if status == "finalized" {
		return computerevent.FrozenSupervisionPlan{}, false, nil
	}
	var plan computerevent.FrozenSupervisionPlan
	if raw == "" || json.Unmarshal([]byte(raw), &plan) != nil {
		return computerevent.FrozenSupervisionPlan{}, false, computerevent.ErrNeedsProjectionRepair
	}
	if status == "input_frozen" {
		if err := plan.ValidatePrivateInputs(computerID, commandID, commandDigest); err != nil {
			return computerevent.FrozenSupervisionPlan{}, false, computerevent.ErrNeedsProjectionRepair
		}
	}
	if status != "input_frozen" && status != "frozen" && status != "pinned" {
		return computerevent.FrozenSupervisionPlan{}, false, computerevent.ErrNeedsProjectionRepair
	}
	if status != "input_frozen" && (plan.EventID == "" || !computerevent.IsSHA256(plan.ArtifactDigest)) {
		return computerevent.FrozenSupervisionPlan{}, false, computerevent.ErrNeedsProjectionRepair
	}
	return plan, true, nil
}

func (s *Store) FreezeSupervisionPlan(ctx context.Context, computerID, commandID, commandDigest string, plan computerevent.FrozenSupervisionPlan) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("computer event projection: nil store")
	}
	if plan.EventID == "" || plan.OccurredAt == "" || plan.PinReceipt != nil || !computerevent.IsSHA256(plan.ArtifactDigest) {
		return fmt.Errorf("computer event projection: invalid frozen supervision plan")
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE computer_supervision_commands SET status='frozen', artifact_digest=?, event_head_receipt_json=?, updated_at=? WHERE computer_id=? AND command_id=? AND command_digest=? AND status IN ('reserved','input_frozen')`, plan.ArtifactDigest, string(raw), time.Now().UTC(), computerID, commandID, commandDigest)
	if err != nil {
		return fmt.Errorf("computer event projection: freeze supervision plan: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return computerevent.ErrNeedsProjectionRepair
	}
	return nil
}

func (s *Store) RecordSupervisionPin(ctx context.Context, computerID, commandID, commandDigest string, receipt computerevent.Receipt) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("computer event projection: nil store")
	}
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(event_head_receipt_json, '') FROM computer_supervision_commands WHERE computer_id=? AND command_id=? AND command_digest=? AND status='frozen'`, computerID, commandID, commandDigest).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.ErrNeedsProjectionRepair
	}
	if err != nil {
		return fmt.Errorf("computer event projection: load frozen supervision plan: %w", err)
	}
	var plan computerevent.FrozenSupervisionPlan
	if json.Unmarshal([]byte(raw), &plan) != nil || plan.PinReceipt != nil {
		return computerevent.ErrNeedsProjectionRepair
	}
	plan.PinReceipt = &receipt
	updated, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE computer_supervision_commands SET status='pinned', event_head_receipt_json=?, updated_at=? WHERE computer_id=? AND command_id=? AND command_digest=? AND status='frozen'`, string(updated), time.Now().UTC(), computerID, commandID, commandDigest)
	if err != nil {
		return fmt.Errorf("computer event projection: record supervision pin: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return computerevent.ErrNeedsProjectionRepair
	}
	return nil
}

func nullableEventString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

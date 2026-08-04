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
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// RebuildComputerEventProjection replaces the local, replay-owned projection
// with a complete externally verified canonical tape. It deliberately does not
// call Prepare or Finalize: those public recovery operations commit separately.
func (s *Store) RebuildComputerEventProjection(ctx context.Context, records []computerevent.DurableEvent, expectedHead *computerevent.Head) error {
	if s == nil || s.db == nil || s.ogStore == nil {
		return fmt.Errorf("computer event projection: store unavailable")
	}
	computerID, replayHead, err := validateRebuildTape(records, expectedHead)
	if err != nil {
		return err
	}
	ownedIDs, trajectories, err := rebuildOwnedObjectIDs(records)
	if err != nil {
		return err
	}

	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("computer event projection: begin rebuild: %w", err)
	}
	defer tx.Rollback()

	if err := s.deleteRebuildProjectionTx(ctx, tx, computerID, ownedIDs, trajectories); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM computer_supervision_commands WHERE computer_id=? AND status='finalized'`, computerID); err != nil {
		return fmt.Errorf("computer event projection: clear finalized supervision commands: %w", err)
	}
	// A locally prepared supervision event is not durable until corpusd's CAS
	// appears on the canonical tape. Rebuild discards the orphaned prepared
	// index row and rewinds its command to the last durable pre-CAS state while
	// retaining the frozen private plan needed for an exact retry.
	if _, err := tx.ExecContext(ctx, `UPDATE computer_supervision_commands SET status='pinned', event_digest=NULL WHERE computer_id=? AND status='prepared'`, computerID); err != nil {
		return fmt.Errorf("computer event projection: rewind unacknowledged supervision commands: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM computer_event_index WHERE computer_id=?`, computerID); err != nil {
		return fmt.Errorf("computer event projection: clear event index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM computer_event_projection_heads WHERE computer_id=?`, computerID); err != nil {
		return fmt.Errorf("computer event projection: clear head: %w", err)
	}

	var current *computerevent.Head
	for index := range records {
		record := records[index]
		occurredAt, err := time.Parse(time.RFC3339Nano, record.Request.Event.OccurredAt)
		if err != nil || occurredAt.Location() != time.UTC {
			return fmt.Errorf("computer event projection: invalid replay occurrence time at sequence %d", record.Request.Event.Sequence)
		}
		next, err := computerevent.Reduce(current, record.Request.Event, record.Request.Input)
		if err != nil || !rebuildSameHead(&next, &record.Request.Next) {
			return fmt.Errorf("%w: replay head at sequence %d", computerevent.ErrProjectionMismatch, record.Request.Event.Sequence)
		}
		if err := s.insertFinalizedRebuildEventTx(ctx, tx, record, occurredAt); err != nil {
			return err
		}
		if record.Request.SupervisionTransaction != nil {
			if err := s.applySupervisionProjectionTx(ctx, tx, record.Request.Event.Sequence, record.Request.Event.PreviousHead, record.Request.EventDigest, occurredAt, *record.Request.SupervisionTransaction); err != nil {
				return err
			}
		}
		if err := upsertRebuildHeadTx(ctx, tx, next, occurredAt); err != nil {
			return err
		}
		current = &next
	}
	if !rebuildSameHead(current, replayHead) {
		return fmt.Errorf("%w: rebuilt final head", computerevent.ErrProjectionMismatch)
	}
	if err := verifyRebuiltProjectionTx(ctx, tx, computerID, records, replayHead); err != nil {
		return err
	}
	if s.rebuildComputerEventProjectionBeforeCommit != nil {
		if err := s.rebuildComputerEventProjectionBeforeCommit(); err != nil {
			return fmt.Errorf("computer event projection: injected rebuild failure: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("computer event projection: commit rebuild: %w", err)
	}
	s.markDoltHistoryDirty()
	return nil
}

func validateRebuildTape(records []computerevent.DurableEvent, expected *computerevent.Head) (string, *computerevent.Head, error) {
	if len(records) == 0 {
		if expected == nil || strings.TrimSpace(expected.ComputerID) == "" || expected.Sequence != 0 {
			return "", nil, fmt.Errorf("computer event projection: empty replay requires an explicit zero head")
		}
		return expected.ComputerID, nil, nil
	}
	computerID := records[0].Request.Event.ComputerID
	if computerID == "" {
		return "", nil, fmt.Errorf("computer event projection: replay computer is required")
	}
	var current *computerevent.Head
	for index := range records {
		record := records[index]
		request := record.Request
		if request.Event.ComputerID != computerID || request.Event.Sequence != uint64(index+1) {
			return "", nil, fmt.Errorf("%w: replay sequence is incomplete", computerevent.ErrProjectionMismatch)
		}
		if computed, err := request.Event.Digest(); err != nil || computed != request.EventDigest || request.EventArtifactDigest != request.EventDigest {
			return "", nil, fmt.Errorf("computer event projection: replay event digest mismatch at sequence %d", request.Event.Sequence)
		}
		if pinIntent, err := computerevent.ComputePinIntentCommitment(request.Event, request.Input); err != nil || pinIntent != request.PinIntentCommitment {
			return "", nil, fmt.Errorf("computer event projection: replay pin intent mismatch at sequence %d", request.Event.Sequence)
		}
		if commitment, err := computerevent.ComputeRequestCommitment(request.Event, request.Input, request.PinIntentCommitment, request.PayloadPinReceiptDigests); err != nil || commitment != request.Event.RequestCommitment {
			return "", nil, fmt.Errorf("computer event projection: replay request commitment mismatch at sequence %d", request.Event.Sequence)
		}
		if !computerevent.IsSHA256(request.EventPinReceiptDigest) {
			return "", nil, fmt.Errorf("computer event projection: replay event pin receipt digest at sequence %d", request.Event.Sequence)
		}
		for _, digest := range request.PayloadPinReceiptDigests {
			if !computerevent.IsSHA256(digest) {
				return "", nil, fmt.Errorf("computer event projection: replay payload pin receipt digest at sequence %d", request.Event.Sequence)
			}
		}
		if request.SupervisionTransaction != nil {
			if err := computerevent.ValidateSupervisionEventBinding(request.Event, *request.SupervisionTransaction); err != nil {
				return "", nil, fmt.Errorf("computer event projection: replay supervision binding: %w", err)
			}
		} else if request.Event.EventKind == computerevent.EventSupervisionTransaction {
			return "", nil, fmt.Errorf("computer event projection: replay supervision transaction is missing")
		}
		if err := validateRebuildReceipt(record.Receipt, request); err != nil {
			return "", nil, err
		}
		next, err := computerevent.Reduce(current, request.Event, request.Input)
		if err != nil || !rebuildSameHead(&next, &request.Next) {
			return "", nil, fmt.Errorf("%w: replay next head at sequence %d", computerevent.ErrProjectionMismatch, request.Event.Sequence)
		}
		current = &next
	}
	if !rebuildSameHead(current, expected) {
		return "", nil, fmt.Errorf("%w: expected final head", computerevent.ErrProjectionMismatch)
	}
	return computerID, current, nil
}

func validateRebuildReceipt(receipt computerevent.Receipt, request computerevent.CASRequest) error {
	if receipt.ReceiptKind != "EventHeadReceipt" {
		return fmt.Errorf("computer event projection: replay receipt kind")
	}
	if digest, _ := receipt.KindFields["event_digest"].(string); digest != request.EventDigest {
		return fmt.Errorf("computer event projection: replay receipt does not bind event")
	}
	if computerID, _ := receipt.KindFields["computer_id"].(string); computerID != "" && computerID != request.Event.ComputerID {
		return fmt.Errorf("computer event projection: replay receipt computer mismatch")
	}
	if _, err := receipt.CanonicalBytes(); err != nil {
		return fmt.Errorf("computer event projection: replay receipt canonical form: %w", err)
	}
	return nil
}

func (s *Store) insertFinalizedRebuildEventTx(ctx context.Context, tx *sql.Tx, record computerevent.DurableEvent, occurredAt time.Time) error {
	request := record.Request
	eventJSON, err := request.Event.CanonicalBytes()
	if err != nil {
		return err
	}
	pinsJSON, err := json.Marshal(request.PayloadPinReceiptDigests)
	if err != nil {
		return err
	}
	supervisionJSON := ""
	if request.SupervisionTransaction != nil {
		canonical, err := request.SupervisionTransaction.CanonicalBytes()
		if err != nil {
			return err
		}
		supervisionJSON = string(canonical)
	}
	receiptJSON, err := record.Receipt.CanonicalBytes()
	if err != nil {
		return err
	}
	receiptDigest := computerevent.DigestBytes(receiptJSON)
	_, err = tx.ExecContext(ctx, `INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_json, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, supervision_transaction_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_pending_transition_ref, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, target_state_commitment, restored_prior_effective, event_head_receipt_json, event_head_receipt_digest, prepared_at, finalized_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'finalized', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, request.EventDigest, request.Event.ComputerID, request.Event.Sequence, request.Event.PreviousHead, request.Event.EventKind, string(eventJSON), request.EventArtifactDigest, request.EventPinReceiptDigest, string(pinsJSON), supervisionJSON, request.Event.RequestCommitment, request.Event.IdempotencyKey, request.Next.DesiredEventHead, request.Next.EffectiveEventHead, nullableEventString(request.Next.PendingTransitionRef), request.Next.DesiredStateCommitment, request.Next.EffectiveStateCommitment, request.Next.ReducerVersion, request.Next.CredentialRevocationEpoch, nullableEventString(request.Input.TargetStateCommitment), request.Input.RestoredPriorEffective, string(receiptJSON), receiptDigest, occurredAt, occurredAt)
	if err != nil {
		return fmt.Errorf("computer event projection: rebuild index sequence %d: %w", request.Event.Sequence, err)
	}
	if request.SupervisionTransaction != nil {
		transaction := request.SupervisionTransaction
		var existingDigest string
		existingErr := tx.QueryRowContext(ctx, `SELECT command_digest FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, request.Event.ComputerID, transaction.CommandID).Scan(&existingDigest)
		if existingErr == nil {
			if existingDigest != transaction.CommandDigest {
				return fmt.Errorf("%w: rebuilt supervision command conflicts with pending reservation", computerevent.ErrProjectionMismatch)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, request.Event.ComputerID, transaction.CommandID); err != nil {
				return fmt.Errorf("computer event projection: replace acknowledged supervision command: %w", err)
			}
		} else if !errors.Is(existingErr, sql.ErrNoRows) {
			return fmt.Errorf("computer event projection: inspect pending supervision command: %w", existingErr)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO computer_supervision_commands (computer_id, command_id, command_digest, status, event_digest, artifact_digest, event_head_receipt_json, created_at, updated_at) VALUES (?, ?, ?, 'finalized', ?, ?, ?, ?, ?)`, request.Event.ComputerID, transaction.CommandID, transaction.CommandDigest, request.EventDigest, request.Event.DecisionRef, string(receiptJSON), occurredAt, occurredAt)
		if err != nil {
			return fmt.Errorf("computer event projection: rebuild supervision command: %w", err)
		}
	}
	return nil
}

func upsertRebuildHeadTx(ctx context.Context, tx *sql.Tx, head computerevent.Head, occurredAt time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, pending_transition_ref, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE sequence=VALUES(sequence), canonical_event_head=VALUES(canonical_event_head), desired_event_head=VALUES(desired_event_head), effective_event_head=VALUES(effective_event_head), pending_transition_ref=VALUES(pending_transition_ref), desired_state_commitment=VALUES(desired_state_commitment), effective_state_commitment=VALUES(effective_state_commitment), reducer_version=VALUES(reducer_version), credential_revocation_epoch=VALUES(credential_revocation_epoch), updated_at=VALUES(updated_at)`, head.ComputerID, head.Sequence, head.CanonicalEventHead, head.DesiredEventHead, head.EffectiveEventHead, nullableEventString(head.PendingTransitionRef), head.DesiredStateCommitment, head.EffectiveStateCommitment, head.ReducerVersion, head.CredentialRevocationEpoch, occurredAt)
	if err != nil {
		return fmt.Errorf("computer event projection: rebuild head: %w", err)
	}
	return nil
}

func verifyRebuiltProjectionTx(ctx context.Context, tx *sql.Tx, computerID string, records []computerevent.DurableEvent, expected *computerevent.Head) error {
	var count uint64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_index WHERE computer_id=? AND status='finalized'`, computerID).Scan(&count); err != nil {
		return err
	}
	if count != uint64(len(records)) {
		return fmt.Errorf("%w: rebuilt event index cardinality", computerevent.ErrProjectionMismatch)
	}
	if expected == nil {
		var headCount uint64
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_projection_heads WHERE computer_id=?`, computerID).Scan(&headCount); err != nil {
			return err
		}
		if headCount != 0 {
			return fmt.Errorf("%w: zero replay head", computerevent.ErrProjectionMismatch)
		}
		return nil
	}
	var actual computerevent.Head
	var headPending sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, pending_transition_ref, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch FROM computer_event_projection_heads WHERE computer_id=?`, computerID).Scan(&actual.ComputerID, &actual.Sequence, &actual.CanonicalEventHead, &actual.DesiredEventHead, &actual.EffectiveEventHead, &headPending, &actual.DesiredStateCommitment, &actual.EffectiveStateCommitment, &actual.ReducerVersion, &actual.CredentialRevocationEpoch)
	actual.PendingTransitionRef = headPending.String
	if err != nil || !rebuildSameHead(&actual, expected) {
		return fmt.Errorf("%w: rebuilt head snapshot", computerevent.ErrProjectionMismatch)
	}
	for _, record := range records {
		request := record.Request
		var sequence uint64
		var previousHead, eventKind, eventArtifact, pinReceipt, requestCommitment, status, desiredHead, effectiveHead, desiredCommitment, effectiveCommitment, rawReceipt, receiptDigest string
		var pending sql.NullString
		var reducerVersion, revocationEpoch uint64
		err := tx.QueryRowContext(ctx, `SELECT sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, request_commitment, status, next_desired_event_head, next_effective_event_head, next_pending_transition_ref, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, event_head_receipt_json, event_head_receipt_digest FROM computer_event_index WHERE computer_id=? AND event_digest=?`, computerID, request.EventDigest).Scan(&sequence, &previousHead, &eventKind, &eventArtifact, &pinReceipt, &requestCommitment, &status, &desiredHead, &effectiveHead, &pending, &desiredCommitment, &effectiveCommitment, &reducerVersion, &revocationEpoch, &rawReceipt, &receiptDigest)
		receiptJSON, receiptErr := record.Receipt.CanonicalBytes()
		if err != nil || receiptErr != nil || sequence != request.Event.Sequence || previousHead != request.Event.PreviousHead || eventKind != string(request.Event.EventKind) || eventArtifact != request.EventArtifactDigest || pinReceipt != request.EventPinReceiptDigest || requestCommitment != request.Event.RequestCommitment || status != "finalized" || desiredHead != request.Next.DesiredEventHead || effectiveHead != request.Next.EffectiveEventHead || pending.String != request.Next.PendingTransitionRef || desiredCommitment != request.Next.DesiredStateCommitment || effectiveCommitment != request.Next.EffectiveStateCommitment || reducerVersion != uint64(request.Next.ReducerVersion) || revocationEpoch != request.Next.CredentialRevocationEpoch || rawReceipt != string(receiptJSON) || receiptDigest != computerevent.DigestBytes(receiptJSON) {
			return fmt.Errorf("%w: rebuilt event snapshot", computerevent.ErrProjectionMismatch)
		}
		if request.SupervisionTransaction == nil {
			continue
		}
		transaction := request.SupervisionTransaction
		var commandDigest, commandStatus, eventDigest, artifactDigest, commandReceipt string
		err = tx.QueryRowContext(ctx, `SELECT command_digest, status, event_digest, artifact_digest, event_head_receipt_json FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, computerID, transaction.CommandID).Scan(&commandDigest, &commandStatus, &eventDigest, &artifactDigest, &commandReceipt)
		if err != nil || commandDigest != transaction.CommandDigest || commandStatus != "finalized" || eventDigest != request.EventDigest || artifactDigest != request.Event.DecisionRef || commandReceipt != string(receiptJSON) {
			return fmt.Errorf("%w: rebuilt supervision reservation", computerevent.ErrProjectionMismatch)
		}
		stateID, stateErr := lifecycleCanonicalID(ogKindSupervisionState, transaction.OwnerID, transaction.ComputerID, transaction.TrajectoryID)
		if stateErr != nil {
			return stateErr
		}
		var rawState string
		if err := tx.QueryRowContext(ctx, `SELECT body FROM og_objects WHERE canonical_id=?`, stateID).Scan(&rawState); err != nil {
			return fmt.Errorf("%w: rebuilt supervision state", computerevent.ErrProjectionMismatch)
		}
		var state supervisionProjectionState
		if json.Unmarshal([]byte(rawState), &state) != nil || state.OwnerID != transaction.OwnerID || state.ComputerID != computerID || state.TrajectoryID != transaction.TrajectoryID || state.CanonicalSequence < request.Event.Sequence || state.CanonicalEventHead == "" {
			return fmt.Errorf("%w: rebuilt supervision snapshot", computerevent.ErrProjectionMismatch)
		}
	}
	return nil
}

func rebuildSameHead(left, right *computerevent.Head) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func rebuildOwnedObjectIDs(records []computerevent.DurableEvent) (map[string]struct{}, map[string]struct{}, error) {
	ids, trajectories := map[string]struct{}{}, map[string]struct{}{}
	add := func(kind objectgraph.ObjectKind, owner, computer, key string) error {
		id, err := lifecycleCanonicalID(kind, owner, computer, key)
		if err == nil {
			ids[id] = struct{}{}
		}
		return err
	}
	for _, record := range records {
		transaction := record.Request.SupervisionTransaction
		if transaction == nil {
			continue
		}
		trajectories[transaction.TrajectoryID] = struct{}{}
		if err := add(ogKindSupervisionState, transaction.OwnerID, transaction.ComputerID, transaction.TrajectoryID); err != nil {
			return nil, nil, err
		}
		for index, mutation := range transaction.Mutations {
			eventKey := fmt.Sprintf("%s:%d", transaction.TransactionID, index)
			if err := add(ogKindSupervisionEvent, transaction.OwnerID, transaction.ComputerID, eventKey); err != nil {
				return nil, nil, err
			}
			if err := add(ogKindLifecycleEvent, transaction.OwnerID, transaction.ComputerID, eventKey); err != nil {
				return nil, nil, err
			}
			body, err := supervisionBodyMap(mutation.Body)
			if err != nil {
				return nil, nil, err
			}
			if mutation.Kind == "projection_imported" {
				continue
			}
			switch mutation.Kind {
			case "trajectory_started":
				if err := add(ogKindTrajectory, transaction.OwnerID, transaction.ComputerID, transaction.TrajectoryID); err != nil {
					return nil, nil, err
				}
				for _, id := range bodyStrings(body, "initial_assignment_ids") {
					if err := add(ogKindWorkItem, transaction.OwnerID, transaction.ComputerID, id); err != nil {
						return nil, nil, err
					}
				}
				if id := bodyString(body, "texture_actor_id"); id != "" {
					if err := add(ogKindAgent, transaction.OwnerID, transaction.ComputerID, id); err != nil {
						return nil, nil, err
					}
				}
			case "assignment_opened", "assignment_cancelled", "disposition_recorded":
				id := bodyString(body, "assignment_id")
				if mutation.Kind == "disposition_recorded" {
					if target, ok := body["target"].(map[string]any); ok && bodyString(target, "kind") == "assignment" {
						id = bodyString(target, "id")
					}
				}
				if id != "" {
					if err := add(ogKindWorkItem, transaction.OwnerID, transaction.ComputerID, id); err != nil {
						return nil, nil, err
					}
				}
				if mutation.Kind == "assignment_opened" {
					if id := bodyString(body, "assigned_actor_id"); id != "" {
						if err := add(ogKindAgent, transaction.OwnerID, transaction.ComputerID, id); err != nil {
							return nil, nil, err
						}
					}
				}
			case "texture_revision":
				for kind, key := range map[objectgraph.ObjectKind]string{ogKindTexDoc: bodyString(body, "artifact_id"), ogKindTexRev: bodyString(body, "revision_id")} {
					if key != "" {
						if err := add(kind, transaction.OwnerID, transaction.ComputerID, key); err != nil {
							return nil, nil, err
						}
					}
				}
			case "artifact_archived":
				if id := bodyString(body, "artifact_id"); id != "" {
					if err := add(ogKindTexDoc, transaction.OwnerID, transaction.ComputerID, id); err != nil {
						return nil, nil, err
					}
				}
			}
		}
	}
	return ids, trajectories, nil
}

func (s *Store) deleteRebuildProjectionTx(ctx context.Context, tx *sql.Tx, computerID string, ids, trajectories map[string]struct{}) error {
	rows, err := tx.QueryContext(ctx, `SELECT canonical_id, body FROM og_objects WHERE computer_id=? AND object_kind=? FOR UPDATE`, computerID, ogKindSupervisionState)
	if err != nil {
		return fmt.Errorf("computer event projection: list prior supervision state: %w", err)
	}
	for rows.Next() {
		var id, body string
		if err := rows.Scan(&id, &body); err != nil {
			rows.Close()
			return err
		}
		ids[id] = struct{}{}
		var state supervisionProjectionState
		if json.Unmarshal([]byte(body), &state) == nil && state.TrajectoryID != "" {
			trajectories[state.TrajectoryID] = struct{}{}
			if err := rebuildStateOwnedObjectIDs(state, ids); err != nil {
				rows.Close()
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	kinds := []objectgraph.ObjectKind{ogKindSupervisionState, ogKindSupervisionEvent, ogKindLifecycleEvent}
	placeholders := make([]string, len(kinds))
	args := make([]any, 0, len(kinds)+1)
	args = append(args, computerID)
	for index, kind := range kinds {
		placeholders[index] = "?"
		args = append(args, kind)
	}
	candidateRows, err := tx.QueryContext(ctx, `SELECT canonical_id, metadata FROM og_objects WHERE computer_id=? AND object_kind IN (`+strings.Join(placeholders, ",")+`) FOR UPDATE`, args...)
	if err != nil {
		return fmt.Errorf("computer event projection: list replay objects: %w", err)
	}
	for candidateRows.Next() {
		var id, metadata string
		if err := candidateRows.Scan(&id, &metadata); err != nil {
			candidateRows.Close()
			return err
		}
		var fields map[string]any
		if json.Unmarshal([]byte(metadata), &fields) == nil {
			if trajectoryID, _ := fields["trajectory_id"].(string); trajectoryID != "" {
				if _, owned := trajectories[trajectoryID]; owned {
					ids[id] = struct{}{}
				}
			}
		}
	}
	if err := candidateRows.Err(); err != nil {
		candidateRows.Close()
		return err
	}
	if err := candidateRows.Close(); err != nil {
		return err
	}
	return deleteRebuildIDsTx(ctx, tx, ids)
}

func deleteRebuildIDsTx(ctx context.Context, tx *sql.Tx, ids map[string]struct{}) error {
	for id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM og_objects WHERE canonical_id=?`, id); err != nil {
			return fmt.Errorf("computer event projection: delete replay object: %w", err)
		}
	}
	return nil
}

func rebuildStateOwnedObjectIDs(state supervisionProjectionState, ids map[string]struct{}) error {
	add := func(kind objectgraph.ObjectKind, key string) error {
		if key == "" {
			return nil
		}
		id, err := lifecycleCanonicalID(kind, state.OwnerID, state.ComputerID, key)
		if err == nil {
			ids[id] = struct{}{}
		}
		return err
	}
	if err := add(ogKindTrajectory, state.TrajectoryID); err != nil {
		return err
	}
	if err := add(ogKindTexDoc, state.ArtifactID); err != nil {
		return err
	}
	if err := add(ogKindTexRev, state.ArtifactHeadRevisionID); err != nil {
		return err
	}
	for _, raw := range state.Entities["assignment_opened"] {
		body, err := supervisionBodyMap(raw)
		if err != nil {
			return err
		}
		if err := add(ogKindWorkItem, bodyString(body, "assignment_id")); err != nil {
			return err
		}
		if err := add(ogKindAgent, bodyString(body, "assigned_actor_id")); err != nil {
			return err
		}
	}
	for _, raw := range state.Entities["trajectory_started"] {
		body, err := supervisionBodyMap(raw)
		if err != nil {
			return err
		}
		for _, assignmentID := range bodyStrings(body, "initial_assignment_ids") {
			if err := add(ogKindWorkItem, assignmentID); err != nil {
				return err
			}
		}
		if err := add(ogKindAgent, bodyString(body, "texture_actor_id")); err != nil {
			return err
		}
	}
	for _, raw := range state.Entities["texture_revision"] {
		body, err := supervisionBodyMap(raw)
		if err != nil {
			return err
		}
		if err := add(ogKindTexDoc, bodyString(body, "artifact_id")); err != nil {
			return err
		}
		if err := add(ogKindTexRev, bodyString(body, "revision_id")); err != nil {
			return err
		}
	}
	for entityKind, entities := range state.Entities {
		var kind objectgraph.ObjectKind
		switch entityKind {
		case "imported_trajectory":
			kind = ogKindTrajectory
		case "imported_document":
			kind = ogKindTexDoc
		case "imported_revision":
			kind = ogKindTexRev
		case "imported_assignment":
			kind = ogKindWorkItem
		case "imported_agent":
			kind = ogKindAgent
		case "imported_update":
			kind = ogKindWorkerUpdate
		case "imported_source_entity":
			kind = TextureSourceEntityObjectKind
		case "imported_source_ref":
			kind = TextureSourceRefObjectKind
		default:
			continue
		}
		for identity := range entities {
			if err := add(kind, identity); err != nil {
				return err
			}
		}
	}
	return nil
}

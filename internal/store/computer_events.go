package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func (s *Store) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("computer event projection: nil store")
	}
	return scanProjectionHead(s.db.QueryRowContext(ctx, projectionHeadSQL, computerID))
}

// ReadDB returns the store's read pool: WAL readers that never take the write
// connection, so a long read (a full-store witness) does not queue writers.
// Each read transaction sees one committed snapshot. It falls back to the
// write handle for a store with no separate read pool. The caller must not
// close it, and must call ReadDB again after a Reopen rather than keep it.
func (s *Store) ReadDB() *sql.DB {
	if s == nil {
		return nil
	}
	if s.readDB != nil {
		return s.readDB
	}
	return s.db
}

// ReadSnapshotTx begins a read-only transaction on the read pool. In WAL mode
// the snapshot is fixed by the transaction's first read, so a caller that
// reads the projection head first (HeadTx) while the event appender is held
// gets rows and head of one state, then may release the hold and keep
// reading. The caller rolls the transaction back.
func (s *Store) ReadSnapshotTx(ctx context.Context) (*sql.Tx, error) {
	db := s.ReadDB()
	if db == nil {
		return nil, fmt.Errorf("computer event projection: nil store")
	}
	return db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
}

// HeadTx reads the projection head inside tx. It is the first read of a
// ReadSnapshotTx transaction when the head must belong to the snapshot.
func (s *Store) HeadTx(ctx context.Context, tx *sql.Tx, computerID string) (*computerevent.Head, error) {
	if tx == nil {
		return nil, fmt.Errorf("computer event projection: transaction is required")
	}
	return scanProjectionHead(tx.QueryRowContext(ctx, projectionHeadSQL, computerID))
}

const projectionHeadSQL = `SELECT computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, COALESCE(pending_transition_ref, ''), desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch FROM computer_event_projection_heads WHERE computer_id=?`

func scanProjectionHead(row *sql.Row) (*computerevent.Head, error) {
	var head computerevent.Head
	err := row.Scan(&head.ComputerID, &head.Sequence, &head.CanonicalEventHead, &head.DesiredEventHead, &head.EffectiveEventHead, &head.PendingTransitionRef, &head.DesiredStateCommitment, &head.EffectiveStateCommitment, &head.ReducerVersion, &head.CredentialRevocationEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("computer event projection: read head: %w", err)
	}
	return &head, nil
}

// NewestEventOfKind reports the newest finalized event of kind on this
// store's local index: its sequence and when it finalized. It reads the local
// index only, never the tape.
func (s *Store) NewestEventOfKind(ctx context.Context, computerID string, kind computerevent.EventKind) (sequence uint64, at time.Time, found bool, err error) {
	db := s.ReadDB()
	if db == nil {
		return 0, time.Time{}, false, fmt.Errorf("computer event index: nil store")
	}
	var finalized sql.NullTime
	var prepared time.Time
	row := db.QueryRowContext(ctx, `SELECT sequence, finalized_at, prepared_at FROM computer_event_index WHERE computer_id=? AND event_kind=? AND status='finalized' ORDER BY sequence DESC LIMIT 1`, computerID, string(kind))
	if err := row.Scan(&sequence, &finalized, &prepared); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, time.Time{}, false, nil
		}
		return 0, time.Time{}, false, fmt.Errorf("computer event index: newest %s: %w", kind, err)
	}
	at = prepared
	if finalized.Valid {
		at = finalized.Time
	}
	return sequence, at, true, nil
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
	_, err = s.db.ExecContext(ctx, `INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, prepared_event_json, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_pending_transition_ref, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, target_state_commitment, restored_prior_effective, prepared_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'prepared', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, request.EventDigest, event.ComputerID, event.Sequence, event.PreviousHead, event.EventKind, string(eventJSON), request.EventArtifactDigest, request.EventPinReceiptDigest, string(pinsJSON), event.RequestCommitment, event.IdempotencyKey, request.Next.DesiredEventHead, request.Next.EffectiveEventHead, nullableEventString(request.Next.PendingTransitionRef), request.Next.DesiredStateCommitment, request.Next.EffectiveStateCommitment, request.Next.ReducerVersion, request.Next.CredentialRevocationEpoch, nullableEventString(request.Input.TargetStateCommitment), request.Input.RestoredPriorEffective, now)
	if err != nil {
		return fmt.Errorf("computer event projection: prepare: %w", err)
	}
	s.eventBodies.putEvent(event.ComputerID, request.EventDigest, event)
	return nil
}

func (s *Store) Finalize(ctx context.Context, computerID, eventDigest string, receipt computerevent.Receipt) error {
	return s.FinalizeBatch(ctx, computerID, eventDigest, receipt, nil)
}

func (s *Store) FinalizeBatch(ctx context.Context, computerID, eventDigest string, receipt computerevent.Receipt, batch *computerevent.ProjectionBatch) error {
	return s.finalizeBatch(ctx, computerID, eventDigest, receipt, batch, false)
}

// FinalizeReplayBatch applies a resolved projection while reconstructing a
// canonical tape into a disposable store. It is the only path allowed to use
// legacy-row compatibility; live finalization remains strict. The resolved
// deposit view is upcast to the active serving vocabulary in canonical event
// order (vocab_upcast_deposit.go) when this store's deposits are upcast:
// fresh reconstructions and cut-over stores, never a retained V1 store whose
// one-time cutover is MigrateAndFenceServingVocabulary's job.
func (s *Store) FinalizeReplayBatch(ctx context.Context, computerID, eventDigest string, receipt computerevent.Receipt, batch *computerevent.ProjectionBatch) error {
	return s.finalizeBatch(ctx, computerID, eventDigest, receipt, batch, true)
}

// DryRunProjectionBatch validates a projection batch against committed state
// without writing (implements computerevent.BatchDryRunner). The appender
// calls it before pin/CAS so a batch that could never finalize is refused to
// the caller instead of becoming canonical poison (2026-09-03 seq 138612).
// Live guards remain authoritative: state that changes between dry-run and
// finalize can still fail closed at finalize time.
func (s *Store) DryRunProjectionBatch(ctx context.Context, computerID string, batch *computerevent.ProjectionBatch) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("computer event projection: nil store")
	}
	if batch == nil {
		return fmt.Errorf("computer event projection: dry-run batch is required")
	}
	// Validation must see the same deposit view finalize will apply. The
	// dry-run is side-effect free: it never registers aliases.
	if err := s.upcastProjectionDeposit(batch, false, false, false); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("computer event projection: dry-run begin: %w", err)
	}
	defer tx.Rollback()
	pos, err := dryRunTapePosition(ctx, tx, computerID)
	if err != nil {
		return err
	}
	if s.dryRunPositionHook != nil {
		s.dryRunPositionHook(pos)
	}
	// A retire op's body is validated here and applied only at finalize:
	// applying it to roll it back would pay the delete twice, under the lock
	// a retirement is bounded to keep short (object_retire.go).
	if err := batch.Validate(); err != nil {
		return err
	}
	dry := *batch
	dry.Ops = make([]computerevent.ProjectionOp, 0, len(batch.Ops))
	for _, op := range batch.Ops {
		if strings.TrimSpace(op.Kind) != computerevent.ProjectionOpObjectRetire {
			dry.Ops = append(dry.Ops, op)
		}
	}
	if len(dry.Ops) == 0 {
		return nil
	}
	return projectBatch(ctx, tx, dry, pos, nil)
}

func (s *Store) finalizeBatch(ctx context.Context, computerID, eventDigest string, receipt computerevent.Receipt, batch *computerevent.ProjectionBatch, allowLegacyTextureBootstrap bool) error {
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sequence, nextReducerVersion, nextCredentialRevocationEpoch uint64
	var previousHead, status, desiredHead, effectiveHead, desiredCommitment, effectiveCommitment string
	var pending sql.NullString
	var eventKind string
	var preparedEvent sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT sequence, previous_head, status, event_kind, prepared_event_json, next_desired_event_head, next_effective_event_head, next_pending_transition_ref, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch FROM computer_event_index WHERE computer_id=? AND event_digest=?`, computerID, eventDigest).Scan(&sequence, &previousHead, &status, &eventKind, &preparedEvent, &desiredHead, &effectiveHead, &pending, &desiredCommitment, &effectiveCommitment, &nextReducerVersion, &nextCredentialRevocationEpoch)
	if err != nil {
		return fmt.Errorf("computer event projection: load prepared event: %w", err)
	}
	if status == "finalized" {
		return nil
	}
	if status != "prepared" {
		return fmt.Errorf("computer event projection: cannot finalize status %q", status)
	}
	if eventKind == string(computerevent.EventProjectionBatchRecorded) {
		if batch == nil {
			return fmt.Errorf("computer event projection: projection_batch_recorded requires a resolved batch")
		}
	} else if batch != nil {
		return fmt.Errorf("computer event projection: batch requires projection_batch_recorded")
	}
	if batch != nil {
		if err := batch.Validate(); err != nil {
			return err
		}
		if batch.ComputerID != computerID || batch.EventDigest != eventDigest {
			return fmt.Errorf("%w: batch identity", computerevent.ErrProjectionBatchInvalid)
		}
	}
	var currentSequence uint64
	var currentHead string
	headErr := tx.QueryRowContext(ctx, `SELECT sequence, canonical_event_head FROM computer_event_projection_heads WHERE computer_id=?`, computerID).Scan(&currentSequence, &currentHead)
	// A missing projection head means this computer has never been projected
	// in this store: a from-scratch reconstruction, which is when the
	// versioned replayed-deposit upcast activates (vocab_upcast_deposit.go).
	storeWasEmpty := errors.Is(headErr, sql.ErrNoRows)
	err = headErr
	if storeWasEmpty {
		if sequence != 1 || previousHead != computerevent.ZeroHead {
			return computerevent.ErrProjectionMismatch
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, pending_transition_ref, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, computerID, sequence, eventDigest, desiredHead, effectiveHead, nullableEventString(pending.String), desiredCommitment, effectiveCommitment, nextReducerVersion, nextCredentialRevocationEpoch, time.Now().UTC())
	} else if err == nil {
		if currentSequence+1 != sequence || currentHead != previousHead {
			return computerevent.ErrProjectionMismatch
		}
		_, err = tx.ExecContext(ctx, `UPDATE computer_event_projection_heads SET sequence=?, canonical_event_head=?, desired_event_head=?, effective_event_head=?, pending_transition_ref=?, desired_state_commitment=?, effective_state_commitment=?, reducer_version=?, credential_revocation_epoch=?, updated_at=? WHERE computer_id=? AND sequence=? AND canonical_event_head=?`, sequence, eventDigest, desiredHead, effectiveHead, nullableEventString(pending.String), desiredCommitment, effectiveCommitment, nextReducerVersion, nextCredentialRevocationEpoch, time.Now().UTC(), computerID, currentSequence, currentHead)
	}
	if err != nil {
		return fmt.Errorf("computer event projection: update head: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE computer_event_index SET status='finalized', prepared_event_json=NULL, event_head_receipt_digest=?, finalized_at=? WHERE computer_id=? AND event_digest=? AND status='prepared'`, receiptDigest, time.Now().UTC(), computerID, eventDigest)
	if err != nil {
		return fmt.Errorf("computer event projection: finalize event: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return fmt.Errorf("computer event projection: finalize CAS lost")
	}
	// Versioned projection-deposit upcast: the event, receipt, payload digest
	// and reducer commitment were verified upstream; only the resolved deposit
	// view is transformed, in canonical event order. The decision is resolved
	// on every finalize (including the batchless genesis) so a from-scratch
	// reconstruction is detected while its missing head row is still
	// observable (vocab_upcast_deposit.go).
	if err := s.upcastProjectionDeposit(batch, allowLegacyTextureBootstrap, storeWasEmpty, true); err != nil {
		return err
	}
	if allowLegacyTextureBootstrap {
		// Replay bypasses the OG write guard: record what it writes, and
		// durably mark a fenced store before the first deposit commits
		// (vocab_replay_fence.go).
		if err := s.noteVocabReplayDeposit(batch, sequence); err != nil {
			return fmt.Errorf("computer event projection: %w", err)
		}
	}
	if batch != nil {
		var projectErr error
		if allowLegacyTextureBootstrap {
			// Replay writes og_objects without the side table: withdraw the
			// families (a replayed row would otherwise read as missing) and
			// let the online pass fill it.
			s.derivedKeys().InvalidateAll("replay projection")
			projectErr = projectBatchForReplay(ctx, tx, *batch, sequence)
			// Again before the commit, so a pass that published while the
			// batch was written cannot leave a family readable.
			s.derivedKeys().InvalidateAll("replay projection")
		} else {
			projectErr = projectBatch(ctx, tx, *batch, sequence, s.derivedKeys())
		}
		if projectErr != nil {
			return projectErr
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// The guest keeps no event bodies once an event is final (build sheet
	// decision 8): remember this one for the lookups that follow, and let the
	// tape serve it after that.
	if preparedEvent.Valid && preparedEvent.String != "" {
		if event, decodeErr := computerevent.DecodeHistoricEvent([]byte(preparedEvent.String)); decodeErr == nil {
			s.eventBodies.putEvent(computerID, eventDigest, event)
		}
	}
	s.eventBodies.putReceipt(computerID, eventDigest, receipt)
	return nil
}

// CommitReplay is the boundary a complete replay ends on. The deposit upcast
// alias ledger is synced first, so a crash-resumed replay can still resolve
// references recorded against legacy IDs. SQL COMMIT already made each
// replayed event durable; there is no history commit.
func (s *Store) CommitReplay(ctx context.Context) error {
	if err := s.syncDepositUpcastLedger(); err != nil {
		return err
	}
	return s.syncRelaxedReplay(ctx)
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
	return nil
}

func (s *Store) Prepared(ctx context.Context, computerID string) ([]computerevent.CASRequest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT prepared_event_json, event_digest, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, next_desired_event_head, next_effective_event_head, COALESCE(next_pending_transition_ref, ''), next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, COALESCE(target_state_commitment, ''), restored_prior_effective FROM computer_event_index WHERE computer_id=? AND status='prepared' ORDER BY sequence`, computerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []computerevent.CASRequest
	for rows.Next() {
		var request computerevent.CASRequest
		var rawEvent sql.NullString
		var rawPins string
		if err := rows.Scan(&rawEvent, &request.EventDigest, &request.EventArtifactDigest, &request.EventPinReceiptDigest, &rawPins, &request.Next.DesiredEventHead, &request.Next.EffectiveEventHead, &request.Next.PendingTransitionRef, &request.Next.DesiredStateCommitment, &request.Next.EffectiveStateCommitment, &request.Next.ReducerVersion, &request.Next.CredentialRevocationEpoch, &request.Input.TargetStateCommitment, &request.Input.RestoredPriorEffective); err != nil {
			return nil, err
		}
		if !rawEvent.Valid || rawEvent.String == "" {
			return nil, fmt.Errorf("computer event projection: prepared event body is missing")
		}
		event, err := computerevent.DecodeHistoricEvent([]byte(rawEvent.String))
		if err != nil {
			return nil, err
		}
		request.Event = event
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

// EventByIdempotency returns the finalized event recorded under an
// idempotency key, for recovery of a controller action that was interrupted
// after the canonical append. The guest keeps no event bodies: the body comes
// from the recent-event cache or from the tape (event_bodies.go).
func (s *Store) EventByIdempotency(ctx context.Context, computerID, idempotencyKey string) (computerevent.Event, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: nil store")
	}
	var digest, status string
	var sequence uint64
	err := s.db.QueryRowContext(ctx, `SELECT event_digest, sequence, status FROM computer_event_index WHERE computer_id=? AND idempotency_key=?`, computerID, idempotencyKey).Scan(&digest, &sequence, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Event{}, false, nil
	}
	if err != nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: read idempotent event: %w", err)
	}
	if status != "finalized" {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: event is %s", status)
	}
	record, err := s.finalizedEventBody(ctx, computerID, digest, sequence)
	if err != nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: decode event: %w", err)
	}
	return record.Request.Event, true, nil
}

func (s *Store) EventByDigest(ctx context.Context, computerID, eventDigest string) (computerevent.Event, bool, error) {
	computerID, eventDigest = strings.TrimSpace(computerID), strings.TrimSpace(eventDigest)
	var sequence uint64
	err := s.db.QueryRowContext(ctx, `SELECT sequence FROM computer_event_index WHERE computer_id=? AND event_digest=? AND status='finalized'`, computerID, eventDigest).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Event{}, false, nil
	}
	if err != nil {
		return computerevent.Event{}, false, err
	}
	record, err := s.finalizedEventBody(ctx, computerID, eventDigest, sequence)
	if err != nil {
		return computerevent.Event{}, false, err
	}
	return record.Request.Event, true, nil
}

// LatestComputerEventByIdempotencyPrefix returns the most recent finalized
// event whose idempotency key starts with prefix — the sweep surface for
// transition families that key each phase as <prefix><id>. A missing row is
// absence, never an error.
//
// It reads the family by a range on the unique (computer_id,
// idempotency_key) key, narrow columns only, picks the newest finalized
// member in Go and then reads that one event. The earlier LIKE ... ORDER BY
// sequence DESC LIMIT 1 walked every finalized event of the computer
// newest-first and read event_json per row, and held the paper's engine for
// minutes after every boot
// (docs/problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md).
func (s *Store) LatestComputerEventByIdempotencyPrefix(ctx context.Context, computerID, prefix string) (computerevent.Event, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: nil store")
	}
	// No utf8mb4 key starts with a prefix that is not UTF-8, and the engine
	// refuses one as a parameter.
	if !utf8.ValidString(prefix) {
		return computerevent.Event{}, false, nil
	}
	query, args := idempotencyFamilySQL(computerID, prefix)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: read idempotency family: %w", err)
	}
	var latestDigest string
	var latestSequence uint64
	for rows.Next() {
		var sequence uint64
		var digest, status, key string
		if err := rows.Scan(&sequence, &digest, &status, &key); err != nil {
			_ = rows.Close()
			return computerevent.Event{}, false, fmt.Errorf("computer event projection: scan idempotency family: %w", err)
		}
		// The range is exact under the column's binary collation; the
		// prefix check keeps the result literal under any other.
		if status != "finalized" || !strings.HasPrefix(key, prefix) {
			continue
		}
		if latestDigest == "" || sequence > latestSequence {
			latestDigest, latestSequence = digest, sequence
		}
	}
	if err := rows.Close(); err != nil {
		return computerevent.Event{}, false, err
	}
	if err := rows.Err(); err != nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: read idempotency family: %w", err)
	}
	if latestDigest == "" {
		return computerevent.Event{}, false, nil
	}
	event, found, err := s.EventByDigest(ctx, computerID, latestDigest)
	if err != nil {
		return computerevent.Event{}, false, fmt.Errorf("computer event projection: read latest event by idempotency prefix: %w", err)
	}
	return event, found, nil
}

// idempotencyFamilySQL is the range read of one idempotency-key family:
// keys >= prefix and below the least string greater than every string that
// starts with prefix (code-point order, which the binary collation keeps).
func idempotencyFamilySQL(computerID, prefix string) (string, []any) {
	const columns = `SELECT sequence, event_digest, status, idempotency_key FROM computer_event_index WHERE computer_id=? AND idempotency_key >= ?`
	if upper, ok := prefixUpperBound(prefix); ok {
		return columns + ` AND idempotency_key < ?`, []any{computerID, prefix, upper}
	}
	return columns, []any{computerID, prefix}
}

// prefixUpperBound returns the least string that sorts after every string
// starting with prefix: prefix with its last incrementable rune incremented
// (surrogates skipped) and what follows it dropped. ok is false when no rune
// can be incremented (an empty prefix, or only U+10FFFF) or the prefix is not
// UTF-8, and then the family is unbounded above (the caller returns absence
// for a prefix that is not UTF-8 before reading).
func prefixUpperBound(prefix string) (string, bool) {
	if !utf8.ValidString(prefix) {
		return "", false
	}
	runes := []rune(prefix)
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] >= utf8.MaxRune {
			continue
		}
		next := runes[i] + 1
		if next >= 0xD800 && next <= 0xDFFF {
			next = 0xE000
		}
		return string(runes[:i]) + string(next), true
	}
	return "", false
}

func (s *Store) EventReceiptByIdempotency(ctx context.Context, computerID, idempotencyKey string) (computerevent.Receipt, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.Receipt{}, false, fmt.Errorf("computer event projection: nil store")
	}
	var digest, status string
	var sequence uint64
	err := s.db.QueryRowContext(ctx, `SELECT event_digest, sequence, status FROM computer_event_index WHERE computer_id=? AND idempotency_key=?`, computerID, idempotencyKey).Scan(&digest, &sequence, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Receipt{}, false, nil
	}
	if err != nil || status != "finalized" {
		return computerevent.Receipt{}, false, fmt.Errorf("computer event projection: finalized receipt unavailable")
	}
	record, err := s.finalizedEventBody(ctx, computerID, digest, sequence)
	if err != nil {
		return computerevent.Receipt{}, false, fmt.Errorf("computer event projection: finalized receipt unavailable: %w", err)
	}
	return record.Receipt, true, nil
}

// EventReceiptByDigest returns the finalized append receipt for the event
// identified by digest — the join the checkpoint authority requires between
// `event_head_receipt_id` and the event it receipts. Absence is not an error.
func (s *Store) EventReceiptByDigest(ctx context.Context, computerID, eventDigest string) (computerevent.Receipt, bool, error) {
	if s == nil || s.db == nil {
		return computerevent.Receipt{}, false, fmt.Errorf("computer event projection: nil store")
	}
	computerID, eventDigest = strings.TrimSpace(computerID), strings.TrimSpace(eventDigest)
	var status string
	var sequence uint64
	err := s.db.QueryRowContext(ctx, `SELECT sequence, status FROM computer_event_index WHERE computer_id=? AND event_digest=?`, computerID, eventDigest).Scan(&sequence, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Receipt{}, false, nil
	}
	if err != nil || status != "finalized" {
		return computerevent.Receipt{}, false, fmt.Errorf("computer event projection: finalized receipt for digest unavailable")
	}
	record, err := s.finalizedEventBody(ctx, computerID, eventDigest, sequence)
	if err != nil {
		return computerevent.Receipt{}, false, fmt.Errorf("computer event projection: finalized receipt for digest unavailable: %w", err)
	}
	return record.Receipt, true, nil
}

func nullableEventString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

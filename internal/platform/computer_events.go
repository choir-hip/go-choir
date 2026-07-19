package platform

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const computerEventSchemaDDL = `
CREATE TABLE IF NOT EXISTS computer_event_heads (
  computer_id VARCHAR(128) NOT NULL,
  sequence BIGINT UNSIGNED NOT NULL,
  canonical_event_head CHAR(64) NOT NULL,
  desired_event_head CHAR(64) NOT NULL,
  effective_event_head CHAR(64) NOT NULL,
  desired_state_commitment CHAR(64) NOT NULL,
  effective_state_commitment CHAR(64) NOT NULL,
  pending_transition_ref CHAR(64) NULL,
  reducer_version BIGINT UNSIGNED NOT NULL,
  credential_revocation_epoch BIGINT UNSIGNED NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (computer_id),
  CHECK (sequence > 0),
  CHECK (CHAR_LENGTH(canonical_event_head) = 64),
  CHECK (CHAR_LENGTH(desired_event_head) = 64),
  CHECK (CHAR_LENGTH(effective_event_head) = 64),
  CHECK (CHAR_LENGTH(desired_state_commitment) = 64),
  CHECK (CHAR_LENGTH(effective_state_commitment) = 64)
);
CREATE TABLE IF NOT EXISTS computer_event_append_receipts (
  computer_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  request_commitment CHAR(64) NOT NULL,
  sequence BIGINT UNSIGNED NOT NULL,
  previous_head CHAR(64) NOT NULL,
  event_kind VARCHAR(64) NOT NULL,
  event_digest CHAR(64) NOT NULL,
  event_artifact_ref TEXT NOT NULL,
  event_pin_receipt_digest CHAR(64) NOT NULL,
  pin_receipt_digests_json JSON NOT NULL,
  event_head_receipt_id VARCHAR(255) NOT NULL,
  event_head_receipt_json JSON NOT NULL,
  event_head_receipt_digest CHAR(64) NOT NULL,
  desired_event_head CHAR(64) NOT NULL,
  effective_event_head CHAR(64) NOT NULL,
  desired_state_commitment CHAR(64) NOT NULL,
  effective_state_commitment CHAR(64) NOT NULL,
  pending_transition_ref CHAR(64) NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (computer_id, idempotency_key),
  UNIQUE KEY computer_event_append_sequence_uq (computer_id, sequence),
  UNIQUE KEY computer_event_append_digest_uq (computer_id, event_digest)
);
CREATE TABLE IF NOT EXISTS control_key_history (
  signer_domain VARCHAR(64) NOT NULL,
  computer_id VARCHAR(128) NOT NULL DEFAULT '',
  key_id VARCHAR(255) NOT NULL,
  public_key BINARY(32) NOT NULL,
  status VARCHAR(32) NOT NULL,
  activation_sequence BIGINT UNSIGNED NULL,
  activation_time DATETIME(6) NULL,
  first_invalid_sequence BIGINT UNSIGNED NULL,
  first_invalid_time DATETIME(6) NULL,
  replacement_key_id VARCHAR(255) NULL,
  authorizing_receipt_json JSON NOT NULL,
  authorizing_receipt_digest CHAR(64) NOT NULL,
  inserted_at DATETIME(6) NOT NULL,
  PRIMARY KEY (signer_domain, computer_id, key_id),
  KEY control_key_history_status_idx (signer_domain, computer_id, status, activation_sequence),
  KEY control_key_history_receipt_idx (authorizing_receipt_digest)
);
CREATE TABLE IF NOT EXISTS computer_self_development_modes (
  computer_id VARCHAR(128) NOT NULL,
  mode VARCHAR(32) NOT NULL,
  generation BIGINT UNSIGNED NOT NULL,
  operation_id VARCHAR(255) NULL,
  bundle_digest CHAR(64) NULL,
  expected_desired_event_head CHAR(64) NULL,
  expected_effective_event_head CHAR(64) NULL,
  expected_pending_transition_ref VARCHAR(255) NULL,
  expected_desired_state_commitment CHAR(64) NULL,
  expected_effective_state_commitment CHAR(64) NULL,
  expires_at DATETIME(6) NULL,
  last_idempotency_key VARCHAR(255) NOT NULL,
  last_request_commitment CHAR(64) NOT NULL,
  mode_receipt_json JSON NOT NULL,
  mode_receipt_digest CHAR(64) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (computer_id),
  KEY computer_self_development_modes_expiry_idx (mode, expires_at)
);
CREATE TABLE IF NOT EXISTS computer_lifecycle_operations (
  computer_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  request_commitment CHAR(64) NOT NULL,
  action VARCHAR(32) NOT NULL,
  prior_lifecycle_state VARCHAR(32) NOT NULL,
  prior_realization_epoch BIGINT UNSIGNED NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  PRIMARY KEY (computer_id, idempotency_key)
);
CREATE TABLE IF NOT EXISTS computer_lifecycle_receipts (
  computer_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  receipt_id VARCHAR(255) NOT NULL,
  request_commitment CHAR(64) NOT NULL,
  action VARCHAR(32) NOT NULL,
  prior_lifecycle_state VARCHAR(32) NOT NULL,
  resulting_lifecycle_state VARCHAR(32) NOT NULL,
  generation BIGINT UNSIGNED NOT NULL,
  receipt_json JSON NOT NULL,
  receipt_digest CHAR(64) NOT NULL,
  completed_at DATETIME(6) NOT NULL,
  joined_event_digest CHAR(64) NULL,
  PRIMARY KEY (computer_id, idempotency_key),
  UNIQUE KEY computer_lifecycle_receipt_id_uq (receipt_id),
  KEY computer_lifecycle_generation_idx (computer_id, generation)
);
CREATE TABLE IF NOT EXISTS computer_checkpoints (
  computer_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  request_commitment CHAR(64) NOT NULL,
  checkpoint_digest CHAR(64) NOT NULL,
  checkpoint_artifact_ref TEXT NOT NULL,
  checkpoint_json JSON NOT NULL,
  receipt_json JSON NOT NULL,
  receipt_digest CHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (computer_id, idempotency_key),
  UNIQUE KEY computer_checkpoint_digest_uq (checkpoint_digest)
);
CREATE TABLE IF NOT EXISTS computer_route_projection_certificates (
  computer_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  request_commitment CHAR(64) NOT NULL,
  certificate_digest CHAR(64) NOT NULL,
  certificate_json JSON NOT NULL,
  receipt_json JSON NOT NULL,
  receipt_digest CHAR(64) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (computer_id, idempotency_key),
  UNIQUE KEY computer_route_projection_certificate_digest_uq (certificate_digest)
);`

var ErrComputerEventCASConflict = errors.New("computer event head CAS conflict")

type PinReceiptValidator interface {
	ValidateEventPins(ctx context.Context, request computerevent.CASRequest) error
}

type ComputerEventCAS struct {
	locks      [64]sync.Mutex
	store      *Store
	issuer     string
	signingKey computerevent.SigningKey
	pins       PinReceiptValidator
	now        func() time.Time
}

func NewComputerEventCAS(store *Store, issuer string, signingKey computerevent.SigningKey, pins PinReceiptValidator) (*ComputerEventCAS, error) {
	if store == nil || store.db == nil || issuer != "corpusd" || pins == nil || len(signingKey.PrivateKey) != ed25519.PrivateKeySize || signingKey.SignerDomain != "platform-control" || signingKey.KeyID == "" {
		return nil, fmt.Errorf("computer event CAS: complete store, signer, and pin validator are required")
	}
	return &ComputerEventCAS{store: store, issuer: issuer, signingKey: signingKey, pins: pins, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (c *ComputerEventCAS) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	return readComputerEventHead(ctx, c.store.db, computerID, false)
}

func (c *ComputerEventCAS) CompareAndSwap(ctx context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	if request.Event.ComputerID == "" || request.EventDigest == "" || request.EventArtifactDigest != request.EventDigest || request.EventPinReceiptDigest == "" {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: incomplete request")
	}
	pinIntentCommitment, err := computerevent.ComputePinIntentCommitment(request.Event, request.Input)
	if err != nil || request.PinIntentCommitment != pinIntentCommitment {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: pin intent commitment mismatch")
	}
	requestCommitment, err := computerevent.ComputeRequestCommitment(request.Event, request.Input, pinIntentCommitment, request.PayloadPinReceiptDigests)
	if err != nil || request.Event.RequestCommitment != requestCommitment {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: request commitment mismatch")
	}
	lock := &c.locks[computerEventLockIndex(request.Event.ComputerID)]
	lock.Lock()
	defer lock.Unlock()
	if err := c.pins.ValidateEventPins(ctx, request); err != nil {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: pin verification: %w", err)
	}
	if receipt, found, err := existingComputerEventReceipt(ctx, c.store.db, request.Event.ComputerID, request.Event.IdempotencyKey, request.Event.RequestCommitment); err != nil || found {
		return receipt, err
	}

	tx, err := c.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: begin: %w", err)
	}
	defer tx.Rollback()
	if receipt, found, err := existingComputerEventReceipt(ctx, tx, request.Event.ComputerID, request.Event.IdempotencyKey, request.Event.RequestCommitment); err != nil || found {
		return receipt, err
	}
	current, err := readComputerEventHead(ctx, tx, request.Event.ComputerID, true)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	next, err := computerevent.Reduce(current, request.Event, request.Input)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	if !reflect.DeepEqual(next, request.Next) {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: guest projection mismatch")
	}
	computedDigest, err := request.Event.Digest()
	if err != nil || computedDigest != request.EventDigest {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: event digest mismatch")
	}

	now := c.now().UTC()
	fields := eventHeadReceiptFields(request)
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", c.issuer, fields, []computerevent.SigningKey{c.signingKey}, now)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	receiptJSON, err := receipt.CanonicalBytes()
	if err != nil {
		return computerevent.Receipt{}, err
	}
	receiptDigest := computerevent.DigestBytes(receiptJSON)
	pending := nullableString(next.PendingTransitionRef)
	if current == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO computer_event_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, pending_transition_ref, reducer_version, credential_revocation_epoch, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, next.ComputerID, next.Sequence, next.CanonicalEventHead, next.DesiredEventHead, next.EffectiveEventHead, next.DesiredStateCommitment, next.EffectiveStateCommitment, pending, next.ReducerVersion, next.CredentialRevocationEpoch, now, now)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `UPDATE computer_event_heads SET sequence=?, canonical_event_head=?, desired_event_head=?, effective_event_head=?, desired_state_commitment=?, effective_state_commitment=?, pending_transition_ref=?, reducer_version=?, credential_revocation_epoch=?, updated_at=? WHERE computer_id=? AND sequence=? AND canonical_event_head=?`, next.Sequence, next.CanonicalEventHead, next.DesiredEventHead, next.EffectiveEventHead, next.DesiredStateCommitment, next.EffectiveStateCommitment, pending, next.ReducerVersion, next.CredentialRevocationEpoch, now, next.ComputerID, current.Sequence, current.CanonicalEventHead)
		if err == nil {
			if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
				return computerevent.Receipt{}, ErrComputerEventCASConflict
			}
		}
	}
	if err != nil {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: update head: %w", err)
	}
	pinsJSON, err := json.Marshal(request.PayloadPinReceiptDigests)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, previous_head, event_kind, event_digest, event_artifact_ref, event_pin_receipt_digest, pin_receipt_digests_json, event_head_receipt_id, event_head_receipt_json, event_head_receipt_digest, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, pending_transition_ref, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, request.Event.ComputerID, request.Event.IdempotencyKey, request.Event.RequestCommitment, request.Event.Sequence, request.Event.PreviousHead, request.Event.EventKind, request.EventDigest, request.EventArtifactDigest, request.EventPinReceiptDigest, string(pinsJSON), receipt.ReceiptID, string(receiptJSON), receiptDigest, next.DesiredEventHead, next.EffectiveEventHead, next.DesiredStateCommitment, next.EffectiveStateCommitment, pending, now)
	if err != nil {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: insert receipt: %w", err)
	}
	if request.Event.EventKind == computerevent.EventLifecycleObserved {
		result, joinErr := tx.ExecContext(ctx, `UPDATE computer_lifecycle_receipts SET joined_event_digest=? WHERE computer_id=? AND receipt_digest=? AND action IN ('start','stop','restart') AND joined_event_digest IS NULL`, request.EventDigest, request.Event.ComputerID, request.Event.ProposedEffectRef)
		if joinErr != nil {
			return computerevent.Receipt{}, fmt.Errorf("computer event CAS: join lifecycle receipt: %w", joinErr)
		}
		if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
			return computerevent.Receipt{}, fmt.Errorf("computer event CAS: lifecycle receipt join unavailable")
		}
	}
	if err := tx.Commit(); err != nil {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: commit: %w", err)
	}
	if err := c.store.commitDolt(ctx, "append computer event "+request.EventDigest); err != nil {
		return computerevent.Receipt{}, err
	}
	return receipt, nil
}

func existingComputerEventReceipt(ctx context.Context, db queryRower, computerID, idempotencyKey, requestCommitment string) (computerevent.Receipt, bool, error) {
	var storedCommitment, receiptJSON string
	err := db.QueryRowContext(ctx, `SELECT request_commitment, event_head_receipt_json FROM computer_event_append_receipts WHERE computer_id=? AND idempotency_key=?`, computerID, idempotencyKey).Scan(&storedCommitment, &receiptJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return computerevent.Receipt{}, false, nil
	}
	if err != nil {
		return computerevent.Receipt{}, false, err
	}
	if storedCommitment != requestCommitment {
		return computerevent.Receipt{}, true, fmt.Errorf("%w: idempotency commitment changed", ErrComputerEventCASConflict)
	}
	var receipt computerevent.Receipt
	if err := json.Unmarshal([]byte(receiptJSON), &receipt); err != nil {
		return computerevent.Receipt{}, true, fmt.Errorf("computer event CAS: decode durable receipt: %w", err)
	}
	return receipt, true, nil
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readComputerEventHead(ctx context.Context, db queryRower, computerID string, forUpdate bool) (*computerevent.Head, error) {
	query := `SELECT computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, COALESCE(pending_transition_ref, ''), desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch FROM computer_event_heads WHERE computer_id=?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var head computerevent.Head
	err := db.QueryRowContext(ctx, query, computerID).Scan(&head.ComputerID, &head.Sequence, &head.CanonicalEventHead, &head.DesiredEventHead, &head.EffectiveEventHead, &head.PendingTransitionRef, &head.DesiredStateCommitment, &head.EffectiveStateCommitment, &head.ReducerVersion, &head.CredentialRevocationEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("computer event CAS: read head: %w", err)
	}
	return &head, nil
}

func eventHeadReceiptFields(request computerevent.CASRequest) map[string]any {
	return map[string]any{
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
}
func computerEventLockIndex(computerID string) int {
	var hash uint64 = 1469598103934665603
	for index := range len(computerID) {
		hash ^= uint64(computerID[index])
		hash *= 1099511628211
	}
	return int(hash % 64)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

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
	"github.com/yusefmosiah/go-choir/internal/dbx"
)

const computerEventSchemaDDL = `
CREATE TABLE IF NOT EXISTS computer_event_heads (
  computer_id VARCHAR(128) NOT NULL,
  sequence BIGINT NOT NULL,
  canonical_event_head CHAR(64) NOT NULL,
  desired_event_head CHAR(64) NOT NULL,
  effective_event_head CHAR(64) NOT NULL,
  desired_state_commitment CHAR(64) NOT NULL,
  effective_state_commitment CHAR(64) NOT NULL,
  pending_transition_ref CHAR(64) NULL,
  reducer_version BIGINT NOT NULL,
  credential_revocation_epoch BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
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
  sequence BIGINT NOT NULL,
  event_kind VARCHAR(64) NOT NULL,
  event_digest CHAR(64) NOT NULL,
  event_pin_receipt_digest CHAR(64) NOT NULL,
  event_head_receipt_id VARCHAR(255) NOT NULL,
  event_head_receipt_json TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (computer_id, idempotency_key),
  CONSTRAINT computer_event_append_sequence_uq UNIQUE (computer_id, sequence),
  CONSTRAINT computer_event_append_digest_uq UNIQUE (computer_id, event_digest)
);
CREATE INDEX IF NOT EXISTS computer_event_append_receipt_id_idx ON computer_event_append_receipts (computer_id, event_head_receipt_id);
CREATE INDEX IF NOT EXISTS computer_event_append_kind_seq_idx ON computer_event_append_receipts (computer_id, event_kind, sequence);
CREATE TABLE IF NOT EXISTS control_key_history (
  signer_domain VARCHAR(64) NOT NULL,
  computer_id VARCHAR(128) NOT NULL DEFAULT '',
  key_id VARCHAR(255) NOT NULL,
  public_key BYTEA NOT NULL,
  status VARCHAR(32) NOT NULL,
  activation_sequence BIGINT NULL,
  activation_time TIMESTAMPTZ NULL,
  first_invalid_sequence BIGINT NULL,
  first_invalid_time TIMESTAMPTZ NULL,
  replacement_key_id VARCHAR(255) NULL,
  authorizing_receipt_json TEXT NOT NULL,
  authorizing_receipt_digest CHAR(64) NOT NULL,
  inserted_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (signer_domain, computer_id, key_id)
);
CREATE INDEX IF NOT EXISTS control_key_history_status_idx ON control_key_history (signer_domain, computer_id, status, activation_sequence);
CREATE INDEX IF NOT EXISTS control_key_history_receipt_idx ON control_key_history (authorizing_receipt_digest);
CREATE TABLE IF NOT EXISTS computer_self_development_modes (
  computer_id VARCHAR(128) NOT NULL,
  mode VARCHAR(32) NOT NULL,
  generation BIGINT NOT NULL,
  operation_id VARCHAR(255) NULL,
  bundle_digest CHAR(64) NULL,
  expected_desired_event_head CHAR(64) NULL,
  expected_effective_event_head CHAR(64) NULL,
  expected_pending_transition_ref VARCHAR(255) NULL,
  expected_desired_state_commitment CHAR(64) NULL,
  expected_effective_state_commitment CHAR(64) NULL,
  expires_at TIMESTAMPTZ NULL,
  last_idempotency_key VARCHAR(255) NOT NULL,
  last_request_commitment CHAR(64) NOT NULL,
  mode_receipt_json TEXT NOT NULL,
  mode_receipt_digest CHAR(64) NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (computer_id)
);
CREATE INDEX IF NOT EXISTS computer_self_development_modes_expiry_idx ON computer_self_development_modes (mode, expires_at);
CREATE TABLE IF NOT EXISTS computer_lifecycle_operations (
  computer_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  request_commitment CHAR(64) NOT NULL,
  action VARCHAR(32) NOT NULL,
  prior_lifecycle_state VARCHAR(32) NOT NULL,
  prior_realization_epoch BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ NULL,
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
  generation BIGINT NOT NULL,
  receipt_json TEXT NOT NULL,
  receipt_digest CHAR(64) NOT NULL,
  completed_at TIMESTAMPTZ NOT NULL,
  joined_event_digest CHAR(64) NULL,
  PRIMARY KEY (computer_id, idempotency_key),
  CONSTRAINT computer_lifecycle_receipt_id_uq UNIQUE (receipt_id)
);
CREATE INDEX IF NOT EXISTS computer_lifecycle_generation_idx ON computer_lifecycle_receipts (computer_id, generation);
CREATE TABLE IF NOT EXISTS computer_checkpoints (
  computer_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  request_commitment CHAR(64) NOT NULL,
  checkpoint_digest CHAR(64) NOT NULL,
  checkpoint_artifact_ref TEXT NOT NULL,
  checkpoint_json TEXT NOT NULL,
  receipt_json TEXT NOT NULL,
  receipt_digest CHAR(64) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (computer_id, idempotency_key),
  CONSTRAINT computer_checkpoint_digest_uq UNIQUE (checkpoint_digest)
);
CREATE TABLE IF NOT EXISTS computer_route_projection_certificates (
  computer_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  request_commitment CHAR(64) NOT NULL,
  certificate_digest CHAR(64) NOT NULL,
  certificate_json TEXT NOT NULL,
  receipt_json TEXT NOT NULL,
  receipt_digest CHAR(64) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (computer_id, idempotency_key),
  CONSTRAINT computer_route_projection_certificate_digest_uq UNIQUE (certificate_digest)
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
	// ownerActs is the S1 effect_accepted admission gate; nil is the
	// pre-S1 rule (the reducer alone decides).
	ownerActs *OwnerActAuthority
}

// ConfigureEffectAdmission installs the S1 gate: an effect_accepted append
// is admitted only when its authority traces to an offer corpusd minted or
// to an owner-armed ModeReceipt corpusd issued.
func (c *ComputerEventCAS) ConfigureEffectAdmission(authority *OwnerActAuthority) {
	c.ownerActs = authority
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
	// A tombstoned computer's tape is archived/deleted: refuse the append
	// explicitly (the tape sweep writes the tombstone before any file goes).
	// The gate is held through the commit so the sweep's post-tombstone
	// head check sees every append that passed this check.
	c.store.tombstones.gate.RLock()
	defer c.store.tombstones.gate.RUnlock()
	if err := c.store.refuseTombstonedTape(ctx, request.Event.ComputerID); err != nil {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: %w", err)
	}
	if err := c.pins.ValidateEventPins(ctx, request); err != nil {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: pin verification: %w", err)
	}
	if receipt, found, err := existingComputerEventReceipt(ctx, c.store.db, request.Event.ComputerID, request.Event.IdempotencyKey, request.Event.RequestCommitment); err != nil || found {
		return receipt, err
	}
	// The per-computer lock serializes this computer's appends, but
	// SERIALIZABLE can still fail one against another computer's transaction
	// (predicate locks are per page). The retry re-reads the head.
	return dbx.Retry(ctx, 5, func() (computerevent.Receipt, error) {
		return c.compareAndSwapTx(ctx, request)
	})
}

// compareAndSwapTx is one serializable attempt at the append; the caller
// holds the computer's lock and the tombstone gate.
func (c *ComputerEventCAS) compareAndSwapTx(ctx context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
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
	if request.Event.EventKind == computerevent.EventEffectAccepted && c.ownerActs != nil {
		if err := c.ownerActs.admitEffectAccepted(ctx, tx, request.Event, computedDigest); err != nil {
			return computerevent.Receipt{}, err
		}
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
	// The receipt JSON is stored once: previous head, the next heads and
	// commitments and the pin digests are all fields of the signed receipt.
	_, err = tx.ExecContext(ctx, `INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, request.Event.ComputerID, request.Event.IdempotencyKey, request.Event.RequestCommitment, request.Event.Sequence, request.Event.EventKind, request.EventDigest, request.EventPinReceiptDigest, receipt.ReceiptID, string(receiptJSON), now)
	if err != nil {
		return computerevent.Receipt{}, fmt.Errorf("computer event CAS: insert receipt: %w", err)
	}
	if request.Event.EventKind == computerevent.EventLifecycleObserved || request.Event.EventKind == computerevent.EventKeyRevoked {
		allowedActions := "('start','stop','restart','refresh')"
		if request.Event.EventKind == computerevent.EventKeyRevoked {
			allowedActions = "('credential_envelope_consumed')"
		}
		result, joinErr := tx.ExecContext(ctx, `UPDATE computer_lifecycle_receipts SET joined_event_digest=? WHERE computer_id=? AND receipt_digest=? AND action IN `+allowedActions+` AND joined_event_digest IS NULL`, request.EventDigest, request.Event.ComputerID, request.Event.ProposedEffectRef)
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

package platform

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

type ControlKey struct {
	SignerDomain         string
	ComputerID           string
	KeyID                string
	PublicKey            ed25519.PublicKey
	Status               string
	ActivationSequence   *uint64
	ActivationTime       *time.Time
	FirstInvalidSequence *uint64
	FirstInvalidTime     *time.Time
	ReplacementKeyID     string
	AuthorizingReceipt   computerevent.Receipt
}

type ControlKeyResolver struct {
	Store *Store
}

func (r ControlKeyResolver) ResolveReceiptKey(domain, computerID, keyID string, sequence uint64, issuedAt time.Time) (ed25519.PublicKey, error) {
	if r.Store == nil || r.Store.db == nil {
		return nil, fmt.Errorf("control key resolver: nil store")
	}
	key, err := r.Store.ControlKey(context.Background(), domain, computerID, keyID)
	if errors.Is(err, sql.ErrNoRows) && computerID != "" {
		key, err = r.Store.ControlKey(context.Background(), domain, "", keyID)
	}
	if err != nil {
		return nil, err
	}
	if key.Status != "active" && key.Status != "revoked" {
		return nil, fmt.Errorf("control key resolver: key is not active")
	}
	if key.ActivationSequence != nil && sequence > 0 && sequence < *key.ActivationSequence {
		return nil, fmt.Errorf("control key resolver: receipt predates key activation sequence")
	}
	if key.ActivationTime != nil && issuedAt.Before(*key.ActivationTime) {
		return nil, fmt.Errorf("control key resolver: receipt predates key activation time")
	}
	if key.FirstInvalidSequence != nil && sequence >= *key.FirstInvalidSequence {
		return nil, fmt.Errorf("control key resolver: key invalid at sequence")
	}
	if key.FirstInvalidTime != nil && !issuedAt.Before(*key.FirstInvalidTime) {
		return nil, fmt.Errorf("control key resolver: key invalid at receipt time")
	}
	return append(ed25519.PublicKey(nil), key.PublicKey...), nil
}

func (s *Store) insertControlKey(ctx context.Context, key ControlKey) error {
	if s == nil || s.db == nil || key.SignerDomain == "" || key.KeyID == "" || len(key.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("control key history: invalid key")
	}
	if key.Status != "pending" && key.Status != "active" && key.Status != "revoked" {
		return fmt.Errorf("control key history: invalid status %q", key.Status)
	}
	receiptJSON, err := key.AuthorizingReceipt.CanonicalBytes()
	if err != nil {
		return fmt.Errorf("control key history: authorizing receipt: %w", err)
	}
	if key.AuthorizingReceipt.ReceiptID == "" {
		return fmt.Errorf("control key history: authorizing receipt is required")
	}
	activationSequence := nullableUint64(key.ActivationSequence)
	activationTime := nullableTime(key.ActivationTime)
	invalidSequence := nullableUint64(key.FirstInvalidSequence)
	invalidTime := nullableTime(key.FirstInvalidTime)
	_, err = s.db.ExecContext(ctx, `INSERT INTO control_key_history (signer_domain, computer_id, key_id, public_key, status, activation_sequence, activation_time, first_invalid_sequence, first_invalid_time, replacement_key_id, authorizing_receipt_json, authorizing_receipt_digest, inserted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, key.SignerDomain, key.ComputerID, key.KeyID, []byte(key.PublicKey), key.Status, activationSequence, activationTime, invalidSequence, invalidTime, nullableControlString(key.ReplacementKeyID), string(receiptJSON), computerevent.DigestBytes(receiptJSON), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("control key history: insert: %w", err)
	}
	return s.commitDolt(ctx, "insert control key "+key.SignerDomain+"/"+key.KeyID)
}

func (s *Store) activateControlKey(ctx context.Context, domain, computerID, keyID string, sequence uint64, at time.Time) error {
	if sequence == 0 || at.IsZero() {
		return fmt.Errorf("control key history: activation sequence and time required")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE control_key_history SET status='active', activation_sequence=?, activation_time=? WHERE signer_domain=? AND computer_id=? AND key_id=? AND status='pending'`, sequence, at.UTC(), domain, computerID, keyID)
	if err != nil {
		return err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return fmt.Errorf("control key history: activation CAS failed")
	}
	return s.commitDolt(ctx, "activate control key "+domain+"/"+keyID)
}

func (s *Store) revokeControlKey(ctx context.Context, domain, computerID, keyID, replacementKeyID string, firstInvalidSequence uint64, firstInvalidTime time.Time) error {
	if replacementKeyID == "" || firstInvalidSequence == 0 || firstInvalidTime.IsZero() {
		return fmt.Errorf("control key history: replacement and invalid cutoff required")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE control_key_history SET status='revoked', first_invalid_sequence=?, first_invalid_time=?, replacement_key_id=? WHERE signer_domain=? AND computer_id=? AND key_id=? AND status='active'`, firstInvalidSequence, firstInvalidTime.UTC(), replacementKeyID, domain, computerID, keyID)
	if err != nil {
		return err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return fmt.Errorf("control key history: revocation CAS failed")
	}
	return s.commitDolt(ctx, "revoke control key "+domain+"/"+keyID)
}

func (s *Store) ControlKey(ctx context.Context, domain, computerID, keyID string) (ControlKey, error) {
	var key ControlKey
	var publicKey []byte
	var activationSequence, invalidSequence sql.NullInt64
	var activationTime, invalidTime sql.NullTime
	var replacement sql.NullString
	var receiptJSON string
	err := s.db.QueryRowContext(ctx, `SELECT signer_domain, computer_id, key_id, public_key, status, activation_sequence, activation_time, first_invalid_sequence, first_invalid_time, replacement_key_id, authorizing_receipt_json FROM control_key_history WHERE signer_domain=? AND computer_id=? AND key_id=?`, domain, computerID, keyID).Scan(&key.SignerDomain, &key.ComputerID, &key.KeyID, &publicKey, &key.Status, &activationSequence, &activationTime, &invalidSequence, &invalidTime, &replacement, &receiptJSON)
	if err != nil {
		return ControlKey{}, err
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return ControlKey{}, fmt.Errorf("control key history: corrupt public key")
	}
	key.PublicKey = append(ed25519.PublicKey(nil), publicKey...)
	if activationSequence.Valid {
		value := uint64(activationSequence.Int64)
		key.ActivationSequence = &value
	}
	if activationTime.Valid {
		value := activationTime.Time.UTC()
		key.ActivationTime = &value
	}
	if invalidSequence.Valid {
		value := uint64(invalidSequence.Int64)
		key.FirstInvalidSequence = &value
	}
	if invalidTime.Valid {
		value := invalidTime.Time.UTC()
		key.FirstInvalidTime = &value
	}
	key.ReplacementKeyID = replacement.String
	if err := json.Unmarshal([]byte(receiptJSON), &key.AuthorizingReceipt); err != nil {
		return ControlKey{}, fmt.Errorf("control key history: decode authorizing receipt: %w", err)
	}
	return key, nil
}

func nullableUint64(value *uint64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullableControlString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

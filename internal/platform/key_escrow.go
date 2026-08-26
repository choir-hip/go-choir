package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrKeyEscrowNotFound    = errors.New("key escrow not found")
	ErrSelfApproval         = errors.New("key unwrap request cannot be self-approved")
	ErrDuplicateApproval    = errors.New("key unwrap request already approved by operator")
	ErrKeyUnwrapNotPending  = errors.New("key unwrap request is not pending")
	ErrKeyUnwrapNotApproved = errors.New("key unwrap request is not approved")
)

type KeyEscrowStatus struct {
	Protector  string    `json:"protector"`
	KeyDigest  string    `json:"key_digest"`
	EscrowedAt time.Time `json:"escrowed_at"`
}

type KeyUnwrapRequest struct {
	RequestID   string     `json:"request_id"`
	ComputerID  string     `json:"computer_id"`
	RequestedBy string     `json:"requested_by"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	RevealedAt  *time.Time `json:"revealed_at,omitempty"`
}

func (s *Store) UpsertKeyEscrow(ctx context.Context, computerID, protector string, wrappedJSON []byte, keyDigest string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("key escrow: nil database")
	}
	if strings.TrimSpace(computerID) == "" || strings.TrimSpace(protector) == "" || len(wrappedJSON) == 0 || strings.TrimSpace(keyDigest) == "" {
		return fmt.Errorf("key escrow: complete escrow record is required")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO computer_key_escrows (computer_id, protector, wrap_version, wrapped_key_json, key_digest, escrowed_at, updated_at)
		VALUES (?, ?, 1, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE wrapped_key_json=VALUES(wrapped_key_json), key_digest=VALUES(key_digest), updated_at=VALUES(updated_at)`,
		computerID, protector, string(wrappedJSON), keyDigest, now, now); err != nil {
		return fmt.Errorf("key escrow: upsert: %w", err)
	}
	return s.commitDolt(ctx, "upsert key escrow "+computerID+"/"+protector)
}

func (s *Store) GetKeyEscrow(ctx context.Context, computerID, protector string) (wrappedJSON []byte, keyDigest string, err error) {
	if s == nil || s.db == nil {
		return nil, "", fmt.Errorf("key escrow: nil database")
	}
	var wrapped string
	err = s.db.QueryRowContext(ctx, `SELECT wrapped_key_json, key_digest FROM computer_key_escrows WHERE computer_id=? AND protector=?`, computerID, protector).Scan(&wrapped, &keyDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrKeyEscrowNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("key escrow: get: %w", err)
	}
	return []byte(wrapped), keyDigest, nil
}

func (s *Store) ListKeyEscrowStatus(ctx context.Context, computerID string) ([]KeyEscrowStatus, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("key escrow: nil database")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT protector, key_digest, escrowed_at FROM computer_key_escrows WHERE computer_id=? ORDER BY protector`, computerID)
	if err != nil {
		return nil, fmt.Errorf("key escrow: list status: %w", err)
	}
	defer rows.Close()
	statuses := make([]KeyEscrowStatus, 0)
	for rows.Next() {
		var status KeyEscrowStatus
		if err := rows.Scan(&status.Protector, &status.KeyDigest, &status.EscrowedAt); err != nil {
			return nil, fmt.Errorf("key escrow: scan status: %w", err)
		}
		statuses = append(statuses, status)
	}
	return statuses, rows.Err()
}

func (s *Store) CreateKeyUnwrapRequest(ctx context.Context, computerID, requestedBy, reason, idempotencyKey string) (KeyUnwrapRequest, error) {
	if s == nil || s.db == nil {
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: nil database")
	}
	if strings.TrimSpace(computerID) == "" || strings.TrimSpace(requestedBy) == "" || strings.TrimSpace(reason) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: complete unwrap request is required")
	}
	if request, err := s.keyUnwrapRequestByIdempotency(ctx, idempotencyKey); err == nil {
		return request, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return KeyUnwrapRequest{}, err
	}
	request := KeyUnwrapRequest{
		RequestID: uuid.NewString(), ComputerID: computerID, RequestedBy: requestedBy, Reason: reason,
		Status: "pending", CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO computer_key_unwrap_requests (request_id, computer_id, requested_by, reason, status, idempotency_key, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		request.RequestID, request.ComputerID, request.RequestedBy, request.Reason, request.Status, idempotencyKey, request.CreatedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			return s.keyUnwrapRequestByIdempotency(ctx, idempotencyKey)
		}
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: create unwrap request: %w", err)
	}
	if err := s.commitDolt(ctx, "create key unwrap request "+request.RequestID); err != nil {
		return KeyUnwrapRequest{}, err
	}
	return request, nil
}

func (s *Store) ApproveKeyUnwrapRequest(ctx context.Context, requestID, approver string) (KeyUnwrapRequest, error) {
	if s == nil || s.db == nil {
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: nil database")
	}
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(approver) == "" {
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: request id and approver are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: begin approval: %w", err)
	}
	defer tx.Rollback()
	request, err := scanKeyUnwrapRequest(tx.QueryRowContext(ctx, `SELECT request_id, computer_id, requested_by, reason, status, created_at, revealed_at FROM computer_key_unwrap_requests WHERE request_id=? FOR UPDATE`, requestID))
	if errors.Is(err, sql.ErrNoRows) {
		return KeyUnwrapRequest{}, ErrKeyEscrowNotFound
	}
	if err != nil {
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: get approval request: %w", err)
	}
	if request.RequestedBy == approver {
		return KeyUnwrapRequest{}, ErrSelfApproval
	}
	if request.Status != "pending" {
		return KeyUnwrapRequest{}, ErrKeyUnwrapNotPending
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO computer_key_unwrap_approvals (request_id, approver, approved_at) VALUES (?, ?, ?)`, requestID, approver, time.Now().UTC().Truncate(time.Microsecond)); isDuplicateKeyError(err) {
		return KeyUnwrapRequest{}, ErrDuplicateApproval
	} else if err != nil {
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: insert approval: %w", err)
	}
	var approvalCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_key_unwrap_approvals WHERE request_id=?`, requestID).Scan(&approvalCount); err != nil {
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: count approvals: %w", err)
	}
	if approvalCount >= 2 {
		if _, err := tx.ExecContext(ctx, `UPDATE computer_key_unwrap_requests SET status='approved' WHERE request_id=? AND status='pending'`, requestID); err != nil {
			return KeyUnwrapRequest{}, fmt.Errorf("key escrow: mark request approved: %w", err)
		}
		request.Status = "approved"
	}
	if err := tx.Commit(); err != nil {
		return KeyUnwrapRequest{}, fmt.Errorf("key escrow: commit approval: %w", err)
	}
	if err := s.commitDolt(ctx, "approve key unwrap request "+requestID); err != nil {
		return KeyUnwrapRequest{}, err
	}
	return request, nil
}

func (s *Store) GetKeyUnwrapRequest(ctx context.Context, requestID string) (KeyUnwrapRequest, []string, error) {
	if s == nil || s.db == nil {
		return KeyUnwrapRequest{}, nil, fmt.Errorf("key escrow: nil database")
	}
	request, err := scanKeyUnwrapRequest(s.db.QueryRowContext(ctx, `SELECT request_id, computer_id, requested_by, reason, status, created_at, revealed_at FROM computer_key_unwrap_requests WHERE request_id=?`, requestID))
	if errors.Is(err, sql.ErrNoRows) {
		return KeyUnwrapRequest{}, nil, ErrKeyEscrowNotFound
	}
	if err != nil {
		return KeyUnwrapRequest{}, nil, fmt.Errorf("key escrow: get unwrap request: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT approver FROM computer_key_unwrap_approvals WHERE request_id=? ORDER BY approver`, requestID)
	if err != nil {
		return KeyUnwrapRequest{}, nil, fmt.Errorf("key escrow: list approvals: %w", err)
	}
	defer rows.Close()
	approvers := make([]string, 0)
	for rows.Next() {
		var approver string
		if err := rows.Scan(&approver); err != nil {
			return KeyUnwrapRequest{}, nil, fmt.Errorf("key escrow: scan approval: %w", err)
		}
		approvers = append(approvers, approver)
	}
	if err := rows.Err(); err != nil {
		return KeyUnwrapRequest{}, nil, err
	}
	return request, approvers, nil
}

func (s *Store) MarkKeyUnwrapRevealed(ctx context.Context, requestID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("key escrow: nil database")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	result, err := s.db.ExecContext(ctx, `UPDATE computer_key_unwrap_requests SET status='revealed', revealed_at=? WHERE request_id=? AND status='approved'`, now, requestID)
	if err != nil {
		return fmt.Errorf("key escrow: mark revealed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("key escrow: mark revealed rows: %w", err)
	}
	if affected == 0 {
		if _, _, err := s.GetKeyUnwrapRequest(ctx, requestID); errors.Is(err, ErrKeyEscrowNotFound) {
			return ErrKeyEscrowNotFound
		}
		return ErrKeyUnwrapNotApproved
	}
	return s.commitDolt(ctx, "reveal key unwrap request "+requestID)
}

func (s *Store) AppendKeyEscrowTransparency(ctx context.Context, payloadJSON []byte) (seq int64, entryHash string, err error) {
	if s == nil || s.db == nil {
		return 0, "", fmt.Errorf("key escrow: nil database")
	}
	if !json.Valid(payloadJSON) {
		return 0, "", fmt.Errorf("key escrow: transparency payload must be JSON")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", fmt.Errorf("key escrow: begin transparency append: %w", err)
	}
	defer tx.Rollback()
	prevHash := ""
	var priorSeq int64
	var priorHash string
	err = tx.QueryRowContext(ctx, `SELECT seq, entry_hash FROM computer_key_escrow_transparency ORDER BY seq DESC LIMIT 1 FOR UPDATE`).Scan(&priorSeq, &priorHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, "", fmt.Errorf("key escrow: transparency head: %w", err)
	}
	if err == nil {
		prevHash = priorHash
	}
	preimage := make([]byte, 0, len(prevHash)+len(payloadJSON))
	preimage = append(preimage, prevHash...)
	preimage = append(preimage, payloadJSON...)
	digest := sha256.Sum256(preimage)
	entryHash = hex.EncodeToString(digest[:])
	result, err := tx.ExecContext(ctx, `INSERT INTO computer_key_escrow_transparency (prev_hash, entry_hash, payload_json, created_at) VALUES (?, ?, ?, ?)`, prevHash, entryHash, string(payloadJSON), time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		return 0, "", fmt.Errorf("key escrow: insert transparency: %w", err)
	}
	seq, err = result.LastInsertId()
	if err != nil {
		return 0, "", fmt.Errorf("key escrow: transparency sequence: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("key escrow: commit transparency: %w", err)
	}
	if err := s.commitDolt(ctx, fmt.Sprintf("append key escrow transparency %d", seq)); err != nil {
		return 0, "", err
	}
	return seq, entryHash, nil
}

func (s *Store) KeyEscrowTransparencyHead(ctx context.Context) (seq int64, headHash string, err error) {
	if s == nil || s.db == nil {
		return 0, "", fmt.Errorf("key escrow: nil database")
	}
	err = s.db.QueryRowContext(ctx, `SELECT seq, entry_hash FROM computer_key_escrow_transparency ORDER BY seq DESC LIMIT 1`).Scan(&seq, &headHash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", fmt.Errorf("key escrow: transparency head: %w", err)
	}
	return seq, headHash, nil
}

func (s *Store) keyUnwrapRequestByIdempotency(ctx context.Context, idempotencyKey string) (KeyUnwrapRequest, error) {
	request, err := scanKeyUnwrapRequest(s.db.QueryRowContext(ctx, `SELECT request_id, computer_id, requested_by, reason, status, created_at, revealed_at FROM computer_key_unwrap_requests WHERE idempotency_key=?`, idempotencyKey))
	if err != nil {
		return KeyUnwrapRequest{}, err
	}
	return request, nil
}

type keyUnwrapRequestRow interface {
	Scan(dest ...any) error
}

func scanKeyUnwrapRequest(row keyUnwrapRequestRow) (KeyUnwrapRequest, error) {
	var request KeyUnwrapRequest
	var revealedAt sql.NullTime
	if err := row.Scan(&request.RequestID, &request.ComputerID, &request.RequestedBy, &request.Reason, &request.Status, &request.CreatedAt, &revealedAt); err != nil {
		return KeyUnwrapRequest{}, err
	}
	if revealedAt.Valid {
		value := revealedAt.Time.UTC()
		request.RevealedAt = &value
	}
	return request, nil
}

func isDuplicateKeyError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate")
}

package bundle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// The reads below are the platform's own queries on its tables
// (internal/platform computer_events.go, snapshot_roots.go, file_cas.go). The
// db must be opened through dbx.OpenPostgres, which rebinds "?".

const receiptColumns = `computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at`

func readHead(ctx context.Context, q dbx.Queryer, computerID string, forUpdate bool) (*computerevent.Head, error) {
	query := `SELECT computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, COALESCE(pending_transition_ref, ''), desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch FROM computer_event_heads WHERE computer_id=?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var head computerevent.Head
	err := q.QueryRowContext(ctx, query, computerID).Scan(&head.ComputerID, &head.Sequence, &head.CanonicalEventHead, &head.DesiredEventHead, &head.EffectiveEventHead, &head.PendingTransitionRef, &head.DesiredStateCommitment, &head.EffectiveStateCommitment, &head.ReducerVersion, &head.CredentialRevocationEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("bundle: read head: %w", err)
	}
	return &head, nil
}

// readReceipts returns up to limit receipts with after < sequence <= upTo.
func readReceipts(ctx context.Context, q dbx.Queryer, computerID string, after, upTo int64, limit int) ([]ReceiptRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+receiptColumns+` FROM computer_event_append_receipts WHERE computer_id=? AND sequence>? AND sequence<=? ORDER BY sequence LIMIT ?`, computerID, after, upTo, limit)
	if err != nil {
		return nil, fmt.Errorf("bundle: read receipts: %w", err)
	}
	defer rows.Close()
	var out []ReceiptRow
	for rows.Next() {
		var r ReceiptRow
		if err := rows.Scan(&r.ComputerID, &r.IdempotencyKey, &r.RequestCommitment, &r.Sequence, &r.EventKind, &r.EventDigest, &r.EventPinReceiptDigest, &r.EventHeadReceiptID, &r.EventHeadReceiptJSON, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("bundle: scan receipt: %w", err)
		}
		r.CreatedAt = r.CreatedAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// receiptDigestAt returns the event digest of the receipt at seq, "" when
// there is none.
func receiptDigestAt(ctx context.Context, q dbx.Queryer, computerID string, seq int64) (string, error) {
	var digest string
	err := q.QueryRowContext(ctx, `SELECT event_digest FROM computer_event_append_receipts WHERE computer_id=? AND sequence=?`, computerID, seq).Scan(&digest)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("bundle: read receipt at %d: %w", seq, err)
	}
	return digest, nil
}

// readSnapshotRoot returns the named root, or the newest one when root is ""
// (RecoveryBase's order). nil when there is none.
func readSnapshotRoot(ctx context.Context, q dbx.Queryer, computerID, root string) (*SnapshotRoot, error) {
	query := `SELECT computer_id, root, head_sequence, chunks, created_at FROM computer_snapshot_roots WHERE computer_id=?`
	args := []any{computerID}
	if root != "" {
		query += ` AND root=?`
		args = append(args, root)
	}
	query += ` ORDER BY head_sequence DESC, created_at DESC, root DESC LIMIT 1`
	var rec SnapshotRoot
	var encoded string
	err := q.QueryRowContext(ctx, query, args...).Scan(&rec.ComputerID, &rec.Root, &rec.HeadSequence, &encoded, &rec.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("bundle: read snapshot root: %w", err)
	}
	if err := json.Unmarshal([]byte(encoded), &rec.Chunks); err != nil {
		return nil, fmt.Errorf("bundle: snapshot root %s chunk list unreadable: %w", rec.Root, err)
	}
	rec.CreatedAt = rec.CreatedAt.UTC()
	return &rec, nil
}

// tombstoned reports whether the computer's tape is tombstoned (archived or
// deleted by the tape sweep).
func tombstoned(ctx context.Context, q dbx.Queryer, computerID string) (bool, error) {
	exists, err := dbx.TableExists(ctx, q, dbx.Postgres, "computer_tape_tombstones")
	if err != nil || !exists {
		return false, err
	}
	var n int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_tape_tombstones WHERE computer_id=?`, computerID).Scan(&n); err != nil {
		return false, fmt.Errorf("bundle: read tombstone: %w", err)
	}
	return n > 0, nil
}

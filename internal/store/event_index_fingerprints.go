package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// EventIndexFingerprint is one computer_event_index row reduced to digests
// for the replay-completeness diagnostic: the columns the content witness
// hashes (every column but prepared_at and finalized_at), none of their
// values. It names which rows and which columns differ between the live
// store and a replay when the table digest says only that they do.
type EventIndexFingerprint struct {
	EventDigest string `json:"event_digest"`
	Sequence    int64  `json:"sequence"`
	EventKind   string `json:"event_kind"`
	Status      string `json:"status"`
	RowDigest   string `json:"row_digest"`
	// FieldDigests maps a column to the first 16 hex characters of its
	// digest: enough to tell two values apart in a diagnostic, and small
	// enough that a 100,000-row index fits in memory.
	FieldDigests map[string]string `json:"field_digests"`
}

// readQuerier is the part of *sql.DB and *sql.Tx the fingerprint listers use.
type readQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// ListEventIndexFingerprints returns a fingerprint of every row of
// computer_event_index the probe compares at the trim boundary (see
// EventIndexWitnessFilter), ordered by sequence, and the number of rows it
// left out as disposable. A null pin list is fingerprinted as "[]", as the
// probe's extractor reads it.
func (s *Store) ListEventIndexFingerprints(ctx context.Context, trimBoundary uint64) ([]EventIndexFingerprint, int, error) {
	return listEventIndexFingerprints(ctx, s.db, trimBoundary)
}

// ListEventIndexFingerprintsTx is ListEventIndexFingerprints inside a
// caller's read snapshot (ReadSnapshotTx), so the fingerprints are of the
// state the snapshot's other reads see.
func (s *Store) ListEventIndexFingerprintsTx(ctx context.Context, tx *sql.Tx, trimBoundary uint64) ([]EventIndexFingerprint, int, error) {
	if tx == nil {
		return nil, 0, fmt.Errorf("event index fingerprints: transaction is required")
	}
	return listEventIndexFingerprints(ctx, tx, trimBoundary)
}

func listEventIndexFingerprints(ctx context.Context, q readQuerier, boundary uint64) ([]EventIndexFingerprint, int, error) {
	disposable, err := countEventIndexDisposable(ctx, q, boundary)
	if err != nil {
		return nil, 0, err
	}
	query := `SELECT * FROM computer_event_index`
	if filter := EventIndexWitnessFilter(boundary); filter != "" {
		query += ` WHERE ` + filter
	}
	rows, err := q.QueryContext(ctx, query+` ORDER BY sequence ASC, event_digest ASC`)
	if err != nil {
		return nil, 0, fmt.Errorf("query event index fingerprints: %w", err)
	}
	defer func() { _ = rows.Close() }()
	names, err := rows.Columns()
	if err != nil {
		return nil, 0, fmt.Errorf("event index fingerprint columns: %w", err)
	}
	ignored := EventIndexIgnoredColumns()
	values := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range values {
		ptrs[i] = &values[i]
	}
	var out []EventIndexFingerprint
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, 0, fmt.Errorf("scan event index fingerprint: %w", err)
		}
		fp := EventIndexFingerprint{FieldDigests: make(map[string]string, len(names))}
		whole := sha256.New()
		for i, name := range names {
			if _, skip := ignored[name]; skip {
				continue
			}
			value := values[i]
			if name == EventIndexPinListColumn {
				value = emptyListFromNull(value)
			}
			encoded := fingerprintValue(value)
			sum := sha256.Sum256(encoded)
			fp.FieldDigests[name] = hex.EncodeToString(sum[:8])
			whole.Write([]byte(name))
			whole.Write(encoded)
			switch name {
			case "event_digest":
				fp.EventDigest = fmt.Sprint(value)
			case "sequence":
				if n, ok := value.(int64); ok {
					fp.Sequence = n
				}
			case "event_kind":
				fp.EventKind = fmt.Sprint(value)
			case "status":
				fp.Status = fmt.Sprint(value)
			}
		}
		fp.RowDigest = hex.EncodeToString(whole.Sum(nil))
		out = append(out, fp)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate event index fingerprints: %w", err)
	}
	return out, disposable, nil
}

func countEventIndexDisposable(ctx context.Context, q readQuerier, boundary uint64) (int, error) {
	if boundary == 0 {
		return 0, nil
	}
	rows, err := q.QueryContext(ctx, `SELECT COUNT(*) FROM computer_event_index WHERE event_kind = ? AND status = 'finalized' AND sequence <= ?`,
		string(computerevent.EventProjectionBatchRecorded), int64(boundary))
	if err != nil {
		return 0, fmt.Errorf("count disposable event index rows: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var n int
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return 0, err
		}
	}
	return n, rows.Err()
}

func emptyListFromNull(value any) any {
	switch v := value.(type) {
	case string:
		if v == "null" {
			return "[]"
		}
	case []byte:
		if string(v) == "null" {
			return "[]"
		}
	}
	return value
}

// fingerprintValue is a type-tagged encoding, so NULL, 0 and ” differ.
func fingerprintValue(value any) []byte {
	switch v := value.(type) {
	case nil:
		return []byte("n:")
	case int64:
		return []byte(fmt.Sprintf("i:%d", v))
	case float64:
		return []byte(fmt.Sprintf("r:%v", v))
	case bool:
		return []byte(fmt.Sprintf("i:%d", map[bool]int{false: 0, true: 1}[v]))
	case string:
		return append([]byte("t:"), v...)
	case []byte:
		return append([]byte("b:"), v...)
	case time.Time:
		return []byte("d:" + v.UTC().Format(time.RFC3339Nano))
	}
	return []byte(fmt.Sprintf("?:%v", value))
}

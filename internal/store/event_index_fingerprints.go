package store

import (
	"context"
	"database/sql"
)

// EventIndexFingerprint (STUB) is one computer_event_index row reduced to digests.
type EventIndexFingerprint struct {
	EventDigest  string            `json:"event_digest"`
	Sequence     int64             `json:"sequence"`
	EventKind    string            `json:"event_kind"`
	Status       string            `json:"status"`
	RowDigest    string            `json:"row_digest"`
	FieldDigests map[string]string `json:"field_digests"`
}

// ListEventIndexFingerprints (STUB).
func (s *Store) ListEventIndexFingerprints(ctx context.Context, trimBoundary uint64) ([]EventIndexFingerprint, int, error) {
	return nil, 0, nil
}

// ListEventIndexFingerprintsTx (STUB).
func (s *Store) ListEventIndexFingerprintsTx(ctx context.Context, tx *sql.Tx, trimBoundary uint64) ([]EventIndexFingerprint, int, error) {
	return nil, 0, nil
}

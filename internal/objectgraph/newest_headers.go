package objectgraph

import (
	"context"
	"encoding/json"
	"fmt"
)

// The two reads of the event stream_seq seed. Both do work bounded by their
// argument, never by the size of a kind: the old seed listed the event kind
// ordered by updated_at, which the engine plans as the kind index, a TopN
// over every row of the kind, and a cold read of each one's body
// (docs/problems/stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md).

// NewestObjectHeadersSQL reads the newest `limit` rows of og_objects of every
// kind: kind and metadata only, in idx_og_objects_updated order walked
// backwards. The engine reads exactly `limit` index entries and fetches those
// rows; no body, no sort. The plan test pins the shape.
const NewestObjectHeadersSQL = `SELECT object_kind, metadata FROM og_objects ORDER BY updated_at DESC LIMIT ?`

// ObjectKindExistsSQL is one seek of the kind index.
const ObjectKindExistsSQL = `SELECT 1 FROM og_objects WHERE object_kind = ? LIMIT 1`

// ObjectHeader is an object's kind and metadata, without its body.
type ObjectHeader struct {
	Kind     ObjectKind
	Metadata json.RawMessage
}

// ListNewestObjectHeaders returns the headers of the `limit` objects most
// recently updated, of any kind, newest first. A shorter answer means the
// store holds no more. limit must be positive.
func (s *DoltStore) ListNewestObjectHeaders(ctx context.Context, limit int) ([]ObjectHeader, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("objectgraph dolt: newest headers: limit must be positive")
	}
	const op = "(*DoltStore).ListNewestObjectHeaders"
	s.engineMu.LockAs(op)
	defer s.engineMu.UnlockAs(op)
	rows, err := s.db.QueryContext(ctx, NewestObjectHeadersSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: newest headers: %w", err)
	}
	defer rows.Close()
	out := make([]ObjectHeader, 0, min(limit, 4096))
	for rows.Next() {
		var kind, metadata string
		if err := rows.Scan(&kind, &metadata); err != nil {
			return nil, fmt.Errorf("objectgraph dolt: newest headers: %w", err)
		}
		out = append(out, ObjectHeader{Kind: ObjectKind(kind), Metadata: json.RawMessage(metadata)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("objectgraph dolt: newest headers: %w", err)
	}
	return out, nil
}

// HasObjectOfKind reports whether any object of the kind exists, by one seek
// of the kind index.
func (s *DoltStore) HasObjectOfKind(ctx context.Context, kind ObjectKind) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("objectgraph dolt: nil store")
	}
	const op = "(*DoltStore).HasObjectOfKind"
	s.engineMu.LockAs(op)
	defer s.engineMu.UnlockAs(op)
	rows, err := s.db.QueryContext(ctx, ObjectKindExistsSQL, string(kind))
	if err != nil {
		return false, fmt.Errorf("objectgraph dolt: kind exists: %w", err)
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("objectgraph dolt: kind exists: %w", err)
	}
	return found, nil
}

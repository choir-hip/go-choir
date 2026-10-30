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
func (s *SQLStore) ListNewestObjectHeaders(ctx context.Context, limit int) ([]ObjectHeader, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("objectgraph sql: newest headers: limit must be positive")
	}
	rows, err := s.db.QueryContext(ctx, NewestObjectHeadersSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: newest headers: %w", err)
	}
	defer rows.Close()
	out := make([]ObjectHeader, 0, min(limit, 4096))
	for rows.Next() {
		var kind, metadata string
		if err := rows.Scan(&kind, &metadata); err != nil {
			return nil, fmt.Errorf("objectgraph sql: newest headers: %w", err)
		}
		out = append(out, ObjectHeader{Kind: ObjectKind(kind), Metadata: json.RawMessage(metadata)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("objectgraph sql: newest headers: %w", err)
	}
	return out, nil
}

// HasObjectOfKind reports whether any object of the kind exists, by one seek
// of the kind index.
func (s *SQLStore) HasObjectOfKind(ctx context.Context, kind ObjectKind) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("objectgraph sql: nil store")
	}
	rows, err := s.db.QueryContext(ctx, ObjectKindExistsSQL, string(kind))
	if err != nil {
		return false, fmt.Errorf("objectgraph sql: kind exists: %w", err)
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("objectgraph sql: kind exists: %w", err)
	}
	return found, nil
}

// The exact stream_seq seed walks the whole event kind
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, section 3).
// Two page queries, both metadata only and never the body:
//
//   - KindIDRangePageSQL, the walk: the kind's rows are the primary-key range
//     of ids "obj:<kind>:" to "obj:<kind>;" (BuildCanonicalID builds every id
//     that way), so a page is a primary-key range read in canonical-id order
//     that stops after limit rows. Measured: this is the plan with no sort.
//   - KindMetadataPageSQL, the fallback: a keyset page by the kind predicate.
//     The engine plans it as an index range of the kind and a TopN over it for
//     every page, so a walk by it costs the kind once per page; it runs only
//     when a row of the kind sits outside the id range (none is built by the
//     code; a test or an old import may), which keeps the seed exact.
const (
	KindIDRangePageSQL  = `SELECT canonical_id, object_kind, metadata FROM og_objects WHERE canonical_id > ? AND canonical_id < ? ORDER BY canonical_id ASC LIMIT ?`
	KindMetadataPageSQL = `SELECT canonical_id, metadata FROM og_objects WHERE object_kind = ? AND canonical_id > ? ORDER BY canonical_id ASC LIMIT ?`
	KindOutsideRangeSQL = `SELECT canonical_id FROM og_objects WHERE object_kind = ? AND (canonical_id <= ? OR canonical_id >= ?) LIMIT 1`
)

// KindIDRange bounds the canonical ids of a kind: (lo, hi) exclusive.
func KindIDRange(kind ObjectKind) (lo, hi string) {
	return "obj:" + string(kind) + ":", "obj:" + string(kind) + ";"
}

// KindMetadataRow is one row of a metadata page.
type KindMetadataRow struct {
	CanonicalID string
	Metadata    json.RawMessage
}

// ListKindIDRangePage returns the rows of kind with canonical id strictly
// after the cursor inside the kind's id range, up to limit scanned rows, and
// the last canonical id scanned (the next cursor). scanned counts the rows the
// page read, including any whose kind is not the one asked for (kept out of
// rows); scanned < limit means the range holds no more. It holds the engine
// mutex for this page only.
func (s *SQLStore) ListKindIDRangePage(ctx context.Context, kind ObjectKind, afterCanonicalID string, limit int) (rows []KindMetadataRow, next string, scanned int, err error) {
	if s == nil || s.db == nil {
		return nil, "", 0, fmt.Errorf("objectgraph sql: nil store")
	}
	if limit <= 0 {
		return nil, "", 0, fmt.Errorf("objectgraph sql: kind id range page: limit must be positive")
	}
	lo, hi := KindIDRange(kind)
	if afterCanonicalID > lo {
		lo = afterCanonicalID
	}
	result, err := s.db.QueryContext(ctx, KindIDRangePageSQL, lo, hi, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("objectgraph sql: kind id range page: %w", err)
	}
	defer result.Close()
	rows = make([]KindMetadataRow, 0, min(limit, 4096))
	for result.Next() {
		var id, rowKind, metadata string
		if err := result.Scan(&id, &rowKind, &metadata); err != nil {
			return nil, "", 0, fmt.Errorf("objectgraph sql: kind id range page: %w", err)
		}
		scanned++
		next = id
		if rowKind == string(kind) {
			rows = append(rows, KindMetadataRow{CanonicalID: id, Metadata: json.RawMessage(metadata)})
		}
	}
	if err := result.Err(); err != nil {
		return nil, "", 0, fmt.Errorf("objectgraph sql: kind id range page: %w", err)
	}
	return rows, next, scanned, nil
}

// KindHasRowsOutsideIDRange reports whether any row of kind has a canonical id
// outside the kind's id range: one read of the kind index.
func (s *SQLStore) KindHasRowsOutsideIDRange(ctx context.Context, kind ObjectKind) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("objectgraph sql: nil store")
	}
	lo, hi := KindIDRange(kind)
	result, err := s.db.QueryContext(ctx, KindOutsideRangeSQL, string(kind), lo, hi)
	if err != nil {
		return false, fmt.Errorf("objectgraph sql: kind outside id range: %w", err)
	}
	defer result.Close()
	found := result.Next()
	if err := result.Err(); err != nil {
		return false, fmt.Errorf("objectgraph sql: kind outside id range: %w", err)
	}
	return found, nil
}

// ListKindMetadataPage returns up to limit rows of one kind with canonical id
// strictly after afterCanonicalID, in canonical-id order, by the kind
// predicate. Exact for any id; costs the kind once per page (see
// KindMetadataPageSQL). A shorter answer than limit means the kind holds no
// more.
func (s *SQLStore) ListKindMetadataPage(ctx context.Context, kind ObjectKind, afterCanonicalID string, limit int) ([]KindMetadataRow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("objectgraph sql: kind metadata page: limit must be positive")
	}
	rows, err := s.db.QueryContext(ctx, KindMetadataPageSQL, string(kind), afterCanonicalID, limit)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: kind metadata page: %w", err)
	}
	defer rows.Close()
	out := make([]KindMetadataRow, 0, min(limit, 4096))
	for rows.Next() {
		var id, metadata string
		if err := rows.Scan(&id, &metadata); err != nil {
			return nil, fmt.Errorf("objectgraph sql: kind metadata page: %w", err)
		}
		out = append(out, KindMetadataRow{CanonicalID: id, Metadata: json.RawMessage(metadata)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("objectgraph sql: kind metadata page: %w", err)
	}
	return out, nil
}

package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// ObjectGraphFingerprint is one og_objects row reduced to comparable digests
// for the replay-completeness diagnostic. Bodies and metadata are never
// returned, only their digests.
type ObjectGraphFingerprint struct {
	CanonicalID    string `json:"canonical_id"`
	ObjectKind     string `json:"object_kind"`
	VersionID      string `json:"version_id,omitempty"`
	ContentHash    string `json:"content_hash"`
	MetadataDigest string `json:"metadata_digest"`
	BodyDigest     string `json:"body_digest"`
	UpdatedAt      string `json:"updated_at"`
	Tombstone      bool   `json:"tombstone,omitempty"`
	SupersededBy   string `json:"superseded_by,omitempty"`
	// Table is "choir_events" for a row of that table, whose CanonicalID is
	// then its tape position, "choir_events/<tape_seq>/<op_index>": the
	// event's canonical id is not unique there. Empty for og_objects.
	Table string `json:"table,omitempty"`
}

// ListObjectGraphFingerprints returns every og_objects row as a fingerprint,
// ordered by canonical id, then every choir_events row, ordered by tape
// position (event-table design 3.3 item 3), so a replay inequivalence names
// the rows of either table.
func (s *Store) ListObjectGraphFingerprints(ctx context.Context) ([]ObjectGraphFingerprint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT canonical_id, object_kind, version_id, content_hash, COALESCE(body, ''), metadata,
		       updated_at, tombstone, superseded_by
		  FROM og_objects
		 ORDER BY canonical_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query object graph fingerprints: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ObjectGraphFingerprint
	for rows.Next() {
		var (
			fp        ObjectGraphFingerprint
			body      []byte
			metadata  string
			updatedAt any
		)
		if err := rows.Scan(&fp.CanonicalID, &fp.ObjectKind, &fp.VersionID, &fp.ContentHash, &body, &metadata,
			&updatedAt, &fp.Tombstone, &fp.SupersededBy); err != nil {
			return nil, fmt.Errorf("scan object graph fingerprint: %w", err)
		}
		bodySum := sha256.Sum256(body)
		metaSum := sha256.Sum256([]byte(metadata))
		fp.BodyDigest = hex.EncodeToString(bodySum[:])
		fp.MetadataDigest = hex.EncodeToString(metaSum[:])
		fp.UpdatedAt = fmt.Sprint(updatedAt)
		out = append(out, fp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate object graph fingerprints: %w", err)
	}
	events, err := s.eventTableFingerprints(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, events...)
	parts, err := s.telemetryPartitionFingerprints(ctx)
	if err != nil {
		return nil, err
	}
	return append(out, parts...), nil
}

// telemetryPartitionFingerprints are the rows of the live telemetry
// partitions, keyed by tape position like choir_events' (a position is in
// one table only).
func (s *Store) telemetryPartitionFingerprints(ctx context.Context) ([]ObjectGraphFingerprint, error) {
	var out []ObjectGraphFingerprint
	for _, p := range s.partitions.live() {
		if _, err := p.withRead(func(db *sql.DB) error {
			rows, err := readStoredRows(ctx, db, "")
			if err != nil {
				return err
			}
			for _, r := range rows {
				bodySum := sha256.Sum256(r.row.body)
				metaSum := sha256.Sum256([]byte(r.row.metadata))
				out = append(out, ObjectGraphFingerprint{
					Table:          telemetryPartitionStamp,
					CanonicalID:    fmt.Sprintf("%s/%d/%d", telemetryPartitionStamp, r.pos, r.index),
					ObjectKind:     r.row.objectKind,
					VersionID:      r.row.versionID,
					ContentHash:    r.row.contentHash,
					BodyDigest:     hex.EncodeToString(bodySum[:]),
					MetadataDigest: hex.EncodeToString(metaSum[:]),
					UpdatedAt:      fmt.Sprint(r.row.updatedAt),
				})
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("fingerprints of telemetry partition %s: %w", p.name, err)
		}
	}
	return out, nil
}

func (s *Store) eventTableFingerprints(ctx context.Context) ([]ObjectGraphFingerprint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tape_seq, op_index, object_kind, version_id, content_hash, body, metadata,
		       updated_at, tombstone, superseded_by
		  FROM choir_events
		 ORDER BY tape_seq ASC, op_index ASC`)
	if err != nil {
		return nil, fmt.Errorf("query %s fingerprints: %w", choirEventsTable, err)
	}
	defer func() { _ = rows.Close() }()
	var out []ObjectGraphFingerprint
	for rows.Next() {
		fp := ObjectGraphFingerprint{Table: choirEventsTable}
		var (
			tapeSeq, opIndex uint64
			body             []byte
			metadata         string
			updatedAt        any
		)
		if err := rows.Scan(&tapeSeq, &opIndex, &fp.ObjectKind, &fp.VersionID, &fp.ContentHash, &body, &metadata,
			&updatedAt, &fp.Tombstone, &fp.SupersededBy); err != nil {
			return nil, fmt.Errorf("scan %s fingerprint: %w", choirEventsTable, err)
		}
		bodySum := sha256.Sum256(body)
		metaSum := sha256.Sum256([]byte(metadata))
		fp.CanonicalID = fmt.Sprintf("%s/%d/%d", choirEventsTable, tapeSeq, opIndex)
		fp.BodyDigest = hex.EncodeToString(bodySum[:])
		fp.MetadataDigest = hex.EncodeToString(metaSum[:])
		fp.UpdatedAt = fmt.Sprint(updatedAt)
		out = append(out, fp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s fingerprints: %w", choirEventsTable, err)
	}
	return out, nil
}

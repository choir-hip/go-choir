package store

import (
	"context"
	"crypto/sha256"
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
}

// ListObjectGraphFingerprints returns every og_objects row as a fingerprint,
// ordered by canonical id.
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
	return out, nil
}

package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const fileCASSchemaDDL = `
CREATE TABLE IF NOT EXISTS computer_file_roots (
	computer_id VARCHAR(255) NOT NULL,
	root VARCHAR(128) NOT NULL,
	manifest_ref VARCHAR(255) NOT NULL,
	head_sequence BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY(computer_id, root)
);
CREATE INDEX IF NOT EXISTS idx_computer_file_roots_computer_created ON computer_file_roots(computer_id, created_at);
CREATE TABLE IF NOT EXISTS computer_replay_watermarks (
	computer_id VARCHAR(255) PRIMARY KEY,
	watermark_sequence BIGINT NOT NULL,
	base_ref VARCHAR(255) NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);
`

var ErrNoWatermark = errors.New("file cas: no replay watermark")

type FileRootRecord struct {
	ComputerID   string    `json:"computer_id"`
	Root         string    `json:"root"`
	ManifestRef  string    `json:"manifest_ref"`
	HeadSequence int64     `json:"head_sequence"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Store) RecordFileRoot(ctx context.Context, computerID, root, manifestRef string, headSequence int64) error {
	if s == nil || s.db == nil || !safeFileCASComponent(computerID) || !validFileCASDigest(root) || strings.TrimSpace(manifestRef) == "" || headSequence < 0 {
		return fmt.Errorf("file cas: invalid file root")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_file_roots (computer_id,root,manifest_ref,head_sequence,created_at) VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING`, computerID, root, manifestRef, headSequence, time.Now().UTC()); err != nil {
		return fmt.Errorf("file cas: record root: %w", err)
	}
	return nil
}

func (s *Store) LatestFileRoots(ctx context.Context, computerID string, limit int) ([]FileRootRecord, error) {
	if s == nil || s.db == nil || !safeFileCASComponent(computerID) {
		return nil, fmt.Errorf("file cas: computer ID is required")
	}
	if limit <= 0 {
		return []FileRootRecord{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT computer_id,root,manifest_ref,head_sequence,created_at FROM computer_file_roots WHERE computer_id=? ORDER BY created_at DESC, root DESC LIMIT ?`, computerID, limit)
	if err != nil {
		return nil, fmt.Errorf("file cas: list roots: %w", err)
	}
	defer rows.Close()
	out := make([]FileRootRecord, 0)
	for rows.Next() {
		var record FileRootRecord
		if err := rows.Scan(&record.ComputerID, &record.Root, &record.ManifestRef, &record.HeadSequence, &record.CreatedAt); err != nil {
			return nil, fmt.Errorf("file cas: scan root: %w", err)
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// RecoveryBase is the base a lost disk recovers from: the computer's newest
// snapshot root (its head sequence and manifest root). The rows of
// computer_replay_watermarks named projection bases, which the artifact GC
// now collects, and nothing has written them since the data system cutover;
// vmctl's empty-store admission read them and refused a lost-disk boot on a
// base days old (2026-10-30: the paper's row named 4,526,210, its snapshot
// 5,231,495).
func (s *Store) RecoveryBase(ctx context.Context, computerID string) (seq int64, root string, err error) {
	if s == nil || s.db == nil || !safeFileCASComponent(computerID) {
		return 0, "", fmt.Errorf("file cas: computer ID is required")
	}
	err = s.db.QueryRowContext(ctx, `SELECT head_sequence,root FROM computer_snapshot_roots WHERE computer_id=? ORDER BY head_sequence DESC, created_at DESC LIMIT 1`, computerID).Scan(&seq, &root)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", ErrNoWatermark
	}
	if err != nil {
		return 0, "", fmt.Errorf("file cas: get recovery base: %w", err)
	}
	return seq, root, nil
}

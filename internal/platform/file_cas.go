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

func (s *Store) RecordReplayWatermark(ctx context.Context, computerID string, seq int64, baseRef string) error {
	return s.RecordReplayWatermarkWithRoot(ctx, computerID, seq, baseRef, false)
}

// RecordReplayWatermarkWithRoot advances the watermark; epochRoot records the
// base as the computer's epoch root in the same transaction (its sidecar is
// marked live_captured): from then on no job the platform admits for the
// computer replays the tape below it (owner, 2026-10-27). A root only moves
// up, because the watermark does.
func (s *Store) RecordReplayWatermarkWithRoot(ctx context.Context, computerID string, seq int64, baseRef string, epochRoot bool) error {
	if s == nil || s.db == nil || !safeFileCASComponent(computerID) || seq < 0 || strings.TrimSpace(baseRef) == "" {
		return fmt.Errorf("file cas: invalid replay watermark")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// The job row is locked before the watermark row (review 364).
	var jobGeneration uint64
	if err := tx.QueryRowContext(ctx, `SELECT generation FROM computer_projection_jobs WHERE computer_id=? FOR UPDATE`, computerID).Scan(&jobGeneration); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var priorSeq int64
	var priorRef string
	err = tx.QueryRowContext(ctx, `SELECT watermark_sequence,base_ref FROM computer_replay_watermarks WHERE computer_id=? FOR UPDATE`, computerID).Scan(&priorSeq, &priorRef)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && seq <= priorSeq {
		return tx.Commit()
	}
	now := time.Now().UTC()
	// Protect the predecessor in the same transaction that advances W.
	if priorRef != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO computer_projection_bases (computer_id,base_ref,sequence,created_at) VALUES (?,?,?,?) ON CONFLICT DO NOTHING`, computerID, priorRef, priorSeq, now.Add(-time.Microsecond)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO computer_projection_bases (computer_id,base_ref,sequence,created_at) VALUES (?,?,?,?) ON CONFLICT DO NOTHING`, computerID, baseRef, seq, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO computer_replay_watermarks (computer_id,watermark_sequence,base_ref,updated_at) VALUES (?,?,?,?) ON CONFLICT (computer_id) DO UPDATE SET watermark_sequence=excluded.watermark_sequence,base_ref=excluded.base_ref,updated_at=excluded.updated_at`, computerID, seq, baseRef, now); err != nil {
		return fmt.Errorf("file cas: record replay watermark: %w", err)
	}
	if epochRoot {
		if _, err := tx.ExecContext(ctx, `INSERT INTO computer_projection_epoch_roots (computer_id,base_ref,sequence,created_at) VALUES (?,?,?,?) ON CONFLICT (computer_id) DO UPDATE SET base_ref=excluded.base_ref,sequence=excluded.sequence,created_at=excluded.created_at`, computerID, baseRef, seq, now); err != nil {
			return fmt.Errorf("file cas: record epoch root: %w", err)
		}
	}
	// A cadence job on another seed below the new base would only rebuild
	// from further back: supersede it, so the next enqueue re-admits the row
	// on the new base (S2, docs/problems/paper-never-checkpointed-2026-10-27.md;
	// by seed and watermark, not target, review 364). Checkpoint verification
	// jobs and genesis repairs are their own targets.
	if _, err := tx.ExecContext(ctx, `UPDATE computer_projection_jobs SET status='superseded',error_text=?,lease_token='',updated_at=? WHERE computer_id=? AND status IN ('queued','running','retry','failed','blocked') AND seed_base_ref<>? AND watermark_sequence<? AND genesis_repair=FALSE AND reason NOT LIKE ?`,
		fmt.Sprintf("superseded by base %s at %d", baseRef, seq), now, computerID, baseRef, seq, checkpointVerifyReason+"%"); err != nil {
		return fmt.Errorf("file cas: supersede covered projection job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ReplayWatermark(ctx context.Context, computerID string) (seq int64, baseRef string, err error) {
	if s == nil || s.db == nil || !safeFileCASComponent(computerID) {
		return 0, "", fmt.Errorf("file cas: computer ID is required")
	}
	err = s.db.QueryRowContext(ctx, `SELECT watermark_sequence,base_ref FROM computer_replay_watermarks WHERE computer_id=?`, computerID).Scan(&seq, &baseRef)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", ErrNoWatermark
	}
	if err != nil {
		return 0, "", fmt.Errorf("file cas: get replay watermark: %w", err)
	}
	return seq, baseRef, nil
}

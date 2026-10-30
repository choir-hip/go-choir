package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Snapshot roots make the chunks of a guest snapshot (internal/snapshot)
// reachable for the file-CAS chunk sweep. A snapshot's manifest is sealed in
// the snapshot_recorded event, which the host cannot read, and it is not a
// file root (it would displace the Files tree's latest root). So the guest
// also declares the chunk digests in plaintext: digests are not content.

const snapshotRootSchemaDDL = `
CREATE TABLE IF NOT EXISTS computer_snapshot_roots (
	computer_id VARCHAR(255) NOT NULL,
	root VARCHAR(128) NOT NULL,
	head_sequence BIGINT NOT NULL,
	chunks TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY(computer_id, root)
);
CREATE INDEX IF NOT EXISTS idx_computer_snapshot_roots_computer_head ON computer_snapshot_roots(computer_id, head_sequence, created_at);
`

const (
	// snapshotRootRetention is how many snapshot roots per computer stay live;
	// older ones become collectable.
	snapshotRootRetention = 3
	// maxSnapshotRootChunks bounds one declaration (65536 x 4 MiB = 256 GiB).
	maxSnapshotRootChunks = 65536
	maxSnapshotRootBody   = 8 << 20
)

// SnapshotRootRecord is one declared snapshot: its manifest root, the store
// head sequence it captures, and the chunk digests it needs.
type SnapshotRootRecord struct {
	ComputerID   string    `json:"computer_id"`
	Root         string    `json:"root"`
	HeadSequence int64     `json:"head_sequence"`
	Chunks       []string  `json:"chunks"`
	CreatedAt    time.Time `json:"created_at"`
}

type snapshotRootRequest struct {
	ComputerID   string   `json:"computer_id"`
	Root         string   `json:"root"`
	HeadSequence int64    `json:"head_sequence"`
	Chunks       []string `json:"chunks"`
}

// RecordSnapshotRoot declares a snapshot's chunks and keeps only the newest
// snapshotRootRetention rows (by head sequence, then time) for the computer.
// It is idempotent per (computer, root).
func (s *Store) RecordSnapshotRoot(ctx context.Context, computerID, root string, headSequence int64, chunks []string) error {
	if s == nil || s.db == nil || !safeFileCASComponent(computerID) || !validFileCASDigest(root) || headSequence < 0 || len(chunks) == 0 || len(chunks) > maxSnapshotRootChunks {
		return fmt.Errorf("snapshot root: invalid declaration")
	}
	for _, digest := range chunks {
		if !validFileCASDigest(digest) {
			return fmt.Errorf("snapshot root: invalid chunk digest")
		}
	}
	encoded, err := json.Marshal(chunks)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO computer_snapshot_roots (computer_id,root,head_sequence,chunks,created_at) VALUES (?,?,?,?,?) ON CONFLICT (computer_id, root) DO UPDATE SET head_sequence=excluded.head_sequence,chunks=excluded.chunks,created_at=excluded.created_at`,
		computerID, root, headSequence, string(encoded), time.Now().UTC()); err != nil {
		return fmt.Errorf("snapshot root: record: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM computer_snapshot_roots WHERE computer_id=? AND root NOT IN (SELECT root FROM computer_snapshot_roots WHERE computer_id=? ORDER BY head_sequence DESC, created_at DESC, root DESC LIMIT ?)`,
		computerID, computerID, snapshotRootRetention); err != nil {
		return fmt.Errorf("snapshot root: prune: %w", err)
	}
	return tx.Commit()
}

// LiveSnapshotRoots returns the computer's newest snapshotRootRetention roots.
func (s *Store) LiveSnapshotRoots(ctx context.Context, computerID string) ([]SnapshotRootRecord, error) {
	if s == nil || s.db == nil || !safeFileCASComponent(computerID) {
		return nil, fmt.Errorf("snapshot root: computer ID is required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT computer_id,root,head_sequence,chunks,created_at FROM computer_snapshot_roots WHERE computer_id=? ORDER BY head_sequence DESC, created_at DESC, root DESC LIMIT ?`, computerID, snapshotRootRetention)
	if err != nil {
		return nil, fmt.Errorf("snapshot root: list: %w", err)
	}
	defer rows.Close()
	var out []SnapshotRootRecord
	for rows.Next() {
		var record SnapshotRootRecord
		var encoded string
		if err := rows.Scan(&record.ComputerID, &record.Root, &record.HeadSequence, &encoded, &record.CreatedAt); err != nil {
			return nil, fmt.Errorf("snapshot root: scan: %w", err)
		}
		if err := json.Unmarshal([]byte(encoded), &record.Chunks); err != nil {
			return nil, fmt.Errorf("snapshot root: %s chunk list unreadable: %w", record.Root, err)
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// HandleSnapshotRoot is PUT /internal/computers/files/snapshot-root. The guest
// calls it before it appends the snapshot_recorded event, so the event never
// names chunks the sweep could collect.
func (h *Handler) HandleSnapshotRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var input snapshotRootRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxSnapshotRootBody)
	if !decodeFileCASJSON(r, &input) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if !h.authorizeFileCAS(r, input.ComputerID, "event:append") {
		writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
		return
	}
	if h == nil || h.service == nil || h.service.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "file storage unavailable"})
		return
	}
	if err := h.service.store.RecordSnapshotRoot(r.Context(), strings.TrimSpace(input.ComputerID), input.Root, input.HeadSequence, input.Chunks); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"root": input.Root})
}

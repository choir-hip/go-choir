package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	snapshotRootRetention = 4
	// A store over largeSnapshotChunks chunks (8 GiB) keeps
	// largeSnapshotRetention roots: the paper's 31 GB store changes most of its
	// page ranges between snapshots, so each root holds about 25 GB of its
	// own, and four of them do not fit beside the tape on one host
	// (2026-10-30). The guest's restore tries the newest first.
	largeSnapshotChunks    = 2048
	largeSnapshotRetention = 2
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
// snapshotRootRetention rows (by head sequence, then time) for the computer,
// or largeSnapshotRetention when the declared snapshot is large.
// It is idempotent per (computer, root).
func (s *Store) RecordSnapshotRoot(ctx context.Context, computerID, root string, headSequence int64, chunks []string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("snapshot root: invalid declaration")
	}
	if err := validateSnapshotRootDeclaration(computerID, root, headSequence, chunks); err != nil {
		return err
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
	retention := snapshotRootRetention
	if len(chunks) > largeSnapshotChunks {
		retention = largeSnapshotRetention
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM computer_snapshot_roots WHERE computer_id=? AND root NOT IN (SELECT root FROM computer_snapshot_roots WHERE computer_id=? ORDER BY head_sequence DESC, created_at DESC, root DESC LIMIT ?)`,
		computerID, computerID, retention); err != nil {
		return fmt.Errorf("snapshot root: prune: %w", err)
	}
	return tx.Commit()
}

// ErrSnapshotChunkMissing is returned (wrapped) when a snapshot root names a
// chunk the host does not hold.
var ErrSnapshotChunkMissing = errors.New("snapshot root names a chunk the host does not hold")

// RecordVerifiedSnapshotRoot registers a snapshot root only if the computer's
// namespace holds every chunk it names, and returns the first missing digest
// (with ErrSnapshotChunkMissing) when it does not; a refusal records and
// prunes nothing.
//
// It holds casReuseMu shared from the first stat to the commit. The artifact
// collector unlinks a chunk only under the exclusive lock, and re-checks the
// live snapshot roots inside it (unlinkIfStillUnreachable), so an unlink falls
// either before the check (the chunk is missing and the root is refused) or
// after the commit (the chunk is named by a live root and is kept). Nothing
// falls between. The shared lock is the one every chunk writer already takes,
// so registrations and uploads do not exclude each other.
func (s *Service) RecordVerifiedSnapshotRoot(ctx context.Context, computerID, root string, headSequence int64, chunks []string) (string, error) {
	if s == nil || s.store == nil {
		return "", fmt.Errorf("snapshot root: store unavailable")
	}
	if err := validateSnapshotRootDeclaration(computerID, root, headSequence, chunks); err != nil {
		return "", err
	}
	s.store.casReuseMu.RLock()
	defer s.store.casReuseMu.RUnlock()
	seen := make(map[string]struct{}, len(chunks))
	for _, digest := range chunks {
		if _, dup := seen[digest]; dup {
			continue
		}
		seen[digest] = struct{}{}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		ref, err := fileCASChunkRef(computerID, digest)
		if err != nil {
			return "", err
		}
		path, err := s.artifactPath(ref)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) || (err == nil && (!info.Mode().IsRegular() || info.Size() == 0)) {
			return digest, ErrSnapshotChunkMissing
		}
		if err != nil {
			return "", fmt.Errorf("snapshot root: stat chunk: %w", err)
		}
	}
	if hook := s.snapshotRootAfterVerify; hook != nil {
		hook()
	}
	return "", s.store.RecordSnapshotRoot(ctx, computerID, root, headSequence, chunks)
}

// chunkNamedByLiveSnapshotRoot reports whether the chunk at rel
// (sha256/file-cas-chunks/<computer>/<digest>) is named by one of the
// computer's live snapshot roots. Any other path is not a chunk and is not
// named. The caller holds casReuseMu exclusively.
func (s *Service) chunkNamedByLiveSnapshotRoot(rel string) (bool, error) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 4 || parts[0] != "sha256" || parts[1] != "file-cas-chunks" || !safeFileCASComponent(parts[2]) || !validFileCASDigest(parts[3]) {
		return false, nil
	}
	computerID, digest := parts[2], parts[3]
	s.snapshotRootRecheckQueries.Add(1)
	var n int
	// Digests are 64 hex characters, so the quoted needle has no pattern
	// characters and matches only a whole JSON element.
	err := s.store.db.QueryRow(`SELECT COUNT(*) FROM (SELECT root, chunks FROM computer_snapshot_roots WHERE computer_id=? ORDER BY head_sequence DESC, created_at DESC, root DESC LIMIT ?) t WHERE chunks LIKE ?`,
		computerID, snapshotRootRetention, `%"`+digest+`"%`).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("snapshot root: live check: %w", err)
	}
	return n > 0, nil
}

func validateSnapshotRootDeclaration(computerID, root string, headSequence int64, chunks []string) error {
	if !safeFileCASComponent(computerID) || !validFileCASDigest(root) || headSequence < 0 || len(chunks) == 0 || len(chunks) > maxSnapshotRootChunks {
		return fmt.Errorf("snapshot root: invalid declaration")
	}
	for _, digest := range chunks {
		if !validFileCASDigest(digest) {
			return fmt.Errorf("snapshot root: invalid chunk digest")
		}
	}
	return nil
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
	// A root that names a chunk the host does not hold would be a recovery
	// base that cannot be restored, and recording it prunes a good older one.
	// Refuse it; the guest takes the snapshot again (its probe finds the
	// chunk missing and uploads it).
	missing, err := h.service.RecordVerifiedSnapshotRoot(r.Context(), strings.TrimSpace(input.ComputerID), input.Root, input.HeadSequence, input.Chunks)
	if errors.Is(err, ErrSnapshotChunkMissing) {
		writeJSON(w, http.StatusConflict, apiError{Error: "snapshot root names chunk " + missing + ", which the host does not hold; take the snapshot again"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"root": input.Root})
}

package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrComputerTapeDeleted is the explicit refusal for a computer whose event
// tape the tape sweep archived and deleted (or is archiving). Readers and the
// head CAS return it instead of a chain-gap or file-not-found error
// (docs/problems/tape-sweeps-destroyed-and-orphans-2026-10-20.md).
var ErrComputerTapeDeleted = errors.New("computer tape archived/deleted")

// Tombstone statuses, in order. A tombstone exists from the moment the sweep
// commits to a computer; every reader fails closed from then on.
const (
	tapeTombstoneArchiving = "archiving"
	tapeTombstoneArchived  = "archived"
	tapeTombstoneDeleted   = "deleted"
)

const tapeTombstoneSchemaDDL = `
CREATE TABLE IF NOT EXISTS computer_tape_tombstones (
  computer_id VARCHAR(128) NOT NULL PRIMARY KEY,
  status VARCHAR(32) NOT NULL,
  head_sequence BIGINT NOT NULL,
  canonical_event_head CHAR(64) NOT NULL,
  manifest_run VARCHAR(64) NOT NULL,
  manifest_sha256 CHAR(64) NOT NULL,
  archive_path VARCHAR(1024) NOT NULL DEFAULT '',
  archive_sha256 VARCHAR(64) NOT NULL DEFAULT '',
  archive_manifest_sha256 VARCHAR(64) NOT NULL DEFAULT '',
  files_deleted BIGINT NOT NULL DEFAULT 0,
  bytes_deleted BIGINT NOT NULL DEFAULT 0,
  proof_json TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);`

// TapeTombstone is one computer's tape-deletion record. The computer's head
// and append-receipt rows are kept: they are the sequence-to-digest index
// that makes the archive verifiable, and the head row stops a reused
// ComputerID from starting a second genesis.
type TapeTombstone struct {
	ComputerID            string    `json:"computer_id"`
	Status                string    `json:"status"`
	HeadSequence          uint64    `json:"head_sequence"`
	CanonicalEventHead    string    `json:"canonical_event_head"`
	ManifestRun           string    `json:"manifest_run"`
	ManifestSHA256        string    `json:"manifest_sha256"`
	ArchivePath           string    `json:"archive_path,omitempty"`
	ArchiveSHA256         string    `json:"archive_sha256,omitempty"`
	ArchiveManifestSHA256 string    `json:"archive_manifest_sha256,omitempty"`
	FilesDeleted          int64     `json:"files_deleted"`
	BytesDeleted          int64     `json:"bytes_deleted"`
	ProofJSON             string    `json:"proof_json"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// tapeTombstoneCache is the in-memory view of computer_tape_tombstones. Only
// corpusd writes tombstones, so the cache is authoritative once loaded; a load
// failure is returned to every caller (fail closed), and the next call retries.
type tapeTombstoneCache struct {
	mu     sync.RWMutex
	loaded bool
	byID   map[string]TapeTombstone
	// gate orders head CAS against tombstone creation: a CAS holds it for
	// read from its tombstone check through its commit, and the sweep holds
	// it for write while it records a tombstone and re-reads the head, so
	// no append can land after the sweep's head check.
	gate sync.RWMutex
}

func (s *Store) ensureTapeTombstoneSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, tapeTombstoneSchemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap tape tombstones: %w", err)
	}
	return nil
}

func (s *Store) loadTapeTombstones(ctx context.Context) error {
	s.tombstones.mu.RLock()
	loaded := s.tombstones.loaded
	s.tombstones.mu.RUnlock()
	if loaded {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT computer_id,status,head_sequence,canonical_event_head,manifest_run,manifest_sha256,archive_path,archive_sha256,archive_manifest_sha256,files_deleted,bytes_deleted,proof_json,created_at,updated_at FROM computer_tape_tombstones`)
	if err != nil {
		return fmt.Errorf("tape tombstones: load: %w", err)
	}
	defer rows.Close()
	byID := map[string]TapeTombstone{}
	for rows.Next() {
		var t TapeTombstone
		if err := rows.Scan(&t.ComputerID, &t.Status, &t.HeadSequence, &t.CanonicalEventHead, &t.ManifestRun, &t.ManifestSHA256, &t.ArchivePath, &t.ArchiveSHA256, &t.ArchiveManifestSHA256, &t.FilesDeleted, &t.BytesDeleted, &t.ProofJSON, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return fmt.Errorf("tape tombstones: scan: %w", err)
		}
		byID[t.ComputerID] = t
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("tape tombstones: iterate: %w", err)
	}
	s.tombstones.mu.Lock()
	if !s.tombstones.loaded {
		s.tombstones.byID = byID
		s.tombstones.loaded = true
	}
	s.tombstones.mu.Unlock()
	return nil
}

// TapeTombstone returns the computer's tombstone, if any.
func (s *Store) TapeTombstone(ctx context.Context, computerID string) (TapeTombstone, bool, error) {
	if s == nil || s.db == nil {
		return TapeTombstone{}, false, fmt.Errorf("tape tombstones: nil store")
	}
	if err := s.loadTapeTombstones(ctx); err != nil {
		return TapeTombstone{}, false, err
	}
	s.tombstones.mu.RLock()
	defer s.tombstones.mu.RUnlock()
	t, ok := s.tombstones.byID[strings.TrimSpace(computerID)]
	return t, ok, nil
}

// refuseTombstonedTape returns ErrComputerTapeDeleted (wrapped with the status
// and archive) for a tombstoned computer, a load error when the tombstone set
// is unknown, and nil otherwise.
func (s *Store) refuseTombstonedTape(ctx context.Context, computerID string) error {
	t, ok, err := s.TapeTombstone(ctx, computerID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	archive := t.ArchivePath
	if archive == "" {
		archive = "pending"
	}
	return fmt.Errorf("%w: computer %s (status %s, archive %s)", ErrComputerTapeDeleted, t.ComputerID, t.Status, archive)
}

// putTapeTombstone upserts a tombstone; the SQL commit makes the refusal
// durable before any file of the computer is touched.
func (s *Store) putTapeTombstone(ctx context.Context, t TapeTombstone) error {
	return s.writeTapeTombstone(ctx, t)
}

// writeTapeTombstone upserts the row and the cache.
func (s *Store) writeTapeTombstone(ctx context.Context, t TapeTombstone) error {
	if err := s.loadTapeTombstones(ctx); err != nil {
		return err
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO computer_tape_tombstones (computer_id,status,head_sequence,canonical_event_head,manifest_run,manifest_sha256,archive_path,archive_sha256,archive_manifest_sha256,files_deleted,bytes_deleted,proof_json,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT (computer_id) DO UPDATE SET status=excluded.status,head_sequence=excluded.head_sequence,canonical_event_head=excluded.canonical_event_head,manifest_run=excluded.manifest_run,manifest_sha256=excluded.manifest_sha256,archive_path=excluded.archive_path,archive_sha256=excluded.archive_sha256,archive_manifest_sha256=excluded.archive_manifest_sha256,files_deleted=excluded.files_deleted,bytes_deleted=excluded.bytes_deleted,proof_json=excluded.proof_json,updated_at=excluded.updated_at`,
		t.ComputerID, t.Status, t.HeadSequence, t.CanonicalEventHead, t.ManifestRun, t.ManifestSHA256, t.ArchivePath, t.ArchiveSHA256, t.ArchiveManifestSHA256, t.FilesDeleted, t.BytesDeleted, t.ProofJSON, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("tape tombstones: write %s: %w", t.ComputerID, err)
	}
	s.tombstones.mu.Lock()
	if s.tombstones.byID == nil {
		s.tombstones.byID = map[string]TapeTombstone{}
	}
	s.tombstones.byID[t.ComputerID] = t
	s.tombstones.mu.Unlock()
	return nil
}

// removeTapeTombstone deletes a tombstone that never reached deletion (an
// aborted archive, or a computer that turned out not to be destroyed). A
// tombstone in status deleted is never removed: its files are gone.
func (s *Store) removeTapeTombstone(ctx context.Context, computerID string) error {
	t, ok, err := s.TapeTombstone(ctx, computerID)
	if err != nil || !ok {
		return err
	}
	if t.Status == tapeTombstoneDeleted || t.FilesDeleted > 0 {
		return fmt.Errorf("tape tombstones: %s has deleted files; restore from %s before removing its tombstone", computerID, t.ArchivePath)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM computer_tape_tombstones WHERE computer_id=?`, computerID); err != nil {
		return fmt.Errorf("tape tombstones: remove %s: %w", computerID, err)
	}
	s.tombstones.mu.Lock()
	delete(s.tombstones.byID, computerID)
	s.tombstones.mu.Unlock()
	return nil
}

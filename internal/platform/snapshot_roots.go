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
	"sync"
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

// recordSnapshotRoot declares a snapshot's chunks and keeps only the newest
// snapshotRootRetention rows (by head sequence, then time) for the computer,
// or largeSnapshotRetention when the declared snapshot is large.
// It is idempotent per (computer, root). It does not look at the chunks and
// is unexported on purpose: production registers through
// Service.RecordVerifiedSnapshotRoot. It bumps snapshotRootGen after the
// commit.
func (s *Store) recordSnapshotRoot(ctx context.Context, computerID, root string, headSequence int64, chunks []string) error {
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
	err = tx.Commit()
	if hook := s.snapshotRootCommitResult; hook != nil {
		err = hook(err)
	}
	if err != nil {
		return err
	}
	s.snapshotRootGen.Add(1)
	return nil
}

// snapshotRootCommitTimeout bounds the database part of a registration.
const snapshotRootCommitTimeout = 30 * time.Second

// snapshotChunkKey names one chunk of one computer, as the pending set keys it.
func snapshotChunkKey(computerID, digest string) string { return computerID + "/" + digest }

// snapshotRootPending is the set of chunks whose snapshot root has been
// verified but not yet committed (and its generation bumped).
type snapshotRootPending struct {
	mu sync.Mutex
	n  map[string]int
}

func (p *snapshotRootPending) add(keys []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.n == nil {
		p.n = make(map[string]int)
	}
	for _, k := range keys {
		p.n[k]++
	}
}

func (p *snapshotRootPending) remove(keys []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range keys {
		if p.n[k] <= 1 {
			delete(p.n, k)
		} else {
			p.n[k]--
		}
	}
}

func (p *snapshotRootPending) has(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.n[key] > 0
}

// ErrSnapshotChunkMissing is returned (wrapped) when a snapshot root names a
// chunk the host does not hold.
var ErrSnapshotChunkMissing = errors.New("snapshot root names a chunk the host does not hold")

// RecordVerifiedSnapshotRoot registers a snapshot root only if the computer's
// namespace holds every chunk it names, and returns the first missing digest
// (with ErrSnapshotChunkMissing) when it does not; a refusal records and
// prunes nothing.
//
// A computer holding a deletion tombstone is refused (ErrComputerTapeDeleted):
// its tree is being or has been purged, and a new root would pin chunks the
// purge removes. The tombstone set is an in-memory cache.
//
// The invariant against the artifact collector, whose only chunk unlink for a
// live computer is unlinkIfStillUnreachable under the exclusive casReuseMu:
//
//  1. Under the shared lock, every named chunk is stat'ed, and the digests
//     are added to the in-memory pending set before the lock is released. An
//     unlink serialised before this section removed the chunk, so the stat
//     fails and the root is refused; an unlink serialised after it sees the
//     chunk in the pending set and keeps it.
//  2. The database commit runs after the lock is released (a slow commit
//     must not queue event appends behind a waiting collector Lock; it is
//     bounded by snapshotRootCommitTimeout). After it commits, the root
//     generation is bumped, and only then is the pending set cleared. An
//     unlink after the clear sees a changed generation (or a live-root
//     query) and keeps the chunk through the committed root.
//  3. A refused or failed registration clears its pending digests and
//     records nothing; a failed commit leaves the chunks unrooted and
//     subject to the grace as before.
func (s *Service) RecordVerifiedSnapshotRoot(ctx context.Context, computerID, root string, headSequence int64, chunks []string) (string, error) {
	if s == nil || s.store == nil {
		return "", fmt.Errorf("snapshot root: store unavailable")
	}
	if err := validateSnapshotRootDeclaration(computerID, root, headSequence, chunks); err != nil {
		return "", err
	}
	if err := s.store.refuseTombstonedTape(ctx, computerID); err != nil {
		return "", err
	}
	keys := make([]string, 0, len(chunks))
	if missing, err := s.verifyAndPinSnapshotChunks(ctx, computerID, chunks, &keys); err != nil {
		return missing, err
	}
	defer s.store.snapshotPending.remove(keys)
	if hook := s.snapshotRootAfterVerify; hook != nil {
		hook()
	}
	commitCtx, cancel := context.WithTimeout(ctx, snapshotRootCommitTimeout)
	defer cancel()
	return "", s.store.recordSnapshotRoot(commitCtx, computerID, root, headSequence, chunks)
}

// verifyAndPinSnapshotChunks stats every distinct chunk under the shared CAS
// reuse lock and, if all exist, adds them to the pending set before the lock
// is released. keys receives the pinned keys (nil on failure).
func (s *Service) verifyAndPinSnapshotChunks(ctx context.Context, computerID string, chunks []string, keys *[]string) (string, error) {
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
		*keys = append(*keys, snapshotChunkKey(computerID, digest))
	}
	s.store.snapshotPending.add(*keys)
	return "", nil
}

// parseFileCASChunkRel splits sha256/file-cas-chunks/<computer>/<digest>.
func parseFileCASChunkRel(rel string) (computerID, digest string, ok bool) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 4 || parts[0] != "sha256" || parts[1] != "file-cas-chunks" || !safeFileCASComponent(parts[2]) || !validFileCASDigest(parts[3]) {
		return "", "", false
	}
	return parts[2], parts[3], true
}

// chunkProtectedBySnapshotRoot reports whether the chunk at rel must survive
// an unlink the collector is about to make. The caller holds casReuseMu
// exclusively. In order:
//
//   - a chunk of a registration that verified but has not finished (pending
//     set) is kept;
//   - if the candidate carries the root generation read before the sweep read
//     its live roots, and no root has committed since, the sweep's live set
//     already covered every root, and no query is needed;
//   - otherwise the computer's newest snapshot roots are asked for the digest
//     (the query is bound to ctx and a deadline; any error keeps the chunk).
//
// Any path that is not a file-CAS chunk is not protected.
func (s *Service) chunkProtectedBySnapshotRoot(ctx context.Context, c gcCandidate) (bool, error) {
	computerID, digest, ok := parseFileCASChunkRel(c.rel)
	if !ok {
		return false, nil
	}
	if s.store.snapshotPending.has(snapshotChunkKey(computerID, digest)) {
		return true, nil
	}
	if c.rootGenSet && s.store.snapshotRootGen.Load() == c.rootGen {
		return false, nil
	}
	s.snapshotRootRecheckQueries.Add(1)
	qctx, cancel := context.WithTimeout(ctx, snapshotRootCommitTimeout)
	defer cancel()
	var n int
	// Digests are 64 hex characters, so the quoted needle has no pattern
	// characters and matches only a whole JSON element.
	err := s.store.db.QueryRowContext(qctx, `SELECT COUNT(*) FROM (SELECT root, chunks FROM computer_snapshot_roots WHERE computer_id=? ORDER BY head_sequence DESC, created_at DESC, root DESC LIMIT ?) t WHERE chunks LIKE ?`,
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
	if errors.Is(err, ErrComputerTapeDeleted) {
		writeJSON(w, http.StatusConflict, apiError{Error: "snapshot root refused: " + err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"root": input.Root})
}

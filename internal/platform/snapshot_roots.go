package platform

import (
	"context"
	"database/sql"
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

// recordSnapshotRoot declares a snapshot's chunks and prunes the computer's
// rows (pruneSnapshotRoots): the newest snapshotRootRetention roots the tape
// names (largeSnapshotRetention when the declared snapshot is large), plus the
// newest root it does not yet name.
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
	if err := pruneSnapshotRoots(ctx, tx, computerID, retention); err != nil {
		return err
	}
	err = tx.Commit()
	if hook := s.snapshotRootCommitResult; hook != nil {
		err = hook(err)
	}
	// Bump whatever Commit returned, and never before it: an errored commit
	// (a lost reply, a cancelled request, the deadline) is an unknown
	// outcome, and the row may be durable. The caller clears the pending
	// set only after this returns, so any unlink after that clear sees a
	// changed generation and asks the database, which answers for whichever
	// way the commit went. A rolled-back root needs no protection: its
	// chunks are simply unrooted.
	s.snapshotRootGen.Add(1)
	return err
}

// snapshotRootNamedSQL is "the tape names this row": a snapshot_recorded
// receipt whose idempotency key is 'snapshot:'||root and whose sequence is
// above the root's head (internal/bundle readNamedSnapshotRoot says the same;
// internal/snapshot Take registers the root, then appends that event). Boot
// restore uses only roots the tape names. r is the roots table's alias.
const snapshotRootNamedSQL = `EXISTS (SELECT 1 FROM computer_event_append_receipts a WHERE a.computer_id=r.computer_id AND a.idempotency_key='snapshot:' || r.root AND a.event_kind='snapshot_recorded' AND a.sequence>r.head_sequence)`

// snapshotRootOrder is the one order of roots: newest head first.
const snapshotRootOrder = `ORDER BY r.head_sequence DESC, r.created_at DESC, r.root DESC`

// pruneSnapshotRoots keeps, for one computer: the newest retention roots the
// tape names, and the newest root it does not name when that root is above
// the newest named one (an attempt whose append is still coming, or that is
// retried under the same idempotency key). Every other row goes: an older
// unnamed root belongs to an attempt that is over, because the producer runs
// one capture at a time. Registration used to prune by head alone, so two
// registrations whose appends failed evicted the newest root the tape names,
// the only one a boot can restore (docs/problems/
// unappended-snapshot-root-evicts-the-on-tape-root-2026-11-13.md). The rows
// of a computer are few (retention + 1), so the prune reads them all.
func pruneSnapshotRoots(ctx context.Context, tx *sql.Tx, computerID string, retention int) error {
	rows, err := tx.QueryContext(ctx, `SELECT r.root, `+snapshotRootNamedSQL+` FROM computer_snapshot_roots r WHERE r.computer_id=? `+snapshotRootOrder, computerID)
	if err != nil {
		return fmt.Errorf("snapshot root: prune read: %w", err)
	}
	var drop []string
	named, keptUnnamed := 0, false
	for rows.Next() {
		var root string
		var isNamed bool
		if err := rows.Scan(&root, &isNamed); err != nil {
			rows.Close()
			return fmt.Errorf("snapshot root: prune scan: %w", err)
		}
		switch {
		case isNamed && named < retention:
			named++
		case isNamed:
			drop = append(drop, root)
		case named == 0 && !keptUnnamed:
			// The newest unnamed root, above every named one (rows are in
			// head order, and none is named yet).
			keptUnnamed = true
		default:
			drop = append(drop, root)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("snapshot root: prune read: %w", err)
	}
	rows.Close()
	for _, root := range drop {
		if _, err := tx.ExecContext(ctx, `DELETE FROM computer_snapshot_roots WHERE computer_id=? AND root=?`, computerID, root); err != nil {
			return fmt.Errorf("snapshot root: prune: %w", err)
		}
	}
	return nil
}

// snapshotRootDBDeadline bounds each database step the snapshot-root path
// adds under a lock or a gate: the root commit and the collector's live-root
// query.
const snapshotRootDBDeadline = 30 * time.Second

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
// Against the artifact collector and the tape sweep's non-purge items (the
// callers of unlinkIfStillUnreachable, under the exclusive casReuseMu; other
// removals are the destroyed purge and operator tools):
//
//  1. Under the shared lock, every named chunk is stat'ed, and the digests
//     are added to the in-memory pending set before the lock is released. An
//     unlink serialised before this section removed the chunk, so the stat
//     fails and the root is refused; an unlink serialised after it sees the
//     chunk in the pending set and keeps it.
//  2. The database commit runs after the lock is released (a slow commit
//     must not queue event appends behind a waiting collector Lock; it is
//     bounded by snapshotRootDBDeadline). After it commits, the root
//     generation is bumped, and only then is the pending set cleared. An
//     unlink after the clear sees a changed generation (or a live-root
//     query) and keeps the chunk through the committed root.
//  3. A refused or failed registration clears its pending digests and
//     records nothing. A commit of unknown outcome is covered by the
//     generation bump in recordSnapshotRoot.
//  4. The tombstone is re-checked under tombstones.gate (shared), held
//     through the commit, as CompareAndSwap does: applyDestroyedComputer
//     writes the tombstone under gate.Lock, so a root either commits before
//     the tombstone (the purge's row-count check then sees it and rolls the
//     rows step back for a fresh dry run) or is refused (ErrComputerTapeDeleted).
//
// Lock order: tombstones.gate, then the pending mutex (a leaf). casReuseMu is
// taken and released in step 1, before the gate, never nested with it, and
// applyDestroyedComputer takes gate.Lock without casReuseMu, so there is no
// inversion. The gate is now held across a database commit of up to
// snapshotRootDBDeadline; the most that can stall is a destroy apply's
// tombstone write (rare, operator-run, retryable) and, queued behind it, event
// appends, which already wait on the same gate. The collector's guarantee is
// as strong as these steps and the database are: a registration whose commit
// hangs holds its chunks pending until the deadline.
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
	s.store.tombstones.gate.RLock()
	defer s.store.tombstones.gate.RUnlock()
	if err := s.store.refuseTombstonedTape(ctx, computerID); err != nil {
		return "", err
	}
	if hook := s.snapshotRootInCommit; hook != nil {
		hook()
	}
	commitCtx, cancel := context.WithTimeout(ctx, snapshotRootDBDeadline)
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
//   - otherwise every snapshot root the computer keeps is asked for the digest
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
	qctx, cancel := context.WithTimeout(ctx, snapshotRootDBDeadline)
	defer cancel()
	var n int
	// Digests are 64 hex characters, so the quoted needle has no pattern
	// characters and matches only a whole JSON element.
	err := s.store.db.QueryRowContext(qctx, `SELECT COUNT(*) FROM computer_snapshot_roots WHERE computer_id=? AND chunks LIKE ?`,
		computerID, `%"`+digest+`"%`).Scan(&n)
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

// LiveSnapshotRoots returns every snapshot root the computer keeps, newest
// first. The prune bounds the rows at retention + 1, and a row that
// bundle.Import added or a concurrent registration left is protected too:
// reading more rows is the safe direction for the collector.
func (s *Store) LiveSnapshotRoots(ctx context.Context, computerID string) ([]SnapshotRootRecord, error) {
	if s == nil || s.db == nil || !safeFileCASComponent(computerID) {
		return nil, fmt.Errorf("snapshot root: computer ID is required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT computer_id,root,head_sequence,chunks,created_at FROM computer_snapshot_roots WHERE computer_id=? ORDER BY head_sequence DESC, created_at DESC, root DESC`, computerID)
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
		writeJSON(w, http.StatusGone, apiError{Error: "snapshot root refused: " + err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"root": input.Root})
}

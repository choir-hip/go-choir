package store

// Compaction of the state file (S5 of
// docs/design/one-tape-durability-2026-10-31.md, section 6b). SQLite never
// shrinks the file when rows are deleted: pages go to the freelist, and the
// snapshot streams every page. Compaction is its own heavy operation and never
// runs inside boot's envelope (owner, 2026-10-31: "boot should be light, as
// light as possible. then essential services. compaction is its own heavy
// operation"), so it is split in two:
//
//  1. While the computer runs, PrepareCompactCopy writes a compact copy of the
//     store beside it with VACUUM INTO (one read transaction: writers never
//     wait, the WAL is pinned for the copy's duration), checks it, and stages
//     it with a sidecar naming the event head it holds.
//  2. At the next planned restart, before the store opens,
//     RebaseOntoCompactCopy swaps the copy in when it is still on this store's
//     chain and its tail is short; the boot's ordinary tape replay then brings
//     it from the copy's head to the canonical head. This is the snapshot
//     restore's own claim (a state at a head plus the tape after it is the
//     state), with a local copy instead of fetched chunks. A crash boot never
//     swaps: the copy waits for a planned one.
//
// The original, with its WAL, is kept as the rollback until the replay is done
// (removePrecompact). A crash between the swap's renames is finished, or
// undone, by the next boot before anything opens the file.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

var (
	// compactFreelistFraction is the freelist share of the file that makes a
	// copy worth it; compactFreelistBytes makes it worth it on a large file
	// whose share is smaller.
	compactFreelistFraction       = 0.10
	compactFreelistBytes    int64 = 2 << 30
	// compactSpaceFactor is the free space, as a multiple of the live size,
	// the copy needs.
	compactSpaceFactor = 1.2
	// compactFreeBytes reports the bytes available to an unprivileged writer
	// on the filesystem holding dir; replaced by tests.
	compactFreeBytes = filesystemFreeBytes
	// compactCopyBudget bounds the copy's read transaction, which pins the
	// WAL while it runs.
	compactCopyBudget = 90 * time.Minute
	// compactRebaseMaxTail is the most events the boot may replay onto a
	// swapped-in copy; an older copy is dropped and the next one is fresher.
	compactRebaseMaxTail uint64 = 50000
	// incrementalVacuumChunk and incrementalVacuumPause pace reclaimFreePages.
	incrementalVacuumChunk = 2000
	incrementalVacuumPause = 100 * time.Millisecond
	// incrementalVacuumMaxPages bounds the pages one retirement run returns.
	incrementalVacuumMaxPages = 250000
)

const (
	// The files beside state.db: the copy being written, the staged copy and
	// its sidecar, and the original kept as the rollback after a swap.
	compactStagingSuffix = ".compact-staging"
	compactStagedSuffix  = ".compact"
	compactSidecarSuffix = ".compact.json"
	precompactSuffix     = ".precompact"
	leaseMarkerRebase    = "compact-rebase"
)

// compactSidecar names what the staged copy holds.
type compactSidecar struct {
	ComputerID string    `json:"computer_id"`
	Sequence   uint64    `json:"sequence"`
	Head       string    `json:"head"`
	Size       int64     `json:"size"`
	CreatedAt  time.Time `json:"created_at"`
}

func filesystemFreeBytes(dir string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

func removeSQLiteFiles(path string) {
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		_ = os.Remove(path + suffix)
	}
}

// PrepareCompactCopy stages a compact copy of the running store for the next
// planned restart, when enough of the file is free and the disk has room. It
// reports whether it staged one; an error is a skip, logged by the caller. A
// staged copy from an earlier run is replaced.
func (s *Store) PrepareCompactCopy(ctx context.Context) (bool, error) {
	if s == nil || s.statePath == "" || s.ReadDB() == nil {
		return false, nil
	}
	statePath := s.statePath
	staging := statePath + compactStagingSuffix
	// A copy from a crashed earlier attempt is never trusted.
	removeSQLiteFiles(staging)

	read := s.ReadDB()
	var pageCount, freelist, pageSize int64
	for q, dst := range map[string]*int64{"PRAGMA page_count": &pageCount, "PRAGMA freelist_count": &freelist, "PRAGMA page_size": &pageSize} {
		if err := read.QueryRowContext(ctx, q).Scan(dst); err != nil {
			return false, fmt.Errorf("%s: %w", q, err)
		}
	}
	if pageCount == 0 || (float64(freelist) <= compactFreelistFraction*float64(pageCount) && freelist*pageSize < compactFreelistBytes) {
		return false, nil
	}
	live := (pageCount - freelist) * pageSize
	free, err := compactFreeBytes(filepath.Dir(statePath))
	if err != nil {
		return false, fmt.Errorf("skipped: free space unreadable: %w", err)
	}
	if float64(free) < compactSpaceFactor*float64(live) {
		return false, fmt.Errorf("skipped: %d bytes free, %.1fx the %d-byte live size is needed", free, compactSpaceFactor, live)
	}
	log.Printf("store: compact copy: %d of %d pages free, live ~%d bytes, copying", freelist, pageCount, live)
	started := time.Now()

	// Its own connection: the read pool is query-only, and VACUUM INTO writes
	// the target file.
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(statePath, dbx.SQLiteOptions{}, false))
	if err != nil {
		return false, err
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	copyCtx, cancel := context.WithTimeout(ctx, compactCopyBudget)
	defer cancel()
	conn, err := db.Conn(copyCtx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	// The copy takes the connection's auto_vacuum, so later deletes can
	// return pages with incremental_vacuum (reclaimFreePages).
	if _, err := conn.ExecContext(copyCtx, `PRAGMA auto_vacuum=INCREMENTAL`); err != nil {
		return false, fmt.Errorf("auto_vacuum: %w", err)
	}
	if _, err := conn.ExecContext(copyCtx, `VACUUM INTO `+sqlQuote(staging)); err != nil {
		removeSQLiteFiles(staging)
		return false, fmt.Errorf("vacuum into: %w", err)
	}
	_ = conn.Close()
	side, err := checkCompactCopy(ctx, staging)
	if err != nil {
		removeSQLiteFiles(staging)
		return false, fmt.Errorf("copy refused: %w", err)
	}
	// The copy must be on this store's chain now; the boot checks again.
	if err := compactOnChain(ctx, read, side); err != nil {
		removeSQLiteFiles(staging)
		return false, fmt.Errorf("copy refused: %w", err)
	}
	if err := fsyncFile(staging); err != nil {
		removeSQLiteFiles(staging)
		return false, err
	}
	info, err := os.Stat(staging)
	if err != nil {
		removeSQLiteFiles(staging)
		return false, err
	}
	side.Size, side.CreatedAt = info.Size(), time.Now().UTC()
	// Stage: the copy first, the sidecar last. A copy with no sidecar is not
	// staged.
	_ = os.Remove(statePath + compactSidecarSuffix)
	if err := os.Rename(staging, statePath+compactStagedSuffix); err != nil {
		removeSQLiteFiles(staging)
		return false, err
	}
	raw, err := json.Marshal(side)
	if err != nil {
		return false, err
	}
	if err := writeFileDurable(statePath+compactSidecarSuffix, raw); err != nil {
		return false, err
	}
	log.Printf("store: compact copy: staged %d bytes at sequence %d in %s, for the next planned restart", side.Size, side.Sequence, time.Since(started).Round(time.Second))
	return true, nil
}

// checkCompactCopy checks the copy's structure and reads the head it holds.
func checkCompactCopy(ctx context.Context, path string) (compactSidecar, error) {
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
	if err != nil {
		return compactSidecar{}, err
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	var verdict string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&verdict); err != nil {
		return compactSidecar{}, fmt.Errorf("integrity_check: %w", err)
	}
	if verdict != "ok" {
		return compactSidecar{}, fmt.Errorf("integrity_check: %s", verdict)
	}
	var av int
	if err := db.QueryRowContext(ctx, `PRAGMA auto_vacuum`).Scan(&av); err != nil || av != 2 {
		return compactSidecar{}, fmt.Errorf("copy auto_vacuum = %d (%v), want 2 (incremental)", av, err)
	}
	side, err := compactHead(ctx, db)
	if err != nil {
		return compactSidecar{}, err
	}
	// Close here so the copy is one file: a checkpointed, closed database
	// leaves no WAL to carry.
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode=DELETE`); err != nil {
		return compactSidecar{}, err
	}
	return side, nil
}

// compactHead reads the one projection head a guest store holds.
func compactHead(ctx context.Context, db *sql.DB) (compactSidecar, error) {
	rows, err := db.QueryContext(ctx, `SELECT computer_id, sequence, canonical_event_head FROM computer_event_projection_heads`)
	if err != nil {
		return compactSidecar{}, err
	}
	defer rows.Close()
	var out []compactSidecar
	for rows.Next() {
		var h compactSidecar
		if err := rows.Scan(&h.ComputerID, &h.Sequence, &h.Head); err != nil {
			return compactSidecar{}, err
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return compactSidecar{}, err
	}
	if len(out) != 1 || out[0].Sequence == 0 {
		return compactSidecar{}, fmt.Errorf("%d projection heads, want one at a nonzero sequence", len(out))
	}
	return out[0], nil
}

// compactOnChain checks that side's head is on db's chain: db is at that head,
// or db's event after it follows it. It returns the tail between them.
func compactOnChain(ctx context.Context, db *sql.DB, side compactSidecar) error {
	_, err := compactTail(ctx, db, side)
	return err
}

func compactTail(ctx context.Context, db *sql.DB, side compactSidecar) (uint64, error) {
	cur, err := compactHead(ctx, db)
	if err != nil {
		return 0, err
	}
	switch {
	case cur.ComputerID != side.ComputerID:
		return 0, fmt.Errorf("copy is of %s, the store of %s", side.ComputerID, cur.ComputerID)
	case cur.Sequence < side.Sequence:
		return 0, fmt.Errorf("copy at sequence %d is past the store at %d", side.Sequence, cur.Sequence)
	case cur.Sequence == side.Sequence:
		if cur.Head != side.Head {
			return 0, errors.New("copy and store differ at the same sequence")
		}
		return 0, nil
	}
	var previous string
	err = db.QueryRowContext(ctx, `SELECT previous_head FROM computer_event_index WHERE computer_id=? AND sequence=? AND status='finalized'`, side.ComputerID, side.Sequence+1).Scan(&previous)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("the store holds no finalized event %d after the copy", side.Sequence+1)
	}
	if err != nil {
		return 0, err
	}
	if previous != side.Head {
		return 0, fmt.Errorf("the store's event %d does not follow the copy's head", side.Sequence+1)
	}
	return cur.Sequence - side.Sequence, nil
}

// RebaseOntoCompactCopy runs at boot before the store opens. Every boot first
// finishes or undoes a swap a crash interrupted. A planned boot then swaps in
// a staged compact copy that is still on the store's chain within the replay
// bound, and drops one that is not. It returns what it did, for the boot log
// ("" for nothing).
func RebaseOntoCompactCopy(dbPath string, planned bool) (string, error) {
	statePath := StateDBPath(dbPath)
	staged, sidecarPath, pre := statePath+compactStagedSuffix, statePath+compactSidecarSuffix, statePath+precompactSuffix
	stagedSide, stagedOK := readCompactSidecar(staged, sidecarPath)

	if _, err := os.Stat(statePath); errors.Is(err, os.ErrNotExist) {
		if _, preErr := os.Stat(pre); preErr != nil {
			return "", nil // a fresh store
		}
		// A crash between the renames.
		if err := moveWAL(statePath, pre); err != nil {
			return "", err
		}
		if stagedOK {
			if err := finishSwap(dbPath, staged, sidecarPath, statePath); err != nil {
				return "", err
			}
			return fmt.Sprintf("finished an interrupted rebase onto the copy at sequence %d", stagedSide.Sequence), nil
		}
		if err := undoSwap(statePath, pre); err != nil {
			return "", err
		}
		return "undid an interrupted rebase (no intact staged copy)", nil
	}
	if !planned {
		return "", nil
	}
	if _, err := os.Stat(staged); err != nil {
		_ = os.Remove(sidecarPath)
		return "", nil
	}
	discard := func(why string) (string, error) {
		removeSQLiteFiles(staged)
		_ = os.Remove(sidecarPath)
		return "discarded the staged compact copy: " + why, nil
	}
	if !stagedOK {
		return discard("its sidecar is missing, torn, or names another size")
	}
	tail, err := func() (uint64, error) {
		db, err := sql.Open("sqlite", dbx.SQLiteDSN(statePath, dbx.SQLiteOptions{}, false))
		if err != nil {
			return 0, err
		}
		db.SetMaxOpenConns(1)
		defer db.Close()
		return compactTail(context.Background(), db, stagedSide)
	}()
	if err != nil {
		return discard(err.Error())
	}
	if tail > compactRebaseMaxTail {
		return discard(fmt.Sprintf("its tail of %d events is past the %d-event bound", tail, compactRebaseMaxTail))
	}
	before := fileSizeOrZero(statePath)
	// The swapped-in history is older than the stream_seq lease the tape
	// holds: distrust it, durably, before the swap (storeschema).
	if err := storeschema.WriteLeaseUntrusted(deriveTextureWorkspacePath(dbPath), leaseMarkerRebase); err != nil {
		return "", err
	}
	removeSQLiteFiles(pre)
	if err := os.Rename(statePath, pre); err != nil {
		return "", err
	}
	if err := moveWAL(statePath, pre); err != nil {
		return "", errors.Join(err, undoSwap(statePath, pre))
	}
	if err := finishSwap(dbPath, staged, sidecarPath, statePath); err != nil {
		return "", errors.Join(err, undoSwap(statePath, pre))
	}
	return fmt.Sprintf("rebased onto the compact copy at sequence %d: %d -> %d bytes, %d events to replay", stagedSide.Sequence, before, stagedSide.Size, tail), nil
}

func readCompactSidecar(staged, sidecarPath string) (compactSidecar, bool) {
	raw, err := os.ReadFile(sidecarPath)
	if err != nil {
		return compactSidecar{}, false
	}
	var side compactSidecar
	if json.Unmarshal(raw, &side) != nil || side.Sequence == 0 || side.Head == "" {
		return compactSidecar{}, false
	}
	info, err := os.Stat(staged)
	if err != nil || info.Size() != side.Size {
		return compactSidecar{}, false
	}
	return side, true
}

// moveWAL moves the original's -wal and -shm beside the rollback copy, so
// SQLite never replays them over the new file and the rollback keeps them.
func moveWAL(statePath, pre string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(statePath + suffix); err != nil {
			continue
		}
		if err := os.Rename(statePath+suffix, pre+suffix); err != nil {
			return err
		}
	}
	return nil
}

func finishSwap(dbPath, staged, sidecarPath, statePath string) error {
	if err := storeschema.WriteLeaseUntrusted(deriveTextureWorkspacePath(dbPath), leaseMarkerRebase); err != nil {
		return err
	}
	if err := os.Rename(staged, statePath); err != nil {
		return err
	}
	syncDir(filepath.Dir(statePath))
	_ = os.Remove(sidecarPath)
	return nil
}

func undoSwap(statePath, pre string) error {
	for _, suffix := range []string{"-wal", "-shm", ""} {
		if _, err := os.Stat(pre + suffix); err != nil {
			continue
		}
		if err := os.Rename(pre+suffix, statePath+suffix); err != nil {
			return err
		}
	}
	syncDir(filepath.Dir(statePath))
	return nil
}

// removePrecompact drops the rollback copy once the boot replay is done.
func (s *Store) removePrecompact() {
	if s == nil || s.statePath == "" {
		return
	}
	pre := s.statePath + precompactSuffix
	if _, err := os.Stat(pre); err != nil {
		return
	}
	removeSQLiteFiles(pre)
	log.Printf("store: compact copy: removed the pre-rebase rollback copy")
}

func fileSizeOrZero(path string) int64 {
	if info, err := os.Stat(path); err == nil {
		return info.Size()
	}
	return 0
}

func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}

// writeFileDurable writes raw to path through a synced temp file and a rename.
func writeFileDurable(path string, raw []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o640); err != nil {
		return err
	}
	if err := fsyncFile(tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	syncDir(filepath.Dir(path))
	return nil
}

func sqlQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func fsyncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// reclaimFreePages returns freelist pages to the filesystem with
// PRAGMA incremental_vacuum, at most incrementalVacuumMaxPages in chunks with
// a pause between so the one writer connection is never held long. It does
// nothing on a file whose auto_vacuum is not INCREMENTAL (only a compacted
// file is). It returns the pages freed.
func (s *Store) reclaimFreePages(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	var mode int
	if err := s.db.QueryRowContext(ctx, `PRAGMA auto_vacuum`).Scan(&mode); err != nil || mode != 2 {
		return 0, err
	}
	var freed int64
	for freed < int64(incrementalVacuumMaxPages) {
		var before, after int64
		if err := s.db.QueryRowContext(ctx, `PRAGMA freelist_count`).Scan(&before); err != nil {
			return freed, err
		}
		if before == 0 {
			break
		}
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`PRAGMA incremental_vacuum(%d)`, incrementalVacuumChunk)); err != nil {
			return freed, err
		}
		if err := s.db.QueryRowContext(ctx, `PRAGMA freelist_count`).Scan(&after); err != nil {
			return freed, err
		}
		freed += before - after
		if after >= before {
			break
		}
		select {
		case <-ctx.Done():
			return freed, ctx.Err()
		case <-time.After(incrementalVacuumPause):
		}
	}
	return freed, nil
}

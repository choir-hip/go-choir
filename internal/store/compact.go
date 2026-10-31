package store

// Guarded compaction of the state file (L1 step 3 of
// docs/definitions/choir-lean-system-2026-10-30.md). It is its own heavy
// operation and never runs at boot (owner, 2026-10-31: "boot should be light,
// as light as possible. then essential services. compaction is its own heavy
// operation"); the paper's 11efa88c3 boot showed that Open comes before the
// credential exchange, whose envelope lives at most 15 minutes. SQLite never shrinks the
// file when rows are deleted: pages go to the freelist, and the snapshot
// streams every page. VACUUM INTO writes a compact copy.
//
// The guard, in order. Each failed condition logs and skips.
//
//  1. freelist pages exceed compactFreelistFraction of page_count;
//  2. the estimated live size (page_count - freelist) * page_size is at most
//     compactMaxLiveBytes;
//  3. the filesystem has free space of at least compactSpaceFactor x the
//     estimated live size for the copy;
//  4. the WAL is folded into the main file (checkpoint TRUNCATE);
//  5. VACUUM INTO a sibling temp file, with auto_vacuum=INCREMENTAL set on the
//     connection first so the copy can return later deletes to the OS with
//     incremental_vacuum;
//  6. the copy is fsynced, passes integrity_check, and equals the original on
//     the projection heads and on every table's row count (the heads carry the
//     canonical event head the content witness is derived at; VACUUM INTO is
//     a page-level rewrite, so equal heads and counts is the proof the copy is
//     the same state);
//  7. the original's -wal/-shm are removed (the WAL was truncated, nothing
//     lives there) and the copy is renamed over the state file; the directory
//     is fsynced. A crash before the rename leaves the original untouched and
//     the leftover temp file is removed by the next attempt.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

var (
	// compactFreelistFraction is the freelist share of the file that triggers
	// a compaction; compactFreelistBytes triggers it on a large file whose
	// share is smaller (the paper's event index trim frees about 7 GB of 32).
	compactFreelistFraction       = 0.10
	compactFreelistBytes    int64 = 2 << 30
	// compactMaxLiveBytes is the largest live size boot will rewrite. Open
	// runs before the guest's credential exchange, and the boot envelope
	// lives at most 15 minutes from when vmctl mints it, before the VM starts
	// (platform.maximumCredentialEnvelopeTTL). A boot that spends it here
	// never becomes ready: the paper's 11efa88c3 boot spent 10 minutes in
	// the event index catch-up and failed
	// (docs/problems/offline-maintenance-outlives-the-boot-credential-2026-10-24.md).
	// 4 GB rewrites in about a minute; a larger store is compacted off the
	// boot path.
	compactMaxLiveBytes int64 = 4 << 30
	// compactSpaceFactor is the free space, as a multiple of the live size,
	// the copy needs.
	compactSpaceFactor = 1.2
	// compactFreeBytes reports the bytes available to an unprivileged writer
	// on the filesystem holding dir; replaced by tests.
	compactFreeBytes = filesystemFreeBytes
	// incrementalVacuumChunk and incrementalVacuumPause pace reclaimFreePages.
	incrementalVacuumChunk = 2000
	incrementalVacuumPause = 100 * time.Millisecond
	// incrementalVacuumMaxPages bounds the pages one retirement run returns.
	incrementalVacuumMaxPages = 250000
)

const compactTempSuffix = ".compact"

func filesystemFreeBytes(dir string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

// compactStateFile runs the guarded compaction on statePath. It must run
// before the store's pools open the file. It reports whether it replaced the
// file; any error is a skip, logged by the caller.
func compactStateFile(statePath string) (compacted bool, err error) {
	// A temp file from a crashed earlier attempt is never trusted.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(statePath + compactTempSuffix + suffix)
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(statePath, dbx.SQLiteOptions{}, false))
	if err != nil {
		return false, err
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	var pageCount, freelist, pageSize int64
	for q, dst := range map[string]*int64{"PRAGMA page_count": &pageCount, "PRAGMA freelist_count": &freelist, "PRAGMA page_size": &pageSize} {
		if err := conn.QueryRowContext(ctx, q).Scan(dst); err != nil {
			return false, fmt.Errorf("%s: %w", q, err)
		}
	}
	if pageCount == 0 || (float64(freelist) <= compactFreelistFraction*float64(pageCount) && freelist*pageSize < compactFreelistBytes) {
		return false, nil
	}
	live := (pageCount - freelist) * pageSize
	if live > compactMaxLiveBytes {
		return false, fmt.Errorf("skipped: live size %d bytes is over the %d-byte boot bound", live, compactMaxLiveBytes)
	}
	free, err := compactFreeBytes(dirOf(statePath))
	if err != nil {
		return false, fmt.Errorf("skipped: free space unreadable: %w", err)
	}
	if float64(free) < compactSpaceFactor*float64(live) {
		return false, fmt.Errorf("skipped: %d bytes free, %.1fx the %d-byte live size is needed", free, compactSpaceFactor, live)
	}
	log.Printf("store: compaction: %d of %d pages free, live ~%d bytes, copying", freelist, pageCount, live)

	if err := conn.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(new(int), new(int), new(int)); err != nil {
		return false, fmt.Errorf("checkpoint: %w", err)
	}
	tmp := statePath + compactTempSuffix
	cleanup := func() {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(tmp + suffix)
		}
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA auto_vacuum=INCREMENTAL`); err != nil {
		return false, fmt.Errorf("auto_vacuum: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `VACUUM INTO `+sqlQuote(tmp)); err != nil {
		cleanup()
		return false, fmt.Errorf("vacuum into: %w", err)
	}
	if err := fsyncFile(tmp); err != nil {
		cleanup()
		return false, err
	}
	if err := verifyCompactCopy(ctx, conn, tmp); err != nil {
		cleanup()
		return false, fmt.Errorf("copy refused: %w", err)
	}
	// Release the original before swapping it.
	if err := conn.Close(); err != nil {
		cleanup()
		return false, err
	}
	if err := db.Close(); err != nil {
		cleanup()
		return false, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(statePath + suffix); err != nil && !os.IsNotExist(err) {
			cleanup()
			return false, err
		}
	}
	if err := os.Rename(tmp, statePath); err != nil {
		cleanup()
		return false, fmt.Errorf("rename: %w", err)
	}
	if d, err := os.Open(dirOf(statePath)); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return true, nil
}

func dirOf(path string) string {
	if i := strings.LastIndexByte(path, '/'); i > 0 {
		return path[:i]
	}
	return "."
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

// verifyCompactCopy checks the copy at path against the original on conn.
func verifyCompactCopy(ctx context.Context, orig *sql.Conn, path string) error {
	copyDB, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
	if err != nil {
		return err
	}
	copyDB.SetMaxOpenConns(1)
	defer copyDB.Close()
	var verdict string
	if err := copyDB.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&verdict); err != nil {
		return fmt.Errorf("integrity_check: %w", err)
	}
	if verdict != "ok" {
		return fmt.Errorf("integrity_check: %s", verdict)
	}
	var av int
	if err := copyDB.QueryRowContext(ctx, `PRAGMA auto_vacuum`).Scan(&av); err != nil || av != 2 {
		return fmt.Errorf("copy auto_vacuum = %d (%v), want 2 (incremental)", av, err)
	}
	type q interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	}
	tables := func(c q) ([]string, error) {
		rows, err := c.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var names []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				return nil, err
			}
			names = append(names, n)
		}
		return names, rows.Err()
	}
	origTables, err := tables(orig)
	if err != nil {
		return err
	}
	copyTables, err := tables(copyDB)
	if err != nil {
		return err
	}
	if strings.Join(origTables, ",") != strings.Join(copyTables, ",") {
		return fmt.Errorf("table sets differ")
	}
	count := func(c interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}, table string) (int64, error) {
		var n int64
		err := c.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+strings.ReplaceAll(table, `"`, `""`)+`"`).Scan(&n)
		return n, err
	}
	for _, table := range origTables {
		a, err := count(orig, table)
		if err != nil {
			return fmt.Errorf("count %s: %w", table, err)
		}
		b, err := count(copyDB, table)
		if err != nil {
			return fmt.Errorf("count %s in copy: %w", table, err)
		}
		if a != b {
			return fmt.Errorf("table %s has %d rows, the copy %d", table, a, b)
		}
	}
	// The heads: the canonical event head each computer's witness is taken at.
	const headQuery = `SELECT group_concat(computer_id || ':' || sequence || ':' || canonical_event_head, ',') FROM (SELECT computer_id, sequence, canonical_event_head FROM computer_event_projection_heads ORDER BY computer_id)`
	var ha, hb sql.NullString
	hasHeads := false
	for _, t := range origTables {
		if t == "computer_event_projection_heads" {
			hasHeads = true
		}
	}
	if hasHeads {
		if err := orig.QueryRowContext(ctx, headQuery).Scan(&ha); err != nil {
			return err
		}
		if err := copyDB.QueryRowContext(ctx, headQuery).Scan(&hb); err != nil {
			return err
		}
		if ha != hb {
			return errors.New("projection heads differ")
		}
	}
	return nil
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

package platform

// What the backfill walk costs the table, and when it is let to start
// (docs/problems/old-receipts-do-not-compact-2026-11-14.md). The walk does not
// depend on the state of the table's indexes: every reader works with either
// set of kind indexes (receipt_indexes.go), and the swap of the live table's
// indexes is a supervised act (scripts/receipts-index-swap), not something the
// binary waits for.

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// receiptWalkMinFree is the free space the walk needs before it starts. The
// record's estimate for the 5.13M rewrites of the paper is about 2.4 GB of new
// heap, 0.8 GB of index churn and 7.7 GB of dead tuples that stay until a
// vacuum makes them reusable, 10.9 GB if no vacuum ran in time, and the WAL
// comes on top; 12 GiB is that estimate rounded up. A host with less free
// would be filled by appends, which is worse than a walk that waits.
const receiptWalkMinFree uint64 = 12 << 30

// receiptWalkFreeSpace reads the free bytes of the volumes it can read, and
// returns the smaller with the directory it measured. It tries PostgreSQL's
// data directory first; the deployed database role may not read that setting
// (it needs pg_read_all_settings), and then the volume measured is the artifacts
// root's alone. The first call says which, once. known is false when nothing
// could be read.
func (s *Store) receiptWalkFreeSpace(ctx context.Context) (free uint64, from string, known bool) {
	var dirs []string
	var dataDir string
	dataErr := s.db.QueryRowContext(ctx, `SELECT current_setting('data_directory')`).Scan(&dataDir)
	if dataErr == nil && dataDir != "" {
		dirs = append(dirs, dataDir)
	}
	if s.artifactsRoot != "" {
		dirs = append(dirs, s.artifactsRoot)
	}
	for _, dir := range dirs {
		usage, err := fileCASVolumeUsage(dir)
		if err != nil {
			continue
		}
		if !known || usage.AvailBytes < free {
			free, from, known = usage.AvailBytes, dir, true
		}
	}
	if s.walkFreeLogged.CompareAndSwap(false, true) {
		log.Print(walkFreeNote(dataErr, from, known))
	}
	return free, from, known
}

// walkFreeNote is the one line that says which volume the free-space reading
// measures.
func walkFreeNote(dataErr error, from string, known bool) string {
	switch {
	case dataErr != nil && known:
		return fmt.Sprintf("corpusd: receipt walk free space: PostgreSQL's data directory is not readable by this role (%v); measuring %s only, which may not be the volume of the database", dataErr, from)
	case dataErr != nil:
		return fmt.Sprintf("corpusd: receipt walk free space: PostgreSQL's data directory is not readable by this role (%v) and no other volume could be measured; the walk is not held back for space", dataErr)
	case known:
		return fmt.Sprintf("corpusd: receipt walk free space: measuring %s (the smaller of the data directory and the artifacts root)", from)
	}
	return "corpusd: receipt walk free space: nothing could be measured; the walk is not held back for space"
}

// receiptBuildRunning reports whether another session holds a maintenance lock
// on the receipt table: SHARE UPDATE EXCLUSIVE is what CREATE INDEX
// CONCURRENTLY, DROP INDEX CONCURRENTLY, a manual VACUUM or ANALYZE hold for
// their whole run. pg_stat_progress_create_index would name the build, but for a
// backend of another role it shows NULL in every column unless the reader has
// pg_read_all_stats, and the deployed role (choir) has not while the swap runs as
// postgres. pg_locks is visible to every role and so is usename, but not
// backend_type or state (NULL for another role's backend, checked by
// TestReceiptBuildRunningIsSeenByAnUnprivilegedRole). usename is NULL for a
// background process, so an autovacuum worker is not counted: the walk makes
// the dead tuples it vacuums, and the walk is not to stop for it.
func (s *Store) receiptBuildRunning(ctx context.Context) (bool, error) {
	var running bool
	err := s.db.QueryRowContext(ctx, receiptBuildRunningSQL).Scan(&running)
	return running, err
}

const receiptBuildRunningSQL = `SELECT EXISTS (
	SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid
	WHERE l.locktype = 'relation' AND l.relation = 'computer_event_append_receipts'::regclass
	  AND l.mode = 'ShareUpdateExclusiveLock' AND l.granted
	  AND l.pid <> pg_backend_pid() AND a.usename IS NOT NULL)`

// deferWalk says why the walk should not run now, or "" when it may: another
// session is building or dropping an index on the table (one heavy job at a time
// on it; the swap is supervised and the walk steps aside, nothing waits for the
// walk), or the measured volume has less room than the walk's rewrites need. An
// unknown free space does not hold the walk back (there is nothing to compare),
// and the load lines show the size. checkSpace is false for the per-batch look,
// which reads the volume only at the load line's cadence.
func (c *ReceiptCompactor) deferWalk(ctx context.Context, checkSpace bool) string {
	running, err := c.buildRunning(ctx)
	c.noteBuildCheck(err)
	if err == nil && running {
		return "an index build or other maintenance holds the receipt table (SHARE UPDATE EXCLUSIVE); one heavy job at a time on it"
	}
	if !checkSpace {
		return ""
	}
	free, from, known := c.freeSpace(ctx)
	if known && free < receiptWalkMinFree {
		return fmt.Sprintf("%d MiB free on %s, under the %d MiB the walk needs", free>>20, from, receiptWalkMinFree>>20)
	}
	return ""
}

// noteBuildCheck says once, on the transition, that the build check cannot be
// read (the walk then runs: nothing is known to hold the table), and once when it
// can again.
func (c *ReceiptCompactor) noteBuildCheck(err error) {
	if err != nil && !c.buildCheckFailed {
		c.buildCheckFailed = true
		log.Printf("corpusd: receipt walk: build check unavailable: %v; the walk runs", err)
	} else if err == nil && c.buildCheckFailed {
		c.buildCheckFailed = false
		log.Printf("corpusd: receipt walk: build check available again")
	}
}

// noteDeferral logs the reason the walk is not running, every time it is asked
// (once a pass, ten minutes apart, and when a pass steps aside), so that the line
// is in any ten-minute journal window while the walk is held; and a "resumes"
// line when it was held and is not.
func (c *ReceiptCompactor) noteDeferral(why string) {
	was := c.lastDeferral
	c.lastDeferral = why
	switch {
	case why != "":
		log.Printf("corpusd: receipt compaction deferred: %s", why)
	case was != "":
		log.Printf("corpusd: receipt compaction resumes")
	}
}

// walkState is the walk's state for the load line: running, or the reason it was
// last held.
func (c *ReceiptCompactor) walkState() string {
	switch {
	case c.lastDeferral == "":
		return "running"
	case strings.Contains(c.lastDeferral, "free on"):
		return "deferred: free space"
	}
	return "deferred: build"
}

// ReceiptTableLoad is what a walk over the table costs it, read from
// PostgreSQL's own counters.
type ReceiptTableLoad struct {
	LiveTuples      int64
	DeadTuples      int64
	Autovacuums     int64
	LastAutovacuum  string
	WALBytesSince   int64
	TableBytes      int64
	WALBytesVisible bool
}

func (c *ReceiptCompactor) walLSN(ctx context.Context) (string, error) {
	var lsn string
	err := c.store.db.QueryRowContext(ctx, `SELECT pg_current_wal_insert_lsn()::text`).Scan(&lsn)
	return lsn, err
}

// receiptTableLoad reads the table's live and dead tuples, its autovacuum
// count and size, and the WAL written since startLSN (when it was read). The
// table is the one the store's queries resolve, not one found by schema name.
func (c *ReceiptCompactor) receiptTableLoad(ctx context.Context, startLSN string) (ReceiptTableLoad, error) {
	var load ReceiptTableLoad
	err := c.store.db.QueryRowContext(ctx, `SELECT COALESCE(n_live_tup,0), COALESCE(n_dead_tup,0), COALESCE(autovacuum_count,0), COALESCE(to_char(last_autovacuum AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), 'never'), pg_total_relation_size(relid)
		FROM pg_stat_user_tables WHERE relid = 'computer_event_append_receipts'::regclass`).
		Scan(&load.LiveTuples, &load.DeadTuples, &load.Autovacuums, &load.LastAutovacuum, &load.TableBytes)
	if err != nil {
		return load, err
	}
	if startLSN != "" {
		if err := c.store.db.QueryRowContext(ctx, `SELECT pg_wal_lsn_diff(pg_current_wal_insert_lsn(), ?::pg_lsn)::bigint`, startLSN).Scan(&load.WALBytesSince); err == nil {
			load.WALBytesVisible = true
		}
	}
	return load, nil
}

// logLoad writes one line: the walk's counts, the table's load and the free
// space of the volume measured.
func (c *ReceiptCompactor) logLoad(ctx context.Context, computerID string, pass ReceiptCompactionPass, startLSN string, walkStart time.Time) {
	load, err := c.receiptTableLoad(ctx, startLSN)
	if err != nil {
		log.Printf("corpusd: receipt compaction load %s through=%d: %v", computerID, pass.Through, err)
		return
	}
	free, from, known := c.freeSpace(ctx)
	freeText := "unknown"
	if known {
		freeText = fmt.Sprintf("%d (%s)", free, from)
	}
	wal := "unknown"
	if load.WALBytesVisible {
		wal = fmt.Sprint(load.WALBytesSince)
	}
	log.Printf("corpusd: receipt compaction load %s through=%d scanned=%d rewritten=%d kept=%d elapsed=%s live_tuples=%d dead_tuples=%d autovacuums=%d last_autovacuum=%s table_bytes=%d wal_bytes_since_start=%s free_bytes=%s walk_state=%s",
		computerID, pass.Through, pass.Scanned, pass.Rewritten, pass.Kept, time.Since(walkStart).Round(time.Second),
		load.LiveTuples, load.DeadTuples, load.Autovacuums, load.LastAutovacuum, load.TableBytes, wal, freeText, c.walkState())
}

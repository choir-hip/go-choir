package platform

// What the backfill walk costs the table, and when it is let to start
// (docs/problems/old-receipts-do-not-compact-2026-11-14.md).

import (
	"context"
	"fmt"
	"log"
	"time"
)

// The states of Store.receiptIndexState: the index migration
// (receipt_index_migration.go) sets them, the walk reads them.
const (
	receiptIndexRunning = 1
	receiptIndexDone    = 2
)

func (s *Store) receiptIndexMigrationRunning() bool {
	return s.receiptIndexState.Load() == receiptIndexRunning
}

// receiptWalkMinFree is the free space the walk needs before it starts. The
// record's estimate for the 5.13M rewrites of the paper is about 2.4 GB of new
// heap, 0.8 GB of index churn and 7.7 GB of dead tuples that stay until a
// vacuum makes them reusable, 10.9 GB if no vacuum ran in time, and the WAL
// comes on top; 12 GiB is that estimate rounded up. A host with less free
// would be filled by appends, which is worse than a walk that waits.
const receiptWalkMinFree uint64 = 12 << 30

// receiptWalkFreeSpace reads the free bytes of the volume that holds the
// database's data directory and of the one that holds the artifacts root, and
// returns the smaller. known is false when neither could be read.
func (s *Store) receiptWalkFreeSpace(ctx context.Context) (free uint64, from string, known bool) {
	var dirs []string
	var dataDir string
	if err := s.db.QueryRowContext(ctx, `SELECT current_setting('data_directory')`).Scan(&dataDir); err == nil && dataDir != "" {
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
	return free, from, known
}

// deferWalk says why a pass should not start now, or "" when it may: the index
// migration is the heavy job on the table at that time, and the volume must
// have room for the walk's rewrites. An unknown free space does not hold the
// walk back (there is nothing to compare), and the load lines show the size.
func (c *ReceiptCompactor) deferWalk(ctx context.Context) string {
	if c.store.receiptIndexMigrationRunning() {
		return "the receipt index migration is running; one heavy job at a time on the table"
	}
	free, from, known := c.freeSpace(ctx)
	if known && free < receiptWalkMinFree {
		return fmt.Sprintf("%d MiB free on %s, under the %d MiB the walk needs", free>>20, from, receiptWalkMinFree>>20)
	}
	return ""
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
// space of the volumes.
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
	log.Printf("corpusd: receipt compaction load %s through=%d scanned=%d rewritten=%d kept=%d elapsed=%s live_tuples=%d dead_tuples=%d autovacuums=%d last_autovacuum=%s table_bytes=%d wal_bytes_since_start=%d free_bytes=%s",
		computerID, pass.Through, pass.Scanned, pass.Rewritten, pass.Kept, time.Since(walkStart).Round(time.Second),
		load.LiveTuples, load.DeadTuples, load.Autovacuums, load.LastAutovacuum, load.TableBytes, load.WALBytesSince, freeText)
}

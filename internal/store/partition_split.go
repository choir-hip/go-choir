package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// partitionSplitHook, when set (tests), is called after each step of
// migratePartitionSplit inside its transaction; an error rolls the file back.
var partitionSplitHook func(step string) error

// migratePartitionSplit migrates one telemetry partition file from the
// pre-split choir_events layout (a body column in the row, primary key
// (tape_seq, op_index), no event_phase) to choirEventsDDL, in one transaction
// (design section 6 step 11). It runs in openFile before the DDL. A file
// already split, or new (no choir_events table), is left as it is.
//
// ev_id is the old rowid, so the physical order is kept. event_phase is
// computed by the same SQL expression the retire predicate evaluated over the
// body. The old table is dropped in the same transaction, and the file is
// vacuumed after the commit (best effort: the data is already migrated).
func migratePartitionSplit(ctx context.Context, db *sql.DB) error {
	return migratePartitionSplitProgress(ctx, db, nil)
}

// partitionSplitBatchRows is how many rows one copy statement moves, so the
// progress reported to the boot gate advances during a large file's copy.
var partitionSplitBatchRows int64 = 50000

// migratePartitionSplitProgress is migratePartitionSplit reporting the rows
// copied so far in this file (monotone, headers and bodies counted alike).
func migratePartitionSplitProgress(ctx context.Context, db *sql.DB, progress func(rows uint64)) error {
	var hasBody int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('choir_events') WHERE name = 'body'`).Scan(&hasBody); err != nil {
		return fmt.Errorf("partition split: detect layout: %w", err)
	}
	if hasBody == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("partition split: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	step := func(name, query string) error {
		if query != "" {
			if _, err := tx.ExecContext(ctx, query); err != nil {
				return fmt.Errorf("partition split: %s: %w", name, err)
			}
		}
		if partitionSplitHook != nil {
			if err := partitionSplitHook(name); err != nil {
				return fmt.Errorf("partition split: %s: %w", name, err)
			}
		}
		return nil
	}
	if err := step("rename", `ALTER TABLE choir_events RENAME TO choir_events_presplit`); err != nil {
		return err
	}
	for _, idx := range []string{"run_seq", "owner_stream", "trajectory_stream", "updated", "kind_updated"} {
		if err := step("drop-index", `DROP INDEX IF EXISTS idx_choir_events_`+idx); err != nil {
			return err
		}
	}
	if err := step("create", choirEventsDDL); err != nil {
		return err
	}
	var minRow, maxRow sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MIN(rowid), MAX(rowid) FROM choir_events_presplit`).Scan(&minRow, &maxRow); err != nil {
		return fmt.Errorf("partition split: rowid range: %w", err)
	}
	var copied uint64
	batch := func(name, query string) error {
		if !minRow.Valid {
			return step(name, "")
		}
		// hi is clamped so neither it nor the cursor step can overflow near
		// MaxInt64. The range is [lo, hi] inclusive.
		for lo := minRow.Int64; ; {
			hi := maxRow.Int64
			if maxRow.Int64-lo >= partitionSplitBatchRows {
				hi = lo + partitionSplitBatchRows - 1
			}
			res, err := tx.ExecContext(ctx, query, lo, hi)
			if err != nil {
				return fmt.Errorf("partition split: %s: %w", name, err)
			}
			n, _ := res.RowsAffected()
			copied += uint64(n)
			if progress != nil {
				progress(copied)
			}
			if hi >= maxRow.Int64 {
				break
			}
			lo = hi + 1
		}
		return step(name, "")
	}
	if err := batch("copy-headers", `INSERT INTO choir_events
		(ev_id, tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata,
		 created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind, event_phase)
		SELECT rowid, tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata,
		 created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind,
		 `+objectgraph.ChoirEventPhaseSQLFor("body", "metadata")+`
		FROM choir_events_presplit WHERE rowid BETWEEN ? AND ? ORDER BY rowid`); err != nil {
		return err
	}
	if err := batch("copy-bodies", `INSERT INTO choir_event_bodies (ev_id, body) SELECT rowid, body FROM choir_events_presplit WHERE rowid BETWEEN ? AND ? ORDER BY rowid`); err != nil {
		return err
	}
	var oldN, hdrN, bodyN int64
	if err := tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM choir_events_presplit), (SELECT COUNT(*) FROM choir_events), (SELECT COUNT(*) FROM choir_event_bodies)`).Scan(&oldN, &hdrN, &bodyN); err != nil {
		return fmt.Errorf("partition split: tally: %w", err)
	}
	if oldN != hdrN || oldN != bodyN {
		return fmt.Errorf("partition split: tally mismatch: old=%d headers=%d bodies=%d", oldN, hdrN, bodyN)
	}
	if err := step("drop-old", `DROP TABLE choir_events_presplit`); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("partition split: commit: %w", err)
	}
	if _, err := db.ExecContext(ctx, `VACUUM`); err != nil {
		log.Printf("store: telemetry partition split: vacuum after migrating %d rows: %v", oldN, err)
	}
	return nil
}

// oldPartitionFile is a live telemetry partition file still in the old layout.
type oldPartitionFile struct {
	path string
	size int64
}

// oldLayoutPartitions lists the partition files of the store at dbPath whose
// choir_events still has a body column. Retired partitions (the state file's
// telemetry_partitions_retired) are skipped: they are never read or rewritten.
// Nothing in a file changes here, though the files are opened with the
// default (read-write) mode, not mode=ro.
func oldLayoutPartitions(dbPath string) ([]oldPartitionFile, error) {
	dir := TelemetryPartitionDir(dbPath)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("telemetry partitions: list %s: %w", dir, err)
	}
	retired, err := retiredPartitionNames(dbPath)
	if err != nil {
		return nil, err
	}
	var out []oldPartitionFile
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".db") {
			continue
		}
		if _, gone := retired[strings.TrimSuffix(name, ".db")]; gone {
			continue
		}
		if _, _, ok := parseTelemetryPartitionName(strings.TrimSuffix(name, ".db")); !ok {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		old, err := partitionFileOldLayout(path)
		if err != nil {
			return nil, fmt.Errorf("telemetry partitions: detect layout of %s: %w", name, err)
		}
		if old {
			out = append(out, oldPartitionFile{path: path, size: info.Size()})
		}
	}
	return out, nil
}

// retiredPartitionNames reads the retired set from the state file beside the
// store (or its presplit copy mid-swap). A missing file or table is an empty
// set.
func retiredPartitionNames(dbPath string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	statePath := StateDBPath(dbPath)
	if _, err := os.Stat(statePath); err != nil {
		statePath += presplitSuffix
		if _, err := os.Stat(statePath); err != nil {
			return out, nil
		}
	}
	db, err := sql.Open("sqlite", "file:"+statePath+"?mode=ro&_pragma=busy_timeout(30000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	var has int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='telemetry_partitions_retired'`).Scan(&has); err != nil {
		return nil, fmt.Errorf("telemetry partitions: read the retired set: %w", err)
	}
	if has == 0 {
		return out, nil
	}
	rows, err := db.Query(`SELECT name FROM telemetry_partitions_retired`)
	if err != nil {
		return nil, fmt.Errorf("telemetry partitions: read the retired set: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = struct{}{}
	}
	return out, rows.Err()
}

func partitionFileOldLayout(path string) (bool, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(30000)")
	if err != nil {
		return false, err
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	return dbx.ColumnExists(context.Background(), db, dbx.SQLite, "choir_events", "body")
}

func largestPartition(parts []oldPartitionFile) int64 {
	var m int64
	for _, p := range parts {
		if p.size > m {
			m = p.size
		}
	}
	return m
}

// partitionRewritePeak is the disk one file's rewrite peaks at: the original,
// the copy, and in WAL mode the copy transaction's WAL, the checkpoint and the
// VACUUM's rebuilt file can reach about 4x transiently.
func partitionRewritePeak(size int64) int64 { return 4 * size }

// partitionSplitHeadroomBytes is what a partition rewrite leaves free.
const partitionSplitHeadroomBytes int64 = 64 << 20

// migrateTelemetryPartitionFiles rewrites every old-layout partition file of
// the store at dbPath, one at a time, reporting rows copied so far across all
// files. A file whose rewrite would not fit refuses with the typed refusal
// before it is touched. Open's own per-file migration stays as the safety net
// and finds nothing to do.
func migrateTelemetryPartitionFiles(dbPath string, progress func(uint64)) error {
	parts, err := oldLayoutPartitions(dbPath)
	if err != nil || len(parts) == 0 {
		return err
	}
	var done uint64
	for _, p := range parts {
		free, err := compactFreeBytes(filepath.Dir(p.path))
		if err != nil {
			return &HeaderBodySplitRefusal{Reason: "free space unreadable: " + err.Error(), Live: p.size}
		}
		need := partitionRewritePeak(p.size) + partitionSplitHeadroomBytes
		if free < need {
			return &HeaderBodySplitRefusal{Reason: "too little free disk to rewrite telemetry partition " + filepath.Base(p.path), Live: p.size, Free: free, Need: need}
		}
		phase := objectgraph.BootPhase("store: open", "header-body-split/partition-rewrite/"+filepath.Base(p.path))
		// The VACUUM's temp files go beside the store: the guest /tmp may be RAM.
		_ = os.Setenv("SQLITE_TMPDIR", filepath.Dir(p.path))
		db, err := dbx.OpenSQLite(p.path, dbx.SQLiteOptions{ReadConns: 1})
		if err != nil {
			phase(err)
			return fmt.Errorf("telemetry partitions: open %s: %w", filepath.Base(p.path), err)
		}
		var fileRows uint64
		err = migratePartitionSplitProgress(context.Background(), db.Write, func(n uint64) {
			fileRows = n
			if progress != nil {
				progress(done + n)
			}
		})
		_ = db.Close()
		phase(err)
		if err != nil {
			return fmt.Errorf("telemetry partitions: split migration of %s: %w", filepath.Base(p.path), err)
		}
		done += fileRows
	}
	return nil
}

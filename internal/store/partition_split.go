package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"

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
	if err := step("copy-headers", `INSERT INTO choir_events
		(ev_id, tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata,
		 created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind, event_phase)
		SELECT rowid, tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, metadata,
		 created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind,
		 `+objectgraph.ChoirEventPhaseSQLFor("body", "metadata")+`
		FROM choir_events_presplit ORDER BY rowid`); err != nil {
		return err
	}
	if err := step("copy-bodies", `INSERT INTO choir_event_bodies (ev_id, body) SELECT rowid, body FROM choir_events_presplit ORDER BY rowid`); err != nil {
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

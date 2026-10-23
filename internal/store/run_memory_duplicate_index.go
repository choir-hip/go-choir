package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// R5 of docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md,
// worked in docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md
// ("R5: the duplicate idx_run_memory_entries_loop_seq").
//
// run_memory_entries has UNIQUE(loop_id, seq) (Dolt names it `loop_id`) and had
// a plain (loop_id, seq) index beside it, from the same commit. The planner
// reads the unique key for every statement that filters on loop_id; the plain
// index was a second tree written on every append (about 12 KiB of journal)
// that nothing reads. schemaDDL no longer creates it, so a fresh store is made
// without it.
//
// It follows the repo's rule for a table with rows, the kind index in reverse:
// Open drops it only from an empty table, where it costs nothing; on a table
// with rows Open only reports it, and dropping it is a step of the explicit
// /internal/store/derived-columns operation, under the engine mutex, at a time
// the operator chooses. The drop does not change the content witness:
// loop_id's SHOW COLUMNS Key is MUL through the composite unique key either way
// (TestRunMemoryLoopSeqIndexDropKeepsTheWitnessSchemaObservation). It is
// refused unless a unique key on exactly (loop_id, seq) exists, so the
// statements that filter on loop_id cannot fall to a table scan.
//
// Rollback: recreate with runMemoryLoopSeqIndexDDL. A reverted binary's own
// schemaDDL line (CREATE INDEX IF NOT EXISTS) rebuilds it at its next boot.
const (
	RunMemoryLoopSeqIndexName = "idx_run_memory_entries_loop_seq"
	runMemoryLoopSeqDropDDL   = `DROP INDEX ` + RunMemoryLoopSeqIndexName + ` ON run_memory_entries`
)

// discoverRunMemoryLoopSeqIndex reads whether the duplicate exists, whether a
// unique key on exactly (loop_id, seq) covers it, and whether the table has
// rows.
func (s *Store) discoverRunMemoryLoopSeqIndex(ctx context.Context) (RunMemoryLoopSeqIndexStatus, error) {
	status := RunMemoryLoopSeqIndexStatus{Name: RunMemoryLoopSeqIndexName}
	rows, err := s.db.QueryContext(ctx, `SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'run_memory_entries' ORDER BY INDEX_NAME, SEQ_IN_INDEX`)
	if err != nil {
		return status, fmt.Errorf("runtime store: list run_memory_entries indexes: %w", err)
	}
	cols := map[string][]string{}
	unique := map[string]bool{}
	for rows.Next() {
		var name, col string
		var nonUnique int
		if err := rows.Scan(&name, &col, &nonUnique); err != nil {
			_ = rows.Close()
			return status, fmt.Errorf("runtime store: scan run_memory_entries indexes: %w", err)
		}
		cols[name] = append(cols[name], col)
		unique[name] = nonUnique == 0
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return status, fmt.Errorf("runtime store: list run_memory_entries indexes: %w", err)
	}
	_ = rows.Close()
	_, status.Present = cols[RunMemoryLoopSeqIndexName]
	for name, c := range cols {
		if name != RunMemoryLoopSeqIndexName && unique[name] && len(c) == 2 && c[0] == "loop_id" && c[1] == "seq" {
			status.CoveredByUniqueKey = true
		}
	}
	var one int
	switch err := s.db.QueryRowContext(ctx, `SELECT 1 FROM run_memory_entries LIMIT 1`).Scan(&one); {
	case err == nil:
		status.HasRows = true
	case !errors.Is(err, sql.ErrNoRows):
		return status, fmt.Errorf("runtime store: probe run_memory_entries rows: %w", err)
	}
	return status, nil
}

// dropRunMemoryLoopSeqIndexAtOpen drops the duplicate from an empty table and
// only reports it on a table with rows. Bounded by the same deadline as the
// kind index hook; its error is a failed boot phase, never a failed Open.
func (s *Store) dropRunMemoryLoopSeqIndexAtOpen(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, runMemoryKindIndexOpenTimeout)
	defer cancel()
	status, err := s.discoverRunMemoryLoopSeqIndex(ctx)
	if err != nil {
		log.Printf("runtime store: run-memory loop_seq index unknown, left as it is: %v", err)
		return err
	}
	switch {
	case !status.Present:
	case !status.HasRows && status.CoveredByUniqueKey:
		if _, err := s.db.ExecContext(ctx, runMemoryLoopSeqDropDDL); err != nil {
			log.Printf("runtime store: run-memory loop_seq index not dropped from the empty table: %v", err)
			return fmt.Errorf("runtime store: drop run-memory loop_seq index: %w", err)
		}
	case status.HasRows:
		log.Printf("runtime store: run-memory loop_seq index %s is a duplicate of UNIQUE(loop_id, seq) on a table with rows, not dropped at open; it costs about 12 KiB of journal per append until POST /internal/store/derived-columns?action=migrate",
			RunMemoryLoopSeqIndexName)
	}
	return nil
}

// migrateRunMemoryLoopSeqIndex is the operation's step for the duplicate:
// nothing when it is absent, a refusal when no unique key on (loop_id, seq)
// covers it, otherwise one DROP INDEX under the engine mutex.
func (s *Store) migrateRunMemoryLoopSeqIndex(ctx context.Context, number int) (*objectgraph.AccessPathStep, error) {
	status, err := s.discoverRunMemoryLoopSeqIndex(ctx)
	if err != nil {
		return nil, err
	}
	if !status.Present {
		return nil, nil
	}
	step := &objectgraph.AccessPathStep{Index: number, Total: number, Kind: "drop_index", Name: RunMemoryLoopSeqIndexName}
	if !status.CoveredByUniqueKey {
		refused := fmt.Errorf("runtime store: %s is not dropped: no unique key on (loop_id, seq) covers it", RunMemoryLoopSeqIndexName)
		step.Err = refused.Error()
		log.Printf("%v", refused)
		return step, refused
	}
	log.Printf("runtime store: run-memory loop_seq index %s: dropping on a table with rows=%t", RunMemoryLoopSeqIndexName, status.HasRows)
	start := time.Now()
	dropErr := s.ogStore.WithEngine("(*Store).MigrateRunMemoryLoopSeqIndex", func(db *sql.DB) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, runMemoryLoopSeqDropDDL)
		return err
	})
	step.DurationMS = time.Since(start).Milliseconds()
	if dropErr != nil {
		step.Err = dropErr.Error()
		log.Printf("runtime store: run-memory loop_seq index drop failed after %s: %v", time.Since(start).Round(time.Millisecond), dropErr)
		return step, dropErr
	}
	log.Printf("runtime store: run-memory loop_seq index dropped in %s", time.Since(start).Round(time.Millisecond))
	return step, nil
}

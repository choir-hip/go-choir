package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The index diet of run_memory_entries
// (docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md, "The index
// diet"; option 4 of docs/problems/store-journal-write-amplification-2026-10-23.md).
// Every secondary index is one prolly-tree path per append, 9 to 13 KiB of
// journal. A reader audit (run_memory_index_audit_test.go) found two indexes
// that no production statement needs once the kind index exists:
//
//   - idx_run_memory_entries_owner_loop_seq (owner_id, loop_id, seq): every
//     loop-keyed statement is served by UNIQUE(loop_id, seq); only the residue
//     import's OR of (owner_id, loop_id) pairs used it, and that statement now
//     filters loop_id IN (...) first (residue_import.go);
//   - idx_run_memory_entries_owner_agent_created (owner_id, agent_id,
//     created_at): its one reader is ListRunMemoryEntriesForAgent without a kind
//     filter, which has no production caller; the kind index
//     (owner_id, agent_id, kind, created_at) serves the same range and the
//     statement sorts on (created_at, seq) above it either way.
//
// Neither moves the content witness: owner_id stays MUL through the kind
// index and loop_id through UNIQUE(loop_id, seq), which is why the drop is
// refused unless both exist (TestRunMemoryIndexDietKeepsTheWitnessSchemaObservation,
// and a control without the kind index that does move owner_id's Key).
// idx_run_memory_entries_parent has no reader either but is NOT dropped: it is
// the only index on parent_entry_id, so the drop moves that column's Key from
// MUL to empty and with it every schema observation, hence replay eligibility,
// base descriptors and recorded checkpoint witnesses by vintage
// (TestRunMemoryParentIndexDropMovesTheWitnessSchemaObservation).
//
// Mechanism: R5's, extended. schemaDDL no longer creates them (a fresh store is
// made without them); Open drops them only from an empty table; on a table with
// rows Open only reports, and the drop is a step of the explicit
// /internal/store/derived-columns operation, after the kind index and the R5
// drop, under the engine mutex. Rollback: recreate with the DDL below; a
// reverted binary's schemaDDL (CREATE INDEX IF NOT EXISTS) rebuilds them at its
// next boot on the populated table, before its listener.
const (
	RunMemoryOwnerLoopSeqIndexName      = "idx_run_memory_entries_owner_loop_seq"
	RunMemoryOwnerAgentCreatedIndexName = "idx_run_memory_entries_owner_agent_created"
)

// runMemoryDietIndex is one index the diet drops, with the DDL that recreates
// it exactly as the old schemaDDL made it.
type runMemoryDietIndex struct {
	name    string
	columns []string
	ddl     string
}

// The order is the operation's order.
var runMemoryDietIndexes = []runMemoryDietIndex{
	{RunMemoryOwnerLoopSeqIndexName, []string{"owner_id", "loop_id", "seq"},
		`CREATE INDEX ` + RunMemoryOwnerLoopSeqIndexName + ` ON run_memory_entries(owner_id, loop_id, seq)`},
	{RunMemoryOwnerAgentCreatedIndexName, []string{"owner_id", "agent_id", "created_at"},
		`CREATE INDEX ` + RunMemoryOwnerAgentCreatedIndexName + ` ON run_memory_entries(owner_id, agent_id, created_at)`},
}

// runMemoryKindIndexColumns is what the kind index must be for the diet to
// rely on it (the first-pass agent-leading order would not carry owner_id).
var runMemoryKindIndexColumns = []string{"owner_id", "agent_id", "kind", "created_at"}

func (d runMemoryDietIndex) dropDDL() string {
	return `DROP INDEX ` + d.name + ` ON run_memory_entries`
}

// runMemoryIndexInventory is the table's index set and whether it has rows.
type runMemoryIndexInventory struct {
	columns map[string][]string
	unique  map[string]bool
	hasRows bool
}

func (s *Store) inventoryRunMemoryIndexes(ctx context.Context) (runMemoryIndexInventory, error) {
	inv := runMemoryIndexInventory{columns: map[string][]string{}, unique: map[string]bool{}}
	rows, err := s.db.QueryContext(ctx, `SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'run_memory_entries' ORDER BY INDEX_NAME, SEQ_IN_INDEX`)
	if err != nil {
		return inv, fmt.Errorf("runtime store: list run_memory_entries indexes: %w", err)
	}
	for rows.Next() {
		var name, col string
		var nonUnique int
		if err := rows.Scan(&name, &col, &nonUnique); err != nil {
			_ = rows.Close()
			return inv, fmt.Errorf("runtime store: scan run_memory_entries indexes: %w", err)
		}
		inv.columns[name] = append(inv.columns[name], col)
		inv.unique[name] = nonUnique == 0
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return inv, fmt.Errorf("runtime store: list run_memory_entries indexes: %w", err)
	}
	_ = rows.Close()
	var one int
	switch err := s.db.QueryRowContext(ctx, `SELECT 1 FROM run_memory_entries LIMIT 1`).Scan(&one); {
	case err == nil:
		inv.hasRows = true
	case !errors.Is(err, sql.ErrNoRows):
		return inv, fmt.Errorf("runtime store: probe run_memory_entries rows: %w", err)
	}
	return inv, nil
}

// uniqueLoopSeq reports whether a unique key on exactly (loop_id, seq) exists
// other than the named index.
func (inv runMemoryIndexInventory) uniqueLoopSeq(except string) bool {
	for name, c := range inv.columns {
		if name != except && inv.unique[name] && reflect.DeepEqual(c, []string{"loop_id", "seq"}) {
			return true
		}
	}
	return false
}

// dietGuard says why the diet's indexes may not be dropped, or "" when they
// may: the statements that filter on loop_id are served by the unique key, and
// the kind index carries owner_id's Key and the agent read.
func (inv runMemoryIndexInventory) dietGuard() string {
	if !inv.uniqueLoopSeq("") {
		return "no unique key on (loop_id, seq) covers the loop-keyed statements"
	}
	if !reflect.DeepEqual(inv.columns[RunMemoryKindIndexName], runMemoryKindIndexColumns) {
		return fmt.Sprintf("the kind index %s with columns %v is required (to carry owner_id's SHOW COLUMNS Key and the agent read), found %v",
			RunMemoryKindIndexName, runMemoryKindIndexColumns, inv.columns[RunMemoryKindIndexName])
	}
	return ""
}

func (s *Store) discoverRunMemoryDietIndexes(ctx context.Context) ([]RunMemoryDietIndexStatus, error) {
	inv, err := s.inventoryRunMemoryIndexes(ctx)
	if err != nil {
		return nil, err
	}
	reason := inv.dietGuard()
	out := make([]RunMemoryDietIndexStatus, 0, len(runMemoryDietIndexes))
	for _, d := range runMemoryDietIndexes {
		_, present := inv.columns[d.name]
		out = append(out, RunMemoryDietIndexStatus{Name: d.name, Present: present, HasRows: inv.hasRows, GuardOK: reason == "", GuardReason: reason})
	}
	return out, nil
}

// dropRunMemoryDietIndexesAtOpen drops the diet's indexes from an empty table
// and only reports them on a table with rows, as the kind index hook and R5's
// hook do. Its error is a failed boot phase, never a failed Open.
func (s *Store) dropRunMemoryDietIndexesAtOpen(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, runMemoryKindIndexOpenTimeout)
	defer cancel()
	inv, err := s.inventoryRunMemoryIndexes(ctx)
	if err != nil {
		log.Printf("runtime store: run-memory diet indexes unknown, left as they are: %v", err)
		return err
	}
	reason := inv.dietGuard()
	for _, d := range runMemoryDietIndexes {
		if _, present := inv.columns[d.name]; !present {
			continue
		}
		switch {
		case inv.hasRows:
			log.Printf("runtime store: run-memory index %s is unread on a table with rows, not dropped at open; it costs 9 to 10 KiB of journal per append until POST /internal/store/derived-columns?action=migrate", d.name)
		case reason != "":
			log.Printf("runtime store: run-memory index %s not dropped from the empty table: %s", d.name, reason)
		default:
			if _, err := s.db.ExecContext(ctx, d.dropDDL()); err != nil {
				log.Printf("runtime store: run-memory index %s not dropped from the empty table: %v", d.name, err)
				return fmt.Errorf("runtime store: drop run-memory index %s: %w", d.name, err)
			}
		}
	}
	return nil
}

// migrateRunMemoryDietIndex is the operation's step for one index: nothing
// when it is absent, a refusal when the guard does not hold, otherwise one
// DROP INDEX under the engine mutex.
func (s *Store) migrateRunMemoryDietIndex(ctx context.Context, number int, d runMemoryDietIndex) (*objectgraph.AccessPathStep, error) {
	inv, err := s.inventoryRunMemoryIndexes(ctx)
	if err != nil {
		return nil, err
	}
	if _, present := inv.columns[d.name]; !present {
		return nil, nil
	}
	step := &objectgraph.AccessPathStep{Index: number, Total: number, Kind: "drop_index", Name: d.name}
	if reason := inv.dietGuard(); reason != "" {
		refused := fmt.Errorf("runtime store: %s is not dropped: %s", d.name, reason)
		step.Err = refused.Error()
		log.Printf("%v", refused)
		return step, refused
	}
	log.Printf("runtime store: run-memory index %s: dropping on a table with rows=%t", d.name, inv.hasRows)
	start := time.Now()
	dropErr := s.ogStore.WithEngine("(*Store).MigrateRunMemoryDietIndex", func(db *sql.DB) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, d.dropDDL())
		return err
	})
	step.DurationMS = time.Since(start).Milliseconds()
	if dropErr != nil {
		step.Err = dropErr.Error()
		log.Printf("runtime store: run-memory index %s drop failed after %s: %v", d.name, time.Since(start).Round(time.Millisecond), dropErr)
		return step, dropErr
	}
	log.Printf("runtime store: run-memory index %s dropped in %s", d.name, time.Since(start).Round(time.Millisecond))
	return step, nil
}

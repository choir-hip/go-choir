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
	// The coupling: with neither the kind index nor owner_agent_created there
	// is no (owner_id, agent_id) index at all. The agent read full-scans the
	// table and owner_id has no leading index, so the content witness differs
	// from a fresh store's. A store gets here by a manual drop of the kind
	// index after the diet, or a kind index that was never built after
	// owner_agent_created was dropped.
	_, hasAgentCreated := inv.columns[RunMemoryOwnerAgentCreatedIndexName]
	if !hasAgentCreated && !reflect.DeepEqual(inv.columns[RunMemoryKindIndexName], runMemoryKindIndexColumns) {
		log.Printf("runtime store: run-memory: no (owner_id, agent_id) index exists (neither %s nor %s): ListRunMemoryEntriesForAgent full-scans the table and owner_id has no leading index, so the content witness differs from a fresh store's; restore with POST /internal/store/derived-columns?action=restore_run_memory_indexes or build the kind index with action=migrate",
			RunMemoryKindIndexName, RunMemoryOwnerAgentCreatedIndexName)
		return fmt.Errorf("runtime store: no (owner_id, agent_id) run-memory index")
	}
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

// Restore: the rollback of R5 and the diet.
//
// A revert of the binary would rebuild the dropped indexes at its next boot,
// on the populated table, before its listener (its schemaDDL is CREATE INDEX IF
// NOT EXISTS; about 1.7 s each at 425,000 rows locally, tens of seconds on the
// paper). RestoreRunMemoryIndexes moves that build off the boot path: the
// operator runs it first, online, with the listener up, and the old binary's
// DDL is then a no-op. It is idempotent (an index that exists is "present"),
// takes the engine mutex per index like the drops do, and reports each build's
// hold. It never runs beside a migration.
//
// Order: owner_agent_created first. After the diet the kind index is the only
// index that leads with owner_id; owner_agent_created is the second, so owner_id's
// SHOW COLUMNS Key never depends on a half-restored set, and a kind-index
// rollback (DROP INDEX) that follows is witness-neutral again.
const (
	RunMemoryIndexRestoreCreated = "created"
	RunMemoryIndexRestorePresent = "present"
)

// RunMemoryIndexRestoreStep is one index of a restore.
type RunMemoryIndexRestoreStep struct {
	Name   string `json:"name"`
	Action string `json:"action"`
	// HoldMS is the time the write connection was held by the build; 0 for a
	// present index.
	HoldMS int64  `json:"hold_ms"`
	Err    string `json:"error,omitempty"`
}

// RunMemoryIndexRestoreReport is the result of RestoreRunMemoryIndexes.
type RunMemoryIndexRestoreReport struct {
	Steps       []RunMemoryIndexRestoreStep `json:"steps"`
	TotalHoldMS int64                       `json:"total_hold_ms"`
	Error       string                      `json:"error,omitempty"`
}

// ErrUnknownRunMemoryIndex refuses a restore naming an index it does not know.
var ErrUnknownRunMemoryIndex = errors.New("runtime store: unknown run-memory index")

// runMemoryRestoreOrder is the restore's order.
func runMemoryRestoreOrder() []runMemoryDietIndex {
	byName := map[string]runMemoryDietIndex{}
	for _, d := range runMemoryDietIndexes {
		byName[d.name] = d
	}
	return []runMemoryDietIndex{
		byName[RunMemoryOwnerAgentCreatedIndexName],
		byName[RunMemoryOwnerLoopSeqIndexName],
		{RunMemoryLoopSeqIndexName, []string{"loop_id", "seq"}, runMemoryLoopSeqCreateDDL},
	}
}

// RestoreRunMemoryIndexes recreates the indexes R5 and the diet dropped. names
// limits it to those indexes (nil is all three).
func (s *Store) RestoreRunMemoryIndexes(ctx context.Context, names []string) (RunMemoryIndexRestoreReport, error) {
	report := RunMemoryIndexRestoreReport{Steps: []RunMemoryIndexRestoreStep{}}
	if s == nil || s.ogStore == nil {
		return report, fmt.Errorf("runtime store: object graph not initialized")
	}
	order := runMemoryRestoreOrder()
	selected := map[string]bool{}
	for _, n := range names {
		known := false
		for _, d := range order {
			known = known || d.name == n
		}
		if !known {
			return report, fmt.Errorf("%w %q", ErrUnknownRunMemoryIndex, n)
		}
		selected[n] = true
	}
	m := &s.derivedColumns
	m.mu.Lock()
	if m.status.State == DerivedColumnMigrationRunning || m.restoring {
		m.mu.Unlock()
		return report, ErrDerivedColumnMigrationRunning
	}
	m.restoring = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.restoring = false
		m.mu.Unlock()
	}()

	for _, d := range order {
		if len(selected) > 0 && !selected[d.name] {
			continue
		}
		step := RunMemoryIndexRestoreStep{Name: d.name, Action: RunMemoryIndexRestorePresent}
		fail := func(err error) (RunMemoryIndexRestoreReport, error) {
			step.Err = err.Error()
			report.Steps = append(report.Steps, step)
			report.Error = err.Error()
			return report, err
		}
		if err := ctx.Err(); err != nil {
			report.Error = err.Error()
			return report, err
		}
		inv, err := s.inventoryRunMemoryIndexes(ctx)
		if err != nil {
			return fail(err)
		}
		if _, present := inv.columns[d.name]; present {
			report.Steps = append(report.Steps, step)
			continue
		}
		log.Printf("runtime store: run-memory index %s: restoring on a table with rows=%t", d.name, inv.hasRows)
		start := time.Now()
		buildErr := s.ogStore.WithEngine("(*Store).RestoreRunMemoryIndexes", func(db *sql.DB) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			_, err := db.ExecContext(ctx, d.ddl)
			return err
		})
		step.HoldMS = time.Since(start).Milliseconds()
		if buildErr != nil {
			log.Printf("runtime store: run-memory index %s restore failed after %s: %v", d.name, time.Since(start).Round(time.Millisecond), buildErr)
			return fail(buildErr)
		}
		step.Action = RunMemoryIndexRestoreCreated
		report.TotalHoldMS += step.HoldMS
		report.Steps = append(report.Steps, step)
		log.Printf("runtime store: run-memory index %s restored in %s", d.name, time.Since(start).Round(time.Millisecond))
	}
	return report, nil
}

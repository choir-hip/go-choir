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

// The kind index of run_memory_entries
// (docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md, shape 3).
// ListRunMemoryEntriesForAgent filters kind above the (owner_id, agent_id,
// created_at) index, so a read of the management desk's 14,965 cursor rows
// walked the desk's whole range and held the write connection 16.6 s before
// its first row. With kind in the index the read is a range on the cursor rows.
//
// It follows the repo's rule for an access path on a table with rows
// (boot-alters-og-objects-when-older-columns-missing-2026-10-21.md): Open
// creates it on an empty table, where it costs nothing, and only discovers it
// on a table with rows; building it there is a step of the explicit
// /internal/store/derived-columns operation, under the engine mutex, at a time
// the operator chooses. It is deliberately not in schemaDDL, which every boot
// applies. It is an additive secondary index: no epoch bump, an older binary
// ignores it (its own CREATE INDEX IF NOT EXISTS lines are untouched), and
// DROP INDEX is the whole rollback.
//
// Column order. The first pass chose (agent_id, kind, owner_id, created_at),
// which also served the owner-empty form. bunch 160 (DeepSeek, HIGH) found it
// changes the content witness: computerversion's queryTableSchemaHash is SHOW
// COLUMNS, whose Key reads MUL for the first column of a non-unique index, so
// agent_id turned MUL, a live store that built the index differed from a fresh
// one (which makes it at open), replay eligibility recorded SchemaDrift and
// checkpoint restore refused. owner_id is already MUL through the existing
// (owner_id, agent_id, created_at) index, so the owner-leading order leaves
// SHOW COLUMNS byte-identical (TestRunMemoryKindIndexKeepsTheWitnessSchemaObservation,
// with an agent-leading control that does drift). It serves what production
// sends: LoadInboxCursorForDesk always passes an owner and one kind. The
// owner-empty form is no production caller and is not served (it stays on the
// table scan it always had).
const (
	RunMemoryKindIndexName = "idx_run_memory_entries_owner_agent_kind_created"
	runMemoryKindIndexDDL  = `CREATE INDEX ` + RunMemoryKindIndexName + ` ON run_memory_entries(owner_id, agent_id, kind, created_at)`
)

// discoverRunMemoryKindIndex reads whether the index exists and whether the
// table has rows. It reads information_schema and one row.
func (s *Store) discoverRunMemoryKindIndex(ctx context.Context) (RunMemoryKindIndexStatus, error) {
	status := RunMemoryKindIndexStatus{Name: RunMemoryKindIndexName}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'run_memory_entries' AND INDEX_NAME = ?`, RunMemoryKindIndexName).Scan(&n); err != nil {
		return status, fmt.Errorf("runtime store: list run_memory_entries indexes: %w", err)
	}
	status.Present = n > 0
	var one int
	switch err := s.db.QueryRowContext(ctx, `SELECT 1 FROM run_memory_entries LIMIT 1`).Scan(&one); {
	case err == nil:
		status.HasRows = true
	case !errors.Is(err, sql.ErrNoRows):
		return status, fmt.Errorf("runtime store: probe run_memory_entries rows: %w", err)
	}
	return status, nil
}

// runMemoryKindIndexOpenTimeout bounds the open hook's two queries.
const runMemoryKindIndexOpenTimeout = 5 * time.Second

// ensureRunMemoryKindIndexAtOpen creates the index on an empty table and only
// reports it missing on a table with rows. It is bounded by a deadline and
// returns its error; Open logs it as a failed boot phase and goes on: the read
// just stays on the slower plan.
func (s *Store) ensureRunMemoryKindIndexAtOpen(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, runMemoryKindIndexOpenTimeout)
	defer cancel()
	status, err := s.discoverRunMemoryKindIndex(ctx)
	if err != nil {
		log.Printf("runtime store: run-memory kind index unknown, the slower plan is kept: %v", err)
		return err
	}
	switch {
	case status.Present:
	case !status.HasRows:
		if _, err := s.db.ExecContext(ctx, runMemoryKindIndexDDL); err != nil {
			log.Printf("runtime store: run-memory kind index not created on the empty table: %v", err)
			return fmt.Errorf("runtime store: create run-memory kind index: %w", err)
		}
	default:
		log.Printf("runtime store: run-memory kind index %s missing on a table with rows, not added at open; ListRunMemoryEntriesForAgent stays on the (owner_id, agent_id, created_at) plan until POST /internal/store/derived-columns?action=migrate",
			RunMemoryKindIndexName)
	}
	return nil
}

// migrateRunMemoryKindIndex is the operation's step for the index: nothing
// when it is present, otherwise one CREATE INDEX under the engine mutex (the
// write connection is held for the build). first is the 1-based number the
// step takes in the operation's step list.
func (s *Store) migrateRunMemoryKindIndex(ctx context.Context, number int) (*objectgraph.AccessPathStep, error) {
	status, err := s.discoverRunMemoryKindIndex(ctx)
	if err != nil {
		return nil, err
	}
	if status.Present {
		return nil, nil
	}
	log.Printf("runtime store: run-memory kind index %s: building on a table with rows=%t", RunMemoryKindIndexName, status.HasRows)
	start := time.Now()
	buildErr := s.ogStore.WithEngine("(*Store).MigrateRunMemoryKindIndex", func(db *sql.DB) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, runMemoryKindIndexDDL)
		return err
	})
	step := &objectgraph.AccessPathStep{Index: number, Total: number, Kind: "index", Name: RunMemoryKindIndexName, DurationMS: time.Since(start).Milliseconds()}
	if buildErr != nil {
		step.Err = buildErr.Error()
		log.Printf("runtime store: run-memory kind index build failed after %s, the slower plan is kept: %v", time.Since(start).Round(time.Millisecond), buildErr)
		return step, buildErr
	}
	log.Printf("runtime store: run-memory kind index built in %s", time.Since(start).Round(time.Millisecond))
	return step, nil
}

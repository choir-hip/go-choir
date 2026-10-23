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
// Column order, from a plan test: agent_id and kind are the equalities every
// shape has; owner_id follows kind so the owner-empty form is served by the
// same index (an owner_id-leading order left it on a table scan).
const (
	RunMemoryKindIndexName = "idx_run_memory_entries_agent_kind_owner_created"
	runMemoryKindIndexDDL  = `CREATE INDEX ` + RunMemoryKindIndexName + ` ON run_memory_entries(agent_id, kind, owner_id, created_at)`
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

// ensureRunMemoryKindIndexAtOpen creates the index on an empty table and only
// reports it missing on a table with rows. A failure is logged and never
// fails the open: the read just stays on the slower plan.
func (s *Store) ensureRunMemoryKindIndexAtOpen(ctx context.Context) error {
	_ = ctx
	ctx = context.Background()
	status, err := s.discoverRunMemoryKindIndex(ctx)
	if err != nil {
		log.Printf("runtime store: run-memory kind index unknown, the slower plan is kept: %v", err)
		return nil
	}
	switch {
	case status.Present:
	case !status.HasRows:
		if _, err := s.db.ExecContext(ctx, runMemoryKindIndexDDL); err != nil {
			log.Printf("runtime store: run-memory kind index not created on the empty table: %v", err)
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

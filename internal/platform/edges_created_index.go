package platform

// The index of the public entity read (entity_page.go): an entity's live
// mention edges newest first. Without it the read finds the entity's edges
// by idx_og_edges_to_kind and sorts all of them for every miss; with it the
// newest 200 are the first 200 index entries. The same shape serves the ready
// list's weight probe (a capped count of an entity's publication mentions).
//
// It is partial (live edges only, as every one of these reads is) and is
// built the way the term table's covering index is: CONCURRENTLY, outside a
// transaction, under a session advisory lock, from MigrateRetrieval and the
// periodic step, never on a request. A plain CREATE INDEX would block capture
// and story writes to og_edges for the build; the table is about 1 GB with its
// indexes on the paper (node-b-storage-review-2026-11-12), so the build is
// tens of seconds, not minutes, and CONCURRENTLY costs only a second pass.
// Until it is valid the planner uses idx_og_edges_to_kind and the read is
// correct, only slower. An interrupted build leaves an invalid index, which
// the next call drops and rebuilds. Rollback: DROP INDEX
// idx_og_edges_to_kind_created.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log"
	"time"
)

const (
	idxEdgesToKindCreated = "idx_og_edges_to_kind_created"
	// edgesToKindCreatedLockKey serializes the build between processes.
	edgesToKindCreatedLockKey = 7330212470813
	edgesToKindCreatedDDL     = `CREATE INDEX CONCURRENTLY ` + idxEdgesToKindCreated + ` ON og_edges (to_id, kind, created_at DESC, edge_id) WHERE NOT tombstone`
)

// ensureEdgesToKindCreatedIndex builds the index if it is missing or invalid.
func (s *Store) ensureEdgesToKindCreatedIndex(ctx context.Context) error {
	conn, err := s.corpus().Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var got bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(?)`, edgesToKindCreatedLockKey).Scan(&got); err != nil {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return err
	}
	if !got {
		return nil // another process is building it
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(?)`, edgesToKindCreatedLockKey)
	}()
	var valid sql.NullBool
	err = conn.QueryRowContext(ctx, `SELECT i.indisvalid FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = ? AND i.indrelid = 'og_edges'::regclass`, idxEdgesToKindCreated).Scan(&valid)
	switch {
	case err == nil && valid.Bool:
		return nil
	case err == nil:
		log.Printf("corpusd retrieval migrate index=%s was left invalid by an interrupted build; rebuilding", idxEdgesToKindCreated)
		if _, err := conn.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS `+idxEdgesToKindCreated); err != nil {
			return fmt.Errorf("drop invalid %s: %w", idxEdgesToKindCreated, err)
		}
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	start := time.Now()
	if _, err := conn.ExecContext(ctx, edgesToKindCreatedDDL); err != nil {
		return fmt.Errorf("build %s: %w", idxEdgesToKindCreated, err)
	}
	log.Printf("corpusd retrieval migrate index=%s table=og_edges ms=%d concurrently", idxEdgesToKindCreated, time.Since(start).Milliseconds())
	return nil
}

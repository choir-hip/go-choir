package platform

// The index of the public entity read (entity_page.go): an entity's live
// mention edges newest first. Without it the read finds the entity's edges
// by idx_og_edges_to_kind and sorts all of them for every miss; with it the
// newest 200 are the first 200 index entries. The same shape serves the ready
// list's weight probe (a capped count of an entity's publication mentions).
//
// It is partial (live edges only, as every one of these reads is) and is
// built the way the term table's covering index is: CONCURRENTLY, outside a
// transaction, under a session advisory lock (waited for), from a goroutine of its own at start
// (StartRetrievalMigrations) and from the periodic step, never on a request
// and never in MigrateRetrieval, so a build that waits cannot keep the
// periodic loop from starting. A plain CREATE INDEX would block capture
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

// edgesIndexBuildTimeout bounds one build attempt (a variable so a test can
// move it). CONCURRENTLY waits for every transaction older than the build; a
// stuck one would otherwise hold the connection, and whatever waits on this
// call, for good. A build that times out leaves an invalid index, which the
// next attempt (the periodic retrieval step) detects, drops and rebuilds.
var edgesIndexBuildTimeout = 20 * time.Minute

// ensureEdgesToKindCreatedIndex builds the index if it is missing or invalid.
// It has its own deadline and is never on the path that starts the periodic
// retrieval loop: StartRetrievalMigrations runs it in a goroutine of its own
// and retrievalStep retries it.
func (s *Store) ensureEdgesToKindCreatedIndex(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, edgesIndexBuildTimeout)
	defer cancel()
	conn, err := s.corpus().Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	// The lock is waited for, not tried: a holder is a build in another
	// process or the session of an earlier attempt that timed out and is still
	// on the server (its cancel is in flight). A try that missed the lock and
	// returned nil left the invalid index in place for a whole period. After
	// the wait the state is read again, so a holder that finished its build
	// leaves nothing to do. The wait ends at this call's own deadline.
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(?)`, edgesToKindCreatedLockKey); err != nil {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return err
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(?)`, edgesToKindCreatedLockKey)
	}()
	// A conflicting lock is waited for a minute, not for good. The setting is
	// the session's, so it is reset before the connection goes back to the pool.
	if _, err := conn.ExecContext(ctx, `SET lock_timeout = '60s'`); err != nil {
		return err
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), `RESET lock_timeout`) }()
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

// kickEdgesToKindCreatedIndex starts a build in the background unless one is
// running; it returns at once. The start and every periodic tick call it, so
// a build that timed out is retried and nothing waits for it.
func (s *Store) kickEdgesToKindCreatedIndex(ctx context.Context) {
	if s == nil || !s.retrievalState.edgesIndexBuilding.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer s.retrievalState.edgesIndexBuilding.Store(false)
		if err := s.ensureEdgesToKindCreatedIndex(ctx); err != nil {
			log.Printf("corpusd retrieval edges index err=%v (retried by the periodic step)", err)
		}
	}()
}

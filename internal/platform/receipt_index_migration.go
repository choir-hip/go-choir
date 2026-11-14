package platform

// The swap of the receipt table's indexes (docs/design/store-once-2026-11-14.md
// R4; the queries it serves are in receipt_indexes.go): build the partial kind
// index, then drop the full kind index and the receipt id index, each by
// itself, concurrently, beside the appends of a 30 GB table. This is the
// second of two releases; the first stopped Bootstrap creating the old indexes
// and made every reader work with either, so a revert of this one finds a
// binary that neither needs them nor rebuilds them.
//
// Ways it could fail, written before the code (tests name these by number):
//
//  M1. the build or a drop blocks appends: every statement is on one
//      connection, outside any transaction, CREATE/DROP INDEX CONCURRENTLY,
//      which hold SHARE UPDATE EXCLUSIVE on the table (appends go on; vacuum
//      waits);
//  M2. an interrupted build leaves an INVALID index that IF NOT EXISTS treats as
//      present: validity is read from pg_index; an invalid one is dropped and
//      rebuilt, and the old indexes are dropped only after the new one is read
//      back valid;
//  M3. two corpusd processes build the same index, or the one holding the lock
//      dies and nobody finishes: a session advisory lock; a process that does
//      not get it retries with backoff, so a dead holder's work is finished by
//      the next;
//  M4. the build waits forever behind a long transaction, holding the lock: a
//      statement_timeout sized for the build (receiptIndexStatementTimeout) and a
//      lock_timeout; either error ends the attempt, which the loop retries and
//      logs;
//  M5. the index of another schema is read or dropped: the table's own
//      namespace is resolved and every index name is schema-qualified;
//  M6. the build slows the busiest computer's appends: owner's ruling N3, one
//      index at a time, stop if the busiest computer's append times rise.
//      Between phases (before the build, after the partial index is valid,
//      before each drop) the migration compares the busiest computer's recent
//      append p95 with the one it read before it began (receipt_index_health.go)
//      and waits while it has risen. That wait is a correctness wait, not a
//      switch: it ends by itself when the appends recover, never by a setting.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	// receiptIndexLockKey is the session advisory lock that serializes the
	// migration across corpusd processes.
	receiptIndexLockKey = 7330212470801
)

var (
	// receiptIndexStatementTimeout bounds one statement. The build reads the
	// 18 GB heap twice; minutes at the disk's speed, so three hours is a wait
	// behind a stuck transaction, not a build.
	receiptIndexStatementTimeout = 3 * time.Hour
	receiptIndexLockTimeout      = time.Minute
	// receiptIndexPoll is how often a health wait looks again.
	receiptIndexPoll = 30 * time.Second
	// receiptIndexBackoff is the first retry delay of the loop; it doubles to
	// receiptIndexBackoffMax.
	receiptIndexBackoff    = 30 * time.Second
	receiptIndexBackoffMax = 10 * time.Minute
)

// StartReceiptIndexMigration runs the swap beside the listener, retrying with
// backoff until it is done (including when another process holds the lock). It
// returns at once.
func (s *Store) StartReceiptIndexMigration(ctx context.Context) {
	if s == nil {
		return
	}
	go func() {
		backoff := receiptIndexBackoff
		for {
			done, err := s.migrateReceiptIndexes(ctx)
			if done && err == nil {
				return
			}
			if err != nil {
				log.Printf("corpusd receipt indexes: attempt failed, retry in %s: %v", backoff, err)
			} else {
				log.Printf("corpusd receipt indexes: another process holds the migration, retry in %s", backoff)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < receiptIndexBackoffMax {
				backoff = min(backoff*2, receiptIndexBackoffMax)
			}
		}
	}()
}

// migrateReceiptIndexes is one attempt. done is false when another process
// holds the lock (nothing was done); an error ends the attempt and is retried.
func (s *Store) migrateReceiptIndexes(ctx context.Context) (done bool, err error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("platform store: nil database")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(?)`, receiptIndexLockKey).Scan(&locked); err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	defer func() {
		// The context may be done; the connection is closed (and the lock with
		// it) either way.
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `RESET statement_timeout`)
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `RESET lock_timeout`)
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(?)`, receiptIndexLockKey)
	}()
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`SET statement_timeout = %d`, receiptIndexStatementTimeout.Milliseconds())); err != nil {
		return false, err
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`SET lock_timeout = %d`, receiptIndexLockTimeout.Milliseconds())); err != nil {
		return false, err
	}
	schema, err := receiptTableSchema(ctx, conn)
	if err != nil {
		return false, err
	}
	health := newAppendHealth(&s.appendTimes, time.Now())
	m := receiptIndexRun{conn: conn, schema: schema, health: health, now: time.Now}
	return true, m.run(ctx)
}

// receiptTableSchema is the namespace the receipt table lives in, resolved as
// the store's own queries resolve it.
func receiptTableSchema(ctx context.Context, conn *sql.Conn) (string, error) {
	var schema string
	err := conn.QueryRowContext(ctx, `SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE c.oid = 'computer_event_append_receipts'::regclass`).Scan(&schema)
	return schema, err
}

type receiptIndexRun struct {
	conn   *sql.Conn
	schema string
	health appendHealth
	now    func() time.Time
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func (m receiptIndexRun) qualified(index string) string {
	return quoteIdent(m.schema) + "." + quoteIdent(index)
}

func (m receiptIndexRun) run(ctx context.Context) error {
	if err := m.waitHealthy(ctx, "before the build"); err != nil {
		return err
	}
	if err := m.ensurePartial(ctx); err != nil {
		return err
	}
	for _, old := range []string{receiptKindOldIdx, receiptIDOldIdx} {
		if err := m.waitHealthy(ctx, "before dropping "+old); err != nil {
			return err
		}
		if err := m.drop(ctx, old); err != nil {
			return err
		}
	}
	log.Printf("corpusd receipt indexes: done")
	return nil
}

// waitHealthy returns when the busiest computer's append times have not risen
// over the baseline (N3); until then it logs and looks again.
func (m receiptIndexRun) waitHealthy(ctx context.Context, phase string) error {
	for first := true; ; first = false {
		bad, load := m.health.degraded(m.now())
		if !bad {
			return nil
		}
		if first {
			log.Printf("corpusd receipt indexes: waiting %s: appends of %s are slow: p95 %s over %d appends, baseline %s",
				phase, load.Computer, load.P95, load.Count, m.health.baseline)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(receiptIndexPoll):
		}
	}
}

// indexState reports whether the named index of the table's schema exists and
// is valid.
func (m receiptIndexRun) indexState(ctx context.Context, name string) (exists, valid bool, err error) {
	err = m.conn.QueryRowContext(ctx, `SELECT COALESCE(bool_or(true), false), COALESCE(bool_and(i.indisvalid AND i.indisready), false)
		FROM pg_class c JOIN pg_index i ON i.indexrelid = c.oid JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = ? AND n.nspname = ?`, name, m.schema).Scan(&exists, &valid)
	return
}

// ensurePartial builds the partial kind index unless a valid one is there. An
// invalid leftover of an interrupted build is dropped first.
func (m receiptIndexRun) ensurePartial(ctx context.Context) error {
	exists, valid, err := m.indexState(ctx, receiptKindPartialIdx)
	if err != nil {
		return err
	}
	if exists && valid {
		log.Printf("corpusd receipt indexes: %s is valid", receiptKindPartialIdx)
		return nil
	}
	if exists {
		log.Printf("corpusd receipt indexes: %s is invalid (an interrupted build); dropping it", receiptKindPartialIdx)
		if _, err := m.conn.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS `+m.qualified(receiptKindPartialIdx)); err != nil {
			return fmt.Errorf("drop invalid %s: %w", receiptKindPartialIdx, err)
		}
	}
	log.Printf("corpusd receipt indexes: building %s", receiptKindPartialIdx)
	start := time.Now()
	if _, err := m.conn.ExecContext(ctx, `CREATE INDEX CONCURRENTLY `+quoteIdent(receiptKindPartialIdx)+` ON `+quoteIdent(m.schema)+`.computer_event_append_receipts (computer_id, event_kind, sequence) WHERE `+receiptKindNotBatchSQL); err != nil {
		return fmt.Errorf("create %s: %w", receiptKindPartialIdx, err)
	}
	if _, valid, err := m.indexState(ctx, receiptKindPartialIdx); err != nil || !valid {
		return fmt.Errorf("create %s: the index is not valid after the build (err=%v)", receiptKindPartialIdx, err)
	}
	log.Printf("corpusd receipt indexes: %s is valid after %s", receiptKindPartialIdx, time.Since(start).Round(time.Second))
	return nil
}

// drop removes one old index concurrently, only when the partial one is valid.
func (m receiptIndexRun) drop(ctx context.Context, name string) error {
	if _, valid, err := m.indexState(ctx, receiptKindPartialIdx); err != nil || !valid {
		return fmt.Errorf("not dropping %s: %s is not valid (err=%v)", name, receiptKindPartialIdx, err)
	}
	exists, _, err := m.indexState(ctx, name)
	if err != nil || !exists {
		return err
	}
	log.Printf("corpusd receipt indexes: dropping %s", name)
	start := time.Now()
	if _, err := m.conn.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS `+m.qualified(name)); err != nil {
		return fmt.Errorf("drop %s: %w", name, err)
	}
	log.Printf("corpusd receipt indexes: dropped %s in %s", name, time.Since(start).Round(time.Millisecond))
	return nil
}

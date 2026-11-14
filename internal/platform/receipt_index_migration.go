package platform

// The swap of the receipt table's indexes (docs/design/store-once-2026-11-14.md
// R4; the queries it serves are in receipt_indexes.go): build the partial kind
// index, then drop the full kind index and the receipt id index, each by
// itself, concurrently, beside the appends of a 30 GB table. The release before
// this one stopped Bootstrap creating the old indexes and made every reader
// work with either, so a revert of this one finds a binary that neither needs
// them nor rebuilds them.
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
//  M6. the swap slows the busiest computer's appends: owner's ruling N3, one
//      index at a time, stop if the busiest computer's append times rise. There
//      is no build until a baseline of that computer's append times exists
//      (awaitAppendBaseline; on the first boot the ring is empty until the
//      listener has served appends); the wait is repeated before the build,
//      after the partial index is valid and before each drop; and the build
//      itself, one statement of minutes, is watched on a second connection and
//      its backend is cancelled when the slowdown holds for two polls. A
//      cancelled build leaves an invalid index, which the next attempt drops and
//      rebuilds after the wait. These waits are correctness waits, not switches:
//      they end by themselves when the appends recover, never by a setting. A
//      host that stays quiet for the whole baseline window has nothing to slow.
//  M7. the backfill walk rewrites millions of rows on the same table while the
//      indexes are built: the walk does not start a pass while this migration is
//      running (receipt_walk.go deferWalk).
//
// A restart during the build cancels it (the connection closes): the invalid
// index is dropped and the two heap scans start over at the next boot, so a
// deploy loop during the build repeats the whole cost each time.

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
	// 18 GB heap twice, minutes at the disk's speed, so three hours is a wait
	// behind a stuck transaction, not a build.
	receiptIndexStatementTimeout = 3 * time.Hour
	receiptIndexLockTimeout      = time.Minute
	// receiptIndexBaselineWindow is how far back the baseline reads, and how
	// long a serving host may show fewer than receiptIndexMinAppends before it
	// counts as quiet.
	receiptIndexBaselineWindow = 10 * time.Minute
	// receiptIndexPoll is how often a wait or the build's watch looks again.
	receiptIndexPoll = 30 * time.Second
	// receiptIndexLogEvery is the steady cadence of the log line of a wait that
	// does not change.
	receiptIndexLogEvery = 5 * time.Minute
	// receiptIndexBackoff is the first retry delay of the loop; it doubles to
	// receiptIndexBackoffMax.
	receiptIndexBackoff    = 30 * time.Second
	receiptIndexBackoffMax = 10 * time.Minute
)

// StartReceiptIndexMigration runs the swap beside the listener, retrying with
// backoff until it is done (including when another process holds the lock). It
// returns at once; corpusd calls it as the listener starts, so the first
// appends it serves are the baseline's.
func (s *Store) StartReceiptIndexMigration(ctx context.Context) {
	if s == nil {
		return
	}
	s.receiptIndexState.Store(receiptIndexRunning)
	s.appendTimes.watch(time.Now())
	go func() {
		backoff := receiptIndexBackoff
		for {
			done, err := s.migrateReceiptIndexes(ctx)
			if done && err == nil {
				s.receiptIndexState.Store(receiptIndexDone)
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
	health, err := awaitAppendBaseline(ctx, &s.appendTimes, time.Now)
	if err != nil {
		return true, err
	}
	m := receiptIndexRun{db: s.db, conn: conn, schema: schema, health: health, now: time.Now}
	return true, m.run(ctx)
}

// awaitAppendBaseline returns when the busiest computer's append times can be
// read, so that no phase runs without something to compare against: when the
// ring holds receiptIndexMinAppends appends in the baseline window, or once
// the whole window has passed since the ring was first watched with fewer (a
// host that is quiet has nothing for the swap to slow). The wait is logged on
// change and at a steady cadence.
func awaitAppendBaseline(ctx context.Context, t *appendTimings, now func() time.Time) (appendHealth, error) {
	born := t.watch(now())
	var lastLog time.Time
	for {
		at := now()
		h := newAppendHealth(t, at)
		if h.known {
			b := h.baseline
			log.Printf("corpusd receipt indexes: baseline %s: append p95 %s over %d appends in %s (%.1f/s)", b.Computer, b.P95, b.Count, b.Span.Round(time.Second), b.rate())
			return h, nil
		}
		if at.Sub(born) >= receiptIndexBaselineWindow {
			log.Printf("corpusd receipt indexes: no baseline: under %d appends in the %s the listener has served; the host is quiet, going on", receiptIndexMinAppends, receiptIndexBaselineWindow)
			return h, nil
		}
		if lastLog.IsZero() || at.Sub(lastLog) >= receiptIndexLogEvery {
			log.Printf("corpusd receipt indexes: waiting for a baseline of append times (%s of %s served)", at.Sub(born).Round(time.Second), receiptIndexBaselineWindow)
			lastLog = at
		}
		select {
		case <-ctx.Done():
			return appendHealth{}, ctx.Err()
		case <-time.After(receiptIndexPoll):
		}
	}
}

// receiptTableSchema is the namespace the receipt table lives in, resolved as
// the store's own queries resolve it.
func receiptTableSchema(ctx context.Context, conn *sql.Conn) (string, error) {
	var schema string
	err := conn.QueryRowContext(ctx, `SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE c.oid = 'computer_event_append_receipts'::regclass`).Scan(&schema)
	return schema, err
}

type receiptIndexRun struct {
	db     *sql.DB // a second connection, for watching and cancelling the build
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

// waitHealthy returns when the baseline computer's append times have not risen
// (N3); until then it looks again, and logs when the reading changes between
// slow and well and at a steady cadence while it stays slow.
func (m receiptIndexRun) waitHealthy(ctx context.Context, phase string) error {
	var lastLog time.Time
	wasBad := false
	for {
		at := m.now()
		bad, why := m.health.degraded(at)
		if !bad {
			if wasBad {
				log.Printf("corpusd receipt indexes: appends recovered %s: %s", phase, why)
			}
			return nil
		}
		if !wasBad || at.Sub(lastLog) >= receiptIndexLogEvery {
			log.Printf("corpusd receipt indexes: waiting %s: %s", phase, why)
			lastLog = at
		}
		wasBad = true
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
// invalid leftover of an interrupted or cancelled build is dropped first.
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
	if err := m.build(ctx); err != nil {
		return err
	}
	if _, valid, err := m.indexState(ctx, receiptKindPartialIdx); err != nil || !valid {
		return fmt.Errorf("create %s: the index is not valid after the build (err=%v)", receiptKindPartialIdx, err)
	}
	log.Printf("corpusd receipt indexes: %s is valid after %s", receiptKindPartialIdx, time.Since(start).Round(time.Second))
	return nil
}

// build runs CREATE INDEX CONCURRENTLY and watches the baseline computer's
// append times on a second connection while it runs; when the slowdown holds
// for two polls it cancels the build's backend. The cancelled build leaves an
// invalid index and returns an error, which ends the attempt; the loop retries
// after its backoff and the next attempt waits for the appends to recover
// before it builds again.
func (m receiptIndexRun) build(ctx context.Context) error {
	var pid int
	if err := m.conn.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		_, err := m.conn.ExecContext(ctx, `CREATE INDEX CONCURRENTLY `+quoteIdent(receiptKindPartialIdx)+` ON `+quoteIdent(m.schema)+`.computer_event_append_receipts (computer_id, event_kind, sequence) WHERE `+receiptKindNotBatchSQL)
		done <- err
	}()
	tick := time.NewTicker(receiptIndexPoll)
	defer tick.Stop()
	slow := 0
	var cancelled string
	for {
		select {
		case err := <-done:
			if err != nil && cancelled != "" {
				return fmt.Errorf("create %s: cancelled by the migration, appends were slow (%s): %w", receiptKindPartialIdx, cancelled, err)
			}
			if err != nil {
				return fmt.Errorf("create %s: %w", receiptKindPartialIdx, err)
			}
			return nil
		case <-tick.C:
			if cancelled != "" {
				continue
			}
			bad, why := m.health.degraded(m.now())
			if !bad {
				slow = 0
				continue
			}
			slow++
			if slow < 2 {
				log.Printf("corpusd receipt indexes: appends look slow during the build (1 of 2 polls): %s", why)
				continue
			}
			var n int
			if err := m.db.QueryRowContext(ctx, `SELECT count(pg_cancel_backend(pid)) FROM pg_stat_activity WHERE pid = ? AND query LIKE 'CREATE INDEX CONCURRENTLY%'`, pid).Scan(&n); err != nil {
				log.Printf("corpusd receipt indexes: cancelling the build: %v", err)
				continue
			}
			cancelled = why
			log.Printf("corpusd receipt indexes: cancelled the build of %s (backend %d, %d cancelled), appends stayed slow for two polls: %s", receiptKindPartialIdx, pid, n, why)
		}
	}
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

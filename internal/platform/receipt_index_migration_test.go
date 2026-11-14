package platform

import (
	"context"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

// liveIndexes puts the table in the state of the live host before the swap:
// the full kind index and the receipt id index, no partial one.
func liveIndexes(t *testing.T, s *Store) {
	t.Helper()
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS ` + receiptKindPartialIdx,
		`CREATE INDEX IF NOT EXISTS ` + receiptKindOldIdx + ` ON computer_event_append_receipts (computer_id, event_kind, sequence)`,
		`CREATE INDEX IF NOT EXISTS ` + receiptIDOldIdx + ` ON computer_event_append_receipts (computer_id, event_head_receipt_id)`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
}

// fastMigration shortens every wait of the migration to milliseconds.
func fastMigration(t *testing.T) {
	t.Helper()
	poll, backoff, maxBackoff, window, timeout := receiptIndexPoll, receiptIndexBackoff, receiptIndexBackoffMax, receiptIndexBaselineWindow, receiptIndexStatementTimeout
	receiptIndexPoll, receiptIndexBackoff, receiptIndexBackoffMax, receiptIndexBaselineWindow = 20*time.Millisecond, 20*time.Millisecond, 100*time.Millisecond, 300*time.Millisecond
	t.Cleanup(func() {
		receiptIndexPoll, receiptIndexBackoff, receiptIndexBackoffMax, receiptIndexBaselineWindow, receiptIndexStatementTimeout = poll, backoff, maxBackoff, window, timeout
	})
}

// seedBaseline records the appends of a busy computer at 10 ms.
func seedBaseline(s *Store, computer string, n int, took time.Duration) {
	for i := 0; i < n; i++ {
		s.appendTimes.record(computer, took)
	}
}

// holdWriter keeps a transaction that has written to the receipt table open,
// which CREATE INDEX CONCURRENTLY waits for; release rolls it back.
func holdWriter(t *testing.T, s *Store) (release func()) {
	t.Helper()
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at)
		VALUES ('computer-held', 'held', 'a', 1, 'projection_batch_recorded', 'h', 'c', 'held', '{}', now())`); err != nil {
		t.Fatal(err)
	}
	var once atomic.Bool
	release = func() {
		if once.CompareAndSwap(false, true) {
			_ = tx.Rollback()
		}
	}
	t.Cleanup(release)
	return release
}

// waitForBuild returns once a CREATE INDEX CONCURRENTLY is running in the database.
func waitForBuild(t *testing.T, s *Store) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE state = 'active' AND query LIKE 'CREATE INDEX CONCURRENTLY%'`).Scan(&n); err == nil && n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the build never started")
}

func partialIndexState(t *testing.T, s *Store) (exists, valid bool) {
	t.Helper()
	if err := s.db.QueryRow(`SELECT COALESCE(bool_or(true), false), COALESCE(bool_and(i.indisvalid), false) FROM pg_class c JOIN pg_index i ON i.indexrelid = c.oid WHERE c.relname = ?`, receiptKindPartialIdx).Scan(&exists, &valid); err != nil {
		t.Fatal(err)
	}
	return
}

// M1, M2, M3: the migration builds the partial index, drops the two old ones
// only afterwards, and is idempotent; only the partial one is left.
func TestReceiptIndexMigrationSwapsTheIndexes(t *testing.T) {
	ctx := context.Background()
	fastMigration(t)
	s, _ := openTestPlatformStore(t)
	loadReceiptRows(t, s, indexTestComputer, 20000, true)
	liveIndexes(t, s)
	if done, err := s.migrateReceiptIndexes(ctx); !done || err != nil {
		t.Fatalf("migrate = %v, %v", done, err)
	}
	idx := receiptIndexNames(t, s)
	if !idx[receiptKindPartialIdx] || idx[receiptKindOldIdx] || idx[receiptIDOldIdx] {
		t.Fatalf("after the migration: %v", idx)
	}
	// Far smaller than the full index it replaces.
	var partial, full int64
	if err := s.db.QueryRow(`SELECT pg_relation_size(?::regclass)`, receiptKindPartialIdx).Scan(&partial); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE INDEX probe_full ON computer_event_append_receipts (computer_id, event_kind, sequence)`); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT pg_relation_size('probe_full'::regclass)`).Scan(&full); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP INDEX probe_full`); err != nil {
		t.Fatal(err)
	}
	if partial*10 > full {
		t.Fatalf("partial index %d bytes, full %d: not far smaller", partial, full)
	}
	if done, err := s.migrateReceiptIndexes(ctx); !done || err != nil {
		t.Fatalf("second migrate = %v, %v", done, err)
	}
	// Of the kind and receipt id indexes only the partial one is left.
	if idx := receiptIndexNames(t, s); !idx[receiptKindPartialIdx] || idx[receiptKindOldIdx] || idx[receiptIDOldIdx] {
		t.Fatalf("after the migration: %v", idx)
	}
}

// M2: an interrupted build leaves an invalid index (here produced the way a
// real one is, by the statement timeout cancelling a build that waits behind a
// writer, with no superuser), and the next attempt rebuilds it; the old indexes
// are not dropped in between.
func TestReceiptIndexMigrationRepairsAnInterruptedBuild(t *testing.T) {
	ctx := context.Background()
	fastMigration(t)
	s, _ := openTestPlatformStore(t)
	loadReceiptRows(t, s, indexTestComputer, 20000, true)
	liveIndexes(t, s)
	release := holdWriter(t, s)
	receiptIndexStatementTimeout = 300 * time.Millisecond
	if _, err := s.migrateReceiptIndexes(ctx); err == nil || !strings.Contains(err.Error(), "statement timeout") {
		t.Fatalf("the build behind a writer should hit the statement timeout, got %v", err)
	}
	if exists, valid := partialIndexState(t, s); !exists || valid {
		t.Fatalf("after the interrupted build exists=%v valid=%v, want an invalid index", exists, valid)
	}
	if idx := receiptIndexNames(t, s); !idx[receiptKindOldIdx] || !idx[receiptIDOldIdx] {
		t.Fatalf("an old index went before a valid one existed: %v", idx)
	}
	release()
	receiptIndexStatementTimeout = 3 * time.Hour
	if done, err := s.migrateReceiptIndexes(ctx); !done || err != nil {
		t.Fatalf("repair migrate = %v, %v", done, err)
	}
	if exists, valid := partialIndexState(t, s); !exists || !valid {
		t.Fatalf("after repair exists=%v valid=%v", exists, valid)
	}
	if idx := receiptIndexNames(t, s); idx[receiptKindOldIdx] || idx[receiptIDOldIdx] {
		t.Fatalf("an old index survived: %v", idx)
	}
}

// M1: appends and reads go on while the swap runs.
func TestReceiptIndexMigrationDoesNotBlockAppends(t *testing.T) {
	ctx := context.Background()
	fastMigration(t)
	s, _ := openTestPlatformStore(t)
	loadReceiptRows(t, s, indexTestComputer, 30000, true)
	liveIndexes(t, s)
	var stop atomic.Bool
	var inserted, slowest atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for seq := 100000; !stop.Load(); seq++ {
			start := time.Now()
			_, err := s.db.Exec(`INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at)
				VALUES (?, ?, 'a', ?, 'projection_batch_recorded', ?, 'c', ?, '{}', now())`, indexTestComputer, "live-"+time.Now().String(), seq, "d"+time.Now().String(), "r"+time.Now().String())
			if err != nil {
				return
			}
			inserted.Add(1)
			if took := time.Since(start).Milliseconds(); took > slowest.Load() {
				slowest.Store(took)
			}
		}
	}()
	if ok, err := s.migrateReceiptIndexes(ctx); !ok || err != nil {
		t.Fatalf("migrate = %v, %v", ok, err)
	}
	stop.Store(true)
	<-done
	if inserted.Load() == 0 || slowest.Load() > 5000 {
		t.Fatalf("inserted %d rows during the swap, slowest %d ms", inserted.Load(), slowest.Load())
	}
}

// M3: a process that does not get the lock does nothing and retries; the work
// is finished when the holder lets go.
func TestReceiptIndexMigrationRetriesWhileAnotherHoldsTheLock(t *testing.T) {
	ctx := context.Background()
	fastMigration(t)
	s, _ := openTestPlatformStore(t)
	liveIndexes(t, s)
	holder, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := holder.ExecContext(ctx, `SELECT pg_advisory_lock(?)`, receiptIndexLockKey); err != nil {
		t.Fatal(err)
	}
	if done, err := s.migrateReceiptIndexes(ctx); done || err != nil {
		t.Fatalf("a second runner = %v, %v, want not done, no error", done, err)
	}
	if idx := receiptIndexNames(t, s); idx[receiptKindPartialIdx] || !idx[receiptKindOldIdx] {
		t.Fatalf("a second runner changed the indexes: %v", idx)
	}
	s.StartReceiptIndexMigration(ctx)
	time.Sleep(150 * time.Millisecond)
	if idx := receiptIndexNames(t, s); idx[receiptKindPartialIdx] {
		t.Fatal("the loop built the index while another held the lock")
	}
	if _, err := holder.ExecContext(ctx, `SELECT pg_advisory_unlock(?)`, receiptIndexLockKey); err != nil {
		t.Fatal(err)
	}
	holder.Close()
	waitSwapped(t, s)
}

func waitSwapped(t *testing.T, s *Store) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if idx := receiptIndexNames(t, s); idx[receiptKindPartialIdx] && !idx[receiptKindOldIdx] && !idx[receiptIDOldIdx] && s.receiptIndexState.Load() == receiptIndexDone {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the swap did not finish: %v state=%d", receiptIndexNames(t, s), s.receiptIndexState.Load())
}

// M6 (B1): through StartReceiptIndexMigration itself, as corpusd calls it: with
// an empty ring nothing is built until a baseline of append times exists, and
// the backfill walk is held back meanwhile.
func TestReceiptIndexMigrationWaitsForABaselineOnTheFirstBoot(t *testing.T) {
	fastMigration(t)
	receiptIndexBaselineWindow = 5 * time.Second
	s, _ := openTestPlatformStore(t)
	liveIndexes(t, s)
	s.StartReceiptIndexMigration(context.Background())
	time.Sleep(250 * time.Millisecond)
	if exists, _ := partialIndexState(t, s); exists {
		t.Fatal("the build started with no baseline of append times")
	}
	if !s.receiptIndexMigrationRunning() {
		t.Fatal("the migration does not say it is running")
	}
	c := noSleepCompactor(s)
	c.freeSpace = func(context.Context) (uint64, string, bool) { return receiptWalkMinFree * 2, "/data", true }
	if why := c.deferWalk(context.Background()); !strings.Contains(why, "migration is running") {
		t.Fatalf("the walk was not held back: %q", why)
	}
	seedBaseline(s, indexTestComputer, 40, 10*time.Millisecond) // the listener serves appends
	waitSwapped(t, s)
	if why := c.deferWalk(context.Background()); why != "" {
		t.Fatalf("the walk is held back after the migration: %s", why)
	}
}

// M6: a host that stays quiet for the whole baseline window has nothing for the
// swap to slow, and it goes on.
func TestReceiptIndexMigrationGoesOnOnAQuietHost(t *testing.T) {
	fastMigration(t)
	s, _ := openTestPlatformStore(t)
	liveIndexes(t, s)
	s.StartReceiptIndexMigration(context.Background())
	waitSwapped(t, s)
}

// M6 (B2): the build is watched while it runs; when the busiest computer's
// appends stay slow for two polls its backend is cancelled (a real cancel, of a
// real build waiting behind a writer), the old indexes stay, and the next
// attempt, after the appends recover, builds.
func TestReceiptIndexMigrationCancelsABuildThatSlowsAppends(t *testing.T) {
	ctx := context.Background()
	fastMigration(t)
	s, _ := openTestPlatformStore(t)
	loadReceiptRows(t, s, indexTestComputer, 20000, true)
	liveIndexes(t, s)
	seedBaseline(s, indexTestComputer, 40, 10*time.Millisecond)
	release := holdWriter(t, s)
	result := make(chan error, 1)
	go func() {
		_, err := s.migrateReceiptIndexes(ctx)
		result <- err
	}()
	waitForBuild(t, s)
	seedBaseline(s, indexTestComputer, 400, 900*time.Millisecond) // the build slows the appends
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "cancelled by the migration") {
			t.Fatalf("migrate = %v, want a cancelled build", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the build was not cancelled although appends stayed slow")
	}
	if idx := receiptIndexNames(t, s); !idx[receiptKindOldIdx] || !idx[receiptIDOldIdx] {
		t.Fatalf("an old index went with the cancelled build: %v", idx)
	}
	release()
	// The appends recover: the next attempt drops the invalid leftover and builds.
	seedBaseline(s, indexTestComputer, 8000, 10*time.Millisecond)
	if done, err := s.migrateReceiptIndexes(ctx); !done || err != nil {
		t.Fatalf("retry = %v, %v", done, err)
	}
	if exists, valid := partialIndexState(t, s); !exists || !valid {
		t.Fatalf("after the retry exists=%v valid=%v", exists, valid)
	}
}

// M6 (the rule): the baseline is the busiest computer's, and it is compared with
// that computer's own appends.
func TestAppendHealthComparesTheBaselineComputerWithItself(t *testing.T) {
	var timings appendTimings
	now := time.Now()
	if bad, why := newAppendHealth(&timings, now).degraded(now); bad || why != "no baseline" {
		t.Fatalf("degraded with no baseline: %v %q", bad, why)
	}
	for i := 0; i < 40; i++ {
		timings.record("computer-a", 10*time.Millisecond)
	}
	for i := 0; i < 5; i++ {
		timings.record("computer-quiet", time.Hour)
	}
	h := newAppendHealth(&timings, now)
	if !h.known || h.baseline.Computer != "computer-a" || h.baseline.P95 != receiptIndexFloor {
		t.Fatalf("baseline = %+v known=%v, want computer-a at the floor", h.baseline, h.known)
	}
	if bad, why := h.degraded(now); bad {
		t.Fatalf("healthy appends read as degraded: %s", why)
	}
	// Another computer becomes busier and is slow-ish: it is not the baseline's.
	for i := 0; i < 400; i++ {
		timings.record("computer-b", 300*time.Millisecond)
	}
	for i := 0; i < 30; i++ {
		timings.record("computer-a", 10*time.Millisecond)
	}
	if bad, why := h.degraded(now); bad {
		t.Fatalf("another computer's p95 read as the baseline computer's: %s", why)
	}
	// The baseline computer slows: 2x and more than 250 ms over its own.
	for i := 0; i < 100; i++ {
		timings.record("computer-a", 400*time.Millisecond)
	}
	if bad, _ := h.degraded(now); !bad {
		t.Fatal("a rise of the baseline computer's own appends was not seen")
	}
}

// M6 (B3): a baseline that had many appends and now has almost none is a
// stall, not health.
func TestAppendHealthTreatsARateCollapseAsDegraded(t *testing.T) {
	var timings appendTimings
	base := time.Now().Add(-time.Minute)
	for i := 0; i < 600; i++ {
		timings.samples[timings.next] = appendSample{at: base.Add(time.Duration(i) * 100 * time.Millisecond), computer: "computer-a", took: 10 * time.Millisecond}
		timings.next++
		timings.filled++
	}
	now := base.Add(time.Minute)
	h := newAppendHealth(&timings, now) // 600 appends in about a minute: 10 a second
	if !h.known || h.baseline.rate() < 5 {
		t.Fatalf("baseline = %+v", h.baseline)
	}
	// Two minutes on with a handful of appends where about 1200 are expected.
	later := now.Add(2 * time.Minute)
	for i := 0; i < 5; i++ {
		timings.samples[timings.next] = appendSample{at: later.Add(-time.Second), computer: "computer-a", took: 10 * time.Millisecond}
		timings.next++
		timings.filled++
	}
	if bad, why := h.degraded(later); !bad || !strings.Contains(why, "expects about") {
		t.Fatalf("a stall read as healthy: %v %q", bad, why)
	}
	// The same few appends right after the baseline say nothing yet.
	if bad, why := h.degraded(now.Add(2 * time.Second)); bad {
		t.Fatalf("a short span read as a collapse: %s", why)
	}
}

// M6: between phases the swap waits while the baseline computer's appends are
// slow, touches no index meanwhile, and goes on by itself when they recover.
func TestReceiptIndexMigrationWaitsWhileAppendsAreSlow(t *testing.T) {
	ctx := context.Background()
	fastMigration(t)
	s, _ := openTestPlatformStore(t)
	liveIndexes(t, s)
	seedBaseline(s, indexTestComputer, 40, 10*time.Millisecond)
	health := newAppendHealth(&s.appendTimes, time.Now())
	seedBaseline(s, indexTestComputer, 100, 600*time.Millisecond) // the appends rise after the baseline
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	schema, err := receiptTableSchema(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	m := receiptIndexRun{db: s.db, conn: conn, schema: schema, health: health, now: time.Now}
	result := make(chan error, 1)
	go func() { result <- m.run(ctx) }()
	select {
	case err := <-result:
		t.Fatalf("the swap ran through slow appends: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if idx := receiptIndexNames(t, s); idx[receiptKindPartialIdx] || !idx[receiptKindOldIdx] || !idx[receiptIDOldIdx] {
		t.Fatalf("the swap touched an index while appends were slow: %v", idx)
	}
	seedBaseline(s, indexTestComputer, 2000, 10*time.Millisecond) // the appends recover
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the swap did not go on after the appends recovered")
	}
	if idx := receiptIndexNames(t, s); !idx[receiptKindPartialIdx] || idx[receiptKindOldIdx] || idx[receiptIDOldIdx] {
		t.Fatalf("after recovery: %v", idx)
	}
}

// M5: the table's own namespace, not current_schema(), names the indexes; here
// the store's search_path puts every table in a schema other than public.
func TestReceiptIndexMigrationUsesTheTablesOwnSchema(t *testing.T) {
	ctx := context.Background()
	base := dbxtest.PostgresDSN(t)
	admin, err := dbx.OpenPostgres(base, dbx.PGOptions{MaxOpenConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`CREATE SCHEMA rx`); err != nil {
		t.Fatal(err)
	}
	admin.Close()
	dsn := base + " search_path=rx"
	if strings.HasPrefix(base, "postgres://") || strings.HasPrefix(base, "postgresql://") {
		u, err := url.Parse(base)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("search_path", "rx")
		u.RawQuery = q.Encode()
		dsn = u.String()
	}
	control, err := dbx.OpenPostgres(dsn, dbx.PGOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(control, nil)
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := s.db.QueryRow(`SELECT schemaname FROM pg_tables WHERE tablename='computer_event_append_receipts'`).Scan(&schema); err != nil || schema != "rx" {
		t.Skipf("the search_path did not take (%q, %v)", schema, err)
	}
	fastMigration(t)
	liveIndexes(t, s)
	if done, err := s.migrateReceiptIndexes(ctx); !done || err != nil {
		t.Fatalf("migrate = %v, %v", done, err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE schemaname = 'rx' AND indexname = ?`, receiptKindPartialIdx).Scan(&n); err != nil || n != 1 {
		t.Fatalf("partial index in rx = %d, %v", n, err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE tablename='computer_event_append_receipts' AND indexname IN (?, ?)`, receiptKindOldIdx, receiptIDOldIdx).Scan(&n); err != nil || n != 0 {
		t.Fatalf("old indexes left = %d, %v", n, err)
	}
}

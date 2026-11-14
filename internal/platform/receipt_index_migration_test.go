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

func fastMigration(t *testing.T) {
	t.Helper()
	poll, backoff, max := receiptIndexPoll, receiptIndexBackoff, receiptIndexBackoffMax
	receiptIndexPoll, receiptIndexBackoff, receiptIndexBackoffMax = 20*time.Millisecond, 20*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() { receiptIndexPoll, receiptIndexBackoff, receiptIndexBackoffMax = poll, backoff, max })
}

// M1, M2, M3: the migration builds the partial index, drops the two old ones
// only afterwards, repairs an interrupted build, and is idempotent.
func TestReceiptIndexMigrationSwapsTheIndexes(t *testing.T) {
	ctx := context.Background()
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
	// An interrupted build leaves an invalid index; the next run rebuilds it,
	// and the old indexes are not dropped before a valid one exists.
	liveIndexes(t, s) // drops the partial one, restores the old two
	if _, err := s.db.Exec(`CREATE INDEX ` + receiptKindPartialIdx + ` ON computer_event_append_receipts (computer_id, event_kind, sequence) WHERE ` + receiptKindNotBatchSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE pg_index SET indisvalid = false WHERE indexrelid = ?::regclass`, receiptKindPartialIdx); err != nil {
		t.Skipf("cannot mark an index invalid here: %v", err)
	}
	if done, err := s.migrateReceiptIndexes(ctx); !done || err != nil {
		t.Fatalf("repair migrate = %v, %v", done, err)
	}
	var valid bool
	if err := s.db.QueryRow(`SELECT indisvalid FROM pg_index WHERE indexrelid = ?::regclass`, receiptKindPartialIdx).Scan(&valid); err != nil || !valid {
		t.Fatalf("after repair valid=%v err=%v", valid, err)
	}
	if idx := receiptIndexNames(t, s); idx[receiptKindOldIdx] || idx[receiptIDOldIdx] {
		t.Fatalf("an old index survived: %v", idx)
	}
}

// M1: appends and reads go on while the swap runs.
func TestReceiptIndexMigrationDoesNotBlockAppends(t *testing.T) {
	ctx := context.Background()
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
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if idx := receiptIndexNames(t, s); idx[receiptKindPartialIdx] && !idx[receiptKindOldIdx] && !idx[receiptIDOldIdx] {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the loop did not finish after the lock was released: %v", receiptIndexNames(t, s))
}

// M6 (the rule): the busiest computer's append p95 against a baseline.
func TestAppendHealthReadsTheBusiestComputer(t *testing.T) {
	var timings appendTimings
	now := time.Now()
	// No samples: no baseline, never degraded.
	if bad, _ := newAppendHealth(&timings, now).degraded(now); bad {
		t.Fatal("degraded with no baseline")
	}
	for i := 0; i < 40; i++ {
		timings.record("computer-busy", 10*time.Millisecond)
	}
	for i := 0; i < 5; i++ {
		timings.record("computer-quiet", time.Hour)
	}
	h := newAppendHealth(&timings, now)
	if !h.known || h.baseline != receiptIndexFloor {
		t.Fatalf("baseline = %v known=%v, want the floor %v from the busiest computer", h.baseline, h.known, receiptIndexFloor)
	}
	if bad, load := h.degraded(now); bad || load.Computer != "computer-busy" {
		t.Fatalf("healthy appends read as degraded: %v %+v", bad, load)
	}
	// The busiest computer slows: 2x and more than 250 ms over.
	for i := 0; i < 100; i++ {
		timings.record("computer-busy", 400*time.Millisecond)
	}
	if bad, load := h.degraded(now); !bad {
		t.Fatalf("a rise to %v was not seen", load.P95)
	}
	// Few appends say nothing.
	var quiet appendTimings
	for i := 0; i < receiptIndexMinAppends-1; i++ {
		quiet.record("c", time.Hour)
	}
	if bad, _ := (appendHealth{timings: &quiet, baseline: receiptIndexFloor, known: true}).degraded(now); bad {
		t.Fatal("a quiet window read as degraded")
	}
}

// M6: the swap waits while the busiest computer's appends are slow, touches no
// index meanwhile, and goes on by itself when they recover.
func TestReceiptIndexMigrationWaitsWhileAppendsAreSlow(t *testing.T) {
	ctx := context.Background()
	fastMigration(t)
	s, _ := openTestPlatformStore(t)
	liveIndexes(t, s)
	for i := 0; i < 40; i++ {
		s.appendTimes.record(indexTestComputer, 10*time.Millisecond)
	}
	health := newAppendHealth(&s.appendTimes, time.Now())
	for i := 0; i < 100; i++ { // the busiest computer's appends rise after the baseline
		s.appendTimes.record(indexTestComputer, 600*time.Millisecond)
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	schema, err := receiptTableSchema(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	m := receiptIndexRun{conn: conn, schema: schema, health: health, now: time.Now}
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
	for i := 0; i < 2000; i++ { // the appends recover
		s.appendTimes.record(indexTestComputer, 10*time.Millisecond)
	}
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

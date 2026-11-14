package platform

import (
	"context"
	"errors"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
)

func jsonbChain(t *testing.T, id string, events int) (*drillComputer, []storedReceiptRow) {
	t.Helper()
	d := newDrillComputer(t, id)
	d.appendEvent(computerevent.EventGenesisImported, "", nil)
	d.appendMixed(events-1, "jsonb")
	// PostgreSQL's own jsonb text is the old rows' spelling.
	if _, err := d.store.db.Exec(`UPDATE computer_event_append_receipts SET event_head_receipt_json = event_head_receipt_json::jsonb::text WHERE computer_id=?`, d.id); err != nil {
		t.Fatal(err)
	}
	return d, storedReceipts(t, d.store, d.id)
}

// The 5.13M rows the canonical codec could not compact are rewritten to format
// 2, byte-exact (the batch re-reads and expands each against the original before
// it commits); replay and verification read what they read before, and expand
// writes the stored bytes back.
func TestReceiptCompactionRewritesJSONBRowsToFormat2(t *testing.T) {
	ctx := context.Background()
	d, old := jsonbChain(t, "computer-59c902c5d46caecc0a180a8557164e72", 7)
	baseline, err := d.artifacts.EventsPage(ctx, d.id, 0, 100)
	if err != nil || len(baseline) != 7 {
		t.Fatalf("baseline replay: %d, %v", len(baseline), err)
	}
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactActive}})
	p := runCompaction(t, noSleepCompactor(d.store))
	if len(p) != 1 || p[0].Rewritten != 7 || p[0].Kept != 0 || !p[0].Done {
		t.Fatalf("pass = %+v, want all 7 jsonb-spelled rows rewritten", p)
	}
	for i, r := range storedReceipts(t, d.store, d.id) {
		if !computerevent.IsCompactHeadReceipt(r.stored) || len(r.stored)*2 > len(old[i].stored) {
			t.Fatalf("sequence %d not compact: %d bytes from %d", r.seq, len(r.stored), len(old[i].stored))
		}
	}
	page, err := d.artifacts.EventsPage(ctx, d.id, 0, 100)
	if err != nil || !reflect.DeepEqual(page, baseline) {
		t.Fatalf("replay of the rewritten chain differs: %v", err)
	}
	verifyReplayReceipts(t, d, page)
	if again := runCompaction(t, noSleepCompactor(d.store)); again[0].Scanned != 0 || !again[0].Done {
		t.Fatalf("second pass = %+v, want nothing scanned", again)
	}
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactExpand}})
	if exp := runCompaction(t, noSleepCompactor(d.store)); exp[0].Rewritten != 7 {
		t.Fatalf("expand = %+v", exp[0])
	}
	if got := storedReceipts(t, d.store, d.id); !reflect.DeepEqual(got, old) {
		t.Fatal("expand did not restore the jsonb-spelled bytes")
	}
}

// The revert hole: a release that does not know this walk's progress advances
// the shared progress row past rows it did not compact (it keeps the
// jsonb-spelled ones). Re-landing this release must still compact the rows in
// the gap. The older release's progress SQL is run here as it is written there.
func TestReceiptCompactionVisitsTheRowsAnOlderReleaseWalkedPast(t *testing.T) {
	ctx := context.Background()
	oldBatch := receiptCompactionBatch
	receiptCompactionBatch = 2
	t.Cleanup(func() { receiptCompactionBatch = oldBatch })
	d, _ := jsonbChain(t, "computer-59c902c5d46caecc0a180a8557164e72", 9)
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactActive}})
	// This release walks the first batch, then stops (the pause ends the pass).
	c := noSleepCompactor(d.store)
	c.sleep = func(context.Context, time.Duration) error { return errors.New("stopped between batches") }
	if _, err := c.RunOnce(ctx); err == nil {
		t.Fatal("the first pass should have been stopped after one batch")
	}
	var through int64
	if err := d.store.db.QueryRow(`SELECT through_sequence FROM computer_event_receipt_compaction_v2 WHERE computer_id=?`, d.id).Scan(&through); err != nil || through != 2 {
		t.Fatalf("v2 progress = %d, %v, want 2", through, err)
	}
	// The release that reverted it walks to the head, keeping every full row,
	// with its own progress SQL (it does not name the v2 table).
	if _, err := d.store.db.Exec(`INSERT INTO computer_event_receipt_compaction (computer_id, mode, through_sequence, rows_rewritten, rows_kept, bytes_before, bytes_after, updated_at) VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT (computer_id) DO UPDATE SET mode=excluded.mode, through_sequence=excluded.through_sequence, rows_rewritten=excluded.rows_rewritten, rows_kept=excluded.rows_kept, bytes_before=excluded.bytes_before, bytes_after=excluded.bytes_after, updated_at=excluded.updated_at`,
		d.id, ReceiptCompactActive, 9, 0, 9, 1, 1, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// This release again: every row, including the gap, is compacted.
	runCompaction(t, noSleepCompactor(d.store))
	for _, r := range storedReceipts(t, d.store, d.id) {
		if !computerevent.IsCompactHeadReceipt(r.stored) {
			t.Fatalf("sequence %d was left full after the older release walked past it", r.seq)
		}
	}
}

// Whatever the older release did after this walk's last save, in whatever order
// (an expand pass, then an active or dry-run one that overwrites the same shared
// row), the shared row is newer than this walk's own, and the walk starts over.
func TestReceiptCompactionRestartsAfterAnyLaterPassOfAnOlderRelease(t *testing.T) {
	for _, sequence := range [][]string{
		{ReceiptCompactExpand},
		{ReceiptCompactExpand, ReceiptCompactActive},
		{ReceiptCompactExpand, ReceiptCompactDryRun},
		{ReceiptCompactActive},
	} {
		t.Run(strings.Join(sequence, "-then-"), func(t *testing.T) {
			d, _ := jsonbChain(t, "computer-59c902c5d46caecc0a180a8557164e72", 5)
			d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactActive}})
			runCompaction(t, noSleepCompactor(d.store))
			if again := runCompaction(t, noSleepCompactor(d.store)); again[0].Scanned != 0 {
				t.Fatalf("a finished walk scanned %d", again[0].Scanned)
			}
			for _, mode := range sequence {
				time.Sleep(5 * time.Millisecond)
				if _, err := d.store.db.Exec(`INSERT INTO computer_event_receipt_compaction (computer_id, mode, through_sequence, rows_rewritten, rows_kept, bytes_before, bytes_after, updated_at) VALUES (?,?,?,?,?,?,?,now())
					ON CONFLICT (computer_id) DO UPDATE SET mode=excluded.mode, updated_at=excluded.updated_at`, d.id, mode, 5, 5, 0, 1, 1); err != nil {
					t.Fatal(err)
				}
			}
			if p := runCompaction(t, noSleepCompactor(d.store)); p[0].Scanned != 5 {
				t.Fatalf("pass = %+v, want the walk to start over after the older release's %v", p[0], sequence)
			}
		})
	}
}

// The walk defers when the measured volume has less than the walk's estimate
// free, and not otherwise; it does not look at any index.
func TestReceiptWalkDefersForFreeSpaceOnly(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestPlatformStore(t)
	c := noSleepCompactor(s)
	c.freeSpace = func(context.Context) (uint64, string, bool) { return receiptWalkMinFree * 2, "/data", true }
	if why := c.deferWalk(ctx, true); why != "" {
		t.Fatalf("deferred with room: %s", why)
	}
	c.freeSpace = func(context.Context) (uint64, string, bool) { return receiptWalkMinFree / 2, "/data", true }
	if why := c.deferWalk(ctx, true); !strings.Contains(why, "under the") || !strings.Contains(why, "/data") {
		t.Fatalf("not deferred with little free space, or the volume is not named: %q", why)
	}
	c.freeSpace = func(context.Context) (uint64, string, bool) { return 0, "", false }
	if why := c.deferWalk(ctx, true); why != "" {
		t.Fatalf("deferred on an unknown free space: %s", why)
	}
}

// The free space the walk reads is the smaller of the volumes it writes to.
func TestReceiptWalkFreeSpaceReadsTheSmallerVolume(t *testing.T) {
	s, root := openTestPlatformStore(t)
	s.artifactsRoot = root
	orig := fileCASVolumeUsage
	t.Cleanup(func() { fileCASVolumeUsage = orig })
	seen := map[string]uint64{}
	fileCASVolumeUsage = func(dir string) (persistentdisk.Usage, error) {
		free := uint64(100 << 30)
		if dir == root {
			free = 7 << 30
		}
		seen[dir] = free
		return persistentdisk.Usage{AvailBytes: free}, nil
	}
	free, from, known := s.receiptWalkFreeSpace(context.Background())
	if !known || free != 7<<30 || from != root || len(seen) < 2 {
		t.Fatalf("free = %d from %q known=%v seen=%v, want the artifacts root's 7 GiB of two volumes", free, from, known, seen)
	}
}

// The walk's cost is read from PostgreSQL's counters, the table found by its
// regclass.
func TestReceiptCompactionReadsTheTablesLoad(t *testing.T) {
	d := newDrillComputer(t, "computer-59c902c5d46caecc0a180a8557164e72")
	d.appendEvent(computerevent.EventGenesisImported, "", nil)
	d.appendMixed(3, "load")
	c := noSleepCompactor(d.store)
	lsn, err := c.walLSN(context.Background())
	if err != nil || lsn == "" {
		t.Fatalf("wal lsn = %q, %v", lsn, err)
	}
	d.appendMixed(2, "more")
	load, err := c.receiptTableLoad(context.Background(), lsn)
	if err != nil || !load.WALBytesVisible || load.WALBytesSince <= 0 || load.TableBytes <= 0 {
		t.Fatalf("load = %+v, %v", load, err)
	}
}

// The free-space reading says once which volume it measured, and the note names
// the directory and why when the role may not read PostgreSQL's data directory.
func TestReceiptWalkFreeSpaceSaysWhichVolumeItMeasured(t *testing.T) {
	denied := errors.New("permission denied to examine \"data_directory\"")
	if note := walkFreeNote(denied, "/var/lib/go-choir/artifacts", true); !strings.Contains(note, "/var/lib/go-choir/artifacts") || !strings.Contains(note, "not readable by this role") || !strings.Contains(note, "may not be the volume of the database") {
		t.Fatalf("note = %q", note)
	}
	if note := walkFreeNote(denied, "", false); !strings.Contains(note, "not held back") {
		t.Fatalf("note = %q", note)
	}
	if note := walkFreeNote(nil, "/pgdata", true); !strings.Contains(note, "/pgdata") {
		t.Fatalf("note = %q", note)
	}
	s, root := openTestPlatformStore(t)
	s.artifactsRoot = root
	var lines strings.Builder
	log.SetOutput(&lines)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	_, from, known := s.receiptWalkFreeSpace(context.Background())
	if !known || !strings.Contains(lines.String(), from) {
		t.Fatalf("the first reading did not name %q: %q", from, lines.String())
	}
	s.receiptWalkFreeSpace(context.Background())
	if strings.Count(lines.String(), "receipt walk free space") != 1 {
		t.Fatalf("the volume was announced more than once: %q", lines.String())
	}
}

// holdBuild starts a CREATE INDEX CONCURRENTLY of an unrelated index that waits
// behind a writer, so that its backend holds SHARE UPDATE EXCLUSIVE on the table
// as a swap's build does; release ends both.
func holdBuild(t *testing.T, s *Store) (release func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO computer_event_append_receipts (computer_id, idempotency_key, request_commitment, sequence, event_kind, event_digest, event_pin_receipt_digest, event_head_receipt_id, event_head_receipt_json, created_at)
		VALUES ('computer-held', 'held', 'a', 1, 'projection_batch_recorded', 'h', 'c', 'held', '{}', now())`); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(done)
		defer conn.Close()
		_, _ = conn.ExecContext(ctx, `CREATE INDEX CONCURRENTLY walk_test_build_idx ON computer_event_append_receipts (created_at)`)
	}()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := s.db.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE state = 'active' AND query LIKE 'CREATE INDEX CONCURRENTLY walk_test_build_idx%'`).Scan(&n); err == nil && n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var once sync.Once
	release = func() {
		once.Do(func() {
			_ = tx.Rollback()
			<-done
			_, _ = s.db.Exec(`DROP INDEX IF EXISTS walk_test_build_idx`)
		})
	}
	t.Cleanup(release)
	return release
}

// One heavy job at a time on the table, with no controller: a session that holds
// SHARE UPDATE EXCLUSIVE on the receipt table (a build, a drop) is seen by the
// walk's own role, which cannot read pg_stat_progress_create_index for another
// role's backend; the lock and the backend type are visible to every role.
func TestReceiptBuildRunningIsSeenByAnUnprivilegedRole(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestPlatformStore(t)
	if running, err := s.receiptBuildRunning(ctx); err != nil || running {
		t.Fatalf("running with no build = %v, %v", running, err)
	}
	release := holdBuild(t, s)
	if running, err := s.receiptBuildRunning(ctx); err != nil || !running {
		t.Fatalf("running with a build = %v, %v", running, err)
	}
	// As a role that has no pg_read_all_stats.
	if _, err := s.db.Exec(`CREATE ROLE walk_reader NOLOGIN`); err != nil {
		t.Skipf("cannot create a role here: %v", err)
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, stmt := range []string{`GRANT SELECT ON computer_event_append_receipts TO walk_reader`, `SET ROLE walk_reader`} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	var seen bool
	if err := conn.QueryRowContext(ctx, receiptBuildRunningSQL).Scan(&seen); err != nil || !seen {
		t.Fatalf("an unprivileged role sees the build = %v, %v", seen, err)
	}
	var progress int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_progress_create_index WHERE relid = 'computer_event_append_receipts'::regclass`).Scan(&progress); err != nil {
		t.Fatal(err)
	}
	if progress != 0 {
		t.Logf("the progress view shows the build to this role (%d rows); the lock view is used anyway", progress)
	}
	release()
	if err := conn.QueryRowContext(ctx, receiptBuildRunningSQL).Scan(&seen); err != nil || seen {
		t.Fatalf("a finished build still seen = %v, %v", seen, err)
	}
}

// The walk steps aside between batches while a build holds the table, logs the
// reason once, keeps its progress, and carries on when the build is gone.
func TestReceiptWalkStepsAsideBetweenBatchesAndResumes(t *testing.T) {
	ctx := context.Background()
	oldBatch := receiptCompactionBatch
	receiptCompactionBatch = 2
	t.Cleanup(func() { receiptCompactionBatch = oldBatch })
	d, _ := jsonbChain(t, "computer-59c902c5d46caecc0a180a8557164e72", 9)
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactActive}})
	var lines strings.Builder
	log.SetOutput(&lines)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	c := noSleepCompactor(d.store)
	building := true
	c.buildRunning = func(context.Context) (bool, error) { return building, nil }
	c.freeSpace = func(context.Context) (uint64, string, bool) { return receiptWalkMinFree * 2, "/data", true }
	for i := 0; i < 3; i++ {
		passes, err := c.RunOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if passes[0].Done || passes[0].Through != 2 && i == 0 {
			t.Fatalf("pass %d = %+v, want one batch then a step aside", i, passes[0])
		}
		if i == 0 {
			continue
		}
		if passes[0].Scanned != 0 && passes[0].Through < 2 {
			t.Fatalf("pass %d lost progress: %+v", i, passes[0])
		}
	}
	var through int64
	if err := d.store.db.QueryRow(`SELECT through_sequence FROM computer_event_receipt_compaction_v2 WHERE computer_id=?`, d.id).Scan(&through); err != nil || through < 2 {
		t.Fatalf("saved progress = %d, %v", through, err)
	}
	if n := strings.Count(lines.String(), "receipt compaction deferred"); n != 1 {
		t.Fatalf("the reason was logged %d times, want once: %q", n, lines.String())
	}
	building = false
	passes, err := c.RunOnce(ctx)
	if err != nil || !passes[0].Done {
		t.Fatalf("after the build = %+v, %v", passes, err)
	}
	for _, r := range storedReceipts(t, d.store, d.id) {
		if !computerevent.IsCompactHeadReceipt(r.stored) {
			t.Fatalf("sequence %d left full", r.seq)
		}
	}
	if !strings.Contains(lines.String(), "receipt compaction resumes") {
		t.Fatalf("no resume line: %q", lines.String())
	}
}

// Free space is read again at the load line's cadence, not only at the start of
// a pass, and a pass that finds too little stops with its progress saved.
func TestReceiptWalkReadsFreeSpaceAgainBetweenBatches(t *testing.T) {
	ctx := context.Background()
	oldBatch, oldEvery := receiptCompactionBatch, receiptCompactionLoadEvery
	receiptCompactionBatch, receiptCompactionLoadEvery = 2, 2
	t.Cleanup(func() { receiptCompactionBatch, receiptCompactionLoadEvery = oldBatch, oldEvery })
	d, _ := jsonbChain(t, "computer-59c902c5d46caecc0a180a8557164e72", 11)
	d.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{d.id: ReceiptCompactActive}})
	c := noSleepCompactor(d.store)
	c.buildRunning = func(context.Context) (bool, error) { return false, nil }
	reads := 0
	c.freeSpace = func(context.Context) (uint64, string, bool) {
		reads++
		if reads >= 3 { // the load line of batch 0, then the check after batch 2, then too little
			return receiptWalkMinFree / 4, "/data", true
		}
		return receiptWalkMinFree * 2, "/data", true
	}
	passes, err := c.RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if passes[0].Done || passes[0].Through >= 11 {
		t.Fatalf("the pass did not stop for space: %+v", passes[0])
	}
	if reads < 3 {
		t.Fatalf("free space read %d times", reads)
	}
}

// The progress row never moves backward in the same mode: a pass that began
// earlier, or a second process, cannot undo what another saved.
func TestReceiptCompactionProgressNeverMovesBackward(t *testing.T) {
	ctx := context.Background()
	d := newDrillComputer(t, "computer-59c902c5d46caecc0a180a8557164e72")
	c := noSleepCompactor(d.store)
	through := func() int64 {
		var n int64
		if err := d.store.db.QueryRow(`SELECT through_sequence FROM computer_event_receipt_compaction_v2 WHERE computer_id=?`, d.id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if err := c.saveProgress(ctx, d.id, ReceiptCompactActive, ReceiptCompactionPass{}, ReceiptCompactionPass{Through: 10}); err != nil {
		t.Fatal(err)
	}
	if err := c.saveProgress(ctx, d.id, ReceiptCompactActive, ReceiptCompactionPass{}, ReceiptCompactionPass{Through: 4}); err != nil {
		t.Fatal(err)
	}
	if n := through(); n != 10 {
		t.Fatalf("a lagging save moved the row to %d", n)
	}
	if err := c.saveProgress(ctx, d.id, ReceiptCompactActive, ReceiptCompactionPass{}, ReceiptCompactionPass{Through: 12}); err != nil || through() != 12 {
		t.Fatalf("a save forward = %d, %v", through(), err)
	}
	// A restart is the one backward move, and it is written on purpose.
	if err := c.resetProgress(ctx, d.id, ReceiptCompactActive); err != nil || through() != 0 {
		t.Fatalf("reset = %d, %v", through(), err)
	}
}

// A failed WAL reading is "unknown" in the load line, not 0.
func TestReceiptCompactionLoadLineSaysWALUnknown(t *testing.T) {
	d := newDrillComputer(t, "computer-59c902c5d46caecc0a180a8557164e72")
	d.appendEvent(computerevent.EventGenesisImported, "", nil)
	c := noSleepCompactor(d.store)
	c.freeSpace = func(context.Context) (uint64, string, bool) { return 1 << 40, "/data", true }
	var lines strings.Builder
	log.SetOutput(&lines)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	c.logLoad(context.Background(), d.id, ReceiptCompactionPass{Through: 1}, "not-an-lsn", time.Now())
	if !strings.Contains(lines.String(), "wal_bytes_since_start=unknown") {
		t.Fatalf("line = %q", lines.String())
	}
}

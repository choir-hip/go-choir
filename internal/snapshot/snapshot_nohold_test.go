package snapshot

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// Ways the no-hold capture could fail, written before the change:
//
//  7. the stream holds the append mutex or the write connection, so a writer
//     blocks for the whole stream (on a 30 GB store, minutes);
//  8. the recorded head is read outside the stream's transaction, so it races
//     the writers and names a head the pages do not hold (restore would then
//     skip or re-apply events).

// headsFixture adds a one-row heads table whose seq counts the blobs rows, and
// makes Head read it inside the stream's transaction.
func headsFixture(t *testing.T, f *fixture) {
	t.Helper()
	if _, err := f.db.Write.Exec(`CREATE TABLE heads (id INTEGER PRIMARY KEY CHECK (id = 1), seq INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	var rows uint64
	if err := f.db.Write.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Write.Exec(`INSERT INTO heads (id, seq) VALUES (1, ?)`, rows); err != nil {
		t.Fatal(err)
	}
	f.cfg.Head = func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) {
		var seq uint64
		if err := tx.QueryRowContext(ctx, `SELECT seq FROM heads WHERE id = 1`).Scan(&seq); err != nil {
			return nil, err
		}
		return &computerevent.Head{ComputerID: testComputer, Sequence: seq, CanonicalEventHead: strings.Repeat("d", 64)}, nil
	}
}

// commitRow inserts one blobs row and advances the head in one transaction on
// the single write connection.
func commitRow(db *dbx.SQLiteDB, id int) error {
	tx, err := db.Write.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, x'00', 'live')`, id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`UPDATE heads SET seq = seq + 1 WHERE id = 1`); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// assertRestoredHeadAgrees restores the newest snapshot and checks the record's
// head equals the head and the row count inside the restored file.
func assertRestoredHeadAgrees(t *testing.T, f *fixture, ref Ref) {
	t.Helper()
	ctx := context.Background()
	dest := filepath.Join(t.TempDir(), "state.db")
	if _, err := Restore(ctx, f.restoreConfig(dest)); err != nil {
		t.Fatal(err)
	}
	restored, err := dbx.OpenSQLite(dest, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var inCopy, headInCopy uint64
	if err := restored.Read.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&inCopy); err != nil {
		t.Fatal(err)
	}
	if err := restored.Read.QueryRow(`SELECT seq FROM heads WHERE id = 1`).Scan(&headInCopy); err != nil {
		t.Fatal(err)
	}
	if ref.Record.StoreHeadSequence != headInCopy || headInCopy != inCopy {
		t.Fatalf("record head %d, head inside the restored file %d, rows inside it %d: they must agree", ref.Record.StoreHeadSequence, headInCopy, inCopy)
	}
}

// TestSnapshotDoesNotBlockWritersAndRecordsTheHeadOfItsPages takes a
// multi-chunk snapshot from the READ pool while a writer commits row+head
// transactions on the single write connection. The writer must make progress
// during the snapshot, and the head the record names must be the head inside
// the pages: the number of rows the restored file holds.
func TestSnapshotDoesNotBlockWritersAndRecordsTheHeadOfItsPages(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*3)
	headsFixture(t, f)

	var commits atomic.Int64
	stop := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		defer close(writerDone)
		for id := 100000; ; id++ {
			select {
			case <-stop:
				return
			default:
			}
			if err := commitRow(f.db, id); err != nil {
				writerDone <- err
				return
			}
			commits.Add(1)
		}
	}()
	for commits.Load() == 0 {
		time.Sleep(time.Millisecond)
	}

	before := commits.Load()
	ref, err := TakeSnapshot(ctx, f.cfg)
	after := commits.Load()
	close(stop)
	if werr := <-writerDone; werr != nil {
		t.Fatalf("the writer failed during the snapshot: %v", werr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if after <= before+1 {
		t.Fatalf("the writer committed %d transactions during the snapshot: it was blocked", after-before)
	}
	assertRestoredHeadAgrees(t, f, ref)
}

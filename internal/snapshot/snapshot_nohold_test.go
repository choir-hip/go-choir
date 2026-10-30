package snapshot

import (
	"context"
	"database/sql"
	"errors"
	"os"
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
//  7. the copy holds the append mutex or the write connection, so a writer
//     blocks for the whole VACUUM (on a 30 GB store, minutes);
//  8. the recorded head is read from the live store, so it races the writers
//     and names a head the file does not hold (restore would then skip or
//     re-apply events);
//  9. a copy over the size cap is uploaded anyway, or leaves chunks, scratch
//     files or an event behind.

// TestSnapshotDoesNotBlockWritersAndRecordsTheHeadInsideTheCopy takes a
// multi-chunk snapshot from the READ pool while a writer commits row+head
// transactions on the single write connection. The writer must make progress
// during the snapshot, and the head the record names must be the head inside
// the copy: the number of rows the restored file holds.
func TestSnapshotDoesNotBlockWritersAndRecordsTheHeadInsideTheCopy(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*3)
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
	f.cfg.Head = func(ctx context.Context, snapshotDB *sql.DB) (*computerevent.Head, error) {
		var seq uint64
		if err := snapshotDB.QueryRowContext(ctx, `SELECT seq FROM heads WHERE id = 1`).Scan(&seq); err != nil {
			return nil, err
		}
		return &computerevent.Head{ComputerID: testComputer, Sequence: seq, CanonicalEventHead: strings.Repeat("d", 64)}, nil
	}

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
			tx, err := f.db.Write.Begin()
			if err != nil {
				writerDone <- err
				return
			}
			if _, err := tx.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, x'00', 'live')`, id); err != nil {
				_ = tx.Rollback()
				writerDone <- err
				return
			}
			if _, err := tx.Exec(`UPDATE heads SET seq = seq + 1 WHERE id = 1`); err != nil {
				_ = tx.Rollback()
				writerDone <- err
				return
			}
			if err := tx.Commit(); err != nil {
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
		t.Fatalf("record head %d, head inside the copy %d, rows inside the copy %d: they must agree", ref.Record.StoreHeadSequence, headInCopy, inCopy)
	}
}

func TestSnapshotOverTheSizeCapUploadsNothing(t *testing.T) {
	f := newFixture(t, ChunkSize)
	f.cfg.MaxPlaintextBytes = 1 << 10
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if len(f.chunks.chunks) != 0 || len(f.tape.events) != 0 {
		t.Fatalf("a refused copy left %d chunks and %d events", len(f.chunks.chunks), len(f.tape.events))
	}
	if entries, _ := os.ReadDir(f.cfg.TempDir); len(entries) != 0 {
		t.Fatalf("temp dir not cleaned: %d entries", len(entries))
	}
}

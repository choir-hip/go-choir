package snapshot

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	_ "modernc.org/sqlite"
)

// The first test of the capture-static-copy lane
// (docs/design/capture-static-copy-2026-11-15.md, section 2). It proves, with
// no production code, that the page copy the design relies on is a consistent
// image of the transaction's head under concurrent writes:
//
//  1. the head row is the first statement of the read transaction, and the
//     copy of every page read through sqlite_dbpage in that transaction opens
//     as exactly that state (page 1 and WAL content included: the writer's
//     autocheckpoint is off, so the pinned state lives mostly in the WAL and
//     the main file on disk is NOT the state);
//  2. writers commit throughout the copy and the file grows meanwhile; none
//     of it leaks into the copy, and no writer waits for the copy;
//  3. the copy opens on a read-only handle without changing a byte of it,
//     and the cross-check (page size, page count, head row) passes on it;
//  4. the cross-check refuses a copy that lost a page.

const copyTestInvariant = 10 // every writer transaction inserts this many rows and bumps the head

func TestStaticCopyIsAConsistentImageOfThePinnedHead(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.db")
	db, err := dbx.OpenSQLite(statePath, dbx.SQLiteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	// The writer keeps every commit in the WAL: the main file lags behind.
	if _, err := db.Write.ExecContext(ctx, `PRAGMA wal_autocheckpoint=0`); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE rows (id INTEGER PRIMARY KEY, body BLOB NOT NULL)`,
		`CREATE TABLE head (id INTEGER PRIMARY KEY CHECK (id = 1), seq INTEGER NOT NULL)`,
		`INSERT INTO head VALUES (1, 0)`,
	} {
		if _, err := db.Write.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	commit := func() error {
		tx, err := db.Write.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		blob := make([]byte, 1024)
		for i := 0; i < copyTestInvariant; i++ {
			if _, err := tx.ExecContext(ctx, `INSERT INTO rows (body) VALUES (?)`, blob); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE head SET seq = seq + 1 WHERE id = 1`); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	for i := 0; i < 600; i++ { // about 6 MB of rows, all in the WAL
		if err := commit(); err != nil {
			t.Fatal(err)
		}
	}
	mainBefore := fileSize(t, statePath)
	walBefore := fileSize(t, statePath+"-wal")
	if walBefore < mainBefore {
		t.Logf("note: the WAL (%d bytes) is smaller than the main file (%d bytes)", walBefore, mainBefore)
	}

	// Writers run for the whole copy.
	var writes atomic.Int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := commit(); err != nil {
				t.Errorf("a writer commit failed during the copy: %v", err)
				return
			}
			writes.Add(1)
		}
	}()

	// The copy: head first, then the page geometry, then every page.
	scratch := filepath.Join(dir, "state.db.snapshot-scratch")
	tx, err := db.Read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	var headSeq int64
	if err := tx.QueryRowContext(ctx, `SELECT seq FROM head WHERE id = 1`).Scan(&headSeq); err != nil {
		t.Fatal(err)
	}
	var pageSize int
	var pageCount int64
	if err := tx.QueryRowContext(ctx, `PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&pageCount); err != nil {
		t.Fatal(err)
	}
	writesAtHead := writes.Load()
	out, err := os.OpenFile(scratch, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT pgno, data FROM sqlite_dbpage`)
	if err != nil {
		t.Fatal(err)
	}
	var pages int64
	for rows.Next() {
		var pgno int64
		var data sql.RawBytes
		if err := rows.Scan(&pgno, &data); err != nil {
			t.Fatal(err)
		}
		if pgno != pages+1 || len(data) != pageSize {
			t.Fatalf("page stream out of order or short: page %d of %d bytes after %d pages", pgno, len(data), pages)
		}
		if _, err := out.Write(data); err != nil {
			t.Fatal(err)
		}
		pages++
		if pages%50 == 0 {
			time.Sleep(5 * time.Millisecond) // let the writer commit in the middle of the copy
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	_ = rows.Close()
	if err := out.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil { // the pin ends here
		t.Fatal(err)
	}
	close(stop)
	wg.Wait()

	writesDuring := writes.Load() - writesAtHead
	if writesDuring < 20 {
		t.Fatalf("only %d writer commits landed during the copy: the test did not exercise concurrent writes", writesDuring)
	}
	if pages != pageCount {
		t.Fatalf("the copy holds %d pages, the transaction's snapshot had %d", pages, pageCount)
	}
	var headNow int64
	if err := db.Read.QueryRowContext(ctx, `SELECT seq FROM head WHERE id = 1`).Scan(&headNow); err != nil {
		t.Fatal(err)
	}
	if headNow <= headSeq {
		t.Fatalf("the live head did not move during the copy (%d -> %d)", headSeq, headNow)
	}
	t.Logf("copy of %d pages of %d bytes at head %d; %d writer commits during it, live head now %d; main file was %d bytes, WAL %d bytes before the copy",
		pages, pageSize, headSeq, writesDuring, headNow, mainBefore, walBefore)

	// The main file on disk is not the pinned state: the copy had to come from
	// the pager (WAL included), not from reading the file.
	if mainSHA := sha256File(t, statePath); mainSHA == sha256File(t, scratch) {
		t.Fatal("the live main file equals the copy: the test did not put the pinned state in the WAL")
	}

	// Open the copy read-only. Nothing may change a byte of it.
	before := sha256File(t, scratch)
	copyDB, err := dbx.OpenSQLite(scratch, dbx.SQLiteOptions{ReadOnly: true, ReadConns: 1})
	if err != nil {
		t.Fatalf("open the copy: %v", err)
	}
	if err := crossCheckCopy(ctx, copyDB.Read, pageSize, pageCount, headSeq); err != nil {
		t.Fatalf("cross-check of a faithful copy: %v", err)
	}
	var rowCount int64
	if err := copyDB.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM rows`).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if rowCount != headSeq*copyTestInvariant {
		t.Fatalf("the copy holds %d rows at head %d, want %d: it is not one transaction's state", rowCount, headSeq, headSeq*copyTestInvariant)
	}
	var quick string
	if err := copyDB.Read.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&quick); err != nil || quick != "ok" {
		t.Fatalf("quick_check of the copy: %q, %v", quick, err)
	}
	if err := copyDB.Close(); err != nil {
		t.Fatal(err)
	}
	if after := sha256File(t, scratch); after != before {
		t.Fatal("opening, querying and closing the copy read-only changed its bytes")
	}
	for _, side := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(scratch + side); err == nil {
			t.Logf("note: %s%s is left beside the copy after close; the design deletes it with the copy", filepath.Base(scratch), side)
		}
	}

	// A copy that lost its last page is refused by the cross-check.
	short := filepath.Join(dir, "short.db")
	raw, err := os.ReadFile(scratch)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(short, raw[:len(raw)-pageSize], 0o600); err != nil {
		t.Fatal(err)
	}
	shortDB, err := dbx.OpenSQLite(short, dbx.SQLiteOptions{ReadOnly: true, ReadConns: 1})
	if err == nil {
		err = crossCheckCopy(ctx, shortDB.Read, pageSize, pageCount, headSeq)
		_ = shortDB.Close()
	}
	if err == nil {
		t.Fatal("a copy missing its last page passed the cross-check")
	}
	t.Logf("a short copy is refused: %v", err)
}

// crossCheckCopy is the check the design puts between the copy and its
// witness: the opened copy's page size, page count and head row must be the
// transaction's.
func crossCheckCopy(ctx context.Context, db *sql.DB, pageSize int, pageCount, headSeq int64) error {
	var gotSize int
	var gotCount, gotSeq int64
	if err := db.QueryRowContext(ctx, `PRAGMA page_size`).Scan(&gotSize); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&gotCount); err != nil {
		return err
	}
	if gotSize != pageSize || gotCount != pageCount {
		return fmt.Errorf("the copy has %d pages of %d bytes, the transaction had %d of %d", gotCount, gotSize, pageCount, pageSize)
	}
	if err := db.QueryRowContext(ctx, `SELECT seq FROM head WHERE id = 1`).Scan(&gotSeq); err != nil {
		return err
	}
	if gotSeq != headSeq {
		return fmt.Errorf("the copy's head is %d, the transaction's is %d", gotSeq, headSeq)
	}
	return nil
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func sha256File(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

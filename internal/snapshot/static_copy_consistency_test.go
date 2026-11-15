package snapshot

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
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
// (docs/design/capture-static-copy-2026-11-15.md, sections 2.1 and 2.3). It
// proves, with no production code, that the page copy the design relies on is
// a consistent image of the transaction's head under concurrent writes:
//
//  1. the head row is the first statement of the read transaction, and the
//     copy of every page read through sqlite_dbpage in that transaction opens
//     as exactly that state (page 1 and WAL content included);
//  2. writers commit throughout the copy, the file grows meanwhile, and none
//     of it leaks into the copy; the copy is paced on writer progress, not on
//     sleeps;
//  3. the copy opens on a read-only handle without changing a byte of it, and
//     the cross-check (page size, page count, EVERY column of the head row,
//     nil head handled) passes on it;
//  4. the cross-check refuses a copy that lost a page, a copy whose head
//     differs in a column other than the sequence, and a nil head on one side;
//  5. it holds in two configurations: the pinned state living only in the WAL
//     (autocheckpoint off), and the production default (autocheckpoint on,
//     concurrent PASSIVE checkpoints, a freelist from deleted rows).

const copyTestInvariant = 10 // every writer transaction inserts this many rows and bumps the head

// copyHead is the projection head's ten columns, as store.Head reads them.
type copyHead struct {
	ComputerID, CanonicalEventHead, DesiredEventHead, EffectiveEventHead, PendingTransitionRef string
	DesiredStateCommitment, EffectiveStateCommitment, ReducerVersion                           string
	Sequence, CredentialRevocationEpoch                                                        int64
}

const copyHeadSQL = `SELECT computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, COALESCE(pending_transition_ref, ''), desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch FROM heads WHERE computer_id = 'c'`

type rowQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// readCopyHead returns nil for an empty table, like store.scanProjectionHead.
func readCopyHead(ctx context.Context, q rowQueryer) (*copyHead, error) {
	var h copyHead
	err := q.QueryRowContext(ctx, copyHeadSQL).Scan(&h.ComputerID, &h.Sequence, &h.CanonicalEventHead, &h.DesiredEventHead, &h.EffectiveEventHead, &h.PendingTransitionRef, &h.DesiredStateCommitment, &h.EffectiveStateCommitment, &h.ReducerVersion, &h.CredentialRevocationEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func TestStaticCopyIsAConsistentImageOfThePinnedHead(t *testing.T) {
	t.Run("everything_in_the_wal", func(t *testing.T) { runStaticCopyCase(t, false) })
	t.Run("production_defaults_with_checkpoints_and_a_freelist", func(t *testing.T) { runStaticCopyCase(t, true) })
}

func runStaticCopyCase(t *testing.T, production bool) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.db")
	db, err := dbx.OpenSQLite(statePath, dbx.SQLiteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	if !production {
		// The writer keeps every commit in the WAL: the main file lags behind.
		if _, err := db.Write.ExecContext(ctx, `PRAGMA wal_autocheckpoint=0`); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{
		`CREATE TABLE rows (id INTEGER PRIMARY KEY, body BLOB NOT NULL)`,
		`CREATE TABLE heads (computer_id TEXT PRIMARY KEY, sequence INTEGER NOT NULL, canonical_event_head TEXT NOT NULL, desired_event_head TEXT NOT NULL, effective_event_head TEXT NOT NULL, pending_transition_ref TEXT, desired_state_commitment TEXT NOT NULL, effective_state_commitment TEXT NOT NULL, reducer_version TEXT NOT NULL, credential_revocation_epoch INTEGER NOT NULL)`,
		`INSERT INTO heads VALUES ('c', 0, 'h0', 'd0', 'e0', NULL, 'ds0', 'es0', 'v1', 0)`,
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
		// Every column that can differ moves with the sequence.
		if _, err := tx.ExecContext(ctx, `UPDATE heads SET sequence = sequence + 1, canonical_event_head = 'h' || (sequence + 1), desired_event_head = 'd' || (sequence + 1), effective_event_head = 'e' || (sequence + 1), desired_state_commitment = 'ds' || (sequence + 1), effective_state_commitment = 'es' || (sequence + 1), credential_revocation_epoch = (sequence + 1) / 7 WHERE computer_id = 'c'`); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	for i := 0; i < 600; i++ { // about 6 MB of rows
		if err := commit(); err != nil {
			t.Fatal(err)
		}
	}
	if production {
		// A freelist: delete the older half of the rows, as retirement does.
		if _, err := db.Write.ExecContext(ctx, `DELETE FROM rows WHERE id <= 3000`); err != nil {
			t.Fatal(err)
		}
		// The deleted rows must not change the invariant the test checks.
	}
	rowsBase := int64(0)
	if production {
		rowsBase = -3000
	}
	mainBefore := fileSize(t, statePath)
	walBefore := fileSize(t, statePath+"-wal")

	// Writers run for the whole copy; in the production case a checkpointer
	// runs beside them.
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
	if production {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				case <-time.After(20 * time.Millisecond):
				}
				var busy, logFrames, done int
				if err := db.Write.QueryRowContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &done); err != nil {
					t.Errorf("a concurrent checkpoint failed: %v", err)
					return
				}
			}
		}()
	}

	// The copy: head first, then the page geometry, then every page.
	scratch := filepath.Join(dir, "state.db.snapshot-scratch")
	tx, err := db.Read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	head, err := readCopyHead(ctx, tx)
	if err != nil || head == nil {
		t.Fatalf("head: %v, %v", head, err)
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
	lastSeen := writesAtHead
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
			// Pace on writer progress: wait for at least one more commit.
			deadline := time.Now().Add(30 * time.Second)
			for writes.Load() <= lastSeen {
				if time.Now().After(deadline) {
					t.Fatal("the writer made no progress for 30 s during the copy")
				}
				time.Sleep(time.Millisecond)
			}
			lastSeen = writes.Load()
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
	if writesDuring < pages/50 {
		t.Fatalf("only %d writer commits landed during the copy of %d pages", writesDuring, pages)
	}
	if pages != pageCount {
		t.Fatalf("the copy holds %d pages, the transaction's snapshot had %d", pages, pageCount)
	}
	headNow, err := readCopyHead(ctx, db.Read)
	if err != nil || headNow == nil || headNow.Sequence <= head.Sequence {
		t.Fatalf("the live head did not move during the copy: %+v, %v", headNow, err)
	}
	t.Logf("production=%v: copy of %d pages of %d bytes at head %d; %d writer commits during it, live head now %d; main file was %d bytes, WAL %d bytes before the copy",
		production, pages, pageSize, head.Sequence, writesDuring, headNow.Sequence, mainBefore, walBefore)

	if !production {
		// The main file on disk is not the pinned state: the copy had to come
		// from the pager (WAL included), not from reading the file.
		if sha256File(t, statePath) == sha256File(t, scratch) {
			t.Fatal("the live main file equals the copy: the test did not put the pinned state in the WAL")
		}
	}

	// Open the copy read-only. Nothing may change a byte of it.
	before := sha256File(t, scratch)
	copyDB, err := dbx.OpenSQLite(scratch, dbx.SQLiteOptions{ReadOnly: true, ReadConns: 1})
	if err != nil {
		t.Fatalf("open the copy: %v", err)
	}
	if err := crossCheckCopy(ctx, copyDB.Read, pageSize, pageCount, head); err != nil {
		t.Fatalf("cross-check of a faithful copy: %v", err)
	}
	var rowCount int64
	if err := copyDB.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM rows`).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if want := head.Sequence*copyTestInvariant + rowsBase; rowCount != want {
		t.Fatalf("the copy holds %d rows at head %d, want %d: it is not one transaction's state", rowCount, head.Sequence, want)
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
			t.Logf("note: %s%s is left beside the copy after close; the design removes it with the copy (removeSQLiteFiles)", filepath.Base(scratch), side)
		}
	}

	// A copy that lost its last page is refused when opened.
	raw, err := os.ReadFile(scratch)
	if err != nil {
		t.Fatal(err)
	}
	short := filepath.Join(dir, "short.db")
	if err := os.WriteFile(short, raw[:len(raw)-pageSize], 0o600); err != nil {
		t.Fatal(err)
	}
	if shortDB, err := dbx.OpenSQLite(short, dbx.SQLiteOptions{ReadOnly: true, ReadConns: 1}); err == nil {
		err = crossCheckCopy(ctx, shortDB.Read, pageSize, pageCount, head)
		_ = shortDB.Close()
		if err == nil {
			t.Fatal("a copy missing its last page passed the cross-check")
		}
	}

	// A same-size copy of the same sequence whose canonical head differs in
	// ONE column other than the sequence is refused by the cross-check.
	other := filepath.Join(dir, "other.db")
	if err := os.WriteFile(other, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	otherDB, err := dbx.OpenSQLite(other, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherDB.Write.ExecContext(ctx, `UPDATE heads SET effective_state_commitment = 'tampered' WHERE computer_id = 'c'`); err != nil {
		t.Fatal(err)
	}
	if err := crossCheckCopy(ctx, otherDB.Read, pageSize, int64(fileSize(t, other))/int64(pageSize), head); err == nil {
		t.Fatal("a copy with the same sequence and a different effective_state_commitment passed the cross-check")
	} else {
		t.Logf("a different column is refused: %v", err)
	}
	// A nil head on the copy side (and the reverse) is refused, not equal.
	if _, err := otherDB.Write.ExecContext(ctx, `DELETE FROM heads`); err != nil {
		t.Fatal(err)
	}
	if err := crossCheckCopy(ctx, otherDB.Read, pageSize, int64(fileSize(t, other))/int64(pageSize), head); err == nil {
		t.Fatal("a copy with no head row passed against a non-nil head")
	}
	if err := crossCheckCopy(ctx, otherDB.Read, pageSize, int64(fileSize(t, other))/int64(pageSize), nil); err != nil {
		t.Fatalf("nil head on both sides must be equal: %v", err)
	}
	_ = otherDB.Close()
}

// crossCheckCopy is the check the design puts between the copy and its
// witness: the opened copy's page size, page count and head row (every
// column; nil equals nil only) must be the transaction's.
func crossCheckCopy(ctx context.Context, db *sql.DB, pageSize int, pageCount int64, want *copyHead) error {
	var gotSize int
	var gotCount int64
	if err := db.QueryRowContext(ctx, `PRAGMA page_size`).Scan(&gotSize); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&gotCount); err != nil {
		return err
	}
	if gotSize != pageSize || gotCount != pageCount {
		return fmt.Errorf("the copy has %d pages of %d bytes, the transaction had %d of %d", gotCount, gotSize, pageCount, pageSize)
	}
	got, err := readCopyHead(ctx, db)
	if err != nil {
		return err
	}
	switch {
	case got == nil && want == nil:
		return nil
	case got == nil || want == nil:
		return fmt.Errorf("the copy's head is %v, the transaction's is %v", got, want)
	case *got != *want:
		return fmt.Errorf("the copy's head is %+v, the transaction's is %+v", *got, *want)
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

package computerversion

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

// A planned restart cancels the snapshot's witness mid-scan
// (docs/problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md). Ways that
// could fail, written before the code:
//
//  W1. the digest of a keyed table runs to the end of the scan after the
//      cancel;
//  W2. the digest of a table without a primary key does (it sorts the rows,
//      and the sorter's merge takes no context);
//  W3. the merge of spilled runs does.

func cancelFixture(t *testing.T) *sql.DB {
	t.Helper()
	return sqliteFixture(t,
		`CREATE TABLE keyed (id INTEGER PRIMARY KEY, body TEXT NOT NULL)`,
		`CREATE TABLE nopk (a INTEGER NOT NULL, body TEXT NOT NULL)`,
		`WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x<400000) INSERT INTO keyed SELECT x, hex(randomblob(40)) FROM c`,
		`WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x<400000) INSERT INTO nopk SELECT x, hex(randomblob(40)) FROM c`,
	)
}

// W1 and W2: cancel while the scan is in rows, on the real driver.
func TestDigestsTxCancelledMidScanUnwindsPromptly(t *testing.T) {
	db := cancelFixture(t)
	e := SQLiteStateExtractor{DB: db, Database: "d"}
	// How long the whole scan takes, so the cancel is known to land inside it.
	full := func() time.Duration {
		tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		start := time.Now()
		if _, err := e.DigestsTx(context.Background(), tx); err != nil {
			t.Fatal(err)
		}
		return time.Since(start)
	}()
	if full < 400*time.Millisecond {
		t.Skipf("the whole scan takes %s; too fast to land a cancel inside it", full)
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	cause := errors.New("cancelled for a planned restart (test)")
	time.AfterFunc(full/4, func() { cancel(cause) })
	start := time.Now()
	_, err = e.DigestsTx(ctx, tx)
	took := time.Since(start)
	if err == nil {
		t.Fatalf("the scan finished (%s of %s) instead of stopping at the cancel", took, full)
	}
	if after := took - full/4; after > time.Second {
		t.Fatalf("the scan took %s after the cancel (%s total of %s), want under a second", after, took, full)
	}
	if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
		t.Fatalf("rollback after the cancel: %v", rbErr)
	}
}

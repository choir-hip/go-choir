package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestOnlineGCSessionOpenTransaction characterizes what a CALL DOLT_GC() does
// to a write transaction that is open on another connection of the same
// embedded engine. Dolt's session-aware safepoint controller can keep such a
// transaction's chunks only for sessions registered through
// SessionCommandBegin; the embedded driver (dolthub/driver v2.2.0) never
// calls it, so a pass issued on one connection cannot see the roots another
// holds in memory. See docs/problems/paper-remaining-store-stall-holders-2026-10-23.md
// section 1, "Is a pass on the read connection safe".
//
// The writer opens an explicit transaction and inserts large text values (their
// chunks are written at statement time and reachable only from the session's
// pending edits), a pass is issued, the writer inserts more and commits.
//
//   - "writeDB" is today's shape: the pass holds the one write connection, so
//     the writer cannot be mid-transaction. Every row must commit.
//   - "readDB" is the proposed shape. The test logs what happens and asserts
//     only that the store stays usable and no row is half-committed.
func TestOnlineGCSessionOpenTransaction(t *testing.T) {
	for _, issue := range []string{"writeDB", "readDB"} {
		issue := issue
		t.Run(issue, func(t *testing.T) {
			s := openTestStore(t)
			ctx := context.Background()
			if _, err := s.db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS gc_mid_tx (id INT PRIMARY KEY, body LONGTEXT NOT NULL)"); err != nil {
				t.Fatal(err)
			}
			writeProbeRows(t, s, 0, 200) // garbage and a baseline, so the pass has work
			big := strings.Repeat("y", 300<<10)
			const pre, post = 40, 10
			insert := func(exec func(string, ...any) error, from, to int) error {
				for i := from; i < to; i++ {
					if err := exec("INSERT INTO gc_mid_tx (id, body) VALUES (?, ?)", i, fmt.Sprintf("%d-%s", i, big)); err != nil {
						return fmt.Errorf("insert %d: %w", i, err)
					}
				}
				return nil
			}
			var txErr error
			switch issue {
			case "writeDB":
				tx, err := s.db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				exec := func(q string, a ...any) error { _, e := tx.ExecContext(ctx, q, a...); return e }
				if txErr = insert(exec, 0, pre); txErr == nil {
					txErr = tx.Commit() // the pass below cannot overlap the transaction: one connection
				}
				if txErr != nil {
					_ = tx.Rollback()
					t.Fatalf("first half: %v", txErr)
				}
				if err := s.runOnlineDoltGC(ctx); err != nil {
					t.Fatal(err)
				}
				if err := insert(func(q string, a ...any) error { _, e := s.db.ExecContext(ctx, q, a...); return e }, pre, pre+post); err != nil {
					t.Fatal(err)
				}
			case "readDB":
				tx, err := s.db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				exec := func(q string, a ...any) error { _, e := tx.ExecContext(ctx, q, a...); return e }
				txErr = insert(exec, 0, pre)
				if txErr == nil {
					// The transaction is open, its large values written, nothing
					// committed: the pass lands here.
					if _, err := s.readDB.ExecContext(ctx, "CALL DOLT_GC()"); err != nil {
						_ = tx.Rollback()
						t.Fatalf("pass on the read connection: %v", err)
					}
					txErr = insert(exec, pre, pre+post)
				}
				if txErr == nil {
					txErr = tx.Commit()
				} else {
					_ = tx.Rollback()
				}
				t.Logf("write transaction open across a pass on the read connection: outcome %v", txErr)
			}
			var n int
			if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM gc_mid_tx").Scan(&n); err != nil {
				t.Fatalf("read after: %v", err)
			}
			switch {
			case issue == "writeDB" && n != pre+post:
				t.Fatalf("rows %d, want %d", n, pre+post)
			case issue == "readDB" && txErr == nil && n != pre+post:
				t.Fatalf("committed with rows %d, want %d", n, pre+post)
			case issue == "readDB" && txErr != nil && n != 0:
				t.Fatalf("failed transaction left %d rows", n)
			}
			assertProbeCount(t, s, 200)
			if _, err := s.db.ExecContext(ctx, "INSERT INTO gc_probe (id, body) VALUES (1000000, 'after')"); err != nil {
				t.Fatalf("write after: %v", err)
			}
			if _, err := s.db.ExecContext(ctx, "CALL DOLT_GC()"); err != nil {
				t.Fatalf("a second pass: %v", err)
			}
		})
	}
}

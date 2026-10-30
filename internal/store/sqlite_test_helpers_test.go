package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// sqliteTableDigest is a SHA-256 over a table's rows in rowid order, a stand-in
// for the Dolt table hash these tests used to compare. It streams the rows.
func sqliteTableDigest(t *testing.T, s *Store, table string) string {
	t.Helper()
	rows, err := s.db.Query("SELECT * FROM " + table + " ORDER BY rowid")
	if err != nil {
		t.Fatalf("digest of %s: %v", table, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%v\x1e", vals)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// The embedded-engine mutex and its per-call-path counters are gone with the
// move to SQLite (one writer plus a read pool; no process-wide engine lock).
// Tests that asserted "this read path is not scanned" through those counters
// keep their functional assertions; the counters read empty.

type engineMutexOpStats struct{ Calls uint64 }

func (s *Store) EngineMutexStats() map[string]engineMutexOpStats { return nil }

type engineMutexLive struct{ HolderOp string }

func (s *Store) EngineMutexLive() *engineMutexLive { return nil }

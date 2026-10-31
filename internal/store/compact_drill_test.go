package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// TestCompactDrillOnFile runs the compaction on a real store file named by
// CHOIR_COMPACT_DRILL_STATE: a private copy of a computer's state.db (the
// paper's, from a reflink snapshot of its disk), never a live store. It
// stages the copy, swaps it in as a planned boot would, and checks the
// swapped file against the original table by table, which the live run
// cannot do while the original moves. Sizes, counts and times only.
func TestCompactDrillOnFile(t *testing.T) {
	statePath := os.Getenv("CHOIR_COMPACT_DRILL_STATE")
	if statePath == "" {
		t.Skip("CHOIR_COMPACT_DRILL_STATE names no store file")
	}
	dbPath := filepath.Join(filepath.Dir(statePath), "store.db")
	if StateDBPath(dbPath) != statePath {
		t.Fatalf("the drill file must be named state.db")
	}
	ctx := context.Background()
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	done, err := s.PrepareCompactCopy(ctx)
	if err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	t.Logf("drill: copy staged in %s", time.Since(start).Round(time.Second))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	outcome, err := RebaseOntoCompactCopy(dbPath, true)
	if err != nil || !strings.HasPrefix(outcome, "rebased") {
		t.Fatalf("RebaseOntoCompactCopy = %q, %v", outcome, err)
	}
	t.Logf("drill: %s in %s", outcome, time.Since(start).Round(time.Millisecond))

	counts := func(path string) map[string]int64 {
		db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for rows.Next() {
			var n string
			_ = rows.Scan(&n)
			names = append(names, n)
		}
		rows.Close()
		out := map[string]int64{}
		for _, n := range names {
			var c int64
			if err := db.QueryRow(`SELECT COUNT(*) FROM "` + strings.ReplaceAll(n, `"`, `""`) + `"`).Scan(&c); err != nil {
				t.Fatal(err)
			}
			out[n] = c
		}
		var head string
		_ = db.QueryRow(`SELECT computer_id || ':' || sequence || ':' || canonical_event_head FROM computer_event_projection_heads`).Scan(&head)
		out["head:"+head] = 0
		return out
	}
	start = time.Now()
	orig, swapped := counts(statePath+precompactSuffix), counts(statePath)
	if len(orig) != len(swapped) {
		t.Fatalf("tables: %d in the original, %d in the copy", len(orig), len(swapped))
	}
	var total int64
	for name, n := range orig {
		m, ok := swapped[name]
		if !ok || m != n {
			t.Fatalf("%s: %d rows in the original, %d (%v) in the copy", name, n, m, ok)
		}
		total += n
	}
	t.Logf("drill: %d tables and the head agree, %d rows, counted in %s", len(orig)-1, total, time.Since(start).Round(time.Second))
}

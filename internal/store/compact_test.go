package store

// Guarded compaction (compact.go). Ways it can fail, written before the tests:
//
//   - C1: it compacts a file that is not mostly free (needless boot cost), or
//     skips one that is;
//   - C2: it runs with too little disk, or over the live-size bound, and
//     fills the disk or stalls boot;
//   - C3: the swap loses rows, or leaves a stale -wal that SQLite would
//     replay over the new file, or leaves the temp file;
//   - C4: the copy is not incremental auto_vacuum, so later deletes never
//     return pages;
//   - C5: a crashed earlier attempt's temp file is trusted or blocks the next;
//   - C6: incremental_vacuum holds the writer past its bound, or runs on a
//     file that is not incremental.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

func compactTestFile(t *testing.T, rowsKept, rowsDeleted int) string {
	t.Helper()
	path := t.TempDir() + "/state.db"
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE blob_rows (id INTEGER PRIMARY KEY, payload TEXT)`); err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("x", 2000)
	tx, _ := db.Begin()
	for i := 0; i < rowsKept+rowsDeleted; i++ {
		if _, err := tx.Exec(`INSERT INTO blob_rows (id, payload) VALUES (?, ?)`, i, payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if rowsDeleted > 0 {
		if _, err := db.Exec(`DELETE FROM blob_rows WHERE id >= ?`, rowsKept); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func compactTestCount(t *testing.T, path string) (rows int, av int) {
	t.Helper()
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.QueryRow(`SELECT COUNT(*) FROM blob_rows`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA auto_vacuum`).Scan(&av); err != nil {
		t.Fatal(err)
	}
	return rows, av
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func TestCompactReplacesAMostlyFreeFile(t *testing.T) {
	path := compactTestFile(t, 100, 2000)
	before := fileSize(t, path)
	// A crashed earlier attempt's leftovers are removed, not trusted (C5).
	if err := os.WriteFile(path+compactTempSuffix, []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	done, err := compactStateFile(path)
	if err != nil || !done {
		t.Fatalf("compactStateFile = %v, %v", done, err)
	}
	after := fileSize(t, path)
	if after*5 > before {
		t.Fatalf("file %d -> %d bytes, want under a fifth", before, after)
	}
	rows, av := compactTestCount(t, path)
	if rows != 100 {
		t.Fatalf("rows = %d, want 100 (C3)", rows)
	}
	if av != 2 {
		t.Fatalf("auto_vacuum = %d, want 2 (C4)", av)
	}
	for _, suffix := range []string{compactTempSuffix, compactTempSuffix + "-wal", compactTempSuffix + "-shm"} {
		if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
			t.Fatalf("%s left behind", suffix)
		}
	}
}

func TestCompactSkipsWhenNotMostlyFree(t *testing.T) {
	path := compactTestFile(t, 2000, 100) // about 5% free
	before := fileSize(t, path)
	done, err := compactStateFile(path)
	if err != nil || done {
		t.Fatalf("compactStateFile = %v, %v; want a quiet skip (C1)", done, err)
	}
	if fileSize(t, path) != before {
		t.Fatal("file changed")
	}
}

func TestCompactSkipsOnTooLittleDiskOrTooMuchLive(t *testing.T) {
	path := compactTestFile(t, 100, 2000)
	before := fileSize(t, path)
	oldFree, oldMax := compactFreeBytes, compactMaxLiveBytes
	defer func() { compactFreeBytes, compactMaxLiveBytes = oldFree, oldMax }()
	compactFreeBytes = func(string) (int64, error) { return 1000, nil }
	if done, err := compactStateFile(path); done || err == nil || !strings.Contains(err.Error(), "free") {
		t.Fatalf("low disk: %v, %v (C2)", done, err)
	}
	compactFreeBytes = oldFree
	compactMaxLiveBytes = 1000
	if done, err := compactStateFile(path); done || err == nil || !strings.Contains(err.Error(), "bound") {
		t.Fatalf("over the live bound: %v, %v (C2)", done, err)
	}
	if fileSize(t, path) != before {
		t.Fatal("a skipped compaction changed the file")
	}
	if _, err := os.Stat(path + compactTempSuffix); !os.IsNotExist(err) {
		t.Fatal("temp file left behind")
	}
}

// C6: after a compaction, deletes return pages with incremental_vacuum, in
// bounded chunks; a file that was not compacted is left alone.
func TestReclaimFreePagesIsBoundedAndOnlyOnIncrementalFiles(t *testing.T) {
	oldChunk, oldPause, oldMax := incrementalVacuumChunk, incrementalVacuumPause, incrementalVacuumMaxPages
	defer func() {
		incrementalVacuumChunk, incrementalVacuumPause, incrementalVacuumMaxPages = oldChunk, oldPause, oldMax
	}()
	incrementalVacuumChunk, incrementalVacuumPause, incrementalVacuumMaxPages = 50, 0, 120

	path := compactTestFile(t, 100, 2000)
	if _, err := compactStateFile(path); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec(`WITH RECURSIVE c(i) AS (SELECT 0 UNION ALL SELECT i+1 FROM c WHERE i<1999) INSERT INTO blob_rows (id, payload) SELECT i+100000, hex(zeroblob(1000)) FROM c`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM blob_rows WHERE id >= 100000`); err != nil {
		t.Fatal(err)
	}
	var free int64
	_ = db.QueryRow(`PRAGMA freelist_count`).Scan(&free)
	if free < 100 {
		t.Fatalf("freelist = %d, the setup freed too little", free)
	}
	s := &Store{db: db}
	freed, err := s.reclaimFreePages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if freed == 0 || freed > int64(incrementalVacuumMaxPages)+int64(incrementalVacuumChunk) {
		t.Fatalf("freed %d pages, want 1..%d", freed, incrementalVacuumMaxPages+incrementalVacuumChunk)
	}
	// A plain (non-incremental) file is left alone.
	plain := compactTestFile(t, 10, 500)
	pdb, _ := sql.Open("sqlite", dbx.SQLiteDSN(plain, dbx.SQLiteOptions{}, false))
	defer pdb.Close()
	if n, err := (&Store{db: pdb}).reclaimFreePages(context.Background()); err != nil || n != 0 {
		t.Fatalf("plain file reclaimed %d, %v", n, err)
	}
	_ = fmt.Sprint
}

// An Open on a store whose file is mostly free compacts it and keeps its rows.
func TestOpenNeverCompacts(t *testing.T) {
	s, path := trimTestOpen(t)
	if _, err := s.db.Exec(`CREATE TABLE compact_probe (id INTEGER PRIMARY KEY, payload TEXT)`); err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("y", 4000)
	tx, _ := s.db.Begin()
	for i := 0; i < 3000; i++ {
		if _, err := tx.Exec(`INSERT INTO compact_probe VALUES (?, ?)`, i, payload); err != nil {
			t.Fatal(err)
		}
	}
	_ = tx.Commit()
	if _, err := s.db.Exec(`DELETE FROM compact_probe WHERE id >= 10`); err != nil {
		t.Fatal(err)
	}
	statePath := s.statePath
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before := fileSize(t, statePath)
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	// Boot stays light: compaction is its own operation, never Open's.
	if after := fileSize(t, statePath); after*3 <= before {
		t.Fatalf("state file %d -> %d, Open compacted", before, after)
	}
}

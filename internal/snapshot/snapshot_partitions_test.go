package snapshot

// Telemetry partitions in a snapshot (docs/design/one-tape-durability-2026-10-31.md
// section 6c). Ways it could fail, written before the code:
//
//   - S1: a partition is not in the snapshot, or is restored with other rows
//     than it held (a restore loses up to a week of telemetry below the head);
//   - S2: an unchanged (closed) partition is uploaded again by the next
//     snapshot: the point of partitions is that its chunks dedup;
//   - S3: a restore keeps a stale partition of another lineage that was in
//     the partition directory, or installs partitions while naming no
//     directory and loses them silently;
//   - S4: a partition path escapes the partition directory;
//   - S5: the host is not told the partition chunks belong to the root, so
//     its sweep collects them;
//   - S6: the hold is not taken before the read transaction, or not released.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

func partitionDB(t *testing.T, dir, name string, rows int) *dbx.SQLiteDB {
	t.Helper()
	db, err := dbx.OpenSQLite(filepath.Join(dir, name), dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Write.Exec(`CREATE TABLE choir_events (tape_seq INTEGER PRIMARY KEY, body TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < rows; i++ {
		if _, err := db.Write.Exec(`INSERT INTO choir_events (tape_seq, body) VALUES (?, ?)`, i+1, fmt.Sprintf("%s row %d %0500d", name, i, i)); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func partitionRows(t *testing.T, path string) []string {
	t.Helper()
	db, err := dbx.OpenSQLite(path, dbx.SQLiteOptions{ReadConns: 1, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Read.Query(`SELECT body FROM choir_events ORDER BY tape_seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestSnapshotCarriesPartitionsAndDedupsClosedOnes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 64<<10)
	live := t.TempDir()
	closed := partitionDB(t, live, "tool_result-20261001.db", 3000) // several pages
	today := partitionDB(t, live, "tool_result-20261031.db", 10)
	held, released := 0, 0
	listed := false
	f.cfg.Partitions = &PartitionSource{
		Hold: func() func() {
			held++
			if listed {
				t.Error("the hold was taken after the listing (S6)")
			}
			return func() { released++; listed = false }
		},
		List: func(context.Context, *sql.Tx) ([]PartitionFile, error) {
			listed = true
			return []PartitionFile{
				{Path: "telemetry/tool_result-20261001.db", DB: closed.Read},
				{Path: "telemetry/tool_result-20261031.db", DB: today.Read},
			}, nil
		},
	}
	first, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if held != 1 || released != 1 || listed {
		t.Fatalf("hold taken %d, released %d (S6)", held, released)
	}
	if len(first.Record.Partitions) != 2 || first.Stats.PartitionFiles != 2 {
		t.Fatalf("partitions in the record: %+v (S1)", first.Record.Partitions)
	}
	// S5: the root declares every chunk, the partitions' included.
	declared := map[string]bool{}
	for _, d := range f.chunks.roots[len(f.chunks.roots)-1] {
		declared[d] = true
	}
	for _, part := range first.Record.Partitions {
		for _, d := range part.Chunks {
			if !declared[d] {
				t.Fatalf("partition chunk %s is not declared with the root (S5)", d)
			}
		}
	}

	// S2: the next snapshot uploads nothing of the closed partition; the open
	// one changed and is uploaded.
	if _, err := today.Write.Exec(`INSERT INTO choir_events (tape_seq, body) VALUES (99, 'new')`); err != nil {
		t.Fatal(err)
	}
	f.chunks.resetPuts()
	second, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Record.Partitions[0].Chunks, second.Record.Partitions[0].Chunks) {
		t.Fatal("the closed partition's chunks changed between snapshots (S2)")
	}
	puts := map[string]bool{}
	for _, d := range f.chunks.puts {
		puts[d] = true
	}
	for _, d := range second.Record.Partitions[0].Chunks {
		if puts[d] {
			t.Fatalf("the closed partition's chunk %s was uploaded again (S2)", d)
		}
	}
	if reflect.DeepEqual(first.Record.Partitions[1].Chunks, second.Record.Partitions[1].Chunks) {
		t.Fatal("the open partition's new row is not in the second snapshot")
	}

	// S1, S3: restore into a directory holding a stale partition; the
	// snapshot's partitions replace it, row for row.
	dest := t.TempDir()
	partDir := filepath.Join(dest, "workspace", "telemetry")
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(partDir, "tool_result-20200101.db")
	if err := os.WriteFile(stale, []byte("another lineage"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := f.restoreConfig(filepath.Join(dest, "state.db"))
	cfg.PartitionDir = partDir
	record, err := Restore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Partitions) != 2 {
		t.Fatalf("restored record names %d partitions", len(record.Partitions))
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("a stale partition survived the restore (S3): %v", err)
	}
	for _, c := range []struct {
		name string
		db   *dbx.SQLiteDB
	}{{"tool_result-20261001.db", closed}, {"tool_result-20261031.db", today}} {
		want := partitionRows(t, c.db.Path)
		got := partitionRows(t, filepath.Join(partDir, c.name))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s restored %d rows, live holds %d (S1)", c.name, len(got), len(want))
		}
	}

	// S3: a restore that names no partition directory refuses a snapshot
	// with partitions (every candidate has them: no usable snapshot).
	bare := f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	if _, err := Restore(ctx, bare); err == nil {
		t.Fatal("a restore with no partition directory accepted a snapshot with partitions (S3)")
	}
}

func TestSnapshotRefusesAPartitionPathOutsideItsDirectory(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 16<<10)
	db := partitionDB(t, t.TempDir(), "x.db", 1)
	for _, bad := range []string{"../state.db", "telemetry/../state.db", "telemetry/a/b.db", "telemetry/", "other/x.db", "telemetry/x"} {
		f.cfg.Partitions = &PartitionSource{List: func(context.Context, *sql.Tx) ([]PartitionFile, error) {
			return []PartitionFile{{Path: bad, DB: db.Read}}, nil
		}}
		if _, err := TakeSnapshot(ctx, f.cfg); err == nil {
			t.Errorf("path %q was accepted (S4)", bad)
		}
	}
}

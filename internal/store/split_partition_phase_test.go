package store

// The partition phase of the header/body split (partition_split.go,
// migrateTelemetryPartitionFiles). Ways it can fail, written before the tests
// (review finding on the first deploy):
//
//   - P1: a state file already split with a partition still in the old layout
//     reports no work, so the retry boot binds no gate and the partition
//     rewrite runs under the 20 s boot window, forever;
//   - P2: the partition rewrite reports no progress, so the host's stall
//     detector kills a boot that is copying;
//   - P3: the preflight does not count the partition rewrite peak, so a disk
//     that holds the state copy runs out mid-partition;
//   - P4: a partition that would not fit is rewritten anyway (ENOSPC mid-copy)
//     instead of refusing with the typed refusal and leaving the file as it was;
//   - P5: the batched copy loses or duplicates a row at a batch edge.

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// plantOldPartition writes an old-layout partition file with n rows beside the
// store at dbPath and returns its path.
func plantOldPartition(t *testing.T, dbPath string, n int) string {
	t.Helper()
	dir := TelemetryPartitionDir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "tool_result-20261030.db")
	ddl, err := os.ReadFile(filepath.Join("testdata", "presplit", "partition.sql"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(string(ddl)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if _, err := db.Exec(`INSERT INTO choir_events (tape_seq, op_index, canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at, run_id, seq, stream_seq, trajectory_id, event_kind)
			VALUES (?, 0, ?, 'choir.event', 'owner', 'h', ?, '{"kind":"tool.result"}', '2026-10-30', '2026-10-30', 'r', ?, ?, 't', 'tool.result')`,
			100+i, fmt.Sprintf("ev-%04d", i), fmt.Sprintf(`{"payload":{"phase":"p%d"}}`, i), i+1, i+1); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func partitionRowCounts(t *testing.T, path string) (hasBodyCol bool, headers, bodies int) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var b int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('choir_events') WHERE name='body'`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM choir_events`).Scan(&headers)
	_ = db.QueryRow(`SELECT COUNT(*) FROM choir_event_bodies`).Scan(&bodies)
	return b > 0, headers, bodies
}

// P1, P2, P5: the state file is split, a partition is not; the store reports
// work, the migration rewrites the partition with advancing progress, and no
// row is lost across batch edges.
func TestSplitPartitionOnlyWorkIsReportedAndProgresses(t *testing.T) {
	f := newSplitFixture(t, 30, 10)
	if err := MigrateHeaderBodySplit(f.dbPath, nil); err != nil {
		t.Fatal(err)
	}
	if needed, err := HeaderBodySplitNeeded(f.dbPath); err != nil || needed {
		t.Fatalf("split state, no partition: needed=%v err=%v", needed, err)
	}
	prev := partitionSplitBatchRows
	partitionSplitBatchRows = 7
	t.Cleanup(func() { partitionSplitBatchRows = prev })
	path := plantOldPartition(t, f.dbPath, 50)

	needed, err := HeaderBodySplitNeeded(f.dbPath)
	if err != nil || !needed {
		t.Fatalf("old partition beside a split state file: needed=%v err=%v", needed, err)
	}
	var seen []uint64
	if err := MigrateHeaderBodySplit(f.dbPath, func(n uint64) { seen = append(seen, n) }); err != nil {
		t.Fatal(err)
	}
	if len(seen) < 10 {
		t.Fatalf("progress reported %d times for 100 copied rows in batches of 7: %v", len(seen), seen)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] < seen[i-1] {
			t.Fatalf("progress fell: %v", seen)
		}
	}
	if seen[len(seen)-1] < 100 {
		t.Fatalf("progress ended at %d, want at least the 100 rows copied", seen[len(seen)-1])
	}
	if hasBody, h, b := partitionRowCounts(t, path); hasBody || h != 50 || b != 50 {
		t.Fatalf("partition after: body col=%v headers=%d bodies=%d", hasBody, h, b)
	}
	if needed, err := HeaderBodySplitNeeded(f.dbPath); err != nil || needed {
		t.Fatalf("after the migration: needed=%v err=%v", needed, err)
	}
}

// P3: the preflight of an old state file counts the largest partition's peak.
func TestSplitPreflightCountsPartitionPeak(t *testing.T) {
	need := func(withPartition bool) (int64, int64) {
		f := newSplitFixture(t, 30, 10)
		var size int64
		if withPartition {
			info, err := os.Stat(plantOldPartition(t, f.dbPath, 400))
			if err != nil {
				t.Fatal(err)
			}
			size = info.Size()
		}
		splitTestDisk(t, 1<<20)
		var refusal *HeaderBodySplitRefusal
		if err := MigrateHeaderBodySplit(f.dbPath, nil); !errors.As(err, &refusal) {
			t.Fatalf("err = %v, want a refusal", err)
		}
		return refusal.Need, size
	}
	base, _ := need(false)
	with, size := need(true)
	if with-base != 3*size || size == 0 {
		t.Fatalf("need with a partition = %d, without = %d, partition size %d: want a difference of 3x the size", with, base, size)
	}
}

// P4: a partition that cannot fit refuses with the typed refusal and stays old.
func TestSplitPartitionRefusesWhenItCannotFit(t *testing.T) {
	f := newSplitFixture(t, 30, 10)
	if err := MigrateHeaderBodySplit(f.dbPath, nil); err != nil {
		t.Fatal(err)
	}
	path := plantOldPartition(t, f.dbPath, 50)
	splitTestDisk(t, 1<<20) // under 3x + headroom
	var refusal *HeaderBodySplitRefusal
	if err := MigrateHeaderBodySplit(f.dbPath, nil); !errors.As(err, &refusal) || refusal.Need <= refusal.Free {
		t.Fatalf("err = %v, want a refusal with need > free", err)
	}
	if hasBody, h, _ := partitionRowCounts(t, path); !hasBody || h != 50 {
		t.Fatalf("a refused partition changed: body col=%v rows=%d", hasBody, h)
	}
	// Open refuses the same way rather than rewriting.
	if _, err := Open(f.dbPath); !errors.As(err, &refusal) {
		t.Fatalf("Open err = %v, want the refusal", err)
	}
}

package snapshot

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// P10 (continued): a chunk repeated inside one snapshot (zeroed pages) is
// sealed to one digest and uploaded once.
func TestRepeatedChunksInOneSnapshotUploadOnce(t *testing.T) {
	f := newFixture(t, 1<<10)
	dek, err := f.cipher.ExportKeyForEscrow(context.Background(), testComputer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	up := newChunkUploader(f.cfg, dek, ChunkSize, 5, cancel)
	up.start(ctx)
	zero := bytes.Repeat([]byte{0}, ChunkSize)
	for i := 0; i < 4; i++ {
		buf := <-up.free
		up.jobs <- chunkJob{index: i, data: append(buf, zero...)}
	}
	buf := <-up.free
	up.jobs <- chunkJob{index: 4, data: append(buf, bytes.Repeat([]byte{7}, ChunkSize)...)}
	up.finish()
	if err := context.Cause(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.chunks.puts) != 2 || up.repeated.Load() != 3 {
		t.Fatalf("%d uploads and %d repeats for four identical chunks and one other", len(f.chunks.puts), up.repeated.Load())
	}
	if up.digests[0] != up.digests[3] || up.digests[0] == up.digests[4] {
		t.Fatalf("digests %v", up.digests)
	}
}

// TestSnapshotThroughputOnALargeStore is a measurement, not a regression test:
// set CHOIR_SNAPSHOT_BENCH_BYTES (for example 2147483648, past SQLite's 1 GiB
// lock-byte page) to build a store of about that size, stream it with real
// sealing into a discarding chunk store, restore it and compare witnesses.
func TestSnapshotThroughputOnALargeStore(t *testing.T) {
	raw := os.Getenv("CHOIR_SNAPSHOT_BENCH_BYTES")
	if raw == "" {
		t.Skip("set CHOIR_SNAPSHOT_BENCH_BYTES to measure")
	}
	size, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || size <= 0 {
		t.Fatalf("CHOIR_SNAPSHOT_BENCH_BYTES=%q", raw)
	}
	ctx := context.Background()
	f := newFixture(t, 1<<10)
	row := make([]byte, 1<<20)
	started := time.Now()
	for built := int64(0); built < size; built += int64(len(row)) {
		if _, err := f.db.Write.Exec(`INSERT INTO blobs (body, note) VALUES (randomblob(1048576), 'bench')`); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("built about %d bytes in %s", size, time.Since(started).Round(time.Millisecond))

	f.cfg.MaxDuration = 30 * time.Minute
	started = time.Now()
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	st := ref.Stats
	t.Logf("snapshot: %d pages (%d bytes), %d chunks, uploaded %d, in %s", st.Pages, ref.Record.PlaintextSize, st.Chunks, st.ChunksUploaded, elapsed.Round(time.Millisecond))
	t.Logf("page stream %s (%.0f pages/s, %.0f MiB/s), witness %s, read transaction %s", st.StreamTime.Round(time.Millisecond), float64(st.Pages)/st.StreamTime.Seconds(), float64(ref.Record.PlaintextSize)/(1<<20)/st.StreamTime.Seconds(), st.WitnessTime.Round(time.Millisecond), st.ReadTxTime.Round(time.Millisecond))

	dest := filepath.Join(t.TempDir(), "state.db")
	started = time.Now()
	if _, err := Restore(ctx, f.restoreConfig(dest)); err != nil {
		t.Fatal(err)
	}
	t.Logf("restore (reassemble only) %s", time.Since(started).Round(time.Millisecond))
	restored, err := dbx.OpenSQLite(dest, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var check string
	if err := restored.Read.QueryRow(`PRAGMA quick_check`).Scan(&check); err != nil || check != "ok" {
		t.Fatalf("quick_check = %q, %v", check, err)
	}
	var rows int
	if err := restored.Read.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	var live int
	if err := f.db.Read.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if rows != live {
		t.Fatalf("restored %d rows, live %d", rows, live)
	}
	_ = sql.ErrNoRows
}

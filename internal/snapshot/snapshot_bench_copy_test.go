package snapshot

// The build-VM bench the design names as a precondition of the deploy
// (docs/design/capture-static-copy-2026-11-15.md, section 4 step 8): the
// writer as built (read, inline hash, buffered write, sync) over a synthetic
// store, reporting the copy rate in MB/s together with the pin, the witness
// and the upload. It is a test input, not a product switch: it runs only when
// SNAPSHOT_BENCH_BYTES (the store's size) and SNAPSHOT_BENCH_DIR (a directory
// on a volume with the store, a scratch of the same size and 2 GiB free) are
// set, and skips otherwise. Nothing is asserted about the rate: it is measured.

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

// discardChunks keeps no chunk: a bench of 8 GB must not hold it in memory.
type discardChunks struct{ puts int }

func (d *discardChunks) PutChunk(context.Context, string, []byte) error { d.puts++; return nil }
func (d *discardChunks) GetChunk(context.Context, string) ([]byte, error) {
	return nil, fmt.Errorf("the bench keeps no chunk")
}

func TestBenchCopyRate(t *testing.T) {
	size, _ := strconv.ParseInt(os.Getenv("SNAPSHOT_BENCH_BYTES"), 10, 64)
	dir := os.Getenv("SNAPSHOT_BENCH_DIR")
	if size <= 0 || dir == "" {
		t.Skip("SNAPSHOT_BENCH_BYTES and SNAPSHOT_BENCH_DIR are not set")
	}
	base, err := os.MkdirTemp(dir, "capture-bench-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	t.Setenv("TMPDIR", base)

	const rowBytes = 1 << 20
	rows := int(size / rowBytes)
	f := pagedFixture(t, 4096, false, 0, 0)
	rng := rand.New(rand.NewSource(1))
	body := make([]byte, rowBytes)
	built := time.Now()
	for i := 0; i < rows; i++ {
		rng.Read(body[:4096]) // distinct leading page; the rest is zero pages
		if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, ?, ?)`, i+1, body, fmt.Sprintf("row %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("built a store of %d rows in %s", rows, time.Since(built).Round(time.Second))
	f.cfg.ScratchDir = base + "/snapshot-tmp"
	f.cfg.Chunks = &discardChunks{}
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	st := ref.Stats
	mbps := float64(st.ScratchBytes) / 1e6 / st.CopyTime.Seconds()
	t.Logf("BENCH store %d bytes (%d pages): copy %s = %.0f MB/s; pin %s; witness %s; upload %s = %.0f MB/s; capture total pin+witness+upload",
		st.ScratchBytes, st.Pages, st.CopyTime.Round(time.Millisecond), mbps, st.PinTime.Round(time.Millisecond),
		st.WitnessTime.Round(time.Millisecond), st.UploadTime.Round(time.Millisecond), float64(st.ScratchBytes)/1e6/st.UploadTime.Seconds())
}

package store

// TestCompactMeasure measures the compact copy that reaches the paper
// (docs/design/paper-compaction-2026-10-31.md) on a synthetic store built by
// TestChurnBench in the "current" layout: the copy's duration, its size, the
// process's peak resident memory while it runs, and the 4 MiB snapshot chunks
// an hour (or CHOIR_COMPACT_MEASURE_HOURS) of the paper's writes leaves
// unchanged on the grown file against the compact copy.
//
//	CHOIR_CHURN_BENCH_OG_ROWS=500000 CHOIR_CHURN_BENCH_DIR=/dir CHOIR_CHURN_BENCH_LAYOUTS=current \
//	  go test ./internal/store -run TestChurnBench -v -timeout 3h
//	CHOIR_COMPACT_MEASURE_DB=/dir/churn-current.db go test ./internal/store -run TestCompactMeasure -v

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// rssKiB is the process's resident set, from ps.
func rssKiB() int64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return n
}

// peakRSS runs fn and reports the highest resident set sampled every 100 ms,
// and the set before it began.
func peakRSS(fn func()) (before, peak int64) {
	before = rssKiB()
	peak = before
	var mu sync.Mutex
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			case <-time.After(100 * time.Millisecond):
				if r := rssKiB(); r > 0 {
					mu.Lock()
					peak = max(peak, r)
					mu.Unlock()
				}
			}
		}
	}()
	fn()
	close(stop)
	wg.Wait()
	return before, max(peak, rssKiB())
}

func copyFileForMeasure(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCompactMeasure(t *testing.T) {
	src := os.Getenv("CHOIR_COMPACT_MEASURE_DB")
	if src == "" {
		t.Skip("CHOIR_COMPACT_MEASURE_DB unset")
	}
	hours, _ := strconv.Atoi(os.Getenv("CHOIR_COMPACT_MEASURE_HOURS"))
	if hours <= 0 {
		hours = 1
	}
	grown, compact := src+".grown", src+".measure-compact"
	for _, p := range []string{grown, compact} {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(p + suffix)
		}
	}
	copyFileForMeasure(t, src, grown)

	var copyErr error
	started := time.Now()
	before, peak := peakRSS(func() { copyErr = vacuumIntoCopy(context.Background(), src, compact) })
	if copyErr != nil {
		t.Fatal(copyErr)
	}
	t.Logf("compact copy: %s, source %d MB -> copy %d MB, RSS before %d MB, peak %d MB (+%d MB)",
		time.Since(started).Round(time.Millisecond), fileSizeOrZero(src)>>20, fileSizeOrZero(compact)>>20, before>>10, peak>>10, (peak-before)>>10)

	layout := churnLayout{name: "current"}
	for _, c := range []struct{ name, path string }{{"grown", grown}, {"compact", compact}} {
		db := openChurnDB(t, c.path, false)
		g := newChurnGen(777)
		w := &churnWriter{g: g, layout: layout}
		w.prepare(t, db)
		tx, _ := db.Begin()
		for i := 0; i < 2000; i++ {
			if err := w.putObject(tx, g.object()); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		checkpointTruncate(t, db)
		base := chunkDigests(t, c.path)
		var line []string
		for h := 1; h <= hours; h++ {
			for i := 0; i < 1520; i++ {
				if err := churnHourTx(w, g, db, layout, i); err != nil {
					t.Fatal(err)
				}
			}
			checkpointTruncate(t, db)
			after := chunkDigests(t, c.path)
			same := 0
			for i, d := range base {
				if i < len(after) && after[i] == d {
					same++
				}
			}
			line = append(line, fmt.Sprintf("after %dh %d of %d unchanged (%.1f%%)", h, same, len(base), 100*float64(same)/float64(len(base))))
		}
		_ = db.Close()
		t.Logf("%s: %d MB, %s", c.name, fileSizeOrZero(c.path)>>20, strings.Join(line, "; "))
	}
}

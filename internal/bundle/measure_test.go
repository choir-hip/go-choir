package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// TestBundleVerifyCost measures verify on a synthetic chain: wall time,
// signatures a second, and the peak heap above the baseline. It runs only
// with CHOIR_BUNDLE_MEASURE_EVENTS set (for example 1000000); the cost
// statement in docs/design/bundle-authenticity-2026-10-31.md section 5 is
// its output. CHOIR_BUNDLE_MEASURE_DIR keeps the generated bundle.
func TestBundleVerifyCost(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("CHOIR_BUNDLE_MEASURE_EVENTS"))
	if n <= 0 {
		t.Skip("set CHOIR_BUNDLE_MEASURE_EVENTS to measure")
	}
	dir := os.Getenv("CHOIR_BUNDLE_MEASURE_DIR")
	if dir == "" {
		dir = filepath.Join(t.TempDir(), "measure")
	}
	key := newSigningKey(t)
	trust := trustOf(t, key)
	if _, err := os.Stat(filepath.Join(dir, ManifestName)); err == nil {
		t.Fatalf("%s holds a bundle signed by another key; remove it", dir)
	}
	genStart := time.Now()
	m := synth{Events: n, Key: key, RevokeEvery: 1000, Tag: "measure"}.write(t, dir).Manifest
	t.Logf("generated %d events in %s", n, time.Since(genStart))
	var size int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
		prev := runtime.GOMAXPROCS(workers)
		runtime.GC()
		debug.FreeOSMemory()
		var base runtime.MemStats
		runtime.ReadMemStats(&base)
		var peak atomic.Uint64
		stop := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			var ms runtime.MemStats
			for {
				select {
				case <-stop:
					return
				case <-time.After(20 * time.Millisecond):
					runtime.ReadMemStats(&ms)
					if ms.HeapInuse > peak.Load() {
						peak.Store(ms.HeapInuse)
					}
				}
			}
		}()
		if p := os.Getenv("CHOIR_BUNDLE_MEASURE_PROFILE"); p != "" {
			f, err := os.Create(fmt.Sprintf("%s.%d", p, workers))
			if err != nil {
				t.Fatal(err)
			}
			_ = pprof.StartCPUProfile(f)
			defer f.Close()
		}
		start := time.Now()
		v, err := VerifyBundle(dir, trust)
		elapsed := time.Since(start)
		pprof.StopCPUProfile()
		close(stop)
		<-done
		runtime.GOMAXPROCS(prev)
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		perM := float64(elapsed) / float64(time.Second) / (float64(m.Counts.Receipts) / 1e6)
		heap := float64(peak.Load()-min(peak.Load(), base.HeapInuse)) / (1 << 20)
		t.Logf("workers=%d receipts=%d objects=%d signatures=%d bundle=%.0f MiB: verify %s (%.1f s per million receipts, %.0f signatures/s), peak heap above baseline %.0f MiB (%.0f MiB per million receipts), allocated %.0f MiB",
			workers, m.Counts.Receipts, m.Counts.Events+m.Counts.Payloads+m.Counts.PinReceipts, v.Signatures, float64(size)/(1<<20),
			elapsed.Round(time.Millisecond), perM, float64(v.Signatures)/elapsed.Seconds(), heap, heap/(float64(m.Counts.Receipts)/1e6),
			float64(after.TotalAlloc-base.TotalAlloc)/(1<<20))
	}
}

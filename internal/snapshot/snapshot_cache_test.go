package snapshot

import (
	"context"
	"os"
	"sync"
	"testing"
)

// Failure modes (written before the code):
//   - The stream never advises the kernel, so a whole-store read leaves the
//     source file cached against the guest's memory.
//   - The advice runs only at the end, so a 20 GB stream still holds the cache
//     while it runs: it must also run every sourceDropBytes.
//   - The path is wrong (an empty or relative name), so the advice drops
//     nothing: assert the path is an existing file.
//   - A failed stream skips the advice: the drop is deferred.
//   - The helper's failure fails the snapshot: it returns nothing and is
//     best effort.
func TestStreamDropsTheSourceCacheAsItGoes(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	type span struct{ off, n int64 }
	var spans []span
	old := dropSourceCache
	dropSourceCache = func(p string, off, n int64) {
		mu.Lock()
		paths = append(paths, p)
		spans = append(spans, span{off, n})
		mu.Unlock()
	}
	t.Cleanup(func() { dropSourceCache = old })

	f := newFixture(t, sourceDropBytes+ChunkSize*4)
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) < 2 {
		t.Fatalf("source cache dropped %d times, want a mid-stream drop and the final one", len(paths))
	}
	for _, p := range paths {
		if p != paths[0] {
			t.Fatalf("drops named different files: %q vs %q", p, paths[0])
		}
	}
	// Mid-stream drops are the ranges just read, in order and contiguous;
	// the last is the whole file.
	var next int64
	for _, s := range spans[:len(spans)-1] {
		if s.off != next || s.n < sourceDropBytes {
			t.Fatalf("mid-stream drop %+v, want offset %d and at least %d bytes", s, next, sourceDropBytes)
		}
		next = s.off + s.n
	}
	if last := spans[len(spans)-1]; last != (span{0, 0}) {
		t.Fatalf("final drop %+v, want the whole file", last)
	}
	if st, err := os.Stat(paths[0]); err != nil || st.IsDir() {
		t.Fatalf("dropped path %q is not the database file: %v", paths[0], err)
	}
}

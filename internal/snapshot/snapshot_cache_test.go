package snapshot

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

// Failure modes (written before the code):
//   - The copy and the upload never advise the kernel, so a whole-store write
//     and read leave the scratch file cached against the guest's memory.
//   - The advice runs only at the end, so a 20 GB copy still holds the cache
//     while it runs: it must also run every scratchSyncBytes written and every
//     sourceDropBytes uploaded.
//   - The path is wrong (an empty or relative name): assert it is the scratch
//     file, never the live store.
//   - The helper's failure fails the snapshot: it returns nothing and is best
//     effort.
func TestTheCopyAndTheUploadDropTheScratchCacheAsTheyGo(t *testing.T) {
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
	want := filepath.Join(f.cfg.ScratchDir, scratchFileName)
	var mid, whole int
	for i, p := range paths {
		if p != want {
			t.Fatalf("a drop named %q, want only the scratch %q", p, want)
		}
		switch {
		case spans[i] == (span{0, 0}):
			whole++
		case spans[i].n >= sourceDropBytes:
			mid++
		default:
			t.Fatalf("drop %+v is neither a whole-file drop nor a range of at least %d bytes", spans[i], sourceDropBytes)
		}
	}
	// A mid-copy and a mid-upload range, and a whole-file drop after the
	// witness and at the end of the upload.
	if mid < 2 || whole < 2 {
		t.Fatalf("%d range drops and %d whole-file drops: want at least 2 of each (%v)", mid, whole, spans)
	}
}

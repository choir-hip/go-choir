package platform

// Measures the collector's per-candidate unlink cost against a paper-scale
// snapshot root (two roots of 10,500 chunks), with and without the root
// generation shortcut. It asserts only that the shortcut asks the database
// nothing; the timings are logged for the report.

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestCollectorUnlinkCostWithAndWithoutTheGenerationShortcut(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	const roots, chunksPerRoot, candidates = 2, 10500, 2000
	for r := 0; r < roots; r++ {
		chunks := make([]string, 0, chunksPerRoot)
		for i := 0; i < chunksPerRoot; i++ {
			d := gcDigest(fmt.Sprintf("cost/%d/%d", r, i))
			f.writeAged("sha256/file-cas-chunks/"+snapVerifyComputer+"/"+d, []byte{1}, time.Hour)
			chunks = append(chunks, d)
		}
		if code := f.snapDeclare(h, fmt.Sprintf("cost-%d", r), int64(r+1), chunks...); code != http.StatusCreated {
			t.Fatalf("declare = %d", code)
		}
	}
	measure := func(prefix string, gen bool) (time.Duration, int64) {
		cands := make([]gcCandidate, 0, candidates)
		for i := 0; i < candidates; i++ {
			d, p := f.snapChunk(fmt.Sprintf("%s %d", prefix, i), 40*time.Hour)
			c := gcCandidate{ns: "file-cas-chunks", rel: "sha256/file-cas-chunks/" + snapVerifyComputer + "/" + d, path: p}
			if gen {
				c.rootGen, c.rootGenSet = f.store.snapshotRootGen.Load(), true
			}
			cands = append(cands, c)
		}
		before := f.service.snapshotRootRecheckQueries.Load()
		start := time.Now()
		for _, c := range cands {
			if op, _, err := f.service.unlinkIfStillUnreachable(f.ctx, c, time.Now().Add(-24*time.Hour)); err != nil || op != "deleted" {
				t.Fatalf("unlink = %q %v", op, err)
			}
		}
		return time.Since(start), f.service.snapshotRootRecheckQueries.Load() - before
	}
	withQuery, queries := measure("noshortcut", false)
	if queries != candidates {
		t.Fatalf("queries without the shortcut = %d", queries)
	}
	withShortcut, queries := measure("shortcut", true)
	if queries != 0 {
		t.Fatalf("queries with the shortcut = %d, want 0", queries)
	}
	t.Logf("%d unlinks: with a live-root query each %s (%s each); with the generation shortcut %s (%s each)",
		candidates, withQuery, withQuery/candidates, withShortcut, withShortcut/candidates)
}

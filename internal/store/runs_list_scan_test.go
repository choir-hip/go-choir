package store

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// GET /api/runs read the run kind twice. ListRunsByOwnerOG, the first scan,
// paged every choir.run row of every owner and computer through
// ListObjectsPage (a TopN over every remaining row per page, quadratic) and
// kept only the rows with no computer_id; ListLifecycleRunsByOwner, the second,
// returns the lifecycle rows
// (docs/archive/problems/paper-remaining-store-stall-holders-2026-10-23.md section 4).
//
// Ways the fix can fail, written before the code:
//  1. The answer changes: a run of another owner leaks in, a legacy run is
//     lost, a lifecycle run (computer_id set) is returned by the legacy half,
//     the order (created_at desc, run_id asc) or the limit window moves.
//  2. The keyset walk skips or repeats a run at a page boundary.
//  3. The read is still proportional to the kind: it calls ListObjectsPage on
//     the kind, or its allocation grows with the number of lifecycle runs.
//  4. The read errors, or answers differently, on a store without the
//     (kind, owner, computer) index (covered in internal/objectgraph).

const (
	scanOwnerA = "owner-scan-a"
	scanOwnerB = "owner-scan-b"
)

// listRunsByOwnerOGReference is ListRunsByOwnerOG as it was before the fix:
// the oracle of the equivalence test.
func (s *Store) listRunsByOwnerOGReference(ctx context.Context, ownerID string, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindRun)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, min(limit, len(objs)))
	for _, obj := range objs {
		if obj.OwnerID != ownerID {
			continue
		}
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if lifecycleRunProjection(obj, rec) {
			continue
		}
		runs = append(runs, rec)
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

type scanRun struct {
	id, owner, computer string
	state               types.RunState
	created             time.Time
	pad                 int // body padding bytes
}

func scanPutRuns(t *testing.T, s *Store, runs []scanRun) {
	t.Helper()
	const batch = 200
	for lo := 0; lo < len(runs); lo += batch {
		var objs []objectgraph.Object
		for _, r := range runs[lo:min(lo+batch, len(runs))] {
			rec := types.RunRecord{RunID: r.id, AgentID: "agent", OwnerID: r.owner, ComputerID: r.computer, State: r.state,
				Prompt: strings.Repeat("p", r.pad), CreatedAt: r.created, UpdatedAt: r.created}
			body, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			meta, err := json.Marshal(map[string]any{"run_id": r.id, "state": string(r.state), "computer_id": r.computer})
			if err != nil {
				t.Fatal(err)
			}
			id, err := objectgraph.BuildCanonicalID(ogKindRun, r.owner, objectgraph.StableSuffixFromKey(r.id))
			if err != nil {
				t.Fatal(err)
			}
			objs = append(objs, objectgraph.Object{CanonicalID: id, ObjectKind: ogKindRun, OwnerID: r.owner, ComputerID: r.computer,
				VersionID: r.id, ContentHash: objectgraph.ContentHash(ogKindRun, body, meta), Body: body, Metadata: meta,
				CreatedAt: r.created, UpdatedAt: r.created})
		}
		if err := s.ogStore.PutBatch(context.Background(), objectgraph.Batch{Objects: objs}); err != nil {
			t.Fatalf("seed runs: %v", err)
		}
	}
}

// scanMix builds n runs over two owners, three computers (one of them the
// empty, legacy value) and every lifecycle state, with created_at ties so the
// run_id tiebreak matters. It also writes non-run objects that must never
// appear.
func scanMix(n int) []scanRun {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	states := []types.RunState{"queued", "running", "completed", "failed", "cancelled"}
	runs := make([]scanRun, 0, n)
	for i := 0; i < n; i++ {
		owner := scanOwnerA
		if i%5 == 2 {
			owner = scanOwnerB
		}
		computer := ""
		switch i % 3 {
		case 1:
			computer = "computer-1"
		case 2:
			computer = "computer-2"
		}
		runs = append(runs, scanRun{
			id: fmt.Sprintf("run-%05d", (i*7919)%100000), owner: owner, computer: computer,
			state: states[i%len(states)], created: base.Add(time.Duration(i/4) * time.Second), // groups of 4 tie
			pad: 200,
		})
	}
	return runs
}

func runIDs(runs []types.RunRecord) string {
	ids := make([]string, len(runs))
	for i, r := range runs {
		ids[i] = r.RunID
	}
	return strings.Join(ids, ",")
}

func TestListRunsByOwnerEquivalentToTheFullKindScan(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	mix := scanMix(2400)
	scanPutRuns(t, s, mix)
	// Another kind that shares owner and computer must not be returned.
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("obj:%s:%s:decoy-%d", ogKindAgent, scanOwnerA, i)
		body := []byte(`{"run_id":"decoy"}`)
		decoy := objectgraph.Object{CanonicalID: id, ObjectKind: ogKindAgent, OwnerID: scanOwnerA, VersionID: "v",
			ContentHash: objectgraph.ContentHash(ogKindAgent, body, []byte(`{}`)), Body: body, Metadata: []byte(`{}`),
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err := s.ogStore.PutObject(ctx, decoy); err != nil {
			t.Fatalf("decoy: %v", err)
		}
	}

	wantLegacy := map[string]int{}
	for _, r := range mix {
		if r.computer == "" {
			wantLegacy[r.owner]++
		}
	}
	for _, owner := range []string{scanOwnerA, scanOwnerB, "owner-nobody"} {
		for _, limit := range []int{0, 1, 7, 100, 500, 5000} {
			want, err := s.listRunsByOwnerOGReference(ctx, owner, limit)
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.ListRunsByOwnerOG(ctx, owner, limit)
			if err != nil {
				t.Fatal(err)
			}
			if runIDs(got) != runIDs(want) {
				t.Fatalf("owner %s limit %d: ListRunsByOwnerOG differs from the full-kind scan\n got %s\nwant %s", owner, limit, runIDs(got), runIDs(want))
			}
			// Store.ListRunsByOwner defaults a non-positive limit to 50.
			viaLimit := limit
			if viaLimit <= 0 {
				viaLimit = 50
			}
			wantVia, err := s.listRunsByOwnerOGReference(ctx, owner, viaLimit)
			if err != nil {
				t.Fatal(err)
			}
			viaStore, err := s.ListRunsByOwner(ctx, owner, limit)
			if err != nil || runIDs(viaStore) != runIDs(wantVia) {
				t.Fatalf("owner %s limit %d: ListRunsByOwner differs (%v)", owner, limit, err)
			}
			for _, r := range got {
				if r.ComputerID != "" || r.OwnerID != owner {
					t.Fatalf("returned a run outside the legacy half: %+v", r)
				}
			}
		}
		if owner != "owner-nobody" {
			all, _ := s.ListRunsByOwnerOG(ctx, owner, 5000)
			if len(all) != wantLegacy[owner] || len(all) < 100 {
				t.Fatalf("owner %s: %d legacy runs, want %d (and at least 100 for the test to mean something)", owner, len(all), wantLegacy[owner])
			}
		}
	}

	// The keyset walk, page by page, at page sizes that cut the set at
	// different boundaries, against the kind scan filtered the old way.
	for _, owner := range []string{scanOwnerA, scanOwnerB} {
		want, err := s.listRunsByOwnerOGReference(ctx, owner, 1<<30)
		if err != nil {
			t.Fatal(err)
		}
		for _, size := range []int{1, 3, 50, 256, 4096} {
			got, err := s.listRunsByOwnerOGPageSize(ctx, owner, 1<<30, size)
			if err != nil {
				t.Fatal(err)
			}
			if runIDs(got) != runIDs(want) {
				t.Fatalf("owner %s page size %d differs from the full-kind scan: %d vs %d runs", owner, size, len(got), len(want))
			}
		}
	}

	// The whole GET /api/runs merge (legacy plus lifecycle) is unchanged by
	// construction: the lifecycle half is untouched and the legacy half is equal.
}

func engineOpCalls(s *Store, contains string) (calls int) {
	for op, st := range s.EngineMutexStats() {
		if strings.Contains(op, contains) {
			calls += int(st.Calls)
		}
	}
	return calls
}

func scanAlloc(fn func()) uint64 {
	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	fn()
	runtime.ReadMemStats(&b)
	return b.TotalAlloc - a.TotalAlloc
}

// lifecycleOnly seeds n lifecycle runs (computer_id set) for the owner and a
// fixed 40 legacy runs, with bodies large enough that reading them shows.
func lifecycleOnly(n int) []scanRun {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	runs := make([]scanRun, 0, n+40)
	for i := 0; i < n; i++ {
		runs = append(runs, scanRun{id: fmt.Sprintf("life-%06d", i), owner: scanOwnerA, computer: "computer-1", state: "completed", created: base.Add(time.Duration(i) * time.Second), pad: 1500})
	}
	for i := 0; i < 40; i++ {
		runs = append(runs, scanRun{id: fmt.Sprintf("legacy-%03d", i), owner: scanOwnerA, state: "completed", created: base.Add(time.Duration(i) * time.Second), pad: 1500})
	}
	return runs
}

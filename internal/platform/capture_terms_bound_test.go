package platform

// The title-and-label search must read what its caller uses, not every
// match (docs/problems/corpus-search-takes-two-seconds-on-the-paper-2026-11-19.md,
// the 07:55Z update: calls that returned about 1,500 rows took 8 to 14 seconds).
// Failure list, written before the code:
//
//   1. a search whose word matches 2,000 captures reads them all from the term
//      index into Go and sorts there, though the caller wants 30;
//   2. the rows read from og_objects are not bounded by the caller's candidates
//      (the id hints are read whatever their number);
//   3. the 30 rows are not the same 30, in the same order, as the old Go sort
//      gave (words matched, newest, id);
//   4. a capped search does not say how many matched;
//   5. the time window is applied after the cut, so a window that holds few of
//      the newest matches returns too few;
//   6. the journal line has no matched= beside rows=.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// broadFixture holds n captures whose title holds "harbor" (and, for every
// third, "quay"), fetched one minute apart, and reconciles the projection.
func broadFixture(t *testing.T, n int) (*ftFixture, []objectgraph.Object, time.Time) {
	t.Helper()
	f := newFTFixture(t, false)
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Duration(n+10) * time.Minute)
	var objs []objectgraph.Object
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("Harbor report %d", i)
		if i%3 == 0 {
			title += " quay"
		}
		objs = append(objs, f.capture(t, "owner-a", "computer-a", fmt.Sprintf("k%d", i), title, fmt.Sprintf("https://example.com/%d", i), "text", base.Add(time.Duration(i)*time.Minute)))
	}
	for start := 0; start < len(objs); start += 250 {
		f.put(t, objs[start:min(start+250, len(objs))]...)
	}
	if err := f.store.ReconcileCaptureIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	return f, objs, base
}

// oldOrder is the order the Go sort of captureTermCandidates gave: words
// matched, then newest fetched, then id, both descending.
func oldOrder(objs []objectgraph.Object, base time.Time, words []string) []string {
	type c struct {
		id      string
		words   int
		fetched time.Time
	}
	var cs []c
	for i, o := range objs {
		n := 0
		title := strings.ToLower(fmt.Sprintf("Harbor report %d", i))
		if i%3 == 0 {
			title += " quay"
		}
		for _, w := range words {
			if strings.Contains(" "+title+" ", " "+w+" ") {
				n++
			}
		}
		if n > 0 {
			cs = append(cs, c{o.CanonicalID, n, base.Add(time.Duration(i) * time.Minute)})
		}
	}
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if a.words != b.words {
			return a.words > b.words
		}
		if !a.fetched.Equal(b.fetched) {
			return a.fetched.After(b.fetched)
		}
		return a.id > b.id
	})
	out := make([]string, len(cs))
	for i, x := range cs {
		out[i] = x.id
	}
	return out
}

func TestTermSearchOfABroadWordReadsOnlyWhatTheCallerUsesAndSaysHowManyMatched(t *testing.T) {
	f, objs, base := broadFixture(t, 2000)
	req := CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor", "quay"}, Need: 1, Candidates: 30}
	before := f.store.retrievalState.reads.snapshot()
	resp, err := f.store.SearchCaptures(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	got := f.store.retrievalState.reads.snapshot().minus(before)
	if resp.Matched != 2000 || !resp.Capped || len(resp.Objects) != 30 {
		t.Fatalf("matched %d capped %v objects %d", resp.Matched, resp.Capped, len(resp.Objects))
	}
	if got.termRows != 30 || got.objectRows != 30 || got.objectStatements != 1 {
		t.Fatalf("read %+v; want 30 term rows, 30 object rows, 1 object statement", got)
	}
	want := oldOrder(objs, base, req.Words)[:30]
	for i, o := range resp.Objects {
		if o.CanonicalID != want[i] {
			t.Fatalf("row %d is %s, the old sort gave %s", i, o.CanonicalID, want[i])
		}
	}
}

func TestTermSearchReadsAtMostTheCapsOfCandidatesAndIdHints(t *testing.T) {
	f, objs, _ := broadFixture(t, 600)
	var ids []string
	for _, o := range objs {
		ids = append(ids, o.CanonicalID)
	}
	req := CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor"}, Need: 1, Candidates: 20, IDs: ids}
	before := f.store.retrievalState.reads.snapshot()
	resp, err := f.store.SearchCaptures(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	got := f.store.retrievalState.reads.snapshot().minus(before)
	if got.objectRows > int64(20+captureSearchMaxIDs) || len(resp.Objects) > 20+captureSearchMaxIDs {
		t.Fatalf("read %d object rows for 20 candidates and %d hints (bound %d)", got.objectRows, len(ids), 20+captureSearchMaxIDs)
	}
}

func TestTermSearchWindowIsAppliedBeforeTheCut(t *testing.T) {
	f, objs, base := broadFixture(t, 600)
	// The 30 newest matches are all after the window; the window holds the
	// oldest 100. A cut before the window would return none of them.
	since, until := base, base.Add(100*time.Minute)
	req := CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor"}, Need: 1, Candidates: 30,
		Since: since.Format(time.RFC3339), Until: until.Format(time.RFC3339)}
	resp, err := f.store.SearchCaptures(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Objects) != 30 || resp.Matched != 100 {
		t.Fatalf("objects %d matched %d, want 30 of the 100 in the window", len(resp.Objects), resp.Matched)
	}
	inWindow := map[string]bool{}
	for _, o := range objs[:100] {
		inWindow[o.CanonicalID] = true
	}
	for _, o := range resp.Objects {
		if !inWindow[o.CanonicalID] {
			t.Fatalf("%s is outside the window", o.CanonicalID)
		}
	}
}

func TestTermSearchJournalLineCarriesMatchedBesideRows(t *testing.T) {
	f, _, _ := broadFixture(t, 120)
	var lines []string
	f.store.retrieval.logf = func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	if _, err := f.store.SearchCaptures(context.Background(), CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor"}, Need: 1, Candidates: 10}); err != nil {
		t.Fatal(err)
	}
	var line string
	for _, l := range lines {
		if strings.Contains(l, "op=capture_search shape=terms") {
			line = l
		}
	}
	if !strings.Contains(line, " rows=10 matched=120 ") {
		t.Fatalf("terms line %q", line)
	}
}

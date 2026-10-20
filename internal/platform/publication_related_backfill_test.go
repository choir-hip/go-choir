package platform

// Ways the cites_capture backfill (related-stories design 5.2) can fail,
// written before the code. The backfill gives versions published before the
// edge existed the edges they would have been born with.
//
//  B1 It is not idempotent: a second run writes edges again, or leaves a
//     different set. The second run must write nothing and say so.
//  B2 It walks every version, not the latest: older versions are never read
//     by the walk, so they get no edges (and cost writes).
//  B3 It edges a publication that is not an Autopaper story (a hand
//     published Texture, another owner's story when an allowlist is set) or
//     one that is not published.
//  B4 It writes an edge to a capture Store B no longer has (a dangling
//     edge), or fails the whole run on one missing capture.
//  B5 A dry run writes.
//  B6 Paging by key skips or repeats a publication at a page boundary.
//  B7 A story whose edges do not match its manifest's distinct cited
//     captures passes silently; the report names it.
//  B8 Anyone can trigger it: the route must refuse a non-internal caller
//     and any method but POST.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// dropCitesCaptureEdges makes the store look as it did before the edge
// existed.
func dropCitesCaptureEdges(t *testing.T, l *relatedLab) {
	t.Helper()
	if _, err := l.store.corpus().Exec(`DELETE FROM og_edges WHERE kind = ?`, string(edgeCitesCapture)); err != nil {
		t.Fatalf("drop edges: %v", err)
	}
}

func countCitesCaptureEdges(t *testing.T, l *relatedLab) int {
	t.Helper()
	var n int
	if err := l.store.corpus().QueryRow(`SELECT COUNT(*) FROM og_edges WHERE kind = ? AND tombstone = FALSE`, string(edgeCitesCapture)).Scan(&n); err != nil {
		t.Fatalf("count edges: %v", err)
	}
	return n
}

func TestBackfillWritesLatestVersionEdgesOnceAndOnlyForStories(t *testing.T) { // B1, B2, B3, B5, B9
	l := newRelatedLab(t)
	a := l.capture("https://wire.example/a", "", "Port authority said the harbor reopens")
	b := l.capture("https://wire.example/b", "", "Ferry operators confirmed Monday service")
	c := l.capture("https://wire.example/c", "", "Inspectors cleared the east pier")

	v1 := l.publish("doc-harbor", 1, "Harbor reopens", a, b)
	v2 := l.publish("doc-harbor", 2, "Harbor reopens", a, c)
	l.publish("doc-canal", 1, "Canal dredged", b)
	// A hand-published Texture (owner mode) citing a capture is not a story.
	l.publishAs(labOwner, PublishModeOwner, nil, "doc-note", 1, "A private note", a)
	dropCitesCaptureEdges(t, l)
	if n := countCitesCaptureEdges(t, l); n != 0 {
		t.Fatalf("fixture: %d edges after the drop", n)
	}

	// A dry run reports and writes nothing.
	dry, err := l.svc.BackfillCitesCapture(context.Background(), true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !dry.DryRun || dry.EdgesWritten != 3 || countCitesCaptureEdges(t, l) != 0 {
		t.Fatalf("dry run = %+v, edges in store = %d; want 3 reported, 0 written", dry, countCitesCaptureEdges(t, l))
	}

	rep, err := l.svc.BackfillCitesCapture(context.Background(), false)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	// harbor v2 (a, c) and canal v1 (b): two stories, three edges. harbor
	// v1 is not the latest and the owner-mode note is not a story.
	if rep.Stories != 2 || rep.EdgesWritten != 3 || rep.EdgesExisting != 0 || rep.MismatchCount != 0 {
		t.Fatalf("backfill report = %+v, want 2 stories, 3 written, 0 existing, 0 mismatches", rep)
	}
	got := l.citesCapture(l.versionCanonicalID(labOwner, v2))
	sort.Strings(got)
	want := []string{a, c}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("harbor v2 edges = %v, want %v", got, want)
	}
	if e := l.citesCapture(l.versionCanonicalID(labOwner, v1)); len(e) != 0 {
		t.Fatalf("older version v1 was backfilled: %v", e)
	}
	if n := countCitesCaptureEdges(t, l); n != 3 {
		t.Fatalf("edges in store = %d, want 3 (note and v1 excluded)", n)
	}

	// Run twice: nothing new.
	again, err := l.svc.BackfillCitesCapture(context.Background(), false)
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if again.EdgesWritten != 0 || again.EdgesExisting != 3 || again.MismatchCount != 0 || countCitesCaptureEdges(t, l) != 3 {
		t.Fatalf("second run = %+v; want 0 written, 3 existing", again)
	}
}

func TestBackfillSkipsAnotherOwnersStoriesWhenAnAllowlistIsSet(t *testing.T) { // B3
	l := newRelatedLab(t)
	a := l.capture("https://wire.example/a", "", "Port authority said the harbor reopens")
	l.publish("doc-harbor", 1, "Harbor reopens", a)
	l.publishAs("tracer-owner", PublishModeAuto, nil, "doc-tracer", 1, "Tracer story", a)
	l.svc.SetAutopaperOwners([]string{labOwner})
	dropCitesCaptureEdges(t, l)
	rep, err := l.svc.BackfillCitesCapture(context.Background(), false)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if rep.Stories != 1 || rep.EdgesWritten != 1 {
		t.Fatalf("report = %+v, want only the allowed owner's story", rep)
	}
}

func TestBackfillSkipsAMissingCaptureAndKeepsGoing(t *testing.T) { // B4, B7
	l := newRelatedLab(t)
	a := l.capture("https://wire.example/a", "", "Port authority said the harbor reopens")
	b := l.capture("https://wire.example/b", "", "Ferry operators confirmed Monday service")
	l.publish("doc-harbor", 1, "Harbor reopens", a, b)
	dropCitesCaptureEdges(t, l)
	if _, err := l.store.corpus().Exec(`DELETE FROM og_objects WHERE canonical_id = ?`, b); err != nil {
		t.Fatalf("delete capture: %v", err)
	}
	rep, err := l.svc.BackfillCitesCapture(context.Background(), false)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if rep.EdgesWritten != 1 || rep.MissingCaptures != 1 {
		t.Fatalf("report = %+v, want 1 edge written and 1 missing capture counted", rep)
	}
	// The story has two distinct cited captures and one edge: the report
	// says so instead of passing it.
	if rep.MismatchCount != 1 || len(rep.Mismatches) != 1 || rep.Mismatches[0].Want != 2 || rep.Mismatches[0].Got != 1 {
		t.Fatalf("mismatches = %+v (count %d), want one story with want 2 got 1", rep.Mismatches, rep.MismatchCount)
	}
}

func TestBackfillPagesWithoutSkippingOrRepeating(t *testing.T) { // B6
	l := newRelatedLab(t)
	a := l.capture("https://wire.example/a", "", "Port authority said the harbor reopens")
	for _, doc := range []string{"doc-1", "doc-2", "doc-3", "doc-4", "doc-5"} {
		l.publish(doc, 1, "Story "+doc, a)
	}
	dropCitesCaptureEdges(t, l)
	prev := backfillPageSize
	backfillPageSize = 2
	t.Cleanup(func() { backfillPageSize = prev })
	rep, err := l.svc.BackfillCitesCapture(context.Background(), false)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if rep.Stories != 5 || rep.EdgesWritten != 5 || countCitesCaptureEdges(t, l) != 5 {
		t.Fatalf("report = %+v, edges = %d; want 5 stories, 5 edges across 3 pages", rep, countCitesCaptureEdges(t, l))
	}
}

func TestBackfillRouteIsInternalPostOnly(t *testing.T) { // B8
	l := newRelatedLab(t)
	h := NewHandler(l.svc)
	call := func(method string, internal bool, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/internal/platform/publications/related-backfill", strings.NewReader(body))
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		w := httptest.NewRecorder()
		h.HandleInternalRelatedBackfill(w, req)
		return w
	}
	if w := call(http.MethodGet, true, ""); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d, want 405", w.Code)
	}
	if w := call(http.MethodPost, false, `{}`); w.Code != http.StatusForbidden {
		t.Fatalf("POST without the internal caller = %d, want 403", w.Code)
	}
	// A body that is not JSON is a bad request, not a silent live run.
	if w := call(http.MethodPost, true, `{not json`); w.Code == http.StatusOK {
		t.Fatalf("a malformed body ran the backfill")
	}
}

func TestBackfillReportIsJSON(t *testing.T) {
	var rep RelatedBackfillReport
	raw, err := json.Marshal(rep)
	if err != nil || !strings.Contains(string(raw), `"dry_run"`) || !strings.Contains(string(raw), `"edges_written"`) || !strings.Contains(string(raw), `"mismatch_count"`) {
		t.Fatalf("report JSON = %s, %v", raw, err)
	}
}

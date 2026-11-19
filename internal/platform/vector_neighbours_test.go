package platform

// The nearest held captures to named captures (vector_neighbours.go; station
// V5, the event layer's vector shortlist). Failure list, written before the
// code:
//
//   N1 the nearest list is out of order, or holds the capture itself;
//   N2 another computer's capture (same words, same owner) is returned, or a
//      bound guest names another owner's or computer's scope and is answered;
//   N3 captures from before the window are returned;
//   N4 a private, tombstoned or superseded capture is returned as a neighbour,
//      or a private capture's own id is answered;
//   N5 a capture not embedded yet fails the request instead of being listed as
//      missing, or a down or absent embedder stops the answer (it needs none);
//   N6 the request costs a search per capture on the caller's side: many ids
//      are one request, and the per-capture limit and the bound on ids hold;
//   N7 the answer differs from the vector search of the same vector;
//   N8 the journal has no line for it, or the line carries a capture id.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func neighbourIDs(list []CaptureNeighbour) []string {
	var out []string
	for _, n := range list {
		out = append(out, n.CaptureID)
	}
	return out
}

func TestCaptureNeighboursNearestFirstScopedWindowedAndEligible(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	treaty := "quorum valdoria amber treaty ratified"
	f.capture(t, "o1", "c1", "q", "Treaty", treaty, now.Add(-time.Hour), false)
	f.capture(t, "o1", "c1", "near", "Treaty again", treaty+" today", now.Add(-2*time.Hour), false)
	f.capture(t, "o1", "c1", "mid", "Harvest", "quorum valdoria harvest", now.Add(-3*time.Hour), false)
	f.capture(t, "o1", "c1", "far", "Garden", "tomato basil garden", now.Add(-4*time.Hour), false)
	f.capture(t, "o1", "c1", "old", "Old treaty", treaty, now.Add(-30*24*time.Hour), false)
	f.capture(t, "o1", "c1", "private", "Private treaty", treaty, now.Add(-time.Hour), true)
	f.capture(t, "o1", "c2", "elsewhere", "Treaty elsewhere", treaty, now.Add(-time.Hour), false)
	f.capture(t, "o2", "c1", "stranger", "Treaty stranger", treaty, now.Add(-time.Hour), false)
	gone := f.capture(t, "o1", "c1", "gone", "Gone treaty", treaty, now.Add(-time.Hour), false)
	f.pass(t)
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, gone.CanonicalID); err != nil {
		t.Fatal(err)
	}
	id := func(key string) string { return mustCaptureID(t, "o1", key) }

	res, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{id("q")}, Limit: 10})
	if err != nil || res.Unavailable != "" {
		t.Fatalf("%+v %v", res, err)
	}
	got := neighbourIDs(res.Neighbours[id("q")])
	// N1: not itself; N2: not another computer's or owner's; N4: not private
	// or tombstoned. The month-old page is in range with no window. (The fake
	// embedder's vectors are word hashes, so the order is checked by score.)
	want := map[string]bool{id("near"): true, id("old"): true, id("mid"): true, id("far"): true}
	if len(got) != 4 {
		t.Fatalf("neighbours %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("neighbour %s is not one of the four held pages of this scope", g)
		}
	}
	for _, n := range res.Neighbours[id("q")] {
		switch n.CaptureID {
		case id("q"), id("private"), id("gone"), mustCaptureID(t, "o1", "elsewhere"), mustCaptureID(t, "o2", "stranger"):
			t.Fatalf("returned %s", n.CaptureID)
		}
	}
	for i := 1; i < len(res.Neighbours[id("q")]); i++ {
		if res.Neighbours[id("q")][i].Score > res.Neighbours[id("q")][i-1].Score {
			t.Fatalf("scores not descending: %+v", res.Neighbours[id("q")])
		}
	}
	// N3: a window of 14 days drops the month-old page.
	res, err = f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{id("q")}, Since: now.Add(-14 * 24 * time.Hour).Format("2006-01-02")})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range res.Neighbours[id("q")] {
		if n.CaptureID == id("old") {
			t.Fatal("a capture from before the window is a neighbour")
		}
	}
	if len(res.Neighbours[id("q")]) != 3 {
		t.Fatalf("windowed neighbours %v", neighbourIDs(res.Neighbours[id("q")]))
	}
	// N4: a private capture's own id is not answered.
	res, err = f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{id("private"), id("gone")}})
	if err != nil || len(res.Neighbours) != 0 || len(res.Missing) != 2 {
		t.Fatalf("private ids answered: %+v %v", res, err)
	}
	// N2: another computer's capture id named under this scope is not answered.
	res, err = f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{mustCaptureID(t, "o1", "elsewhere")}})
	if err != nil || len(res.Neighbours) != 0 || len(res.Missing) != 1 {
		t.Fatalf("another computer's id answered: %+v %v", res, err)
	}
}

func TestCaptureNeighboursNeedNoEmbedderAndListWhatIsNotEmbeddedYetAsMissing(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	f.capture(t, "o1", "c1", "a", "A", "quorum valdoria amber treaty ratified", now.Add(-time.Hour), false)
	f.capture(t, "o1", "c1", "b", "B", "quorum valdoria harvest", now.Add(-2*time.Hour), false)
	f.pass(t)
	late := f.capture(t, "o1", "c1", "late", "Late", "quorum valdoria amber", now, false) // no pass since
	f.emb.Down.Store(true)
	f.store.SetEmbedder(nil)
	res, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1",
		CaptureIDs: []string{mustCaptureID(t, "o1", "a"), late.CanonicalID}})
	if err != nil || res.Unavailable != "" {
		t.Fatalf("%+v %v", res, err)
	}
	if got := neighbourIDs(res.Neighbours[mustCaptureID(t, "o1", "a")]); len(got) != 1 || got[0] != mustCaptureID(t, "o1", "b") {
		t.Fatalf("neighbours of a: %v", got)
	}
	if len(res.Missing) != 1 || res.Missing[0] != late.CanonicalID || res.State != "exact" {
		t.Fatalf("missing %v state %q", res.Missing, res.State)
	}
}

func TestCaptureNeighboursAreOneRequestForManyCapturesWithinTheBounds(t *testing.T) {
	f := newVecFixture(t)
	base := time.Now().UTC().Add(-48 * time.Hour)
	ids := f.captures(t, "o1", "c1", "n", 30, base, func(i int) string { return fmt.Sprintf("alpha common words number%d", i%5) }, time.Minute)
	f.pass(t)
	res, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: ids, Limit: 4})
	if err != nil || len(res.Neighbours) != 30 {
		t.Fatalf("answered %d of 30: %v", len(res.Neighbours), err)
	}
	for id, list := range res.Neighbours {
		if len(list) != 4 {
			t.Fatalf("%s has %d neighbours, want the limit 4", id, len(list))
		}
		for _, n := range list {
			if n.CaptureID == id {
				t.Fatal("a capture is its own neighbour")
			}
		}
	}
	// N7: the same list as the vector search of the same stored vector.
	vec := embedder.FakeVector(embedder.DocumentInput("T n-3", "alpha common words number3"))
	direct, err := f.store.SearchVectors(context.Background(), VectorSearchRequest{Vector: vec, Owner: "o1", Computer: "c1", CaptureLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	var wantScores []float64
	for _, h := range direct.Captures {
		if h.CaptureID != ids[3] {
			wantScores = append(wantScores, h.Score)
		}
	}
	for i, n := range res.Neighbours[ids[3]] {
		if d := n.Score - wantScores[i]; d > 1e-3 || d < -1e-3 {
			t.Fatalf("neighbour %d score %v, vector search %v", i, n.Score, wantScores[i])
		}
	}
	// N6: the bound on ids.
	over := make([]string, captureNeighboursMaxIDs+1)
	for i := range over {
		over[i] = fmt.Sprintf("choir.web_capture:x%d", i)
	}
	if _, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: over}); err == nil {
		t.Fatal("more ids than the bound were accepted")
	}
	// An empty request, and one with no owner.
	if r, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1"}); err != nil || len(r.Neighbours) != 0 {
		t.Fatalf("empty: %+v %v", r, err)
	}
	if _, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{ComputerID: "c1", CaptureIDs: ids}); err == nil {
		t.Fatal("no owner accepted")
	}
}

func TestCaptureNeighboursJournalLineNamesTheCountsAndNoCaptureId(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	f.capture(t, "o1", "c1", "a", "A", "quorum valdoria amber treaty ratified", now.Add(-time.Hour), false)
	f.capture(t, "o1", "c1", "b", "B", "quorum valdoria harvest", now.Add(-2*time.Hour), false)
	f.pass(t)
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	if _, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{mustCaptureID(t, "o1", "a"), "choir.web_capture:nope"}}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"op=vector_neighbours", "shape=captures", "rows=1", "ids=2", "missing=1", "state=exact"} {
		if !strings.Contains(out, want) {
			t.Fatalf("journal lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "choir.web_capture") {
		t.Fatalf("the journal line names a capture:\n%s", out)
	}
}

// N2 over the route: a bound guest is answered for its own owner and computer
// only.
func TestCaptureNeighboursRouteAnswersABoundGuestForItsOwnScopeOnly(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	f.capture(t, "o1", "c1", "a", "A", "quorum valdoria amber treaty ratified", now.Add(-time.Hour), false)
	f.capture(t, "o1", "c1", "b", "B", "quorum valdoria harvest", now.Add(-2*time.Hour), false)
	f.pass(t)
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: f.og}), f.og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{
		"10.200.1.2": {Found: true, UserID: "o1", ComputerID: "c1"},
		"10.200.9.9": {Found: true, UserID: "o2", ComputerID: "c2"},
	}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(f.store)
	post := func(from string, req CaptureNeighboursRequest) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(req)
		r := httptest.NewRequest(http.MethodPost, "/internal/platform/captures/neighbours", bytes.NewReader(raw))
		r.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		h.HandleCaptureNeighbours(rec, r)
		return rec
	}
	req := CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{mustCaptureID(t, "o1", "a")}}
	if rec := post("10.200.9.9", req); rec.Code != http.StatusForbidden {
		t.Fatalf("another computer's guest was answered: %d %s", rec.Code, rec.Body)
	}
	rec := post("10.200.1.2", req)
	var res CaptureNeighboursResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &res) != nil || len(res.Neighbours[mustCaptureID(t, "o1", "a")]) != 1 {
		t.Fatalf("own guest: %d %s", rec.Code, rec.Body)
	}
	if rec := post("10.200.1.2", CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", Since: "not a day", CaptureIDs: req.CaptureIDs}); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad since: %d", rec.Code)
	}
}

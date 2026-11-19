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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
	if err != nil || len(res.Neighbours) != 0 || res.Excluded != 2 || res.Unembedded != 0 {
		t.Fatalf("private ids answered: %+v %v", res, err)
	}
	// N2: another computer's capture id named under this scope is not answered.
	res, err = f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{mustCaptureID(t, "o1", "elsewhere")}})
	if err != nil || len(res.Neighbours) != 0 || res.Excluded != 1 || res.Unembedded != 0 {
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
	if res.Unembedded != 1 || res.Excluded != 0 || res.Cut != 0 || res.State != "exact" {
		t.Fatalf("unembedded %d excluded %d cut %d state %q", res.Unembedded, res.Excluded, res.Cut, res.State)
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
	for _, want := range []string{"op=vector_neighbours", "shape=captures", "rows=1", "ids=2", "unembedded=0", "excluded=1", "cut=0", "state=exact"} {
		if !strings.Contains(out, want) {
			t.Fatalf("journal lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "choir.web_capture") {
		t.Fatalf("the journal line names a capture:\n%s", out)
	}
}

// N9 the state is told: approximate when the index answers a large list, and
// the answer is the exact one when the list came back short; exact when the
// eligible rows are few even if many ineligible vectors share the scope (the
// count uses the same predicate as the query).
func TestCaptureNeighboursStateApproximateAndTheCountUsesTheEligibilityPredicate(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	ids := f.captures(t, "o1", "c1", "s", 12, now.Add(-48*time.Hour), func(i int) string { return fmt.Sprintf("alpha common words number%d", i%3) }, time.Minute)
	f.pass(t)
	req := CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: ids[:2], Limit: 5}

	old := vectorExactMaxRows
	vectorExactMaxRows = 3
	t.Cleanup(func() { vectorExactMaxRows = old })
	res, err := f.store.CaptureNeighbours(context.Background(), req)
	if err != nil || res.State != "approximate" || len(res.Neighbours[ids[0]]) != 5 {
		t.Fatalf("large list: state %q, %d neighbours, %v", res.State, len(res.Neighbours[ids[0]]), err)
	}
	// Nine of the twelve stop being eligible: three eligible rows are an
	// exact list, though twelve vectors are in the scope.
	for _, id := range ids[3:] {
		if _, err := f.store.corpus().Exec(`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}
	res, err = f.store.CaptureNeighbours(context.Background(), req)
	if err != nil || res.State != "exact" || len(res.Neighbours[ids[0]]) != 2 {
		t.Fatalf("ineligible vectors counted: state %q, %d neighbours, %v", res.State, len(res.Neighbours[ids[0]]), err)
	}
}

// N10 a deadline cut is partial, and the captures it did not reach are cut,
// neither unembedded nor excluded.
func TestCaptureNeighboursDeadlineCutIsPartialNotMissing(t *testing.T) {
	f := newVecFixture(t)
	ids := f.captures(t, "o1", "c1", "p", 6, time.Now().UTC().Add(-48*time.Hour), func(i int) string { return fmt.Sprintf("alpha common words number%d", i) }, time.Minute)
	f.pass(t)
	old := vectorSearchTxHook
	vectorSearchTxHook = func(ctx context.Context, tx *sql.Tx) error { time.Sleep(400 * time.Millisecond); return ctx.Err() }
	t.Cleanup(func() { vectorSearchTxHook = old })
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res, err := f.store.CaptureNeighbours(ctx, CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "partial" || res.Cut != len(ids) || res.Unembedded != 0 || res.Excluded != 0 || len(res.Neighbours) != 0 {
		t.Fatalf("%+v", res)
	}
}

// N11 vectors that are not ready say so without the database's words.
func TestCaptureNeighboursTablesNotReadyDoesNotEchoTheDatabaseError(t *testing.T) {
	f := newVecFixture(t)
	f.store.vec.ready.Store(false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the ensure step fails with a database-side error text
	res, err := f.store.CaptureNeighbours(ctx, CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{"choir.web_capture:x"}})
	if err != nil || res.Unavailable != "the vector tables are not ready" {
		t.Fatalf("%+v %v", res, err)
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

// More named captures than one statement takes are answered in groups of the
// same request.
func TestCaptureNeighboursPastOneGroupAreStillOneRequest(t *testing.T) {
	f := newVecFixture(t)
	n := captureNeighboursGroup + 20
	ids := f.captures(t, "o1", "c1", "g", n, time.Now().UTC().Add(-48*time.Hour), func(i int) string { return fmt.Sprintf("alpha common words number%d", i%7) }, time.Second)
	f.pass(t)
	res, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: ids, Limit: 3})
	if err != nil || len(res.Neighbours) != n || res.Cut != 0 || res.State != "exact" {
		t.Fatalf("answered %d of %d, cut %d, state %q: %v", len(res.Neighbours), n, res.Cut, res.State, err)
	}
}

// Review round 2. Failure list:
//   - a refused capture (a row with a NULL embedding) is answered with arbitrary
//     rows at score 0, is never counted, or is returned as a neighbour;
//   - the short-list exact re-read ignores the exact bounds, so a large scope
//     meets the statement timeout on every short list;
//   - a statement timeout is recognised by words in an error string, not by the
//     database's code.

func (f *vecFixture) refuse(t *testing.T, id string) {
	t.Helper()
	if _, err := f.store.corpus().Exec(`UPDATE vec_captures SET embedding = NULL, refused = 'too long' WHERE capture_id = ?`, id); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureNeighboursRefusedCaptureIsCountedAndNeverANeighbour(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	f.capture(t, "o1", "c1", "a", "A", "quorum valdoria amber treaty ratified", now.Add(-time.Hour), false)
	f.capture(t, "o1", "c1", "b", "B", "quorum valdoria harvest", now.Add(-2*time.Hour), false)
	f.capture(t, "o1", "c1", "r", "R", "quorum valdoria amber", now.Add(-3*time.Hour), false)
	f.pass(t)
	a, b, r := mustCaptureID(t, "o1", "a"), mustCaptureID(t, "o1", "b"), mustCaptureID(t, "o1", "r")
	f.refuse(t, r)
	for _, indexPath := range []bool{false, true} {
		old := vectorExactMaxRows
		if indexPath {
			vectorExactMaxRows = 0
		}
		res, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{a, r}, Limit: 10})
		vectorExactMaxRows = old
		if err != nil {
			t.Fatal(err)
		}
		if got := neighbourIDs(res.Neighbours[a]); len(got) != 1 || got[0] != b {
			t.Fatalf("index=%v: a's neighbours %v; the refused row must not be one", indexPath, got)
		}
		if _, answered := res.Neighbours[r]; answered || res.Refused != 1 || res.Unembedded != 0 || res.Excluded != 0 {
			t.Fatalf("index=%v: refused capture answered or miscounted: %+v", indexPath, res)
		}
	}
}

func TestCaptureNeighboursRereadsExactlyOnlyWithinTheExactBounds(t *testing.T) {
	old := neighboursExactWork
	defer func() { neighboursExactWork = old }()
	oldRows := vectorExactMaxRows
	defer func() { vectorExactMaxRows = oldRows }()
	neighboursExactWork, vectorExactMaxRows = 1000, 500
	for _, c := range []struct {
		short    int
		filtered int64
		want     bool
	}{{2, 400, true}, {3, 400, false}, {1, 501, false}, {1, 500, true}, {0, 10, true}} {
		if got := neighbourRereadIsExact(c.short, c.filtered); got != c.want {
			t.Errorf("short %d filtered %d: %v, want %v", c.short, c.filtered, got, c.want)
		}
	}
}

// The call site of the exact re-read gate (CaptureNeighbours, after an index
// list comes back short). The failure list this test must catch:
//
//	F1 the call site stops applying neighbourRereadIsExact (the gate is
//	   deleted, inverted or replaced by a different bound), so an index
//	   answer past the exact work bound is read again with an exact scan;
//	F2 the gate is applied but a refused re-read is still run once, so the
//	   request makes more than one statement for the group;
//	F3 the test passes vacuously because no list was short, so the re-read
//	   was never reachable (checked: the answer holds a list shorter than
//	   the limit while more rows are held than the limit).
//
// Index mode is forced by a work bound of 1 and the statements are counted
// through vectorSearchTxHook. The hook, in the first statement's rolled-back
// transaction only, removes all but one other row so the list comes back
// short while the count taken before it still holds six (asserted before the
// call). The seams are set before CaptureNeighbours starts and restored by
// t.Cleanup after it has returned; the call is synchronous.
func TestCaptureNeighboursShortIndexListIsNotReadAgainPastTheExactBound(t *testing.T) {
	f := newVecFixture(t)
	now := time.Now().UTC()
	for i := 0; i < 6; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("k%d", i), fmt.Sprintf("K%d", i), fmt.Sprintf("quorum valdoria amber treaty number%d", i), now.Add(-time.Duration(i+1)*time.Hour), false)
	}
	f.pass(t)
	q := mustCaptureID(t, "o1", "k0")
	// The precondition the comment names: all six rows are embedded, so more
	// rows are held than the limit and the short-list branch can open.
	if n := f.count(t, vecCaptureTable); n != 6 {
		t.Fatalf("%d vector rows held, want 6: the pass did not embed every capture", n)
	}

	oldWork, oldHook := neighboursExactWork, vectorSearchTxHook
	t.Cleanup(func() { neighboursExactWork, vectorSearchTxHook = oldWork, oldHook })
	var statements int
	neighboursExactWork = 1
	vectorSearchTxHook = func(ctx context.Context, tx *sql.Tx) error {
		statements++
		if statements == 1 {
			// The statement's transaction is never committed, so this
			// removal is undone with it: the list comes back short
			// while the count taken before it still holds six rows.
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+vecCaptureTable+` WHERE capture_id NOT IN (?, ?)`, q, mustCaptureID(t, "o1", "k1")); err != nil {
				return err
			}
		}
		return ctx.Err()
	}
	res, err := f.store.CaptureNeighbours(context.Background(), CaptureNeighboursRequest{OwnerID: "o1", ComputerID: "c1", CaptureIDs: []string{q}, Limit: 4})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "approximate" {
		t.Fatalf("state %q, want approximate (index mode was not forced)", res.State)
	}
	if statements != 1 {
		t.Fatalf("%d statements ran, want 1: a short index list past the exact work bound was read again (F1, F2)", statements)
	}
	if got := len(res.Neighbours[q]); got != 1 {
		t.Fatalf("the index list holds %d, want the 1 row the hook left: no short list, so the re-read was never reachable (F3)", got)
	}
}

func TestIsStatementTimeoutReadsTheDatabasesCode(t *testing.T) {
	if !isStatementTimeout(fmt.Errorf("query: %w", &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"})) {
		t.Error("code 57014 not recognised")
	}
	if isStatementTimeout(&pgconn.PgError{Code: "22P02", Message: "statement timeout"}) || isStatementTimeout(errors.New("statement timeout 57014")) {
		t.Error("words without the code taken for the code")
	}
}

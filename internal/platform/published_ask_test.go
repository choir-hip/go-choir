package platform

// A listener's ask, answered from the paper (docs/design/first-design-2026-11-22.md
// sections 5 and 6a; the defect: the radio's answer was given 40 front-page
// titles and no search). GET /internal/platform/published/ask?q=&limit= is the
// one read a guest makes for it: BM25 over published stories AND the story
// vectors, in one request, published stories only. The failure list, written
// before the code:
//
//  P1. the keyword half finds the stories whose words match, each with its
//      route, dek, update time, and its rank in the keyword list;
//  P2. the vector half finds a story the words do not match and tags it with
//      its vector rank and score, in the same answer (one request, both halves);
//  P3. a story both halves find is one row with both ranks;
//  P4. vectors unavailable: the keyword rows are still answered, and the
//      answer says why the vector half did not run;
//  P5. only published Autopaper stories: an opinion piece, a withdrawn story
//      and a vector hit for a missing publication are never returned;
//  P6. bounds on a stranger: an empty query and a query over the byte bound
//      are errors; the limit is clamped; the vector half is asked for no more
//      than the limit; a deadline is on the call;
//  P8. an owner-mode essay or any non-story that matches strongly does not use
//      up the limit: the stories behind it are still returned, and the ranks
//      are those among stories;
//  P9. a vector half that ended partway names it in the answer and its hits
//      are still used; an index that cannot catch up inside the deadline is
//      answered as capped from what is indexed, never as a failure;
//  P10. a row that cannot be read is counted and logged, and the answer says
//      so (Partial); an answer whose every row failed is never a definite
//      nothing; the row reads have their own short budget after the walk, so
//      keyword rows already indexed survive an expired deadline;
//  P11. the real vector half (no hook): the query is embedded, the passages
//      searched, and a story found by meaning carries its rank;
//  P7. admission is the published search's: a bound guest and a host-internal
//      caller read, an unbound caller is refused, a POST is not served.

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

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func askTitles(a *PublishedAsk) []string {
	out := []string{}
	for _, s := range a.Stories {
		out = append(out, s.Title)
	}
	return out
}

func askByTitle(a *PublishedAsk, title string) *AskedStory {
	for i := range a.Stories {
		if a.Stories[i].Title == title {
			return &a.Stories[i]
		}
	}
	return nil
}

func canonicalOfDoc(t *testing.T, doc string) string {
	t.Helper()
	ident, err := publicationIdentityFor("paper-owner", doc)
	if err != nil {
		t.Fatal(err)
	}
	return ident.CanonicalID
}

func TestAskPublishedKeywordHalfFindsTheStoriesWithRanks(t *testing.T) {
	f := newFrontFixture(t)
	publishStoryRevision(t, f.svc, "doc-taiwan", 1, "Taiwan weighs a chip export rule", "Taipei weighs a rule on chip exports. Taiwan officials spoke on Monday.", nil)
	f.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) { return nil, "no embedder is configured" }
	got, err := f.svc.AskPublished(context.Background(), "more news about Taiwan", 10)
	if err != nil {
		t.Fatal(err)
	}
	s := askByTitle(got, "Taiwan weighs a chip export rule")
	if s == nil {
		t.Fatalf("(P1) the Taiwan story was not found: %v", askTitles(got))
	}
	if s.KeywordRank != 1 || s.VectorRank != 0 {
		t.Fatalf("(P1) ranks = keyword %d vector %d, want 1 and 0", s.KeywordRank, s.VectorRank)
	}
	if s.Route.Bare == "" || s.UpdatedAt.IsZero() || !strings.Contains(s.Dek, "Taipei") {
		t.Fatalf("(P1) the row lacks a route, a time or a dek: %+v", s)
	}
}

func TestAskPublishedVectorHalfRidesTheSameAnswer(t *testing.T) {
	f := newFrontFixture(t)
	publishStoryRevision(t, f.svc, "doc-taiwan", 1, "Taiwan weighs a chip export rule", "Taipei weighs a rule on chip exports.", nil)
	bridge := canonicalOfDoc(t, "doc-bridge")
	taiwan := canonicalOfDoc(t, "doc-taiwan")
	var asked []string
	var askedN int
	f.svc.askVectorHook = func(_ context.Context, q string, n int) ([]askVectorHit, string) {
		asked = append(asked, q)
		askedN = n
		return []askVectorHit{{PublicationID: bridge, Score: 0.61}, {PublicationID: taiwan, Score: 0.5}}, ""
	}
	got, err := f.svc.AskPublished(context.Background(), "Taiwan", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != "Taiwan" || askedN != 10*askOverfetch {
		t.Fatalf("(P2) the vector half asked %v n=%d, want once for the same words and limit x overfetch", asked, askedN)
	}
	b := askByTitle(got, "Bridge closes")
	if b == nil || b.VectorRank != 1 || b.KeywordRank != 0 || b.VectorScore != 0.61 {
		t.Fatalf("(P2) the vector-only story: %+v", b)
	}
	tw := askByTitle(got, "Taiwan weighs a chip export rule")
	if tw == nil || tw.KeywordRank != 1 || tw.VectorRank != 2 {
		t.Fatalf("(P3) the story both halves found: %+v", tw)
	}
	n := 0
	for _, s := range got.Stories {
		if s.Title == "Taiwan weighs a chip export rule" {
			n++
		}
	}
	if n != 1 || got.Vector != "answered" {
		t.Fatalf("(P3) rows for the shared story = %d, vector = %q", n, got.Vector)
	}
}

func TestAskPublishedVectorsDownStillAnswersAndSaysWhy(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) {
		return nil, "the embedder did not answer: timeout"
	}
	got, err := f.svc.AskPublished(context.Background(), "harbor", 10)
	if err != nil {
		t.Fatal(err)
	}
	if askByTitle(got, "Harbor reopens") == nil {
		t.Fatalf("(P4) no keyword rows: %v", askTitles(got))
	}
	if !strings.Contains(got.Vector, "embedder did not answer") {
		t.Fatalf("(P4) vector = %q, want the reason", got.Vector)
	}
}

func TestAskPublishedOnlyPublishedStories(t *testing.T) {
	f := newFrontFixture(t)
	publishAs(t, f.svc, "paper-owner", PublishModeOwner, "doc-essay", 1, "Harbor essay", "A hand published harbor essay.", nil)
	// a story withdrawn after it was published: the keyword index may still
	// hold it and the vector search may still name it
	publishAs(t, f.svc, "paper-owner", PublishModeAuto, "doc-gone", 1, "Harbor withdrawn", "A harbor story no longer published.", nil)
	gone := canonicalOfDoc(t, "doc-gone")
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{state}', '"withdrawn"')::text WHERE canonical_id = ?`, gone); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	f.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) {
		return []askVectorHit{{PublicationID: "pub-that-does-not-exist", Score: 0.9}, {PublicationID: canonicalOfDoc(t, "doc-essay"), Score: 0.8}, {PublicationID: gone, Score: 0.7}}, ""
	}
	got, err := f.svc.AskPublished(context.Background(), "harbor", 10)
	if err != nil {
		t.Fatal(err)
	}
	if titles := askTitles(got); len(titles) != 1 || titles[0] != "Harbor reopens" {
		t.Fatalf("(P5) rows = %v, want only the harbor story", titles)
	}
	if got.Partial != "" {
		t.Fatalf("(P5) rows that are not stories or do not exist are not read failures: %q", got.Partial)
	}
}

func TestAskPublishedBoundsOnAStranger(t *testing.T) {
	f := newFrontFixture(t)
	var askedN int
	var deadline bool
	f.svc.askVectorHook = func(ctx context.Context, _ string, n int) ([]askVectorHit, string) {
		askedN = n
		_, deadline = ctx.Deadline()
		return nil, ""
	}
	if _, err := f.svc.AskPublished(context.Background(), "   ", 10); err == nil {
		t.Fatal("(P6) an empty query was answered")
	}
	if _, err := f.svc.AskPublished(context.Background(), strings.Repeat("a", askQueryMaxBytes+1), 10); err == nil {
		t.Fatal("(P6) a query over the byte bound was answered")
	}
	if _, err := f.svc.AskPublished(context.Background(), "harbor", 5000); err != nil {
		t.Fatal(err)
	}
	if askedN != askMaxLimit*askOverfetch {
		t.Fatalf("(P6) the vector half was asked for %d, want the clamp %d x overfetch", askedN, askMaxLimit)
	}
	if !deadline {
		t.Fatal("(P6) the vector half ran with no deadline")
	}
	if _, err := f.svc.AskPublished(context.Background(), "harbor", 0); err != nil || askedN != askDefaultLimit*askOverfetch {
		t.Fatalf("(P6) default limit: err %v n %d", err, askedN)
	}
}

func TestPublishedAskAdmitsABoundGuestAndRefusesTheRest(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) { return nil, "off" }
	h := NewHandler(f.svc)
	reader := &vmctl.GuestOwnership{Found: true, UserID: "reader-owner", ComputerID: "computer-reader"}
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": reader}); err != nil {
		t.Fatal(err)
	}
	do := func(method, target, from string, marked bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		req.RemoteAddr = from + ":40000"
		if marked {
			req.Header.Set("X-Internal-Caller", "true")
		}
		rec := httptest.NewRecorder()
		h.HandleInternalPublishedAsk(rec, req)
		return rec
	}
	const path = "/internal/platform/published/ask?q=harbor&limit=5"
	rec := do(http.MethodGet, path, "10.200.1.2", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("(P7) bound guest refused: %d %s", rec.Code, rec.Body)
	}
	var body PublishedAsk
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || askByTitle(&body, "Harbor reopens") == nil {
		t.Fatalf("(P7) body %s (%v)", rec.Body, err)
	}
	if rec := do(http.MethodGet, path, "10.200.5.5", false); rec.Code != http.StatusForbidden {
		t.Fatalf("(P7) unbound tap address admitted: %d", rec.Code)
	}
	if rec := do(http.MethodGet, path, "203.0.113.7", false); rec.Code != http.StatusForbidden {
		t.Fatalf("(P7) outside address admitted: %d", rec.Code)
	}
	if rec := do(http.MethodGet, path, "127.0.0.1", true); rec.Code != http.StatusOK {
		t.Fatalf("(P7) host-internal caller refused: %d", rec.Code)
	}
	if rec := do(http.MethodPost, path, "10.200.1.2", false); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("(P7) POST = %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/internal/platform/published/ask?q=", "10.200.1.2", false); rec.Code != http.StatusBadRequest {
		t.Fatalf("(P6) empty query = %d", rec.Code)
	}
	long := "/internal/platform/published/ask?q=" + strings.Repeat("a", askQueryMaxBytes+1)
	if rec := do(http.MethodGet, long, "10.200.1.2", false); rec.Code != http.StatusBadRequest {
		t.Fatalf("(P6) long query = %d", rec.Code)
	}
	if rec := do(http.MethodGet, path+"x", "10.200.1.2", false); rec.Code != http.StatusBadRequest {
		t.Fatalf("(P6) limit \"5x\" = %d", rec.Code)
	}
}

func TestAskPublishedNonStoriesDoNotUseUpTheLimit(t *testing.T) {
	f := newFrontFixture(t)
	for i := 0; i < 4; i++ {
		publishAs(t, f.svc, "paper-owner", PublishModeOwner, fmt.Sprintf("doc-essay-%d", i), 1, fmt.Sprintf("Harbor harbor essay %d", i), "harbor harbor harbor harbor harbor harbor", nil)
	}
	var vecN int
	f.svc.askVectorHook = func(_ context.Context, _ string, n int) ([]askVectorHit, string) {
		vecN = n
		var hits []askVectorHit
		for i := 0; i < 4; i++ {
			hits = append(hits, askVectorHit{PublicationID: canonicalOfDoc(t, fmt.Sprintf("doc-essay-%d", i)), Score: 0.9})
		}
		return append(hits, askVectorHit{PublicationID: canonicalOfDoc(t, "doc-bridge"), Score: 0.5}), ""
	}
	got, err := f.svc.AskPublished(context.Background(), "harbor", 2)
	if err != nil {
		t.Fatal(err)
	}
	h, b := askByTitle(got, "Harbor reopens"), askByTitle(got, "Bridge closes")
	if h == nil || h.KeywordRank != 1 {
		t.Fatalf("(P8) the harbor story behind four essays: %+v (%v)", h, askTitles(got))
	}
	if b == nil || b.VectorRank != 1 {
		t.Fatalf("(P8) the vector story behind four essays: %+v (%v)", b, askTitles(got))
	}
	if vecN < 2*askOverfetch-1 {
		t.Fatalf("(P8) the vector half was asked for %d, it must over-fetch", vecN)
	}
}

func TestAskPublishedPartialVectorAndSlowIndexStillAnswer(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) {
		return []askVectorHit{{PublicationID: canonicalOfDoc(t, "doc-bridge"), Score: 0.5}}, "partial: the exact scan passed its deadline"
	}
	got, err := f.svc.AskPublished(context.Background(), "harbor", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Vector, "partial") || askByTitle(got, "Bridge closes") == nil {
		t.Fatalf("(P9) a partial vector half: vector %q rows %v", got.Vector, askTitles(got))
	}
	// the index cannot catch up inside the deadline: answered capped from what
	// is indexed, never failed. Warm the index, then ask with the deadline gone:
	// the indexed keyword rows are still read (they have their own budget).
	g := newFrontFixture(t)
	g.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) { return nil, "off" }
	if _, err := g.svc.AskPublished(context.Background(), "harbor", 5); err != nil {
		t.Fatal(err)
	}
	old := askDeadline
	askDeadline = time.Nanosecond
	t.Cleanup(func() { askDeadline = old })
	slow, err := g.svc.AskPublished(context.Background(), "harbor", 5)
	if err != nil {
		t.Fatalf("(P9) an index that cannot catch up inside the deadline failed the ask: %v", err)
	}
	// the vector half's own note ("off") or the join's, whichever the deadline let through
	if askByTitle(slow, "Harbor reopens") == nil || slow.Vector == "answered" || slow.Vector == "" {
		t.Fatalf("(P9) the indexed keyword rows must survive an expired deadline, with a vector note: %v vector %q", askTitles(slow), slow.Vector)
	}
	if !slow.Capped && slow.Partial == "" {
		t.Fatalf("(P9) an answer made past the deadline must say it may be incomplete: %+v", slow)
	}
	// the platform answers before the desk's own 1.5 s wait ends, so the
	// capped or partial answer arrives instead of the desk's "did not run"
	askDeadline = old
	if askDeadline+askRowBudget > 1500*time.Millisecond {
		t.Fatalf("(P9) the ask's deadline %v plus row budget %v is longer than the desk's wait of 1.5 s", askDeadline, askRowBudget)
	}
}

func TestAskPublishedRowsThatCannotBeReadAreCountedAndNeverADefiniteNothing(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) {
		return []askVectorHit{{PublicationID: canonicalOfDoc(t, "doc-bridge"), Score: 0.5}}, ""
	}
	harbor := canonicalOfDoc(t, "doc-harbor")
	bridge := canonicalOfDoc(t, "doc-bridge")
	fail := map[string]bool{harbor: true}
	f.svc.askRowFault = func(id string) error {
		if fail[id] {
			return fmt.Errorf("store: disk error")
		}
		return nil
	}
	var logged bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(prev) })
	got, err := f.svc.AskPublished(context.Background(), "harbor", 5)
	if err != nil {
		t.Fatal(err)
	}
	if askByTitle(got, "Bridge closes") == nil || askByTitle(got, "Harbor reopens") != nil {
		t.Fatalf("(P10) rows = %v", askTitles(got))
	}
	if !strings.Contains(got.Partial, "1 row read failed") || !strings.Contains(logged.String(), "1 row read failed") {
		t.Fatalf("(P10) partial %q, log %q", got.Partial, logged.String())
	}
	fail[bridge] = true
	all, err := f.svc.AskPublished(context.Background(), "harbor", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Stories) != 0 || all.Partial == "" || all.Vector != "answered" {
		t.Fatalf("(P10) every row failed: the answer must carry a partial note, not a clean empty: %+v", all)
	}
	// rows filtered because they are not stories are not failures
	fail = map[string]bool{}
	clean, _ := f.svc.AskPublished(context.Background(), "harbor", 5)
	if clean.Partial != "" {
		t.Fatalf("(P10) a clean answer carries a partial note: %q", clean.Partial)
	}
}

func TestAskPublishedRealVectorHalfFindsAStoryByMeaning(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.story(t, "doc-b", 1, "Bridge closes", storyText("bridge"), time.Time{})
	f.pass(t)
	got, err := f.svc.AskPublished(context.Background(), "harbor0 reports the harbor ledger entry", 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.Vector != "answered" {
		t.Fatalf("(P11) vector = %q", got.Vector)
	}
	found := false
	for _, s := range got.Stories {
		if s.VectorRank > 0 && s.VectorScore != 0 {
			found = true
		}
	}
	if !found || len(got.Stories) == 0 {
		t.Fatalf("(P11) no story carries a vector rank: %+v", got.Stories)
	}
}

// (P12) The vector half asks for few enough passages that Postgres keeps the
// fast plan. Measured on staging 2026-11-23 (exact read, the production shape,
// 72,565 story passages): LIMIT 40 to 80 took 67 to 97 ms; LIMIT 100 and 120
// took 2.3 to 2.5 s; the first ask asked for 135 and ran out its whole 1 s
// budget on both proof turns, so the desk was answered by the keyword half
// alone. The limit never exceeds the last measured fast size, and still asks
// for more passages than stories (a story has several).
func TestAskVectorPassageLimitStaysUnderTheMeasuredPlanCliff(t *testing.T) {
	for n := 1; n <= askMaxLimit*askOverfetch*2; n++ {
		got := askVectorPassageLimit(n)
		if got > askMaxPassages {
			t.Fatalf("(P12) %d stories asked %d passages, over the measured fast size %d", n, got, askMaxPassages)
		}
		if want := min(n*2, askMaxPassages); got != want {
			t.Fatalf("(P12) %d stories asked %d passages, want %d (up to twice the stories, at most %d)", n, got, want, askMaxPassages)
		}
	}
	if askMaxPassages > 80 {
		t.Fatalf("(P12) askMaxPassages %d is past the last size measured fast (80)", askMaxPassages)
	}
}

// (P13) The vector half starts with the keyword half, not after it: a vector
// half that is slow must not wait for the index walk first, and the keyword
// rows are not held up by the vector half beyond the join.
func TestAskPublishedStartsTheVectorHalfBeforeTheKeywordHalfFinishes(t *testing.T) {
	f := newFrontFixture(t)
	started := make(chan struct{})
	f.svc.askVectorHook = func(ctx context.Context, _ string, _ int) ([]askVectorHit, string) {
		close(started)
		return nil, ""
	}
	// the keyword half's refresh is held until the vector half has started: if
	// the halves ran one after the other this would never be released
	f.svc.askRefreshHook = func() {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Error("(P13) the vector half had not started while the keyword half waited: the halves run one after the other")
		}
	}
	got, err := f.svc.AskPublished(context.Background(), "harbor", 5)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	default:
		t.Fatal("(P13) the vector half had not started")
	}
	if askByTitle(got, "Harbor reopens") == nil {
		t.Fatalf("(P13) rows = %v", askTitles(got))
	}
}

// (P14) The vector half's own failures never fail, hang or kill the ask, and
// a shortfall is logged with its measure (run under -race).
func askLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&b)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &b
}

func TestAskPublishedAPanicInTheVectorHalfIsAnAnswerNotACrash(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) { panic("nil scan target") }
	logged := askLog(t)
	got, err := f.svc.AskPublished(context.Background(), "harbor", 5)
	if err != nil {
		t.Fatal(err)
	}
	if askByTitle(got, "Harbor reopens") == nil || !strings.Contains(got.Vector, "did not answer") {
		t.Fatalf("(P14) a panic in the vector half: rows %v vector %q", askTitles(got), got.Vector)
	}
	if !strings.Contains(logged.String(), "nil scan target") || !strings.Contains(logged.String(), "goroutine") {
		t.Fatalf("(P14) the panic and its stack must be logged: %q", logged.String())
	}
}

func TestAskPublishedASlowVectorHalfIsCutAtTheDeadline(t *testing.T) {
	f := newFrontFixture(t)
	old := askDeadline
	askDeadline = 150 * time.Millisecond
	t.Cleanup(func() { askDeadline = old })
	f.svc.askVectorHook = func(ctx context.Context, _ string, _ int) ([]askVectorHit, string) {
		<-ctx.Done() // a vector half that honours its context
		return nil, "the caller's deadline passed before the vector half answered"
	}
	start := time.Now()
	got, err := f.svc.AskPublished(context.Background(), "harbor", 5)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > askDeadline+askRowBudget+200*time.Millisecond {
		t.Fatalf("(P14) a slow vector half held the answer for %v", took)
	}
	if askByTitle(got, "Harbor reopens") == nil || !(strings.Contains(got.Vector, "deadline passed") || strings.Contains(got.Vector, "did not answer")) {
		t.Fatalf("(P14) rows %v vector %q", askTitles(got), got.Vector)
	}
}

func TestAskPublishedAVectorHalfThatIgnoresItsDeadlineDoesNotHoldTheAnswer(t *testing.T) {
	f := newFrontFixture(t)
	old := askDeadline
	askDeadline = 100 * time.Millisecond
	t.Cleanup(func() { askDeadline = old })
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	f.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) {
		<-release // returns only after the test, long past the deadline
		return []askVectorHit{{PublicationID: canonicalOfDoc(t, "doc-bridge"), Score: 0.9}}, ""
	}
	start := time.Now()
	got, err := f.svc.AskPublished(context.Background(), "harbor", 5)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > askDeadline+askRowBudget+200*time.Millisecond {
		t.Fatalf("(P14) the join waited %v for a vector half past its deadline", took)
	}
	if askByTitle(got, "Harbor reopens") == nil || got.Vector == "answered" || !strings.Contains(got.Vector, "did not answer") {
		t.Fatalf("(P14) rows %v vector %q", askTitles(got), got.Vector)
	}
}

func TestAskPublishedAFailingVectorHalfKeepsTheKeywordRows(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.askVectorHook = func(context.Context, string, int) ([]askVectorHit, string) {
		return nil, "the vector search failed: connection refused"
	}
	got, err := f.svc.AskPublished(context.Background(), "harbor", 5)
	if err != nil || askByTitle(got, "Harbor reopens") == nil || !strings.Contains(got.Vector, "connection refused") {
		t.Fatalf("(P14) err %v rows %v vector %q", err, askTitles(got), got.Vector)
	}
}

// (P15) every ask with a real vector half logs its query's milliseconds and
// passage count, and says so when the passages gave fewer stories than wanted,
// so the planner cliff moving is seen.
func TestAskPublishedLogsTheVectorQueryAndAShortfall(t *testing.T) {
	f := newVecFixture(t)
	f.story(t, "doc-a", 1, "Harbor reopens", storyText("harbor"), time.Time{})
	f.pass(t)
	logged := askLog(t)
	got, err := f.svc.AskPublished(context.Background(), "harbor0 reports the harbor ledger entry", 15)
	if err != nil {
		t.Fatal(err)
	}
	if got.Vector != "answered" {
		t.Fatalf("(P15) vector = %q", got.Vector)
	}
	line := logged.String()
	for _, want := range []string{"published ask vector", "ms=", "passages=", "asked=", "stories=1", "short"} {
		if !strings.Contains(line, want) {
			t.Fatalf("(P15) the log line lacks %q: %q", want, line)
		}
	}
}

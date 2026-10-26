package platform

// Ways the related-stories walk, cache and route can fail (design 5.5 to
// 5.7, section 6), written before the code. The numbers refer to the
// design's failure list (R) and to the walk's own (W).
//
//  W1 (R2) A hub capture cited by many stories links unrelated stories.
//     More than 6 citing stories give a capture no vote; exactly 6 still do.
//  W2 (R3) A story revised to drop a source keeps its old link, from the
//     old version's edge, in its own list or in the other story's.
//  W3 (R4) A candidate that is not a public Autopaper story is linked: a
//     non-public access policy, an owner-published Texture. The route of
//     such a story, or an unknown route, answers a list too (the same body
//     shape, links []), and leaks no existence signal.
//  W4 (R8) A story links to itself, or its own other versions.
//  W5 A recapture of one page (same canonical URL) is not found: stories
//     citing two captures of one source are not linked. Two different
//     pages are linked by the key, which must not happen. Captures with no
//     canonical URL must not all collide on the hash of "".
//  W6 (R7) Captures of the same canonical URL under another owner or
//     computer link a story across owners.
//  W7 The capture projection is not ready or fails: the walk errors
//     instead of degrading to exact shared captures.
//  W8 Scoring: an old story ranks like a new one (no recency); a
//     same-section tiebreak creates a link below the threshold; the list is
//     longer than the limit or fills up with weak links; ties are not
//     ordered newest first, then by publication id.
//  W9 (R5) The cache serves a stale list after the story is revised (the
//     key lacks the revision), never expires, grows without bound, stores
//     a request that overran its budget, or the budget does not bound the
//     walk.
//  W10 The kill switch CORPUSD_RELATED=off still walks, or errors, or
//     answers anything but links [] with mode off.
//  W11 The route answers a non-internal caller, any method but GET, or a
//     request with no route; the body's links is null instead of [].
//  W12 (R6) basis.captures lies: it must count the story's distinct
//     cited captures (the publish-path proof), even past the fan-out cap.
//  W13 The permalink of an old version answers a different list than the
//     bare link: both show the story's current list.
//  W14 A live page (a live blog: many subjects under one URL, each
//     recapture a different set of entries) links the stories that cite
//     it, by exact capture or by recapture (staging, 2026-10-20: one
//     Guardian live blog meshed five unrelated stories).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A realistic paper: thirty filler stories make the story count (idf's N)
// large enough that one rare shared source clears the threshold.
func newRealisticLab(t *testing.T) *relatedLab {
	t.Helper()
	l := newRelatedLab(t)
	l.fillers(30)
	return l
}

func TestRelatedLinksStoriesThatShareACapture(t *testing.T) { // basics, W4, W12
	l := newRealisticLab(t)
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	y := l.capture("https://wire.example/y", "", "Ferry operators confirmed Monday service")
	z := l.capture("https://wire.example/z", "", "A bridge closed for repairs")
	a := l.publish("doc-a", 1, "Harbor reopens", x, y)
	l.publish("doc-b", 1, "Ferries resume", x)
	c := l.publish("doc-c", 1, "Bridge closes", z)

	got := l.related(a.RoutePath)
	if got.Partial || got.Basis.Captures != 2 || got.Basis.Events != 0 || got.Basis.Mode != "captures+entities" || got.Revision != 1 {
		t.Fatalf("A: partial/basis/revision = %v %+v %d, want a complete walk over 2 captures, no events", got.Partial, got.Basis, got.Revision)
	}
	if len(got.Links) != 1 || got.Links[0].Title != "Ferries resume" {
		t.Fatalf("A links = %v, want only B (shared capture X)", relatedRoutes(got))
	}
	link := got.Links[0]
	if link.Reason != "capture" || link.Shared.Sources != 1 || link.Shared.Events != 0 || link.Route.Bare == "" || link.Route.Permalink == "" || link.PublicationID == "" {
		t.Fatalf("link = %+v, want reason capture, one shared source, a FrontStory row", link)
	}
	if hasRoute(relatedRoutes(got), a.RoutePath) {
		t.Fatalf("A links to itself: %v", relatedRoutes(got))
	}
	if other := l.related(c.RoutePath); len(other.Links) != 0 || other.Links == nil || other.Basis.Captures != 1 {
		t.Fatalf("C links = %+v, want an empty (non-null) list over 1 capture", other)
	}
	if b := l.related(getRoute(t, l, "Ferries resume")); len(b.Links) != 1 || b.Links[0].Title != "Harbor reopens" {
		t.Fatalf("B links = %v, want A", relatedRoutes(b))
	}
}

func getRoute(t *testing.T, l *relatedLab, title string) string {
	t.Helper()
	page, err := l.svc.ListFrontStories(context.Background(), 100, "")
	if err != nil {
		t.Fatalf("front: %v", err)
	}
	for _, s := range page.Stories {
		if s.Title == title {
			return s.Route.Bare
		}
	}
	t.Fatalf("no story %q on the front", title)
	return ""
}

func TestRelatedHubCaptureGivesNoVote(t *testing.T) { // W1
	l := newRealisticLab(t)
	hub := l.capture("https://live.example/blog", "", "Live blog of the day")
	var routes []string
	for i := 0; i < 7; i++ {
		own := l.capture("https://wire.example/own"+string(rune('a'+i)), "", "Own source number "+string(rune('a'+i)))
		resp := l.publish("doc-hub-"+string(rune('a'+i)), 1, "Hub citer "+string(rune('a'+i)), hub, own)
		routes = append(routes, resp.RoutePath)
	}
	for _, route := range routes {
		if got := l.related(route); len(got.Links) != 0 {
			t.Fatalf("%s linked through a hub cited by 7 stories: %v", route, relatedRoutes(got))
		}
	}

	// Exactly six citers is not yet a hub: they link to one another.
	l6 := newRealisticLab(t)
	cap6 := l6.capture("https://wire.example/six", "", "A source six stories cite")
	var first string
	for i := 0; i < 6; i++ {
		resp := l6.publish("doc-six-"+string(rune('a'+i)), 1, "Six citer "+string(rune('a'+i)), cap6)
		if i == 0 {
			first = resp.RoutePath
		}
	}
	if got := l6.related(first); len(got.Links) != 5 {
		t.Fatalf("six citers: %d links, want the other 5", len(got.Links))
	}
}

func TestRelatedIgnoresASourceARevisionDropped(t *testing.T) { // W2, W4, W13
	l := newRealisticLab(t)
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	y := l.capture("https://wire.example/y", "", "Ferry operators confirmed Monday service")
	a1 := l.publish("doc-a", 1, "Harbor reopens", x)
	b := l.publish("doc-b", 1, "Ferries resume", x)
	if got := l.related(a1.RoutePath); len(got.Links) != 1 {
		t.Fatalf("fixture: A v1 links = %v, want B", relatedRoutes(got))
	}
	// A is revised: v2 drops X for Y. B no longer shares anything with A.
	a2 := l.publish("doc-a", 2, "Harbor reopens", y)
	got := l.related(a2.RoutePath)
	if len(got.Links) != 0 || got.Revision != 2 {
		t.Fatalf("A after the revision: revision %d links %v, want none (v1's edge to X is not current)", got.Revision, relatedRoutes(got))
	}
	if gotB := l.related(b.RoutePath); len(gotB.Links) != 0 {
		t.Fatalf("B still links to A through A's old version: %v", relatedRoutes(gotB))
	}
	// The old version's permalink shows the story's current list (W13).
	old := l.related(a2.RoutePath + "/v1")
	if len(old.Links) != 0 || old.Revision != 2 {
		t.Fatalf("/v1 list = %v at revision %d, want the current (empty) list of v2", relatedRoutes(old), old.Revision)
	}
}

func TestRelatedNeverLinksAStoryThatIsNotPublicAndAutopaper(t *testing.T) { // W3
	l := newRealisticLab(t)
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	a := l.publish("doc-a", 1, "Harbor reopens", x)
	l.publish("doc-public", 1, "Public sibling", x)
	private, _ := json.Marshal(map[string]any{"visibility": "private", "route": "public"})
	l.publishAs(labOwner, PublishModeAuto, private, "doc-private", 1, "Private sibling", x)
	l.publishAs(labOwner, PublishModeOwner, nil, "doc-owner", 1, "Owner published sibling", x)

	got := l.related(a.RoutePath)
	titles := []string{}
	for _, link := range got.Links {
		titles = append(titles, link.Title)
	}
	if len(titles) != 1 || titles[0] != "Public sibling" {
		t.Fatalf("links = %v, want only the public Autopaper sibling", titles)
	}

	// The private story's and the owner-published story's own routes, and an
	// unknown route, answer the same shape: 200, links [].
	for _, route := range []string{"/pub/texture/no-such-story-abc", "/pub/texture/", "not-a-route", ""} {
		out := l.related(route)
		if out.Links == nil || len(out.Links) != 0 || out.Partial {
			t.Fatalf("route %q = %+v, want links [] and no error", route, out)
		}
		raw, _ := json.Marshal(out)
		if !strings.Contains(string(raw), `"links":[]`) {
			t.Fatalf("route %q JSON = %s, want links [] not null", route, raw)
		}
	}
	ownerRoute := ""
	if page, err := l.svc.ListFrontStories(context.Background(), 100, ""); err == nil {
		_ = page
	}
	objs, err := l.svc.ogStore().ListObjectsByMetadata(context.Background(), "choir.publication", "$.publish_mode", PublishModeOwner, 5)
	if err != nil || len(objs) != 1 {
		t.Fatalf("owner publication: %v %d", err, len(objs))
	}
	var meta struct {
		RoutePath string `json:"route_path"`
	}
	_ = json.Unmarshal(objs[0].Metadata, &meta)
	ownerRoute = meta.RoutePath
	if out := l.related(ownerRoute); len(out.Links) != 0 || out.Basis.Captures != 0 {
		t.Fatalf("an owner-published Texture got a list: %+v", out)
	}
}

func TestRelatedFindsRecapturesOfOnePageByCanonicalKey(t *testing.T) { // W5, W6
	l := newRealisticLab(t)
	// Two captures of one page: same canonical URL, different text.
	x1 := l.capture("https://dw.example/story?utm=1", "https://dw.example/story", "Deutsche Welle first copy of the story")
	x2 := l.capture("https://dw.example/story?utm=2", "https://dw.example/story", "Deutsche Welle second copy of the story")
	other := l.capture("https://dw.example/other", "https://dw.example/other", "A different page entirely")
	a := l.publish("doc-a", 1, "Story A", x1)
	l.publish("doc-b", 1, "Story B", x2)
	l.publish("doc-c", 1, "Story C", other)

	got := l.related(a.RoutePath)
	if len(got.Links) != 1 || got.Links[0].Title != "Story B" || got.Links[0].Reason != "source" || got.Links[0].Shared.Sources != 1 {
		t.Fatalf("A links = %+v, want B only, reason source (a recapture of one page)", got.Links)
	}

	// A story citing both captures of the page has one signal, not two.
	d := l.publish("doc-d", 1, "Story D", x1, x2)
	gotD := l.related(d.RoutePath)
	for _, link := range gotD.Links {
		if link.Shared.Sources != 1 {
			t.Fatalf("D's link to %q counts %d sources, want 1 (two captures of one page are one signal)", link.Title, link.Shared.Sources)
		}
	}
	if gotD.Basis.Captures != 2 {
		t.Fatalf("D basis captures = %d, want 2 distinct captures", gotD.Basis.Captures)
	}
}

func TestSourceKeyNeverCollapsesCapturesWithNoCanonicalURL(t *testing.T) { // W5
	empty := captureURLKey("")
	rowA := captureKeyRow{id: "a", owner: "o", computer: "c", canonicalKey: empty, urlKey: captureURLKey("https://x.example/a")}
	rowB := captureKeyRow{id: "b", owner: "o", computer: "c", canonicalKey: empty, urlKey: captureURLKey("https://x.example/b")}
	ka, kb := sourceKeyOf(rowA), sourceKeyOf(rowB)
	if ka == "" || kb == "" || ka == kb {
		t.Fatalf("captures with no canonical url share a key: %q %q", ka, kb)
	}
	rowNone := captureKeyRow{id: "n", owner: "o", computer: "c", canonicalKey: empty, urlKey: empty}
	if got := sourceKeyOf(rowNone); got != "capture|n" {
		t.Fatalf("a capture with no url keys by itself, got %q", got)
	}
	if sourceKeyOf(captureKeyRow{id: "x", owner: "o", computer: "c", canonicalKey: "k1", urlKey: "u"}) ==
		sourceKeyOf(captureKeyRow{id: "y", owner: "o2", computer: "c", canonicalKey: "k1", urlKey: "u"}) {
		t.Fatalf("the same canonical key under another owner is one source")
	}
}

func TestRelatedKeyMatchesStayInsideOneOwnerAndComputer(t *testing.T) { // W6
	l := newRealisticLab(t)
	mine := l.capture("https://dw.example/story", "https://dw.example/story", "My copy of the Deutsche Welle story")
	theirs := l.captureAs("tracer-owner", "computer-tracer", "https://dw.example/story", "https://dw.example/story", "Their copy of the Deutsche Welle story")
	a := l.publish("doc-a", 1, "Story A", mine)
	l.publishAs("tracer-owner", PublishModeAuto, nil, "doc-t", 1, "Tracer story", theirs)
	if got := l.related(a.RoutePath); len(got.Links) != 0 {
		t.Fatalf("a story linked across owners through a canonical key: %v", relatedRoutes(got))
	}
}

func TestRelatedStillLinksSharedCapturesWhenTheProjectionIsDown(t *testing.T) { // W7
	l := newRealisticLab(t)
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	a := l.publish("doc-a", 1, "Harbor reopens", x)
	l.publish("doc-b", 1, "Ferries resume", x)
	l.store.retrievalState.captureIndexReady.Store(false)
	got := l.related(a.RoutePath)
	if len(got.Links) != 1 || got.Links[0].Reason != "capture" {
		t.Fatalf("links = %+v, want B through the exact shared capture", got.Links)
	}
	// And a projection query that fails (table gone) degrades the same way.
	l.store.retrievalState.captureIndexReady.Store(true)
	if _, err := l.store.corpus().Exec(`DROP TABLE proj_capture_index`); err != nil {
		t.Fatalf("drop projection: %v", err)
	}
	got = l.related(a.RoutePath)
	if len(got.Links) != 1 || got.Partial {
		t.Fatalf("with the projection table gone: %+v, want B and a complete answer", got)
	}
}

func TestRelatedWalkIsBoundedByItsFanOutCap(t *testing.T) { // W12
	l := newRealisticLab(t)
	var ids []string
	for i := 0; i < 5; i++ {
		ids = append(ids, l.capture("https://wire.example/many"+string(rune('a'+i)), "", "Many sources number "+string(rune('a'+i))))
	}
	a := l.publish("doc-a", 1, "Many sources", ids...)
	l.svc.related.tuning.MaxCaptures = 2
	got := l.related(a.RoutePath)
	if got.Basis.Captures != 5 {
		t.Fatalf("basis captures = %d, want the 5 distinct cited captures even past the cap of 2", got.Basis.Captures)
	}
}

// --- scoring (pure) ---

func rank(t *testing.T, cands []relatedCandidate, tuning relatedTuning, limit int) []RelatedLink {
	t.Helper()
	return rankRelated(FrontStory{PublicationID: "self", Section: "world"}, cands, time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC), tuning, limit)
}

func cand(id string, raw float64, age time.Duration, section string) relatedCandidate {
	return relatedCandidate{
		Story: FrontStory{PublicationID: id, Title: id, UpdatedAt: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC).Add(-age), Section: section},
		Raw:   raw, Sources: 1, Citers: 2,
	}
}

func TestRecencyFactorHalvesEverySevenDaysWithAFloor(t *testing.T) { // W8
	tuning := defaultRelatedTuning()
	for _, tc := range []struct{ days, want float64 }{{0, 1}, {7, 0.5}, {14, 0.25}, {100, 0.2}, {-3, 1}} {
		if got := recencyFactor(tc.days, tuning); got < tc.want-1e-9 || got > tc.want+1e-9 {
			t.Fatalf("recencyFactor(%v) = %v, want %v", tc.days, got, tc.want)
		}
	}
}

func TestRankDropsBelowThresholdAndNeverFillsTheList(t *testing.T) { // W8
	tuning := defaultRelatedTuning()
	day := 24 * time.Hour
	out := rank(t, []relatedCandidate{
		cand("fresh-strong", 3.0, 0, ""),
		cand("fresh-weak", 0.9, 0, ""),      // below 1.0
		cand("old-strong", 3.0, 30*day, ""), // 3.0 * 0.2 floor = 0.6, below
		cand("week-old", 2.5, 7*day, ""),    // 1.25
	}, tuning, 5)
	ids := []string{}
	for _, l := range out {
		ids = append(ids, l.PublicationID)
	}
	if strings.Join(ids, ",") != "fresh-strong,week-old" {
		t.Fatalf("ranked = %v, want fresh-strong then week-old; weak and old ones dropped", ids)
	}
}

func TestRankSameSectionNeverCreatesALink(t *testing.T) { // W8
	tuning := defaultRelatedTuning()
	out := rank(t, []relatedCandidate{cand("same-weak", 0.95, 0, "world"), cand("other-weak", 0.95, 0, "business")}, tuning, 5)
	if len(out) != 0 {
		t.Fatalf("a section bonus pushed a 0.95 story over the threshold: %+v", out)
	}
	out = rank(t, []relatedCandidate{cand("other", 2, 0, "business"), cand("same", 2, 0, "world")}, tuning, 5)
	if len(out) != 2 || out[0].PublicationID != "same" {
		t.Fatalf("equal scores: same section first, got %+v", out)
	}
}

func TestRankLimitsAndBreaksTiesNewestThenID(t *testing.T) { // W8
	tuning := defaultRelatedTuning()
	hour := time.Hour
	cands := []relatedCandidate{
		cand("b", 2, 2*hour, ""), cand("a", 2, 2*hour, ""), cand("newer", 2, 1*hour, ""),
		cand("c1", 2, 3*hour, ""), cand("c2", 2, 4*hour, ""), cand("c3", 2, 5*hour, ""),
	}
	// Equal raw scores differ only by their (tiny) recency; make them exactly
	// equal by flattening the recency factor.
	tuning.HalfLifeDays = 1e9
	out := rank(t, cands, tuning, 5)
	ids := []string{}
	for _, l := range out {
		ids = append(ids, l.PublicationID)
	}
	if strings.Join(ids, ",") != "newer,a,b,c1,c2" {
		t.Fatalf("ranked = %v, want newest first, then id, cut at 5", ids)
	}
	if got := rank(t, cands, tuning, 2); len(got) != 2 {
		t.Fatalf("limit 2 returned %d", len(got))
	}
	if got := rank(t, nil, tuning, 5); got == nil || len(got) != 0 {
		t.Fatalf("no candidates must be an empty, non-nil list, got %#v", got)
	}
}

// --- cache and budget ---

func TestRelatedCacheServesWithinTTLAndKeysOnRevision(t *testing.T) { // W9
	l := newRealisticLab(t)
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	a1 := l.publish("doc-a", 1, "Harbor reopens", x)
	b := l.publish("doc-b", 1, "Ferries resume", x)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	l.svc.related.now = func() time.Time { return now }
	l.svc.related.reset()
	ctx := context.Background()

	first, err := l.svc.RelatedStories(ctx, a1.RoutePath, 0)
	if err != nil || len(first.Links) != 1 {
		t.Fatalf("first: %+v %v", first, err)
	}
	// B is revised away from X. Inside the TTL A's cached list is served.
	l.publish("doc-b", 2, "Ferries resume", l.capture("https://wire.example/y", "", "Another source entirely"))
	cached, err := l.svc.RelatedStories(ctx, a1.RoutePath, 0)
	if err != nil || len(cached.Links) != 1 {
		t.Fatalf("within the TTL: %+v %v, want the cached list", cached, err)
	}
	if !cached.GeneratedAt.Equal(first.GeneratedAt) {
		t.Fatalf("cached answer was recomputed")
	}
	// After the TTL it is recomputed from the graph.
	now = now.Add(l.svc.related.tuning.TTL + time.Second)
	fresh, err := l.svc.RelatedStories(ctx, a1.RoutePath, 0)
	if err != nil || len(fresh.Links) != 0 {
		t.Fatalf("after the TTL: %+v %v, want the recomputed (empty) list", fresh, err)
	}
	// A new revision of the story itself changes the key: no wait for the TTL.
	if got, err := l.svc.RelatedStories(ctx, a1.RoutePath, 0); err != nil || got.Revision != 1 {
		t.Fatalf("fixture: A at v1 = %+v %v", got, err)
	}
	l.publish("doc-a", 2, "Harbor reopens", x)
	a2, err := l.svc.RelatedStories(ctx, a1.RoutePath, 0)
	if err != nil || a2.Revision != 2 {
		t.Fatalf("A after v2: revision %d err %v, want the v2 answer, not the cached v1 one", a2.Revision, err)
	}
	_ = b
}

func TestRelatedCacheIsBoundedAndDoesNotKeepOverrunsOrUnknownRoutes(t *testing.T) { // W9
	l := newRealisticLab(t)
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	a := l.publish("doc-a", 1, "Harbor reopens", x)
	l.publish("doc-b", 1, "Ferries resume", x)
	ctx := context.Background()

	// A request that overruns its budget is partial, empty, and not stored.
	l.svc.related.reset()
	l.svc.related.tuning.Budget = time.Nanosecond
	over, err := l.svc.RelatedStories(ctx, a.RoutePath, 0)
	if err != nil || !over.Partial || len(over.Links) != 0 || over.Links == nil {
		t.Fatalf("overrun = %+v %v, want partial with links []", over, err)
	}
	if n := l.svc.related.size(); n != 0 {
		t.Fatalf("an overrun was cached (%d entries)", n)
	}
	l.svc.related.tuning.Budget = 5 * time.Second
	ok, err := l.svc.RelatedStories(ctx, a.RoutePath, 0)
	if err != nil || ok.Partial || len(ok.Links) != 1 {
		t.Fatalf("after the overrun: %+v %v, want the real list", ok, err)
	}
	if n := l.svc.related.size(); n != 1 {
		t.Fatalf("cache entries = %d, want 1", n)
	}

	// Unknown routes are not cached.
	l.svc.related.reset()
	if _, err := l.svc.RelatedStories(ctx, "/pub/texture/nope-abc", 0); err != nil {
		t.Fatalf("unknown: %v", err)
	}
	if n := l.svc.related.size(); n != 0 {
		t.Fatalf("an unknown route was cached")
	}

	// The cache is capped.
	c := newRelatedCache()
	c.tuning.CacheCap = 3
	for i := 0; i < 10; i++ {
		c.put("k"+string(rune('a'+i)), &RelatedStories{})
	}
	if c.size() > 3 {
		t.Fatalf("cache grew to %d past its cap of 3", c.size())
	}
}

// --- kill switch and routes ---

func TestRelatedKillSwitchAnswersEmptyWithoutWalking(t *testing.T) { // W10
	l := newRealisticLab(t)
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	a := l.publish("doc-a", 1, "Harbor reopens", x)
	l.publish("doc-b", 1, "Ferries resume", x)
	t.Setenv("CORPUSD_RELATED", "off")
	got := l.related(a.RoutePath)
	if got.Links == nil || len(got.Links) != 0 || got.Basis.Mode != "off" || got.Partial {
		t.Fatalf("off = %+v, want links [] and mode off", got)
	}
	if got.Basis.Captures != 0 {
		t.Fatalf("off still walked: %+v", got.Basis)
	}
	t.Setenv("CORPUSD_RELATED", "")
	if got := l.related(a.RoutePath); len(got.Links) != 1 {
		t.Fatalf("default (unset) = %+v, want on", got)
	}
	t.Setenv("CORPUSD_RELATED", "ON ")
	if got := l.related(a.RoutePath); len(got.Links) != 1 {
		t.Fatalf("explicit on = %+v", got)
	}
}

func TestRelatedRouteIsInternalGetOnlyAndAlwaysAList(t *testing.T) { // W11
	l := newRealisticLab(t)
	x := l.capture("https://wire.example/x", "", "Port authority said the harbor reopens")
	a := l.publish("doc-a", 1, "Harbor reopens", x)
	l.publish("doc-b", 1, "Ferries resume", x)
	h := NewHandler(l.svc)
	call := func(method string, internal bool, target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, nil)
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		w := httptest.NewRecorder()
		h.HandleInternalRelatedStories(w, req)
		return w
	}
	if w := call(http.MethodPost, true, "/internal/platform/publications/related?route="+a.RoutePath); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d", w.Code)
	}
	if w := call(http.MethodGet, false, "/internal/platform/publications/related?route="+a.RoutePath); w.Code != http.StatusForbidden {
		t.Fatalf("non-internal = %d", w.Code)
	}
	if w := call(http.MethodGet, true, "/internal/platform/publications/related"); w.Code != http.StatusBadRequest {
		t.Fatalf("no route = %d", w.Code)
	}
	if w := call(http.MethodGet, true, "/internal/platform/publications/related?route="+a.RoutePath+"&limit=abc"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad limit = %d", w.Code)
	}
	w := call(http.MethodGet, true, "/internal/platform/publications/related?route="+a.RoutePath+"&limit=5")
	if w.Code != http.StatusOK {
		t.Fatalf("ok = %d %s", w.Code, w.Body.String())
	}
	var body RelatedStories
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body.Links) != 1 || body.Route != a.RoutePath {
		t.Fatalf("body = %s (%v)", w.Body.String(), err)
	}
	unknown := call(http.MethodGet, true, "/internal/platform/publications/related?route=/pub/texture/nope-abc")
	if unknown.Code != http.StatusOK || !strings.Contains(unknown.Body.String(), `"links":[]`) {
		t.Fatalf("unknown route = %d %s, want 200 with links []", unknown.Code, unknown.Body.String())
	}
}

func TestRelatedLivePageGivesNoVote(t *testing.T) { // W14
	l := newRealisticLab(t)
	const blog = "https://www.theguardian.com/us-news/live/2026/oct/19/trump-kansas-golf-ukraine-latest-news-updates"
	live1 := l.capture(blog+"?page=1", blog, "Live: Kansas rally, golf course case, Ukraine")
	live2 := l.capture(blog+"?page=2", blog, "Live: later entries on the midterms")
	a := l.publish("doc-a", 1, "Kansas rally", live1)
	l.publish("doc-b", 1, "Golf course judge", live2)
	l.publish("doc-c", 1, "Approval poll", live1)
	if got := l.related(a.RoutePath); len(got.Links) != 0 {
		t.Fatalf("A links = %+v, want none: a live page links nothing, by exact capture or recapture", relatedRoutes(got))
	}

	// An ordinary page shared alongside still links.
	shared := l.capture("https://dw.example/kansas", "https://dw.example/kansas", "The Kansas Senate race")
	d := l.publish("doc-d", 1, "Senate race", live1, shared)
	l.publish("doc-e", 1, "Kansas rally follow-up", shared)
	got := l.related(d.RoutePath)
	if len(got.Links) != 1 || got.Links[0].Title != "Kansas rally follow-up" || got.Links[0].Shared.Sources != 1 {
		t.Fatalf("D links = %+v, want the ordinary shared page only", got.Links)
	}
}

func TestIsLivePageURL(t *testing.T) { // W14
	for _, u := range []string{
		"https://www.theguardian.com/us-news/live/2026/oct/19/x-latest-news-updates",
		"https://www.bbc.com/news/live/c1234",
		"https://edition.cnn.com/politics/live-news/trump-10-19-26",
		"https://www.nytimes.com/live/2026/10/19/us/election",
		"https://apnews.com/live/election-updates",
		"https://example.com/world/liveblog/2026/10/19/ukraine",
		"https://example.com/news/2026/10/19/ukraine-war-live-updates",
	} {
		if !isLivePageURL(u) {
			t.Errorf("%s: not seen as a live page", u)
		}
	}
	for _, u := range []string{
		"",
		"not a url",
		"https://thehill.com/video-clips/6158911-watch-live-donald-trump-kansas-rally/",
		"https://www.theguardian.com/us-news/2026/oct/18/trump-dc-golf-course-renovation",
		"https://example.com/culture/live-music-returns-to-the-city",
		"https://live.example.com/news/2026/10/19/story",
	} {
		if isLivePageURL(u) {
			t.Errorf("%s: seen as a live page", u)
		}
	}
}

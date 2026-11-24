package platform

// The corpus as values (corpus_repl.go). Failure list, written before the code:
//
//   P1 a page lists a private, tombstoned, another computer's or another
//      owner's capture;
//   P2 the cursor repeats or skips a row, or the window, source or kind filter
//      lets a row of the wrong kind through;
//   P3 a page past the row bound is not cut, or is cut with no cursor;
//   R1 near lists the capture itself, a capture of another scope, a private
//      one, or does not give the similarity;
//   R2 near a text or a story gives no list, or a story is listed twice, as
//      an older version, or itself;
//   R3 k above the old bound (200) is capped at it;
//   E1 edges of a capture miss the published story that cites it, or list a
//      story that is not published, or the same page's other captures;
//   E2 edges of a story miss its versions, its sources (only this computer's
//      shareable ones) or its related stories, or name an unpublished story;
//   E3 an unknown or private ref is answered as if it held something.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
)

func (f *rankFixture) setSource(t *testing.T, key, source string) {
	t.Helper()
	if _, err := f.store.corpus().Exec(`UPDATE proj_capture_index SET source_id = ? WHERE capture_id = ?`, source, f.id(t, key)); err != nil {
		t.Fatal(err)
	}
}

func corpusIDs(rows []CorpusCaptureRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.CaptureID)
	}
	return out
}

func TestCorpusCapturesPagesScopedFilteredAndCursored(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC()
	for i := 0; i < 7; i++ {
		f.page(t, fmt.Sprintf("p%d", i), "harbor", 50, now.Add(-time.Duration(i)*time.Hour))
	}
	f.capture(t, "o1", "c1", "priv", "Private", "secret", now.Add(-time.Minute), true)
	f.capture(t, "o1", "c2", "other", "Other computer", "x", now.Add(-time.Minute), false)
	f.capture(t, "o2", "c1", "stranger", "Other owner", "x", now.Add(-time.Minute), false)
	gone := f.page(t, "gone", "harbor", 50, now.Add(-time.Minute))
	f.setSource(t, "p1", sourcegraph.ReadSourceID)
	f.setSource(t, "p3", sourcegraph.ReadSourceID)
	f.setSource(t, "p2", "feed:a")
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, gone.CanonicalID); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	all, err := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1"})
	if err != nil || len(all.Captures) != 7 || all.Next != "" {
		t.Fatalf("all: %d rows next %q err %v", len(all.Captures), all.Next, err) // P1
	}
	// P2: pages of 3 walk the 7 rows once, newest first.
	var walked []string
	cursor := ""
	for n := 0; n < 5; n++ {
		pg, err := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", Limit: 3, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		walked = append(walked, corpusIDs(pg.Captures)...)
		if cursor = pg.Next; cursor == "" {
			break
		}
	}
	if strings.Join(walked, ",") != strings.Join(corpusIDs(all.Captures), ",") {
		t.Fatalf("cursor walk %v want %v", walked, corpusIDs(all.Captures))
	}
	reads, _ := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", Kind: "read"})
	if len(reads.Captures) != 2 || reads.Captures[0].Kind != "read" {
		t.Fatalf("reads %+v", reads.Captures)
	}
	srcs, _ := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", Kind: "source"})
	if len(srcs.Captures) != 5 {
		t.Fatalf("source arrivals %d", len(srcs.Captures))
	}
	one, _ := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", SourceID: "feed:a"})
	if len(one.Captures) != 1 || one.Captures[0].CaptureID != f.id(t, "p2") {
		t.Fatalf("source filter %+v", one.Captures)
	}
	win, _ := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", Since: now.Add(-150 * time.Minute).Format(time.RFC3339), Until: now.Add(-30 * time.Minute).Format(time.RFC3339)})
	if len(win.Captures) != 2 {
		t.Fatalf("window %v", corpusIDs(win.Captures))
	}
	if _, err := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", Kind: "x"}); err == nil {
		t.Fatal("a bad kind was accepted")
	}
	if _, err := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", Cursor: "junk"}); err == nil {
		t.Fatal("a bad cursor was accepted")
	}
}

func TestCorpusCapturesStopsAtTheByteBoundWithACursor(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC()
	long := strings.Repeat("w", 230)
	for i := 0; i < 12; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("k%02d", i), long, "t", now.Add(-time.Duration(i)*time.Minute), false)
	}
	f.pass(t)
	// P3: with a tiny byte bound the page is short and carries a cursor.
	was := corpusAnswerBytesVar
	corpusAnswerBytesVar = 3000
	defer func() { corpusAnswerBytesVar = was }()
	pg, err := f.store.CorpusCaptures(context.Background(), CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", Limit: 12})
	if err != nil || len(pg.Captures) == 0 || len(pg.Captures) >= 12 || pg.Next == "" {
		t.Fatalf("rows %d next %q err %v", len(pg.Captures), pg.Next, err)
	}
}

func TestCorpusNearCaptureTextAndStory(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "q", "harbor", 10, now)
	f.page(t, "a", "harbor", 12, now)
	f.page(t, "b", "harbor", 30, now)
	f.capture(t, "o1", "c1", "priv", "Private", "harbor vec11", now, true)
	f.capture(t, "o1", "c2", "other", "Other", "harbor vec11", now, false)
	f.story(t, "doc-a", 1, "Harbor story", "The old edition says vec14 harbor.", time.Time{})
	f.story(t, "doc-a", 2, "Harbor story", "The new edition says vec16 harbor.", time.Time{})
	f.story(t, "doc-b", 1, "Quay story", "The quay says vec40 harbor.", time.Time{})
	f.pass(t)
	ctx := context.Background()
	res, err := f.store.CorpusNear(ctx, CorpusNearRequest{OwnerID: "o1", ComputerID: "c1", CaptureID: f.id(t, "q"), K: 10})
	if err != nil || res.Unavailable != "" {
		t.Fatalf("%+v %v", res, err)
	}
	got := corpusNearIDs(res)
	if strings.Join(got, ",") != strings.Join([]string{f.id(t, "a"), f.id(t, "b")}, ",") { // R1
		t.Fatalf("near %v", got)
	}
	if res.Captures[0].Score <= res.Captures[1].Score || res.Captures[0].Score > 1 || res.Captures[0].Title == "" {
		t.Fatalf("scores/metadata %+v", res.Captures)
	}
	if len(res.Stories) != 2 { // R2: one row for the story with two versions, latest only
		t.Fatalf("stories %+v", res.Stories)
	}
	for _, s := range res.Stories {
		if s.Title == "Harbor story" && s.Revision != 2 {
			t.Fatalf("older version listed: %+v", s)
		}
	}
	txt, err := f.store.CorpusNear(ctx, CorpusNearRequest{OwnerID: "o1", ComputerID: "c1", Text: "harbor", K: 5})
	if err != nil || len(txt.Captures) == 0 || len(txt.Stories) == 0 {
		t.Fatalf("text %+v %v", txt, err)
	}
	pub := res.Stories[0].PublicationID
	st, err := f.store.CorpusNear(ctx, CorpusNearRequest{OwnerID: "o1", ComputerID: "c1", StoryID: pub, K: 5})
	if err != nil || st.Unavailable != "" || len(st.Stories) != 1 {
		t.Fatalf("story %+v %v", st, err)
	}
	if st.Stories[0].PublicationID == pub {
		t.Fatal("a story is near itself")
	}
	if r, _ := f.store.CorpusNear(ctx, CorpusNearRequest{OwnerID: "o1", ComputerID: "c1", CaptureID: f.id(t, "priv")}); r.Unavailable == "" || len(r.Captures) != 0 {
		t.Fatalf("a private capture is a query: %+v", r)
	}
	// A capture of another computer, or another owner, is no query: the same
	// answer as a private one or an id that does not exist.
	f.capture(t, "o2", "c1", "stranger", "Stranger", "harbor vec12", now, false)
	f.pass(t)
	for _, id := range []string{mustCaptureID(t, "o1", "other"), mustCaptureID(t, "o2", "stranger"), f.id(t, "priv"), f.id(t, "ghost")} {
		r, _ := f.store.CorpusNear(ctx, CorpusNearRequest{OwnerID: "o1", ComputerID: "c1", CaptureID: id})
		if r.Unavailable != corpusNoVector || len(r.Captures) != 0 || len(r.Stories) != 0 {
			t.Fatalf("%s was a query: %+v", id, r)
		}
	}
	if _, err := f.store.CorpusNear(ctx, CorpusNearRequest{OwnerID: "o1", ComputerID: "c1"}); err == nil {
		t.Fatal("no subject was accepted")
	}
}

func corpusNearIDs(r CorpusNearResponse) []string {
	var out []string
	for _, c := range r.Captures {
		out = append(out, c.CaptureID)
	}
	return out
}

func TestCorpusNearAsksForMoreThanTwoHundred(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 260; i++ {
		f.page(t, fmt.Sprintf("m%03d", i), "harbor", 20+i%50, now)
	}
	f.pass(t)
	res, err := f.store.CorpusNear(context.Background(), CorpusNearRequest{OwnerID: "o1", ComputerID: "c1", Text: "harbor", K: 300})
	if err != nil || len(res.Captures) != 260 { // R3
		t.Fatalf("%d captures, %v", len(res.Captures), err)
	}
}

func TestCorpusEdgesOfACaptureAndOfAStory(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "c1", "harbor", 10, now)
	f.page(t, "c2", "harbor", 11, now)
	f.capture(t, "o1", "c1", "priv", "Private", "harbor", now, true)
	f.capture(t, "o1", "c2", "other", "Other", "harbor", now, false)
	f.story(t, "doc-a", 1, "Harbor story", "The old edition says vec14 harbor.", time.Time{})
	f.story(t, "doc-a", 2, "Harbor story", "The new edition says vec16 harbor.", time.Time{})
	f.story(t, "doc-b", 1, "Quay story", "The quay says vec40 harbor.", time.Time{})
	f.pass(t)
	pubA, v1 := f.versionOf(t, "Harbor story", 1)
	_, v2 := f.versionOf(t, "Harbor story", 2)
	pubB, _ := f.versionOf(t, "Quay story", 1)
	f.cite(t, v1, f.id(t, "c1"))
	f.cite(t, v2, f.id(t, "c1"), f.id(t, "c2"), f.id(t, "priv"), mustCaptureID(t, "o1", "other"))
	if _, err := f.store.corpus().Exec(`INSERT INTO og_edges (edge_id, from_id, to_id, kind, metadata, created_at, tombstone) VALUES ('rel1', ?, ?, 'related_story', '{"relation":"later_development"}', ?, FALSE)`, pubA, pubB, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c, err := f.store.CorpusEdges(ctx, CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: f.id(t, "c1")})
	if err != nil || c.Kind != "capture" || len(c.Stories) != 2 { // E1: both versions cite it
		t.Fatalf("capture edges %+v %v", c, err)
	}
	latest := 0
	for _, s := range c.Stories {
		if s.Latest {
			latest++
		}
	}
	if latest != 1 {
		t.Fatalf("latest marks %+v", c.Stories)
	}
	s, err := f.store.CorpusEdges(ctx, CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: pubA})
	if err != nil || s.Kind != "story" || len(s.Versions) != 2 || s.Story.VersionID != v2 { // E2
		t.Fatalf("story edges %+v %v", s, err)
	}
	if got := corpusIDs(s.Sources); strings.Join(got, ",") != strings.Join(sorted(f.id(t, "c1"), f.id(t, "c2")), ",") {
		t.Fatalf("sources %v (a private or another computer's capture must not be listed)", got)
	}
	if len(s.Related) != 1 || s.Related[0].PublicationID != pubB || s.Related[0].Relation != "later_development" {
		t.Fatalf("related %+v", s.Related)
	}
	b, _ := f.store.CorpusEdges(ctx, CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: pubB})
	if len(b.Related) != 1 || b.Related[0].PublicationID != pubA {
		t.Fatalf("the edge is not read from its other end: %+v", b.Related)
	}
	v, _ := f.store.CorpusEdges(ctx, CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: v1})
	if v.Kind != "version" || len(v.Sources) != 1 {
		t.Fatalf("version edges %+v", v)
	}
	for _, ref := range []string{f.id(t, "priv"), "pub-nothing", mustCaptureID(t, "o1", "other")} { // E3
		r, err := f.store.CorpusEdges(ctx, CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: ref})
		if err != nil || r.Missing == "" || r.Kind != "" {
			t.Fatalf("ref %s answered: %+v %v", ref, r, err)
		}
	}
}

func sorted(ids ...string) []string {
	out := append([]string(nil), ids...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Review round 1 (seat 1), failure list written before the repairs:
//
//   A1 a guest vmctl binds to another scope, an unbound caller, or a call
//      naming no owner or computer, is answered on any of the four routes (the
//      gate removed leaves every earlier test green);
//   B1 a title or URL of "<", ">" or "&" (six bytes each as the answer is
//      written) carries an answer past its byte bound; near, edges and texts
//      have no byte bound, and a cut answer has no marker;
//   V1 edges lists a version, or a title, of a story whose latest version is
//      not public;
//   M1 the two "missing" answers differ, so a caller learns a story exists;
//   K1 the cursor walk skips or repeats a row with tied or missing fetch times;
//   X1 the bulk text serves a superseded capture, another computer's, or an
//      owner-private one, or one that was never named, or is not in order.

func (f *rankFixture) guestHandler(t *testing.T) *ObjectGraphHandler {
	t.Helper()
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: f.og}), f.og)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{
		"10.200.1.2": {Found: true, UserID: "o1", ComputerID: "c1"},
		"10.200.9.9": {Found: true, UserID: "o2", ComputerID: "c2"},
	}); err != nil {
		t.Fatal(err)
	}
	h.ConfigureCaptureIndex(f.store)
	h.captureBodies = f.svc.captureBodies
	return h
}

func TestCorpusRoutesAdmitOnlyTheGuestBoundToTheScopeNamed(t *testing.T) { // A1
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "a", "harbor", 10, now)
	f.page(t, "b", "harbor", 12, now)
	f.pass(t)
	h := f.guestHandler(t)
	id := f.id(t, "a")
	routes := []struct {
		name string
		fn   http.HandlerFunc
		body func(owner, computer string) map[string]any
	}{
		{"page", h.HandleCorpusCaptures, func(o, c string) map[string]any { return map[string]any{"owner_id": o, "computer_id": c} }},
		{"near", h.HandleCorpusNear, func(o, c string) map[string]any {
			return map[string]any{"owner_id": o, "computer_id": c, "capture_id": id}
		}},
		{"edges", h.HandleCorpusEdges, func(o, c string) map[string]any { return map[string]any{"owner_id": o, "computer_id": c, "ref": id} }},
		{"texts", h.HandleCorpusTexts, func(o, c string) map[string]any {
			return map[string]any{"owner_id": o, "computer_id": c, "ids": []string{id}}
		}},
	}
	for _, rt := range routes {
		post := func(from, owner, computer string) *httptest.ResponseRecorder {
			raw, _ := json.Marshal(rt.body(owner, computer))
			r := httptest.NewRequest(http.MethodPost, "/internal/platform/captures/"+rt.name, bytes.NewReader(raw))
			r.RemoteAddr = from + ":40000"
			rec := httptest.NewRecorder()
			rt.fn(rec, r)
			return rec
		}
		if rec := post("10.200.1.2", "o1", "c1"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "obj:choir.web_capture:") {
			t.Fatalf("%s: the bound guest, own scope: %d %s", rt.name, rec.Code, rec.Body)
		}
		for _, c := range []struct{ from, owner, computer string }{
			{"10.200.1.2", "o2", "c1"}, // another owner named
			{"10.200.1.2", "o1", "c2"}, // another computer named
			{"10.200.9.9", "o1", "c1"}, // a guest of another computer names this one
			{"10.9.9.9", "o1", "c1"},   // a caller vmctl does not bind
			{"10.200.1.2", "", "c1"},   // no owner named
			{"10.200.1.2", "o1", ""},   // no computer named
		} {
			rec := post(c.from, c.owner, c.computer)
			if rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest {
				t.Fatalf("%s: %+v was answered %d %s", rt.name, c, rec.Code, rec.Body)
			}
			if strings.Contains(rec.Body.String(), "obj:choir.web_capture") {
				t.Fatalf("%s: %+v got data: %s", rt.name, c, rec.Body)
			}
		}
	}
}

func (f *rankFixture) angry(t *testing.T, n int, now time.Time) []string {
	t.Helper()
	var ids []string
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("angry%03d", i)
		f.capture(t, "o1", "c1", key, strings.Repeat("<", 240), fmt.Sprintf("harbor vec%02d", 10+i%40), now.Add(-time.Duration(i)*time.Second), false)
		ids = append(ids, mustCaptureID(t, "o1", key))
	}
	return ids
}

func TestCorpusAnswersAreBoundedAsWrittenAndSayWhenCut(t *testing.T) { // B1
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	ids := f.angry(t, 40, now)
	f.story(t, "doc-a", 1, "Harbor story", "The harbor says vec10.", time.Time{})
	f.pass(t)
	_, v1 := f.versionOf(t, "Harbor story", 1)
	f.cite(t, v1, ids...)
	pub, _ := f.versionOf(t, "Harbor story", 1)
	was := corpusAnswerBytesVar
	corpusAnswerBytesVar = 12 << 10
	defer func() { corpusAnswerBytesVar = was }()
	size := func(v any) int { b, _ := json.Marshal(v); return len(b) }
	ctx := context.Background()

	pg, err := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", Limit: 40})
	if err != nil || size(pg) > corpusAnswerBytesVar || pg.Next == "" || len(pg.Captures) == 0 {
		t.Fatalf("page: %d rows, %d bytes, next %q, %v", len(pg.Captures), size(pg), pg.Next, err)
	}
	near, err := f.store.CorpusNear(ctx, CorpusNearRequest{OwnerID: "o1", ComputerID: "c1", Text: "harbor", K: 40})
	if err != nil || size(near) > corpusAnswerBytesVar || len(near.Cut) == 0 {
		t.Fatalf("near: %d bytes, cut %v, %v", size(near), near.Cut, err)
	}
	edges, err := f.store.CorpusEdges(ctx, CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: pub})
	if err != nil || size(edges) > corpusAnswerBytesVar || len(edges.Cut) == 0 || len(edges.Sources) == 0 {
		t.Fatalf("edges: %d bytes, cut %v, %d sources, %v", size(edges), edges.Cut, len(edges.Sources), err)
	}
	h := f.guestHandler(t)
	txts, err := h.CorpusTexts(ctx, CorpusTextsRequest{OwnerID: "o1", ComputerID: "c1", IDs: ids})
	if err != nil || size(txts) > corpusAnswerBytesVar || len(txts.Unreached) == 0 || len(txts.Texts) == 0 {
		t.Fatalf("texts: %d bytes, unreached %d, %v", size(txts), len(txts.Unreached), err)
	}
	if len(txts.Texts)+len(txts.Unreached) != len(ids) {
		t.Fatalf("texts %d + unreached %d != %d asked", len(txts.Texts), len(txts.Unreached), len(ids))
	}
}

func TestCorpusEdgesListOnlyPublicVersionsAndTitlesAndOneMissingAnswer(t *testing.T) { // V1, M1
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "c1", "harbor", 10, now)
	f.story(t, "doc-a", 1, "Old harbor title", "The old edition says vec14 harbor.", time.Time{})
	f.story(t, "doc-a", 2, "Secret harbor title", "The new edition says vec16 harbor.", time.Time{})
	f.pass(t)
	pub, v1 := f.versionOf(t, "Secret harbor title", 1)
	_, v2 := f.versionOf(t, "Secret harbor title", 2)
	f.cite(t, v1, f.id(t, "c1"))
	f.cite(t, v2, f.id(t, "c1"))
	ctx := context.Background()
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, v2); err != nil {
		t.Fatal(err)
	}
	e, err := f.store.CorpusEdges(ctx, CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: v1})
	if err != nil || e.Kind != "version" {
		t.Fatalf("%+v %v", e, err)
	}
	raw, _ := json.Marshal(e)
	if strings.Contains(string(raw), v2) || strings.Contains(string(raw), "Secret harbor title") || len(e.Versions) != 1 {
		t.Fatalf("a version that is not public, or its title, is listed: %s", raw)
	}
	c, _ := f.store.CorpusEdges(ctx, CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: f.id(t, "c1")})
	raw, _ = json.Marshal(c)
	if strings.Contains(string(raw), v2) || strings.Contains(string(raw), "Secret") {
		t.Fatalf("capture edges list the restricted version: %s", raw)
	}
	// With its latest version not public the story itself answers as unknown,
	// and as unknown as an id that never existed or a private capture.
	missing := map[string]bool{}
	for _, ref := range []string{pub, "pub-0123456789abcdef0123456789abcdef", f.id(t, "nope"), mustCaptureID(t, "o9", "x")} {
		r, err := f.store.CorpusEdges(ctx, CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: ref})
		if err != nil || r.Missing == "" || r.Kind != "" {
			t.Fatalf("ref %s: %+v %v", ref, r, err)
		}
		missing[r.Missing] = true
	}
	if len(missing) != 1 {
		t.Fatalf("the answers differ, so existence shows: %v", missing)
	}
}

func TestCorpusCursorWalksTiedAndMissingFetchTimes(t *testing.T) { // K1
	f := newRankFixture(t)
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	for i := 0; i < 6; i++ {
		f.page(t, fmt.Sprintf("tie%d", i), "harbor", 50, at) // the same fetch time
	}
	for i := 0; i < 3; i++ {
		f.page(t, fmt.Sprintf("old%d", i), "harbor", 50, at.Add(-time.Duration(i+1)*time.Hour))
	}
	for i := 0; i < 3; i++ {
		f.page(t, fmt.Sprintf("none%d", i), "harbor", 50, at)
		if _, err := f.store.corpus().Exec(`UPDATE proj_capture_index SET fetched_at = NULL WHERE capture_id = ?`, f.id(t, fmt.Sprintf("none%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	all, err := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1"})
	if err != nil || len(all.Captures) != 12 {
		t.Fatalf("all %d %v", len(all.Captures), err)
	}
	for _, n := range []int{1, 2, 5} {
		var walked []string
		cursor := ""
		for i := 0; i < 20; i++ {
			pg, err := f.store.CorpusCaptures(ctx, CorpusCapturesRequest{OwnerID: "o1", ComputerID: "c1", Limit: n, Cursor: cursor})
			if err != nil {
				t.Fatal(err)
			}
			walked = append(walked, corpusIDs(pg.Captures)...)
			if cursor = pg.Next; cursor == "" {
				break
			}
		}
		if strings.Join(walked, ",") != strings.Join(corpusIDs(all.Captures), ",") {
			t.Fatalf("limit %d: walked %v want %v", n, walked, corpusIDs(all.Captures))
		}
	}
}

func TestCorpusTextsServeOnlyWhatTheScopeMayRead(t *testing.T) { // X1
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "ok", "alpha beta", 10, now)
	f.page(t, "old", "superseded text", 11, now)
	f.capture(t, "o1", "c1", "priv", "Private", "secret words", now, true)
	f.capture(t, "o1", "c2", "elsewhere", "Elsewhere", "other computer words", now, false)
	f.capture(t, "o2", "c1", "stranger", "Stranger", "other owner words", now, false)
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET superseded_by = 'x' WHERE canonical_id = ?`, f.id(t, "old")); err != nil {
		t.Fatal(err)
	}
	h := f.guestHandler(t)
	ask := []string{f.id(t, "priv"), f.id(t, "ok"), f.id(t, "old"), mustCaptureID(t, "o1", "elsewhere"), mustCaptureID(t, "o2", "stranger"), f.id(t, "ghost")}
	res, err := h.CorpusTexts(context.Background(), CorpusTextsRequest{OwnerID: "o1", ComputerID: "c1", IDs: ask})
	if err != nil || len(res.Texts) != len(ask) {
		t.Fatalf("%+v %v", res, err)
	}
	errs := map[string]bool{}
	for i, id := range ask {
		got := res.Texts[i]
		if got.CaptureID != id {
			t.Fatalf("order: %d is %s", i, got.CaptureID)
		}
		if id == f.id(t, "ok") {
			if got.Error != "" || !strings.Contains(got.Body+blocksText(got), "alpha beta") || got.Title == "" {
				t.Fatalf("ok: %+v", got)
			}
			continue
		}
		if got.Error == "" || got.Body != "" || len(got.Blocks) != 0 {
			t.Fatalf("%s was served: %+v", id, got)
		}
		errs[got.Error] = true
	}
	if len(errs) != 1 {
		t.Fatalf("the refusals differ, so which is which shows: %v", errs)
	}
	if _, err := h.CorpusTexts(context.Background(), CorpusTextsRequest{OwnerID: "o1", ComputerID: "c1", IDs: make([]string, 201)}); err == nil {
		t.Fatal("201 ids were accepted")
	}
}

func blocksText(t CorpusTextEntry) string {
	var parts []string
	for _, b := range t.Blocks {
		parts = append(parts, b.Text)
	}
	return strings.Join(parts, "\n")
}

func TestCorpusEdgesSamePageIsOfThisScopeOnly(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "dupA", "harbor", 10, now)
	f.page(t, "dupB", "harbor", 11, now.Add(-time.Minute))
	f.capture(t, "o2", "c1", "dupA", "Same url, another owner", "harbor", now, false) // the same URL
	f.capture(t, "o1", "c2", "dupC", "Same url, another computer", "harbor", now, false)
	for _, key := range []string{"dupB", "dupC"} {
		if _, err := f.store.corpus().Exec(`UPDATE proj_capture_index SET canonical_key = (SELECT canonical_key FROM proj_capture_index WHERE capture_id = ?) WHERE capture_id = ?`, f.id(t, "dupA"), mustCaptureID(t, "o1", key)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.store.corpus().Exec(`UPDATE proj_capture_index SET canonical_key = (SELECT canonical_key FROM proj_capture_index WHERE capture_id = ?) WHERE capture_id = ?`, f.id(t, "dupA"), mustCaptureID(t, "o2", "dupA")); err != nil {
		t.Fatal(err)
	}
	e, err := f.store.CorpusEdges(context.Background(), CorpusEdgesRequest{OwnerID: "o1", ComputerID: "c1", Ref: f.id(t, "dupA")})
	if err != nil || len(e.SamePage) != 1 || e.SamePage[0].CaptureID != f.id(t, "dupB") {
		t.Fatalf("same page %+v %v", e.SamePage, err)
	}
}

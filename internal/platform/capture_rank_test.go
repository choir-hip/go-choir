package platform

// The one ranked corpus-search answer of station V4 (capture_rank.go).
// Failure list, written before the code:
//
//   1. the answer is not in vector order, or BM25's top three pages are not
//      guaranteed a place, or one of them appears twice;
//   2. vectors that cannot answer (the embedder down or absent, the tables
//      gone, the deadline passed) fail the search or change BM25's answer, or
//      the answer does not say so;
//   3. an older version of a story reaches the answer;
//   4. a story hit does not carry the captures it cites, carries more than the
//      bound, or a capture that arrives both ways is listed twice or loses a
//      reason;
//   5. an owner-private, tombstoned or superseded capture reaches the answer
//      through the vector half, through a story's citations or through BM25;
//   6. the time window does not reach the vector half or a story's citations;
//   7. the journal line does not say which half answered.

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestPlaceBM25InsertsMissingPagesAtTheirBM25RankAndNeverDuplicates(t *testing.T) {
	v := func(ns ...int) []string {
		var out []string
		for _, n := range ns {
			out = append(out, fmt.Sprintf("v%02d", n))
		}
		return out
	}
	seq := func(from, to int) []string {
		var ns []int
		for i := from; i <= to; i++ {
			ns = append(ns, i)
		}
		return v(ns...)
	}
	join := func(parts ...[]string) []string {
		var out []string
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	cases := []struct {
		name     string
		vec, bm  []string
		size     int
		want     []string
		wantKept int
	}{
		{"two of the three absent: inserted at ranks 2 and 3, the tail that is not BM25's falls off",
			seq(1, 12), join(v(2), []string{"x", "y", "z"}), 10,
			join(v(1), []string{"x", "y"}, seq(2, 8)), 2},
		{"all three present in the first ten: the vector order untouched, nothing inserted",
			seq(1, 12), v(3, 1, 2), 10, seq(1, 10), 0},
		{"three absent: inserted at ranks 1, 2 and 3 in BM25's order",
			seq(1, 12), []string{"a", "b", "c", "d"}, 10, join([]string{"a", "b", "c"}, seq(1, 7)), 3},
		{"a BM25 page at the very tail of the vector answer is not the one dropped",
			seq(1, 10), join([]string{"a", "b"}, v(10)), 10, join([]string{"a", "b"}, seq(1, 7), v(10)), 2},
		{"a BM25 page that the vector list holds only below the answer counts as absent and is inserted once",
			seq(1, 15), v(14, 1, 2), 10, join(v(14), seq(1, 9)), 1},
		{"a short vector list: BM25's pages are inserted and the answer is not padded",
			v(1), []string{"a", "b", "c"}, 10, join([]string{"a", "b", "c"}, v(1)), 3},
		{"a size under three guarantees only that many of BM25's pages",
			seq(1, 3), []string{"a", "b", "c"}, 2, []string{"a", "b"}, 2},
		{"BM25 found nothing: the vector order, cut to the size",
			seq(1, 12), nil, 10, seq(1, 10), 0},
		{"the vector half found nothing: BM25's top three",
			nil, []string{"a", "b", "c", "d"}, 10, []string{"a", "b", "c"}, 3},
	}
	for _, c := range cases {
		got, kept := placeBM25(c.vec, c.bm, c.size)
		if strings.Join(got, ",") != strings.Join(c.want, ",") || kept != c.wantKept {
			t.Errorf("%s:\n got  %v kept %d\n want %v kept %d", c.name, got, kept, c.want, c.wantKept)
		}
		seen := map[string]bool{}
		for _, id := range got {
			if seen[id] {
				t.Errorf("%s: %q appears twice in %v", c.name, id, got)
			}
			seen[id] = true
		}
		again, _ := placeBM25(c.vec, c.bm, c.size)
		if strings.Join(again, ",") != strings.Join(got, ",") {
			t.Errorf("%s: not deterministic", c.name)
		}
	}
}

// ---- fixtures: a fake embedder whose vector is chosen by a marker in the text ----

var vecMarker = regexp.MustCompile(`vec(\d\d)`)

// markerVector gives the query the first axis and a document "vecNN" a vector at
// cosine 1 - NN/100 from it, so a test chooses the vector order by its text.
// A text with no marker keeps the fake's own vector.
func markerVector(input string) []float32 {
	if strings.Contains(input, "query: ") {
		v := make([]float32, embedder.Dim)
		v[0] = 1
		return v
	}
	m := vecMarker.FindStringSubmatch(input)
	if m == nil {
		return nil
	}
	n, _ := strconv.Atoi(m[1])
	c := 1 - float64(n)/100
	v := make([]float32, embedder.Dim)
	v[0], v[1] = float32(c), float32(math.Sqrt(1-c*c))
	return v
}

type rankFixture struct {
	*vecFixture
	owner, computer string
	mu              sync.Mutex
	lines           []string
}

func newRankFixture(t *testing.T) *rankFixture {
	t.Helper()
	f := &rankFixture{vecFixture: newVecFixture(t), owner: "o1", computer: "c1"}
	if err := f.store.ConfigureCaptureFullText(t.TempDir(), f.svc.captureBodies); err != nil {
		t.Fatal(err)
	}
	f.emb.Override = markerVector
	f.store.retrieval.logf = func(format string, args ...any) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.lines = append(f.lines, fmt.Sprintf(format, args...))
	}
	return f
}

func (f *rankFixture) journal() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lines...)
}

// page puts a public capture whose text carries the vector marker vecNN.
func (f *rankFixture) page(t *testing.T, key, text string, n int, fetched time.Time) objectgraph.Object {
	t.Helper()
	return f.capture(t, f.owner, f.computer, key, "Page "+key, fmt.Sprintf("%s vec%02d", text, n), fetched, false)
}

func (f *rankFixture) id(t *testing.T, key string) string {
	t.Helper()
	return mustCaptureID(t, f.owner, key)
}

func (f *rankFixture) rank(t *testing.T, query string, mod ...func(*CaptureSearchRequest)) CaptureSearchResponse {
	t.Helper()
	req := CaptureSearchRequest{OwnerID: f.owner, ComputerID: f.computer, Query: query, Words: strings.Fields(strings.ToLower(query)), Need: 1, Limit: 50, Size: 10}
	for _, m := range mod {
		m(&req)
	}
	resp, err := f.store.SearchCaptures(context.Background(), req)
	if err != nil {
		t.Fatalf("SearchCaptures: %v", err)
	}
	return resp
}

func entryIDs(resp CaptureSearchResponse) []string {
	if resp.Ranked == nil {
		return nil
	}
	var out []string
	for _, e := range resp.Ranked.Entries {
		out = append(out, e.CaptureID)
	}
	return out
}

func (f *rankFixture) keys(t *testing.T, ids []string, keys ...string) string {
	t.Helper()
	byID := map[string]string{}
	for _, k := range keys {
		byID[f.id(t, k)] = k
	}
	var out []string
	for _, id := range ids {
		if k, ok := byID[id]; ok {
			out = append(out, k)
		} else {
			out = append(out, "?"+id[len(id)-6:])
		}
	}
	return strings.Join(out, ",")
}

func (f *rankFixture) cite(t *testing.T, versionID string, captureIDs ...string) {
	t.Helper()
	for _, id := range captureIDs {
		if _, err := f.store.corpus().Exec(`INSERT INTO og_edges (edge_id, from_id, to_id, kind, metadata, created_at, tombstone) VALUES (?, ?, ?, 'cites_capture', '{"relation":"cites"}', ?, FALSE)`,
			"edge-"+versionID+"-"+id, versionID, id, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *rankFixture) versionOf(t *testing.T, doc string, rev int) (story, version string) {
	t.Helper()
	refs, err := f.svc.eligibleStoryVersions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range refs {
		if strings.Contains(r.StoryID, doc) || r.Title == doc {
			if r.Revision == rev {
				return r.StoryID, r.VersionID
			}
		}
	}
	t.Fatalf("no version %d of %q among %+v", rev, doc, refs)
	return "", ""
}

// disagree builds the case BM25 and the vectors disagree on: twelve pages the
// vectors put first that hold no "zorblax", and three pages that hold it,
// which are BM25's three and sit last in vector order. It checks BM25's order.
func (f *rankFixture) disagree(t *testing.T) (bm []string) {
	t.Helper()
	now := time.Now().UTC().Add(-time.Hour)
	for i := 1; i <= 12; i++ {
		f.page(t, fmt.Sprintf("v%02d", i), "harbor notes about nothing in particular", i, now)
	}
	f.page(t, "bmA", "zorblax zorblax zorblax", 60, now)
	f.page(t, "bmB", "zorblax zorblax", 61, now)
	f.page(t, "bmC", "zorblax", 62, now)
	f.pass(t)
	return []string{"bmA", "bmB", "bmC"}
}

func TestRankedAnswerIsVectorOrderWithBM25sTopThreeInsertedAtTheirRanks(t *testing.T) {
	f := newRankFixture(t)
	bm := f.disagree(t)
	resp := f.rank(t, "zorblax")
	if got := f.keys(t, hitIDs2(resp), bm...); got != "bmA,bmB,bmC" {
		t.Fatalf("fixture: BM25 order is %s, want bmA,bmB,bmC", got)
	}
	if resp.Ranked == nil {
		t.Fatal("no ranked answer")
	}
	all := append([]string{"bmA", "bmB", "bmC"}, "v01", "v02", "v03", "v04", "v05", "v06", "v07")
	if got := f.keys(t, entryIDs(resp), append(all, "v08")...); got != strings.Join(all, ",") {
		t.Fatalf("answer %s, want %s", got, strings.Join(all, ","))
	}
	r := resp.Ranked
	if !r.VectorAnswered || r.VectorRows != 15 || r.BM25Kept != 3 || r.VectorUnavailable != "" || r.VectorState != "exact" {
		t.Fatalf("ranking info %+v", r)
	}
	// Reasons: BM25's pages were found by text (and by meaning, lower); the rest by meaning only.
	for _, e := range r.Entries {
		isBM := strings.HasPrefix(f.keys(t, []string{e.CaptureID}, "bmA", "bmB", "bmC"), "bm")
		if isBM != containsStr(e.Reasons, "text") || !containsStr(e.Reasons, "meaning") {
			t.Fatalf("reasons of %s: %v", e.CaptureID, e.Reasons)
		}
	}
	// The objects of every entry travel with the answer.
	have := map[string]bool{}
	for _, o := range resp.HitObjects {
		have[o.CanonicalID] = true
	}
	for _, id := range entryIDs(resp) {
		if !have[id] {
			t.Fatalf("no object for %s", id)
		}
	}
	// One line says which half answered.
	var line string
	for _, l := range f.journal() {
		if strings.Contains(l, "op=capture_search shape=ranked") {
			line = l
		}
	}
	if !strings.Contains(line, " vector=15 vector_state=exact bm25_kept=3") {
		t.Fatalf("journal line %q", line)
	}
}

func hitIDs2(resp CaptureSearchResponse) []string {
	var out []string
	for _, h := range resp.Hits {
		out = append(out, h.CaptureID)
	}
	return out
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestRankedAnswerHoldsBM25sPagesOnceWhenTheVectorsAlreadyPutThemInTheTen(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	markers := []struct {
		key  string
		text string
		n    int
	}{
		{"v01", "harbor notes", 1}, {"v02", "harbor notes", 2}, {"bmA", "zorblax zorblax zorblax", 3}, {"v04", "harbor notes", 4},
		{"bmB", "zorblax zorblax", 5}, {"v06", "harbor notes", 6}, {"bmC", "zorblax", 7}, {"v08", "harbor notes", 8},
		{"v09", "harbor notes", 9}, {"v10", "harbor notes", 10}, {"v11", "harbor notes", 11}, {"v12", "harbor notes", 12},
	}
	for _, m := range markers {
		f.page(t, m.key, m.text, m.n, now)
	}
	f.pass(t)
	resp := f.rank(t, "zorblax")
	want := "v01,v02,bmA,v04,bmB,v06,bmC,v08,v09,v10"
	if got := f.keys(t, entryIDs(resp), "v01", "v02", "bmA", "v04", "bmB", "v06", "bmC", "v08", "v09", "v10", "v11", "v12"); got != want {
		t.Fatalf("answer %s, want %s", got, want)
	}
	if resp.Ranked.BM25Kept != 0 {
		t.Fatalf("bm25_kept %d, want 0", resp.Ranked.BM25Kept)
	}
	if got := f.keys(t, entryIDs(resp), "bmA"); strings.Count(got, "bmA") != 1 {
		t.Fatalf("duplicate: %s", got)
	}
}

func TestRankedAnswerIsBM25AloneAndSaysSoWhenVectorsCannotAnswer(t *testing.T) {
	f := newRankFixture(t)
	bm := f.disagree(t)
	// Today's answer: what the search gives with no vector half at all.
	before := f.rank(t, "zorblax")
	wantOrder := hitIDs2(before)
	if len(wantOrder) != 3 {
		t.Fatalf("fixture: BM25 holds %d pages", len(wantOrder))
	}
	modes := map[string]func(){
		"the embedder is down": func() { f.emb.Down.Store(true) },
		"no embedder":          func() { f.emb.Down.Store(false); f.store.SetEmbedder(nil) },
		"the tables are gone": func() {
			f.store.SetEmbedder(f.emb)
			if _, err := f.store.corpus().Exec(`DROP TABLE vec_captures, vec_story_passages`); err != nil {
				t.Fatal(err)
			}
		},
	}
	for _, name := range []string{"the embedder is down", "no embedder", "the tables are gone"} {
		modes[name]()
		resp := f.rank(t, "zorblax")
		r := resp.Ranked
		if r == nil || r.VectorAnswered || r.VectorUnavailable == "" || r.BM25Kept != 0 {
			t.Fatalf("%s: ranking info %+v", name, r)
		}
		if got := strings.Join(entryIDs(resp), ","); got != strings.Join(wantOrder, ",") {
			t.Fatalf("%s: answer %s is not BM25's %s", name, got, strings.Join(wantOrder, ","))
		}
		for _, e := range r.Entries {
			if strings.Join(e.Reasons, ",") != "text" {
				t.Fatalf("%s: reasons %v", name, e.Reasons)
			}
		}
		var line string
		for _, l := range f.journal() {
			if strings.Contains(l, "op=capture_search shape=ranked") {
				line = l
			}
		}
		if !strings.Contains(line, " vector=unavailable vector_state=unavailable bm25_kept=0") {
			t.Fatalf("%s: journal line %q", name, line)
		}
	}
	_ = bm
}

func TestRankedAnswerGivesUpOnTheVectorsAtItsDeadlineAndKeepsBM25(t *testing.T) {
	f := newRankFixture(t)
	f.disagree(t)
	old := captureVectorDeadline
	captureVectorDeadline = 80 * time.Millisecond
	t.Cleanup(func() { captureVectorDeadline = old })
	f.emb.Override = func(input string) []float32 {
		if strings.Contains(input, "query: ") {
			time.Sleep(600 * time.Millisecond) // an embedder that does not look at its context
		}
		return markerVector(input)
	}
	start := time.Now()
	resp := f.rank(t, "zorblax")
	if took := time.Since(start); took > 400*time.Millisecond {
		t.Fatalf("the search waited %v for a vector half with an 80 ms deadline", took)
	}
	if r := resp.Ranked; r == nil || r.VectorAnswered || !strings.Contains(r.VectorUnavailable, "within") || len(r.Entries) != 3 {
		t.Fatalf("ranking info %+v", r)
	}
}

func TestRankedAnswerNeverHoldsAnOlderVersionOfAStory(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "old1", "ledger of the old edition", 99, now)
	f.page(t, "new1", "ledger of the new edition", 99, now)
	f.story(t, "doc-a", 1, "Harbor story", "The old edition says vec02 harbor.", time.Time{})
	f.story(t, "doc-a", 2, "Harbor story", "The new edition says vec40 harbor.", time.Time{})
	f.pass(t)
	storyID, v1 := f.versionOf(t, "Harbor story", 1)
	_, v2 := f.versionOf(t, "Harbor story", 2)
	f.cite(t, v1, f.id(t, "old1"))
	f.cite(t, v2, f.id(t, "new1"))
	resp := f.rank(t, "harbor")
	for _, e := range resp.Ranked.Entries {
		viaStory := containsStr(e.Reasons, "story")
		switch e.CaptureID {
		case f.id(t, "old1"):
			if viaStory {
				t.Fatalf("old1 reached through the story's first version: %+v", e)
			}
		case f.id(t, "new1"):
			if !viaStory || len(e.Stories) != 1 || e.Stories[0] != storyID {
				t.Fatalf("new1 not reached through the latest version: %+v", e)
			}
		}
	}
	if len(resp.Ranked.Stories) != 1 || resp.Ranked.Stories[0].Revision != 2 || resp.Ranked.Stories[0].VersionID != v2 || resp.Ranked.Stories[0].Title != "Harbor story" {
		t.Fatalf("stories %+v", resp.Ranked.Stories)
	}
}

func TestRankedAnswerStoryHitCarriesItsCitedCapturesBoundedAndDeduplicated(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "D1", "harbor dock", 2, now)
	f.page(t, "D2", "harbor quay", 8, now)
	var cited []string
	for i := 1; i <= 12; i++ {
		key := fmt.Sprintf("C%02d", i)
		f.page(t, key, "a cited page", 99, now)
		cited = append(cited, f.id(t, key))
	}
	f.story(t, "doc-a", 1, "Harbor story", "The harbor story vec04 is told here.", time.Time{})
	f.pass(t)
	storyID, version := f.versionOf(t, "Harbor story", 1)
	f.cite(t, version, cited...)
	resp := f.rank(t, "harbor")
	r := resp.Ranked
	sort.Strings(cited)
	bounded := cited[:rankStoryCitedMax]
	want := append(append([]string{f.id(t, "D1")}, bounded...), f.id(t, "D2"))
	if got := entryIDs(resp); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("answer\n %v\nwant D1, the story's first %d cited captures in id order, D2:\n %v", got, rankStoryCitedMax, want)
	}
	for _, e := range r.Entries[1 : 1+rankStoryCitedMax] {
		// Found by meaning too (the page's own vector); the better rank, the story's, is kept.
		if !containsStr(e.Reasons, "story") || !containsStr(e.Reasons, "meaning") || len(e.Stories) != 1 || e.Stories[0] != storyID {
			t.Fatalf("a cited capture lacks a reason: %+v", e)
		}
	}
	if len(r.Stories) != 1 || r.Stories[0].PublicationID != storyID || len(r.Stories[0].Captures) != rankStoryCitedMax {
		t.Fatalf("stories %+v", r.Stories)
	}
	seen := map[string]bool{}
	for _, id := range entryIDs(resp) {
		if seen[id] {
			t.Fatalf("%s listed twice", id)
		}
		seen[id] = true
	}
}

func TestRankedAnswerNeverHoldsAPrivateTombstonedOrSupersededCapture(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	// By meaning and by text: public when embedded and indexed, then changed.
	f.page(t, "ok1", "zorblax record", 1, now)
	f.page(t, "turnsprivate", "zorblax record", 2, now)
	f.page(t, "tomb", "zorblax record", 3, now)
	f.page(t, "super", "zorblax record", 4, now)
	// Through a story's citations.
	f.page(t, "okcited", "plain", 99, now)
	f.page(t, "tombcited", "plain", 99, now)
	f.page(t, "supercited", "plain", 99, now)
	f.capture(t, f.owner, f.computer, "privcited", "Page privcited", "plain vec99", now, true)
	f.story(t, "doc-a", 1, "Harbor story", "The harbor story vec05 is told here.", time.Time{})
	f.pass(t)
	_, version := f.versionOf(t, "Harbor story", 1)
	f.cite(t, version, f.id(t, "okcited"), f.id(t, "tombcited"), f.id(t, "supercited"), f.id(t, "privcited"))

	priv, err := f.og.GetObject(context.Background(), f.id(t, "turnsprivate"))
	if err != nil {
		t.Fatal(err)
	}
	priv.Metadata = types.StampMetadataLabels(priv.Metadata, types.NewLabelSet(types.OwnerPrivateLabel("capture")))
	priv.ContentHash += "-private"
	if err := f.og.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{priv}}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tomb", "tombcited"} {
		if _, err := f.store.corpus().Exec(`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, f.id(t, key)); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"super", "supercited"} {
		if _, err := f.store.corpus().Exec(`UPDATE og_objects SET superseded_by = 'obj:newer' WHERE canonical_id = ?`, f.id(t, key)); err != nil {
			t.Fatal(err)
		}
	}
	// No sweep has run: the vector rows of the changed captures are still there.
	if n := f.count(t, "vec_captures"); n < 7 {
		t.Fatalf("fixture: %d capture vectors", n)
	}
	for _, query := range []string{"zorblax", "harbor", "plain record"} {
		resp := f.rank(t, query)
		for _, id := range entryIDs(resp) {
			if strings.Contains(id, "turnsprivate") || f.keys(t, []string{id}, "turnsprivate", "tomb", "super", "tombcited", "supercited", "privcited") != "?"+id[len(id)-6:] {
				t.Fatalf("query %q: %s reached the answer", query, id)
			}
		}
		for _, s := range resp.Ranked.Stories {
			for _, c := range s.Captures {
				if f.keys(t, []string{c}, "tombcited", "supercited", "privcited") != "?"+c[len(c)-6:] {
					t.Fatalf("query %q: story carries %s", query, c)
				}
			}
		}
	}
	resp := f.rank(t, "harbor")
	if got := f.keys(t, entryIDs(resp), "ok1", "okcited"); !strings.Contains(got, "okcited") {
		t.Fatalf("the public cited capture is missing: %s", got)
	}
}

func TestRankedAnswerWindowReachesTheVectorHalfAndAStorysCitations(t *testing.T) {
	f := newRankFixture(t)
	base := time.Now().UTC().Truncate(24 * time.Hour).Add(-10*24*time.Hour + 12*time.Hour)
	for i := 0; i < 6; i++ {
		f.page(t, fmt.Sprintf("w%d", i), "zorblax harbor", i+1, base.AddDate(0, 0, i))
	}
	f.page(t, "cited", "plain", 99, base.AddDate(0, 0, 3))
	f.story(t, "doc-a", 1, "Harbor story", "The harbor story vec02 is told here.", time.Time{})
	f.pass(t)
	_, version := f.versionOf(t, "Harbor story", 1)
	f.cite(t, version, f.id(t, "cited"))
	window := func(r *CaptureSearchRequest) {
		r.Since = base.AddDate(0, 0, 2).Format(time.RFC3339)
		r.Until = base.AddDate(0, 0, 4).Format(time.RFC3339)
	}
	resp := f.rank(t, "zorblax harbor", window)
	got := f.keys(t, entryIDs(resp), "w0", "w1", "w2", "w3", "w4", "w5", "cited")
	for _, k := range []string{"w0", "w1", "w4", "w5"} {
		if strings.Contains(got, k) {
			t.Fatalf("%s is outside the window: %s", k, got)
		}
	}
	if !strings.Contains(got, "w2") || !strings.Contains(got, "w3") {
		t.Fatalf("the window lost a page inside it: %s", got)
	}
	for _, e := range resp.Ranked.Entries {
		if containsStr(e.Reasons, "story") {
			t.Fatalf("a story outside the window was walked: %+v", e)
		}
	}
	// A window that holds a story walks it, and its citations are windowed too.
	resp = f.rank(t, "zorblax harbor", func(r *CaptureSearchRequest) {
		r.Since = base.AddDate(0, 0, 9).Format(time.RFC3339)
	})
	for _, id := range entryIDs(resp) {
		if id == f.id(t, "cited") {
			t.Fatalf("a capture older than the window came through a story")
		}
	}
}

func TestRankedAnswerRecentOrderListsTheAnswerNewestFirst(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-48 * time.Hour)
	f.page(t, "p1", "zorblax harbor", 1, now)
	f.page(t, "p2", "zorblax harbor", 2, now.Add(2*time.Hour))
	f.page(t, "p3", "zorblax harbor", 3, now.Add(time.Hour))
	f.pass(t)
	resp := f.rank(t, "zorblax harbor", func(r *CaptureSearchRequest) { r.Order = "recent" })
	if got := f.keys(t, entryIDs(resp), "p1", "p2", "p3"); got != "p2,p3,p1" {
		t.Fatalf("recent order %s", got)
	}
}

// The measurement script (scripts/proofs/corpus-search-recall) runs against a
// real handler over a test database with the fake embedder, and says when the
// bar is missed.
func TestRankedRecallScriptRunsAgainstARealHandler(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3")
	}
	f := newRankFixture(t)
	f.disagree(t)
	h := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: f.og}), f.og)
	h.ConfigureCaptureIndex(f.store)
	srv := httptest.NewServer(http.HandlerFunc(h.HandleCaptureSearch))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	set := filepath.Join(dir, "set.jsonl")
	line := fmt.Sprintf(`{"id":"s1","query":"zorblax","relevant":[%q,%q,%q,%q],"non_english":[%q]}`+"\n",
		f.id(t, "bmA"), f.id(t, "bmB"), f.id(t, "bmC"), f.id(t, "v01"), f.id(t, "bmC"))
	if err := os.WriteFile(set, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	script, _ := filepath.Abs("../../scripts/proofs/corpus-search-recall")
	run := func(extra ...string) (string, int) {
		args := append([]string{"-I", script, "--set", set, "--url", srv.URL, "--owner", f.owner, "--computer", f.computer, "--header", "X-Internal-Caller: true"}, extra...)
		out, err := exec.Command(py, args...).CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return string(out), code
	}
	out, code := run()
	if code != 0 || !strings.Contains(out, "recall@10 1.000") || !strings.Contains(out, "vector half answered 1 of 1") || !strings.Contains(out, "BM25 pages inserted 3") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	// With the vectors down the measurement says it is not of the ship rule.
	f.emb.Down.Store(true)
	if out, code = run(); code != 4 {
		t.Fatalf("vectors down: exit %d:\n%s", code, out)
	}
	f.emb.Down.Store(false)
	// A question whose answers are not in the ten misses the bar.
	if err := os.WriteFile(set, []byte(`{"id":"s1","query":"zorblax","relevant":["obj:none:a","obj:none:b"]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, code = run(); code != 1 {
		t.Fatalf("a missed bar: exit %d:\n%s", code, out)
	}
}

func TestVectorStateIsExactApproximateOrPartial(t *testing.T) {
	if s, p := vectorStateOf(VectorSearchResult{ExactScan: []string{"stories", "captures"}}); s != "exact" || p != "" {
		t.Fatalf("exact: %q %q", s, p)
	}
	if s, _ := vectorStateOf(VectorSearchResult{ExactScan: []string{"stories"}, Approximate: []string{"captures"}}); s != "approximate" {
		t.Fatalf("approximate: %q", s)
	}
	if s, p := vectorStateOf(VectorSearchResult{Approximate: []string{"captures"}, Partial: "captures: the exact scan did not finish"}); s != "partial" || p == "" {
		t.Fatalf("partial: %q %q", s, p)
	}
}

func TestRankedAnswerSaysApproximateWhenAListIsTheIndexsAndKeepsANumber(t *testing.T) {
	f := newRankFixture(t)
	f.disagree(t)
	old := vectorExactMaxRows
	vectorExactMaxRows = 3 // every list is larger: the index answers
	t.Cleanup(func() { vectorExactMaxRows = old })
	// A list the index fills to its limit stays the index's; size 1 asks for 4 rows of 15.
	resp := f.rank(t, "zorblax", func(r *CaptureSearchRequest) { r.Size = 1 })
	r := resp.Ranked
	if r == nil || !r.VectorAnswered || r.VectorState != "approximate" || r.VectorRows == 0 {
		t.Fatalf("ranking %+v", r)
	}
	var line string
	for _, l := range f.journal() {
		if strings.Contains(l, "shape=ranked") {
			line = l
		}
	}
	if !strings.Contains(line, " vector_state=approximate bm25_kept=") || !regexp.MustCompile(` vector=\d+ vector_state=`).MatchString(line) {
		t.Fatalf("journal line %q", line)
	}
}

func TestVectorStatementDeadlineIsTheCallersOneDeadline(t *testing.T) {
	if got := vectorStatementTimeout(context.Background()); got != vectorSearchTimeout {
		t.Fatalf("no deadline: %q", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	got := vectorStatementTimeout(ctx)
	ms, err := strconv.Atoi(strings.TrimSuffix(got, "ms"))
	if err != nil || ms < 3000 || ms > 3600 {
		t.Fatalf("a 4 s deadline gave %q, want about 3,600 ms (nine tenths)", got)
	}
}

func TestRankedAnswerDoesNotWalkAWithdrawnStory(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "cited", "plain", 99, now)
	f.story(t, "doc-a", 1, "Harbor story", "The harbor story vec02 is told here.", time.Time{})
	f.pass(t)
	storyID, version := f.versionOf(t, "Harbor story", 1)
	f.cite(t, version, f.id(t, "cited"))
	if resp := f.rank(t, "harbor"); len(resp.Ranked.Stories) != 1 {
		t.Fatalf("fixture: the story is not walked: %+v", resp.Ranked)
	}
	for name, stmt := range map[string]string{
		"version tombstoned":     `UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = '` + version + `'`,
		"publication tombstoned": `UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = '` + storyID + `'`,
	} {
		if _, err := f.store.corpus().Exec(`UPDATE og_objects SET tombstone = FALSE WHERE canonical_id IN ('` + version + `', '` + storyID + `')`); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.corpus().Exec(stmt); err != nil {
			t.Fatal(err)
		}
		resp := f.rank(t, "harbor")
		if len(resp.Ranked.Stories) != 0 {
			t.Fatalf("%s: a withdrawn story was walked: %+v", name, resp.Ranked.Stories)
		}
		for _, e := range resp.Ranked.Entries {
			if containsStr(e.Reasons, "story") {
				t.Fatalf("%s: a capture was reached through a withdrawn story: %+v", name, e)
			}
		}
	}
}

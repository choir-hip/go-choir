package researchtools

// The guest side of the one ranked answer (station V4): it lists corpusd's
// order, does not merge, adds what is new in fields old programs ignore, and
// writes vector= and bm25_kept= on its journal line. Failure list:
//   1. the guest re-orders or merges corpusd's answer;
//   2. a page found by meaning or through a story is dropped, or loses its reasons;
//   3. the answer of a corpusd that has no ranking, or whose vectors were
//      unavailable, is not logged as such, or loses BM25's order;
//   4. the journal line has no vector= number when the vector half answered;
//   5. the mirror types drift from platform's (a renamed field).

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

type rankedSearcher struct {
	CaptureSink
	answer CaptureSearchAnswer
	asked  []CaptureSearchQuery
}

func (r *rankedSearcher) SearchCaptures(_ context.Context, q CaptureSearchQuery) (CaptureSearchAnswer, error) {
	r.asked = append(r.asked, q)
	return r.answer, nil
}

func rankedObj(t *testing.T, key, title string) objectgraph.Object {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, "o", objectgraph.StableSuffixFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: "https://example.com/" + key, Title: title, CanonicalURL: "https://example.com/" + key,
		FetchedAt: time.Now().UTC().Format(time.RFC3339Nano), ContentBlobID: "sha256:" + strings.Repeat("a", 64), ExtractedTextBlobID: "sha256:" + strings.Repeat("a", 64),
		BlocksBlobID: "sha256:" + strings.Repeat("a", 64), Extractor: "x1", DocHash: "h-" + key, BlockCount: 3})
	return objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: "o", ComputerID: "c", Metadata: meta}
}

func rankedWorld(t *testing.T, answer CaptureSearchAnswer) (Dependencies, *rankedSearcher) {
	t.Helper()
	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })
	rs := &rankedSearcher{CaptureSink: windowSink{}, answer: answer}
	return Dependencies{Store: guest, Captures: rs}, rs
}

func TestGuestListsCorpusdsRankedAnswerWithoutMergingAndCarriesTheNewFields(t *testing.T) {
	a, b, c := rankedObj(t, "a", "Alpha page"), rankedObj(t, "b", "Beta page"), rankedObj(t, "c", "Gamma page")
	answer := CaptureSearchAnswer{
		HitObjects: []objectgraph.Object{a, b, c},
		Hits:       []CaptureTextHit{{CaptureID: b.CanonicalID, Snippet: "beta passage", BodyMatch: true, Passages: 2}},
		Ranked: &CaptureRanking{VectorAnswered: true, VectorRows: 14, VectorState: "exact", BM25Kept: 1,
			Entries: []RankedCapture{
				{CaptureID: c.CanonicalID, Reasons: []string{"meaning", "story"}, Stories: []string{"pub-1"}},
				{CaptureID: a.CanonicalID, Reasons: []string{"meaning"}},
				{CaptureID: b.CanonicalID, Reasons: []string{"text", "meaning"}},
			},
			Stories: []RankedStory{{PublicationID: "pub-1", Title: "Harbor story", Revision: 2, Captures: []string{c.CanonicalID, "not-listed"}}},
		},
	}
	deps, rs := rankedWorld(t, answer)
	got, _, info, err := searchCapturesFor(context.Background(), deps, "o", "c", corpusRequest{Query: "beta", Terms: corpusQueryTerms("beta")}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(titles(got), ",") != "Gamma page,Alpha page,Beta page" {
		t.Fatalf("order %v: the guest re-ordered corpusd's answer", titles(got))
	}
	if got[0].Passage != "" || got[2].Passage != "beta passage" || !containsString(got[2].Matched, "body") {
		t.Fatalf("passages %+v", got)
	}
	if strings.Join(got[0].Reasons, ",") != "meaning,story" || strings.Join(got[0].ViaStories, ",") != "pub-1" {
		t.Fatalf("reasons %+v", got[0])
	}
	if !info.VectorAnswered || info.VectorRows != 14 || info.BM25Kept != 1 || info.VectorState != "exact" || len(info.Stories) != 1 ||
		strings.Join(info.Stories[0].CaptureIDs, ",") != c.CanonicalID || info.Stories[0].Title != "Harbor story" {
		t.Fatalf("info %+v", info)
	}
	if len(rs.asked) != 1 || rs.asked[0].Size != 10 || rs.asked[0].Query != "beta" {
		t.Fatalf("asked %+v", rs.asked)
	}
	// The old fields are all still in the JSON a cell program reads.
	raw, _ := json.Marshal(wireCorpusResult{Query: "beta", Captures: got, CaptureCount: len(got), Stories: info.Stories})
	for _, key := range []string{`"capture_id"`, `"title"`, `"url"`, `"passage"`, `"matched"`, `"capture_count":3`, `"reasons"`, `"via_stories"`, `"stories"`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("result lacks %s: %s", key, raw)
		}
	}
	if carries, understood := CorpusResultCarriesPassage(raw); !carries || !understood {
		t.Fatalf("passage labelling: %v %v", carries, understood)
	}
}

func titles(cs []CorpusCapture) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Title)
	}
	return out
}

func TestGuestLogsTheVectorFieldsAndFallsBackToBM25Order(t *testing.T) {
	a, b := rankedObj(t, "a", "Alpha page"), rankedObj(t, "b", "Beta page")
	answer := CaptureSearchAnswer{
		HitObjects: []objectgraph.Object{a, b},
		Hits:       []CaptureTextHit{{CaptureID: b.CanonicalID, Snippet: "pb", BodyMatch: true}, {CaptureID: a.CanonicalID, Snippet: "pa", BodyMatch: true}},
		Ranked: &CaptureRanking{VectorUnavailable: "no embedder is configured",
			Entries: []RankedCapture{{CaptureID: b.CanonicalID, Reasons: []string{"text"}}, {CaptureID: a.CanonicalID, Reasons: []string{"text"}}}},
	}
	logs := captureReadLog(t) // holds the log lock until the test ends: take it once
	run := func(ans CaptureSearchAnswer) (string, []CorpusCapture) {
		deps, _ := rankedWorld(t, ans)
		logs.Reset()
		tool := newSearchWireCorpusTool(deps)
		ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-9", OwnerID: "o", ComputerID: "c"})
		out, err := tool.Func(ctx, json.RawMessage(`{"query":"page"}`))
		if err != nil {
			t.Fatal(err)
		}
		var body wireCorpusResult
		if err := json.Unmarshal([]byte(out), &body); err != nil {
			t.Fatal(err)
		}
		return logs.String(), body.Captures
	}
	line, caps := run(answer)
	if !strings.Contains(line, " vector=unavailable vector_state=unavailable bm25_kept=0 caller=cell") || strings.Join(titles(caps), ",") != "Beta page,Alpha page" {
		t.Fatalf("unavailable: %v\n%s", titles(caps), line)
	}
	answer.Ranked = &CaptureRanking{VectorAnswered: true, VectorRows: 7, VectorState: "approximate", BM25Kept: 2, Entries: answer.Ranked.Entries}
	if line, _ = run(answer); !strings.Contains(line, " vector=7 vector_state=approximate bm25_kept=2 caller=cell") {
		t.Fatalf("answered:\n%s", line)
	}
	// A corpusd with no ranking (an older one) is BM25's order and is logged unavailable.
	answer.Ranked = nil
	answer.FullText = &CaptureTextInfo{Used: true, Indexed: 2, Captures: 2}
	line, caps = run(answer)
	if !strings.Contains(line, " vector=unavailable vector_state=unavailable bm25_kept=0 ") || strings.Join(titles(caps), ",") != "Beta page,Alpha page" {
		t.Fatalf("no ranking: %v\n%s", titles(caps), line)
	}
}

// The mirror types decode what platform writes.
func TestRankedMirrorDecodesWhatPlatformWrites(t *testing.T) {
	in := platform.CaptureSearchResponse{Ranked: &platform.CaptureRanking{
		Entries:        []platform.RankedCapture{{CaptureID: "x", Reasons: []string{"text", "story"}, Stories: []string{"p"}}},
		Stories:        []platform.RankedStory{{PublicationID: "p", VersionID: "v", Revision: 3, Title: "T", Score: 0.7, Day: "2026-11-19", Captures: []string{"x"}}},
		VectorAnswered: true, VectorRows: 9, VectorState: "partial", VectorPartial: "captures: x", BM25Kept: 2, Note: "n"}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out CaptureSearchAnswer
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	back, _ := json.Marshal(out.Ranked)
	want, _ := json.Marshal(in.Ranked)
	if string(back) != string(want) {
		t.Fatalf("mirror drifted:\n got  %s\n want %s", back, want)
	}
}

// A common word is in thousands of labels; the guest sends the best few by id,
// not all of them, and asks for a bounded number of candidates.
func TestGuestSendsBoundedLabelHintsAndCandidates(t *testing.T) {
	labels := map[string]store.CaptureLabel{}
	for i := 0; i < 1500; i++ {
		labels[fmt.Sprintf("cap-%04d", i)] = store.CaptureLabel{EventHeadline: "harbor strike"}
	}
	labels["cap-best"] = store.CaptureLabel{EventHeadline: "harbor strike quay dispute"}
	ids := labelMatchedCaptureIDs(labels, []string{"harbor", "quay", "dispute"})
	if len(ids) != corpusIDHints {
		t.Fatalf("%d hints, want %d", len(ids), corpusIDHints)
	}
	found := false
	for _, id := range ids {
		found = found || id == "cap-best"
	}
	if !found {
		t.Fatal("the label holding the most words was dropped")
	}
	again := labelMatchedCaptureIDs(labels, []string{"harbor", "quay", "dispute"})
	if strings.Join(ids, ",") != strings.Join(again, ",") {
		t.Fatal("the hints are not deterministic")
	}
	deps, rs := rankedWorld(t, CaptureSearchAnswer{})
	if _, _, _, err := searchCapturesFor(context.Background(), deps, "o", "c", corpusRequest{Query: "harbor", Terms: []string{"harbor"}}, 10); err != nil {
		t.Fatal(err)
	}
	if len(rs.asked) != 1 || rs.asked[0].Candidates != corpusIndexCandidates || corpusIndexCandidates > 4*corpusMaxLimit {
		t.Fatalf("asked %+v", rs.asked)
	}
}

// Findings of the review of the ranked answer: a page found by meaning is not
// "labels only"; the appended label tail is newest first under order=recent;
// the guest's window drops a capture with neither time like the other halves.
func TestGuestCountsPagesFoundByMeaningApartFromLabelOnlyPages(t *testing.T) {
	a, b, c := rankedObj(t, "a", "Alpha page"), rankedObj(t, "b", "Beta page"), rankedObj(t, "c", "Gamma page")
	answer := CaptureSearchAnswer{
		HitObjects: []objectgraph.Object{a, b, c},
		Hits:       []CaptureTextHit{{CaptureID: a.CanonicalID, Snippet: "pa", BodyMatch: true}},
		FullText:   &CaptureTextInfo{Used: true, Indexed: 3, Captures: 3},
		Ranked: &CaptureRanking{VectorAnswered: true, VectorRows: 3, VectorState: "exact", Entries: []RankedCapture{
			{CaptureID: a.CanonicalID, Reasons: []string{"text", "meaning"}},
			{CaptureID: b.CanonicalID, Reasons: []string{"meaning"}},
			{CaptureID: c.CanonicalID, Reasons: []string{"meaning", "story"}, Stories: []string{"p"}},
		}},
	}
	deps, _ := rankedWorld(t, answer)
	_, _, info, err := searchCapturesFor(context.Background(), deps, "o", "c", corpusRequest{Query: "alpha", Terms: corpusQueryTerms("alpha")}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if info.Body != 1 || info.Meaning != 2 || info.LabelsOnly != 0 {
		t.Fatalf("by_body %d by_meaning %d by_labels_only %d: meaning pages are not label-only pages", info.Body, info.Meaning, info.LabelsOnly)
	}
	logs := captureReadLog(t)
	logCorpusSearch(context.Background(), "q", "c", 0, nil, "", 0, 0, false, info)
	if !strings.Contains(logs.String(), " by_labels_only=0 by_meaning=2 ") {
		t.Fatalf("line %q", logs.String())
	}
}

func TestAppendLabelHalfTailIsNewestFirstUnderRecentOrder(t *testing.T) {
	p := CorpusCapture{CaptureID: "p", FetchedAt: "2026-11-01T00:00:00Z"}
	old := CorpusCapture{CaptureID: "old", FetchedAt: "2026-10-01T00:00:00Z"}
	mid := CorpusCapture{CaptureID: "mid", PublishedAt: "2026-10-15T00:00:00Z", FetchedAt: "2026-11-09T00:00:00Z"}
	newest := CorpusCapture{CaptureID: "new", FetchedAt: "2026-10-30T00:00:00Z"}
	got := appendLabelHalf([]CorpusCapture{p}, []CorpusCapture{old, mid, newest}, 10, "recent")
	if strings.Join(ids(got), ",") != "p,new,mid,old" {
		t.Fatalf("recent tail %v", ids(got))
	}
	// The two halves are one time order under recent: a label page newer than
	// the answer's rows stands above them.
	fresher := CorpusCapture{CaptureID: "fresh", FetchedAt: "2026-11-15T00:00:00Z"}
	got = appendLabelHalf([]CorpusCapture{p}, []CorpusCapture{old, fresher}, 10, "recent")
	if strings.Join(ids(got), ",") != "fresh,p,old" {
		t.Fatalf("merged by time %v", ids(got))
	}
	got = appendLabelHalf([]CorpusCapture{p}, []CorpusCapture{old, mid, newest}, 10, "relevance")
	if strings.Join(ids(got), ",") != "p,old,mid,new" {
		t.Fatalf("relevance tail %v", ids(got))
	}
}

func TestInWindowDropsACaptureWithNeitherTimeOnEveryKindOfWindow(t *testing.T) {
	obj := rankedObj(t, "x", "No time")
	var meta map[string]any
	_ = json.Unmarshal(obj.Metadata, &meta)
	meta["fetched_at"], meta["published_at"] = "", ""
	obj.Metadata, _ = json.Marshal(meta)
	now := time.Now()
	for name, w := range map[string][2]time.Time{"since": {now.Add(-time.Hour), {}}, "until": {{}, now.Add(time.Hour)}, "both": {now.Add(-time.Hour), now.Add(time.Hour)}} {
		if got := inWindow([]objectgraph.Object{obj}, w[0], w[1]); len(got) != 0 {
			t.Fatalf("%s: kept a capture with neither time", name)
		}
	}
	if got := inWindow([]objectgraph.Object{obj}, time.Time{}, time.Time{}); len(got) != 1 {
		t.Fatal("no window must keep it")
	}
}

// The corpus search tells the run whether the graph answered (2026-11-22: it returned
// captures in 224 of 224 searches because it falls back to the nearest by meaning, so a
// run could not tell "the graph lacks this" from "the graph holds ten pages"). Failure
// list: the result opens with no counts; the words count includes pages found by
// meaning alone; a zero count is left out of the JSON (omitempty) so the run reads
// nothing; a search with no word match does not say the graph likely lacks this; the
// counts do not add up to the captures listed.
func TestCorpusResultOpensWithHowManyCapturesMatchedTheWordsAndHowManyAreNearOnly(t *testing.T) {
	a, b, c := rankedObj(t, "a", "Alpha page"), rankedObj(t, "b", "Beta page"), rankedObj(t, "c", "Gamma page")
	call := func(answer CaptureSearchAnswer) (string, wireCorpusResult) {
		deps, _ := rankedWorld(t, answer)
		ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-9", OwnerID: "o", ComputerID: "c"})
		out, err := newSearchWireCorpusTool(deps).Func(ctx, json.RawMessage(`{"query":"alpha"}`))
		if err != nil {
			t.Fatal(err)
		}
		var body wireCorpusResult
		if err := json.Unmarshal([]byte(out), &body); err != nil {
			t.Fatal(err)
		}
		return out, body
	}
	// One page whose text holds the words, two found by meaning alone.
	out, body := call(CaptureSearchAnswer{
		HitObjects: []objectgraph.Object{a, b, c},
		Hits:       []CaptureTextHit{{CaptureID: a.CanonicalID, Snippet: "pa", BodyMatch: true}},
		FullText:   &CaptureTextInfo{Used: true, Indexed: 3, Captures: 3},
		Ranked: &CaptureRanking{VectorAnswered: true, VectorRows: 3, VectorState: "exact", Entries: []RankedCapture{
			{CaptureID: a.CanonicalID, Reasons: []string{"text", "meaning"}},
			{CaptureID: b.CanonicalID, Reasons: []string{"meaning"}},
			{CaptureID: c.CanonicalID, Reasons: []string{"meaning", "story"}, Stories: []string{"p"}},
		}},
	})
	if !strings.HasPrefix(out, `{"captures_matched_words":1,"captures_near_by_meaning_only":2,`) {
		t.Fatalf("the result does not open with the two counts: %.160s", out)
	}
	if body.MatchedWords != 1 || body.NearByMeaningOnly != 2 || body.MatchedWords+body.NearByMeaningOnly != body.CaptureCount || body.WordsNote != "" {
		t.Fatalf("counts %d + %d of %d captures, note %q", body.MatchedWords, body.NearByMeaningOnly, body.CaptureCount, body.WordsNote)
	}
	// No page holds the words: the counts say 0 and 3, and the result says the graph
	// likely lacks this.
	out, body = call(CaptureSearchAnswer{
		HitObjects: []objectgraph.Object{a, b, c},
		FullText:   &CaptureTextInfo{Used: true, Indexed: 3, Captures: 3},
		Ranked: &CaptureRanking{VectorAnswered: true, VectorRows: 3, VectorState: "exact", Entries: []RankedCapture{
			{CaptureID: a.CanonicalID, Reasons: []string{"meaning"}},
			{CaptureID: b.CanonicalID, Reasons: []string{"meaning"}},
			{CaptureID: c.CanonicalID, Reasons: []string{"meaning"}},
		}},
	})
	if !strings.HasPrefix(out, `{"captures_matched_words":0,"captures_near_by_meaning_only":3,`) {
		t.Fatalf("a zero count must be written, not omitted: %.160s", out)
	}
	if !strings.Contains(body.WordsNote, "likely lacks") {
		t.Fatalf("no word matched and the result does not say the graph likely lacks this: %q", body.WordsNote)
	}
	// Part of the page text is not searchable yet: zero word matches does not mean the graph
	// lacks the words, so the note stays silent (the counts are still written).
	out, body = call(CaptureSearchAnswer{
		HitObjects: []objectgraph.Object{a},
		FullText:   &CaptureTextInfo{Used: true, Indexed: 1, Captures: 3},
		Ranked: &CaptureRanking{VectorAnswered: true, VectorRows: 1, VectorState: "exact", Entries: []RankedCapture{
			{CaptureID: a.CanonicalID, Reasons: []string{"meaning"}},
		}},
	})
	if body.WordsNote != "" || !strings.HasPrefix(out, `{"captures_matched_words":0,"captures_near_by_meaning_only":1,`) {
		t.Fatalf("a partly indexed corpus must not claim the graph lacks the words: %q / %.120s", body.WordsNote, out)
	}
}

// A page the label half matched by its title is a word match even when the vector half also
// returned it (review of 6980fdb96, B3): counting only corpusd's text hits called it "near by
// meaning only" and told the run the graph likely lacks a subject whose title it had just listed.
func TestCorpusCountsATitleMatchTheVectorHalfAlsoReturnedAsAWordMatch(t *testing.T) {
	a := rankedObj(t, "a", "Alpha page")
	answer := CaptureSearchAnswer{
		Objects:    []objectgraph.Object{a},
		HitObjects: []objectgraph.Object{a},
		FullText:   &CaptureTextInfo{Used: true, Indexed: 0, Captures: 1},
		Ranked: &CaptureRanking{VectorAnswered: true, VectorRows: 1, VectorState: "exact", Entries: []RankedCapture{
			{CaptureID: a.CanonicalID, Reasons: []string{"meaning"}},
		}},
	}
	deps, _ := rankedWorld(t, answer)
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-9", OwnerID: "o", ComputerID: "c"})
	out, err := newSearchWireCorpusTool(deps).Func(ctx, json.RawMessage(`{"query":"alpha"}`))
	if err != nil {
		t.Fatal(err)
	}
	var body wireCorpusResult
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Captures) != 1 || !containsString(body.Captures[0].Matched, "title") {
		t.Fatalf("the listed row does not carry its title match: %+v", body.Captures)
	}
	if body.MatchedWords != 1 || body.NearByMeaningOnly != 0 || body.WordsNote != "" {
		t.Fatalf("a title-matched page counted as %d matched, %d near only, note %q", body.MatchedWords, body.NearByMeaningOnly, body.WordsNote)
	}
}

// words_note must not say the graph likely lacks the subject when something else already
// answered the query, or when a window hid the matching page (review 5): events or published
// pieces that matched are what to read, and a since/until window only says none matched in it.
func TestWordsNoteDoesNotClaimALackWhenEventsMatchedOrAWindowHidPages(t *testing.T) {
	full := corpusSearchInfo{TextUsed: true, TextIndexed: 3, Captures: 3}
	if got := wordsNote(0, 3, full, true, 0, false); !strings.Contains(got, "the graph likely lacks this") {
		t.Fatalf("the plain zero-match note lost its claim: %q", got)
	}
	got := wordsNote(0, 3, full, true, 2, false)
	if got == "" || strings.Contains(got, "likely lacks") || !strings.Contains(got, "2 events or published pieces matched") {
		t.Fatalf("events or published pieces matched, so the graph does not lack the subject: %q", got)
	}
	got = wordsNote(0, 3, full, true, 0, true)
	if got == "" || strings.Contains(got, "the graph likely lacks this") || !strings.Contains(got, "in this since/until window") {
		t.Fatalf("a window may have hidden the page, so the note is scoped to it: %q", got)
	}
}

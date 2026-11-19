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
		Entries:        []platform.RankedCapture{{CaptureID: "x", Reasons: []string{"text", "story"}, Stories: []string{"p"}, Score: 0.5}},
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

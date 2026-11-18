package researchtools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/objectgraph/gatefixture"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// The corpus search over captured pages' full text, from the research
// desk's side: the tool, corpusd's real routes, a real Store B and a real
// full-text index. Failure modes guarded (docs/reference/CAPTURE-FULLTEXT.md),
// guarded:
//   1. a fact only in a page's body is not found, or is returned with no
//      passage and no way to cite and read the page;
//   2. a 12-word question returns nothing;
//   3. a page is not first for its own title;
//   5. since/until/order do not reach the captures;
//   9. before the catch-up reaches a page, it is no longer found by title;
//  13. the journal line lacks the counts, or carries text.

func TestParseCorpusWindow(t *testing.T) {
	q, w, err := parseCorpusWindow("amber treaty since:2026-11-10 until:2026-11-18 order:recent", "", "", "")
	if err != nil || q != "amber treaty" || w.Order != "recent" ||
		!w.Since.Equal(time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)) || !w.Until.Equal(time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("operators: %q %+v %v", q, w, err)
	}
	// Fields win over operators; the default order is relevance.
	q, w, err = parseCorpusWindow("amber since:2026-01-01", "2026-11-12T08:00:00Z", "", "")
	if err != nil || q != "amber" || w.Order != "relevance" || !w.Since.Equal(time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("fields: %q %+v %v", q, w, err)
	}
	for _, bad := range [][4]string{{"x", "yesterday", "", ""}, {"x", "", "2026-13-45", ""}, {"x", "", "", "oldest"}} {
		if _, _, err := parseCorpusWindow(bad[0], bad[1], bad[2], bad[3]); err == nil {
			t.Fatalf("%v must be refused", bad)
		}
	}
	// Words that merely contain the operator names stay in the query.
	if q, _, _ := parseCorpusWindow("sinceless order of things", "", "", ""); q != "sinceless order of things" {
		t.Fatalf("query %q", q)
	}
}

func TestMergeCaptureHalves(t *testing.T) {
	a := CorpusCapture{CaptureID: "a", Matched: []string{"body"}, Passage: "pa", FetchedAt: "2026-11-01T00:00:00Z"}
	b := CorpusCapture{CaptureID: "b", Matched: []string{"title"}, FetchedAt: "2026-11-05T00:00:00Z"}
	c := CorpusCapture{CaptureID: "c", Matched: []string{"title"}, FetchedAt: "2026-11-03T00:00:00Z"}
	bBody := b
	bBody.Matched = []string{"title", "body"}
	bBody.Passage = "pb"
	// b is found by both halves and rises above a, which only the text found.
	got := mergeCaptureHalves([]CorpusCapture{a, bBody}, []CorpusCapture{b, c}, "relevance", 10)
	if len(got) != 3 || got[0].CaptureID != "b" || got[1].CaptureID != "a" || got[2].CaptureID != "c" || got[0].Passage != "pb" {
		t.Fatalf("relevance merge: %+v", got)
	}
	got = mergeCaptureHalves([]CorpusCapture{a, bBody}, []CorpusCapture{b, c}, "recent", 10)
	if got[0].CaptureID != "b" || got[1].CaptureID != "c" || got[2].CaptureID != "a" {
		t.Fatalf("recent merge: %+v", got)
	}
	if got := mergeCaptureHalves([]CorpusCapture{a, bBody}, nil, "relevance", 1); len(got) != 1 {
		t.Fatalf("limit: %+v", got)
	}
}

type ftWorld struct {
	pstore *platform.Store
	og     *platform.ObjectGraphStore
	h      *platform.ObjectGraphHandler
	deps   Dependencies
	owner  string
	comp   string
}

func newFTWorld(t *testing.T, configureIndex bool) *ftWorld {
	t.Helper()
	gatefixture.Observe(t)
	w := &ftWorld{owner: "owner-paper", comp: "computer-paper"}
	w.pstore = openEmbeddedPlatformStore(t)
	w.og = platform.NewObjectGraphStore(w.pstore)
	w.h = platform.NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: w.og}), w.og)
	if err := w.h.ConfigureGuestBinding(guestBinding{w.owner, w.comp}); err != nil {
		t.Fatal(err)
	}
	w.h.ConfigureCaptureBodies(t.TempDir())
	w.h.ConfigureCaptureIndex(w.pstore)
	if configureIndex {
		w.configure(t)
	}
	srv := server.NewServer("corpusd-test", "0")
	platform.RegisterObjectGraphRoutes(srv, w.h)
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })
	sink := NewCorpusdCaptureSink(hs.URL)
	sink.Client = &http.Client{Transport: internalCaller{http.DefaultTransport}}
	w.deps = Dependencies{Store: guest, Captures: sink}
	return w
}

func (w *ftWorld) configure(t *testing.T) {
	t.Helper()
	if err := w.pstore.ConfigureCaptureFullText(t.TempDir(), w.h.CaptureBodyStore()); err != nil {
		t.Fatal(err)
	}
}

func (w *ftWorld) page(t *testing.T, key, title, text string, fetched time.Time) objectgraph.Object {
	t.Helper()
	ref, err := w.h.CaptureBodyStore().PutBody([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, w.owner, objectgraph.StableSuffixFromKey(key))
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: "https://example.com/" + key, CanonicalURL: "https://example.com/" + key,
		Title: title, FetchedAt: fetched.Format(time.RFC3339Nano), ContentBlobID: ref, ExtractedTextBlobID: ref, BlocksBlobID: ref, Extractor: "x1", DocHash: "h-" + key, BlockCount: 3})
	return objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: w.owner, ComputerID: w.comp, ContentHash: "h-" + key, Metadata: meta, CreatedAt: fetched, UpdatedAt: fetched}
}

func (w *ftWorld) search(t *testing.T, req corpusRequest) ([]CorpusCapture, string, corpusSearchInfo) {
	t.Helper()
	req.Terms = corpusQueryTerms(req.Query)
	got, note, info, err := searchCapturesFor(context.Background(), w.deps, w.owner, w.comp, req, corpusMaxLimit)
	if err != nil {
		t.Fatalf("searchCapturesFor %q: %v", req.Query, err)
	}
	return got, note, info
}

func ids(cs []CorpusCapture) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.CaptureID)
	}
	return out
}

func ftFiller(n int, salt string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "Entry %s%d lists the routine ledger of crates and ropes at the quay, nothing unusual noted.\n\n", salt, i)
	}
	return b.String()
}

func TestCorpusSearchFindsPageTextWithPassageAndKeepsTitleSearch(t *testing.T) {
	w := newFTWorld(t, true)
	now := time.Now().UTC().Truncate(time.Second)
	fact := "The quorum of Valdoria ratified the amber treaty after a long night of argument."
	body := w.page(t, "body", "Weekly digest", ftFiller(12, "a")+fact+"\n\n"+ftFiller(12, "b"), now.Add(-2*time.Hour))
	titled := w.page(t, "titled", "Lighthouse keepers strike over lamp oil", ftFiller(4, "t"), now.Add(-3*time.Hour))
	other := w.page(t, "other", "Quay report", ftFiller(6, "o"), now.Add(-4*time.Hour))
	if err := w.og.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{body, titled, other}}); err != nil {
		t.Fatal(err)
	}

	// 1. A fact only in the body, with its passage and a citable handle.
	got, _, info := w.search(t, corpusRequest{Query: "amber treaty"})
	if len(got) != 1 || got[0].CaptureID != body.CanonicalID {
		t.Fatalf("body fact: %v", ids(got))
	}
	c := got[0]
	if !strings.Contains(c.Passage, "amber treaty") || c.URL != "https://example.com/body" || c.PassageCount < 2 || !containsString(c.Matched, "body") || containsString(c.Matched, "title") {
		t.Fatalf("hit lacks passage, url or evidence: %+v", c)
	}
	if !info.TextUsed || info.Body != 1 || info.Title != 0 || info.Both != 0 || info.TextIndexed != 3 || info.Captures != 3 {
		t.Fatalf("info %+v", info)
	}

	// 2. A 12-word question.
	got, _, _ = w.search(t, corpusRequest{Query: "what did the quorum of valdoria decide about the amber treaty last week"})
	if len(got) == 0 || got[0].CaptureID != body.CanonicalID {
		t.Fatalf("12-word question: %v", ids(got))
	}

	// 3. A page by its own title is first, found by title and by text.
	got, _, info = w.search(t, corpusRequest{Query: "Lighthouse keepers strike over lamp oil"})
	if len(got) == 0 || got[0].CaptureID != titled.CanonicalID || !containsString(got[0].Matched, "title") {
		t.Fatalf("known item: %v", ids(got))
	}
	if info.Title+info.Both == 0 {
		t.Fatalf("info %+v", info)
	}
}

func TestCorpusSearchWindowAndOrder(t *testing.T) {
	w := newFTWorld(t, true)
	day0 := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	var objs []objectgraph.Object
	for i := 0; i < 6; i++ {
		objs = append(objs, w.page(t, fmt.Sprint("d", i), "Quay inspection", "The inspector counted the barrels again.\n\n", day0.AddDate(0, 0, i)))
	}
	if err := w.og.PutBatch(context.Background(), objectgraph.Batch{Objects: objs}); err != nil {
		t.Fatal(err)
	}
	got, _, _ := w.search(t, corpusRequest{Query: "inspector barrels", Since: day0.AddDate(0, 0, 2), Until: day0.AddDate(0, 0, 4)})
	if len(got) != 2 {
		t.Fatalf("window holds %v, want pages 2 and 3", ids(got))
	}
	got, _, _ = w.search(t, corpusRequest{Query: "inspector barrels", Order: "recent"})
	if len(got) != 6 || got[0].CaptureID != objs[5].CanonicalID || got[5].CaptureID != objs[0].CanonicalID {
		t.Fatalf("recent order %v", ids(got))
	}
	// The window reaches the title half too: a title-only query.
	got, _, _ = w.search(t, corpusRequest{Query: "Quay inspection", Since: day0.AddDate(0, 0, 5)})
	if len(got) != 1 || got[0].CaptureID != objs[5].CanonicalID {
		t.Fatalf("window on a title search: %v", ids(got))
	}
}

func TestCorpusSearchBeforeTheCatchUpIsStillFoundByTitle(t *testing.T) {
	w := newFTWorld(t, false)
	now := time.Now().UTC().Truncate(time.Second)
	old := w.page(t, "older", "Harbor ledger of the spring", "Barrel counts; the zanzibar clause was noted.\n\n", now.Add(-time.Hour))
	if err := w.og.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{old}}); err != nil {
		t.Fatal(err)
	}
	w.configure(t) // the index exists now; this page predates it
	got, note, info := w.search(t, corpusRequest{Query: "harbor ledger"})
	if len(got) != 1 || got[0].CaptureID != old.CanonicalID || info.LabelsOnly != 1 {
		t.Fatalf("title search before the catch-up: %v %+v", ids(got), info)
	}
	if !strings.Contains(note, "page text of 0 of this computer's 1 captures is searchable so far") {
		t.Fatalf("the note must say how much text is searchable: %q", note)
	}
	if got, _, _ := w.search(t, corpusRequest{Query: "zanzibar clause"}); len(got) != 0 {
		t.Fatalf("a fact in the body of an uncaught page: %v", ids(got))
	}
	if _, err := w.pstore.CatchUpCaptureFullText(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := w.search(t, corpusRequest{Query: "zanzibar clause"}); len(got) != 1 {
		t.Fatalf("after the catch-up: %v", ids(got))
	}
}

func TestCorpusToolReturnsThePassageAndLogsCounts(t *testing.T) {
	w := newFTWorld(t, true)
	now := time.Now().UTC().Truncate(time.Second)
	fact := "The quorum of Valdoria ratified the amber treaty after a long night of argument."
	body := w.page(t, "body", "Weekly digest", ftFiller(5, "a")+fact+"\n\n", now.Add(-time.Hour))
	if err := w.og.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{body}}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	tool := newSearchWireCorpusTool(w.deps)
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{OwnerID: w.owner, ComputerID: w.comp})
	raw, _ := json.Marshal(map[string]any{"query": "amber treaty since:2020-01-01 order:recent"})
	out, err := tool.Func(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Query    string          `json:"query"`
		Captures []CorpusCapture `json:"captures"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if res.Query != "amber treaty" || len(res.Captures) != 1 {
		t.Fatalf("tool result: %s", out)
	}
	c := res.Captures[0]
	if c.CaptureID != body.CanonicalID || !strings.Contains(c.Passage, "amber treaty") || c.URL == "" || c.PassageOrdinal < 1 {
		t.Fatalf("capture lacks what the desk needs to cite and read it: %+v", c)
	}
	line := buf.String()
	for _, want := range []string{"text_index=true", "by_body=1", "by_title=0", "by_both=0", "capture_ids=" + body.CanonicalID} {
		if !strings.Contains(line, want) {
			t.Fatalf("journal line lacks %q:\n%s", want, line)
		}
	}
	if strings.Contains(line, "Valdoria") {
		t.Fatalf("the journal line carries page text:\n%s", line)
	}
	badRaw, _ := json.Marshal(map[string]any{"query": "amber", "since": "yesterday"})
	if _, err := tool.Func(ctx, badRaw); err == nil {
		t.Fatal("an unreadable date must be refused")
	}
}

func TestParseCorpusWindowKeepsWordsThatEndInAColon(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"executive order: tariff pause", "executive order: tariff pause"},
		{"since: the strike began", "since: the strike began"},
		{"until: friday", "until: friday"},
		{"executive order:recent tariff", "executive tariff"}, // a real operator is removed
	} {
		q, _, err := parseCorpusWindow(c.in, "", "", "")
		if err != nil || q != c.want {
			t.Errorf("%q -> %q, %v; want %q", c.in, q, err, c.want)
		}
	}
	if _, w, _ := parseCorpusWindow("executive order: tariff pause", "", "", ""); w.Order != "relevance" {
		t.Fatalf("a bare order: must not set an order: %+v", w)
	}
}

type textAnswerSink struct {
	CaptureSink
	info *CaptureTextInfo
}

func (s textAnswerSink) SearchCaptures(ctx context.Context, q CaptureSearchQuery) (CaptureSearchAnswer, error) {
	return CaptureSearchAnswer{FullText: s.info}, nil
}

func TestCorpusSearchNamesTheWordsItDidNotSearch(t *testing.T) {
	t.Setenv(corpusSearchModeEnv, "")
	info := &CaptureTextInfo{Used: true, Indexed: 3, Captures: 3, Pruned: []string{"week", "decided"}, TermCapped: []string{"zeta"},
		Excluded: []string{"union"}, PhraseFallbacks: []string{"strike harbor"}}
	_, note, got, err := searchCapturesFor(context.Background(), Dependencies{Captures: textAnswerSink{info: info}}, "o", "c",
		corpusRequest{Query: `"strike harbor" -union`, Terms: corpusQueryTerms("strike harbor")}, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"week", "decided", "zeta", "union", "strike harbor"} {
		if !strings.Contains(note, want) {
			t.Fatalf("the note lacks %q: %q", want, note)
		}
	}
	if len(got.Dropped) != 3 {
		t.Fatalf("dropped words for the model: %v", got.Dropped)
	}
	// No usable word: the note says so; it is not a search with no hits.
	none := &CaptureTextInfo{Used: false, Reason: "the query has no searchable words"}
	_, note, _, _ = searchCapturesFor(context.Background(), Dependencies{Captures: textAnswerSink{info: none}}, "o", "c", corpusRequest{Query: "!!!"}, 10)
	if !strings.Contains(note, "no searchable words") {
		t.Fatalf("note %q", note)
	}
}

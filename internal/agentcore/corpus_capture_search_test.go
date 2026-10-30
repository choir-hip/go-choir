package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Research searches the computer's own corpus before the web: the corpus
// verb reaches captures, not only stories (symptom 3 of
// docs/archive/problems/prompt-bar-research-slow-unsynthesized-mangled-2026-10-14.md).
// Ways it fails, written before the code:
//   - captures are not searched at all, so a capture the paper already holds
//     is fetched again from the web (the defect);
//   - a capture is found by its title but not by its outlet (the URL's host,
//     its source id, or the outlet Jev's label names);
//   - a capture in another script is never found by an English query, though
//     Jev's label ties it to the story's topic;
//   - the search scopes by the CHOIR_OWNER_ID environment variable instead
//     of the run's own owner and computer, so a cell (whose payload carries
//     no owner) searches the wrong corpus or another computer's captures;
//   - results are unbounded: a broad word over a 24/7 paper's captures
//     floods the cell, or the store listing is unbounded;
//   - a query word matches inside other words ("ai" in "talks"), so short
//     words flood the results;
//   - matches are not ranked: an older or weaker match crowds out a newer,
//     stronger one;
//   - the result carries no handle research can cite, or a handle Texture's
//     {{cite}} refuses;
//   - a capture-store outage fails the whole call (stories lost) or reads
//     as "no captures", hiding the outage.

type corpusCapture struct {
	owner, computer, key string
	title, url, sourceID string
	language, fetchedAt  string
	html                 string
	id                   string
	blocks               []sourcefetch.Block
}

type fakeCorpusd struct {
	srv       *httptest.Server
	captures  []*corpusCapture
	listLimit []string
	failList  bool
}

// newFakeCorpusd serves corpusd's object list (filtered by kind, owner and
// computer, newest first, cut at limit) and capture read routes from memory.
func newFakeCorpusd(t *testing.T, captures ...*corpusCapture) *fakeCorpusd {
	t.Helper()
	f := &fakeCorpusd{captures: captures}
	for _, c := range captures {
		id, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, c.owner, objectgraph.StableSuffixFromKey(c.key))
		if err != nil {
			t.Fatal(err)
		}
		c.id = id
		if c.html != "" {
			c.blocks = sourcefetch.ExtractBlocks([]byte(c.html)).Blocks
		}
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/platform/objects":
			q := r.URL.Query()
			f.listLimit = append(f.listLimit, q.Get("limit"))
			if f.failList {
				http.Error(w, `{"error":"failed to list objects"}`, http.StatusInternalServerError)
				return
			}
			var matched []*corpusCapture
			for _, c := range f.captures {
				if q.Get("kind") == string(objectgraph.WebCaptureObjectKind) && q.Get("owner") == c.owner && q.Get("computer") == c.computer {
					matched = append(matched, c)
				}
			}
			sort.SliceStable(matched, func(i, j int) bool { return matched[i].fetchedAt > matched[j].fetchedAt })
			if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 && len(matched) > n {
				matched = matched[:n]
			}
			objs := []objectgraph.Object{}
			for _, c := range matched {
				meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: c.url, CanonicalURL: c.url, Title: c.title,
					FetchedAt: c.fetchedAt, ContentBlobID: "sha256:raw", ExtractedTextBlobID: "sha256:text", BlocksBlobID: "sha256:blocks", Extractor: "x1", DocHash: "h-" + c.key,
					SourceID: c.sourceID, Language: c.language, BlockCount: len(c.blocks)})
				objs = append(objs, objectgraph.Object{CanonicalID: c.id, ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: c.owner, ComputerID: c.computer, Metadata: meta})
			}
			_ = json.NewEncoder(w).Encode(objs)
		case strings.HasPrefix(r.URL.Path, "/internal/platform/captures/"):
			id := strings.TrimPrefix(r.URL.Path, "/internal/platform/captures/")
			for _, c := range f.captures {
				if c.id == id {
					_ = json.NewEncoder(w).Encode(map[string]any{"capture_id": c.id, "url": c.url, "title": c.title, "doc_hash": "h-" + c.key, "block_count": len(c.blocks), "from_block": 0, "blocks": c.blocks})
					return
				}
			}
			http.Error(w, `{"error":"capture not found"}`, http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

type corpusHit struct {
	CaptureID string   `json:"capture_id"`
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	Outlet    string   `json:"outlet"`
	Topic     string   `json:"topic"`
	FetchedAt string   `json:"fetched_at"`
	Matched   []string `json:"matched"`
}

type corpusSearchOut struct {
	Results []struct {
		DocID string `json:"doc_id"`
	} `json:"results"`
	Captures     []corpusHit `json:"captures"`
	CaptureCount int         `json:"capture_count"`
	CapturesNote string      `json:"captures_note"`
}

// searchCorpusAsCell calls search_wire_corpus the way a research cell does:
// the payload carries no owner, the execution context carries the run's.
func searchCorpusAsCell(t *testing.T, deps *researchtools.Dependencies, owner, computer, payload string) corpusSearchOut {
	t.Helper()
	ectx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "research-run", OwnerID: owner, ComputerID: computer, Profile: agentprofile.Research})
	raw, err := deps.HostEgress(ectx, "search_wire_corpus", json.RawMessage(payload))
	if err != nil {
		t.Fatalf("search_wire_corpus %s: %v", payload, err)
	}
	var out corpusSearchOut
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func corpusFixture(t *testing.T) (*store.Store, *fakeCorpusd, *researchtools.Dependencies, string, string) {
	t.Helper()
	t.Setenv("CHOIR_OWNER_ID", "owner-from-env")
	rt, s := testRuntime(t)
	owner, computer := "owner-corpus", rt.cfg.ComputerID
	corpus := newFakeCorpusd(t,
		&corpusCapture{owner: owner, computer: computer, key: "harbor", title: "Harbor reopens to cargo ships", url: "https://www.portnews.example/harbor",
			sourceID: "rss:portnews", language: "en", fetchedAt: "2026-10-14T03:00:00Z",
			html: `<html><head><title>Harbor reopens to cargo ships</title></head><body><article><h1>Harbor reopens</h1><p>The harbor reopened on Tuesday after the storm passed.</p><p>Cargo ships queued outside the breakwater.</p></article></body></html>`},
		&corpusCapture{owner: owner, computer: computer, key: "ceasefire", title: "Ceasefire talks resume", url: "https://www.reuters.com/world/ceasefire",
			sourceID: "rss:reuters-world", language: "en", fetchedAt: "2026-10-14T02:00:00Z"},
		&corpusCapture{owner: owner, computer: computer, key: "bandar", title: "بندر دوباره باز شد", url: "https://www.irna.ir/news/1",
			sourceID: "rss:irna-fa", language: "fa", fetchedAt: "2026-10-14T01:00:00Z"},
		&corpusCapture{owner: owner, computer: "computer-other", key: "other-computer", title: "Harbor reopens elsewhere", url: "https://example.org/a",
			fetchedAt: "2026-10-14T03:30:00Z"},
		&corpusCapture{owner: "owner-other", computer: computer, key: "other-owner", title: "Harbor reopens for another owner", url: "https://example.org/b",
			fetchedAt: "2026-10-14T03:40:00Z"},
	)
	if _, err := s.AppendCaptureLabel(context.Background(), store.CaptureLabel{OwnerID: owner, ComputerID: computer, CaptureID: corpus.captures[2].id,
		Title: "بندر دوباره باز شد", Outlet: "Islamic Republic News Agency", Language: "fa", Script: "Arabic", Event: store.CaptureLabelEventNew, TopicHeadline: "Harbor reopens after the storm"}); err != nil {
		t.Fatal(err)
	}
	deps := &researchtools.Dependencies{Store: s, Captures: researchtools.NewCorpusdCaptureSink(corpus.srv.URL)}
	return s, corpus, deps, owner, computer
}

func hitIDs(hits []corpusHit) []string {
	var ids []string
	for _, h := range hits {
		ids = append(ids, h.CaptureID)
	}
	return ids
}

func TestSearchWireCorpusFindsTheComputersCaptures(t *testing.T) {
	s, corpus, deps, owner, computer := corpusFixture(t)
	story := openTestStory(t, s, owner, computer, "Harbor reopens after the storm")
	harbor, ceasefire, bandar := corpus.captures[0], corpus.captures[1], corpus.captures[2]

	// Title words: the capture whose title matches ranks above the one only
	// Jev's topic hint ties to the query; the story is still found.
	out := searchCorpusAsCell(t, deps, owner, computer, `{"query":"harbor reopens"}`)
	if len(out.Results) != 1 || out.Results[0].DocID != story {
		t.Fatalf("stories = %+v; want the owner's story %s", out.Results, story)
	}
	if got := hitIDs(out.Captures); len(got) != 2 || got[0] != harbor.id || got[1] != bandar.id {
		t.Fatalf("captures = %+v; want the title match %s, then the topic-labelled %s, and no other owner's or computer's capture", out.Captures, harbor.id, bandar.id)
	}
	if out.CaptureCount != 2 || out.Captures[0].Title != harbor.title || out.Captures[0].FetchedAt == "" || out.Captures[1].Topic != "Harbor reopens after the storm" {
		t.Fatalf("capture fields = %+v", out)
	}

	// Outlet: the URL's host or source id, and the outlet Jev's label names.
	if got := hitIDs(searchCorpusAsCell(t, deps, owner, computer, `{"query":"reuters"}`).Captures); len(got) != 1 || got[0] != ceasefire.id {
		t.Fatalf("outlet search = %v; want %s", got, ceasefire.id)
	}
	agency := searchCorpusAsCell(t, deps, owner, computer, `{"query":"news agency"}`)
	if got := hitIDs(agency.Captures); len(got) != 1 || got[0] != bandar.id || agency.Captures[0].Outlet != "Islamic Republic News Agency" {
		t.Fatalf("label outlet search = %+v; want %s", agency.Captures, bandar.id)
	}
	// A query word matches at the start of a word, never inside one: "ai"
	// is in "talks" and "Tuesday" is not a title word here.
	if got := searchCorpusAsCell(t, deps, owner, computer, `{"query":"ai"}`).Captures; len(got) != 0 {
		t.Fatalf("\"ai\" matched %+v; want word starts only", got)
	}
}

func TestSearchWireCorpusBoundsCaptureResults(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-bounds", rt.cfg.ComputerID
	var captures []*corpusCapture
	for i := 0; i < 40; i++ {
		captures = append(captures, &corpusCapture{owner: owner, computer: computer, key: fmt.Sprintf("u%02d", i), title: fmt.Sprintf("Harbor update %02d", i),
			url: fmt.Sprintf("https://example.org/u%02d", i), fetchedAt: fmt.Sprintf("2026-10-14T03:%02d:00Z", i)})
	}
	corpus := newFakeCorpusd(t, captures...)
	deps := &researchtools.Dependencies{Store: s, Captures: researchtools.NewCorpusdCaptureSink(corpus.srv.URL)}

	out := searchCorpusAsCell(t, deps, owner, computer, `{"query":"harbor","limit":5}`)
	if len(out.Captures) != 5 {
		t.Fatalf("limit 5 returned %d captures", len(out.Captures))
	}
	for i, hit := range out.Captures {
		if want := captures[39-i].id; hit.CaptureID != want {
			t.Fatalf("capture %d = %s; want the newest equal matches first (%s)", i, hit.CaptureID, want)
		}
	}
	if huge := searchCorpusAsCell(t, deps, owner, computer, `{"query":"harbor","limit":5000}`); len(huge.Captures) > 25 || len(huge.Captures) == 0 {
		t.Fatalf("limit 5000 returned %d captures; want a hard cap of 25", len(huge.Captures))
	}
	for _, limit := range corpus.listLimit {
		if n, err := strconv.Atoi(limit); err != nil || n <= 0 || n > 2000 {
			t.Fatalf("capture listing asked for limit %q; want a bounded scan", limit)
		}
	}
}

func TestCorpusCaptureHandleIsCitableByTexture(t *testing.T) {
	_, corpus, deps, owner, computer := corpusFixture(t)
	out := searchCorpusAsCell(t, deps, owner, computer, `{"query":"cargo ships"}`)
	if len(out.Captures) != 1 || out.Captures[0].CaptureID != corpus.captures[0].id {
		t.Fatalf("captures = %+v", out.Captures)
	}
	handle := out.Captures[0].CaptureID
	base := texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{Type: "doc", Attrs: map[string]any{"id": "root"}, Content: []texturedoc.Node{
		{Type: "paragraph", Attrs: map[string]any{"id": "p1"}, Content: []texturedoc.Node{{Type: "text", Text: "Opening."}}},
	}}}
	res, err := texturewrite.Apply(context.Background(), texturewrite.Input{OwnerID: owner, DocID: "d", Seed: "s", Base: base,
		Resolver: researchtools.NewTextureCaptures(deps.Captures, computer),
		Script:   "@insert end\nThe harbor reopened on Tuesday {{cite " + handle + " \"harbor reopened on Tuesday\"}}."})
	if err != nil {
		t.Fatalf("Texture refused the corpus handle: %v", err)
	}
	var ref types.Ref
	for _, n := range res.Doc.Doc.Content[len(res.Doc.Doc.Content)-1].Content {
		if r, ok := texturewrite.RefOf(n); ok {
			ref = r
		}
	}
	if ref.Doc != handle || ref.Quote != "harbor reopened on Tuesday" || ref.CID == "" {
		t.Fatalf("ref = %+v; want a block of %s", ref, handle)
	}
}

func TestSearchWireCorpusKeepsStoriesWhenCapturesFail(t *testing.T) {
	s, corpus, deps, owner, computer := corpusFixture(t)
	story := openTestStory(t, s, owner, computer, "Harbor reopens after the storm")
	corpus.failList = true
	out := searchCorpusAsCell(t, deps, owner, computer, `{"query":"harbor"}`)
	if len(out.Results) != 1 || out.Results[0].DocID != story {
		t.Fatalf("stories = %+v; a capture outage must not lose them", out.Results)
	}
	if len(out.Captures) != 0 || !strings.Contains(out.CapturesNote, "500") {
		t.Fatalf("captures = %+v, note %q; want none and a note naming the outage", out.Captures, out.CapturesNote)
	}
	unconfigured := &researchtools.Dependencies{Store: s}
	if note := searchCorpusAsCell(t, unconfigured, owner, computer, `{"query":"harbor"}`).CapturesNote; !strings.Contains(note, "not configured") {
		t.Fatalf("note = %q; want the unconfigured capture store named", note)
	}
}

// The event layer's V2 (docs/reports/event-layer-at-ingestion-design-
// 2026-10-16.md §7): corpus search returns the world events it matches
// before captures, so research reads what the paper already knows about an
// occurrence, from whom and since when, in one result. Ways it fails,
// written before the code:
//   - V2-1 events are not returned, or another computer's or owner's are;
//   - V2-2 a tentative member's outlet counts toward an event's outlets or
//     corroboration, or wire copy counts twice;
//   - V2-3 a corroborated event ranks below a single-outlet one of the same
//     match strength;
//   - V2-4 events are unbounded, or a store failure fails the whole call;
//   - V2-5 an event result carries no capture handle research can read.
func TestSearchWireCorpusReturnsMatchingEventsFirst(t *testing.T) {
	s, _, deps, owner, computer := corpusFixture(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 15, 22, 0, 0, 0, time.UTC)
	put := func(own, comp, eventID, headline string, members ...store.EventMembership) {
		t.Helper()
		if _, _, err := s.PutWorldEvent(ctx, store.WorldEvent{OwnerID: own, ComputerID: comp, EventID: eventID, Headline: headline, OpenedBy: members[0].CaptureID, FirstSeen: at}); err != nil {
			t.Fatal(err)
		}
		for i, m := range members {
			m.OwnerID, m.ComputerID, m.EventID = own, comp, eventID
			m.CapturedAt, m.ResolvedAt = at.Add(time.Duration(i)*time.Minute), at.Add(time.Duration(i)*time.Minute)
			if _, _, err := s.PutEventMembership(ctx, m); err != nil {
				t.Fatal(err)
			}
		}
	}
	put(owner, computer, "ev-harbor", "Harbor reopens after the storm",
		store.EventMembership{CaptureID: "cap-h1", Title: "Harbor reopens after the storm", Outlet: "portnews.example", URL: "https://www.portnews.example/h", Opened: true},
		store.EventMembership{CaptureID: "cap-h2", Title: "Port reopens to cargo traffic", Outlet: "bbc.co.uk", URL: "https://www.bbc.co.uk/h"},
		store.EventMembership{CaptureID: "cap-h3", Title: "Harbour reopening", Outlet: "dw.com", Tentative: true})
	put(owner, computer, "ev-harbor-wire", "Harbor workers strike",
		store.EventMembership{CaptureID: "cap-w1", Title: "Harbor workers strike", Outlet: "thestar.com", Origin: "wire:reuters", Opened: true},
		store.EventMembership{CaptureID: "cap-w2", Title: "Harbor strike spreads", Outlet: "cna.com", Origin: "wire:reuters"})
	put(owner, "computer-other", "ev-other", "Harbor reopens elsewhere",
		store.EventMembership{CaptureID: "cap-o1", Title: "Harbor reopens elsewhere", Outlet: "example.org", Opened: true})

	raw, err := deps.HostEgress(toolregistry.WithExecutionContext(ctx, toolregistry.ExecutionContext{RunID: "research-run", OwnerID: owner, ComputerID: computer, Profile: agentprofile.Research}),
		"search_wire_corpus", json.RawMessage(`{"query":"harbor"}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Events []struct {
			EventID      string   `json:"event_id"`
			Headline     string   `json:"headline"`
			Outlets      []string `json:"outlets"`
			Origins      int      `json:"origins"`
			Corroborated bool     `json:"corroborated"`
			Tentative    int      `json:"tentative"`
			CaptureIDs   []string `json:"capture_ids"`
		} `json:"events"`
		EventCount int `json:"event_count"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	// V2-1, V2-3: this computer's two events, the corroborated one first.
	if out.EventCount != 2 || len(out.Events) != 2 || out.Events[0].EventID != "ev-harbor" || out.Events[1].EventID != "ev-harbor-wire" {
		t.Fatalf("events = %+v", out.Events)
	}
	h, w := out.Events[0], out.Events[1]
	// V2-2: the tentative dw.com member is shown but not counted; wire copy is one origin.
	if !h.Corroborated || h.Origins != 2 || len(h.Outlets) != 2 || h.Tentative != 1 {
		t.Fatalf("harbor event = %+v", h)
	}
	if w.Corroborated || w.Origins != 1 {
		t.Fatalf("wire event = %+v; two outlets of one wire story are one origin", w)
	}
	// V2-5: capture handles to read.
	if len(h.CaptureIDs) != 2 || h.CaptureIDs[0] != "cap-h1" {
		t.Fatalf("capture ids = %v", h.CaptureIDs)
	}
	if got := searchCorpusAsCell(t, deps, owner, computer, `{"query":"zebra"}`); len(got.Captures) != 0 {
		t.Fatalf("unexpected captures %+v", got.Captures)
	}
}

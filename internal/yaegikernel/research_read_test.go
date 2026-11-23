package yaegikernel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/promptstore"
	"github.com/yusefmosiah/go-choir/internal/runtimeprompts"
)

// The research desk's one-cell helpers (research_read.go) and the worked cell
// the overlay teaches. Ways they can fail, written before the code:
//
//  1. ReadAll reads one page after another (a slow page holds the rest), loses
//     the order of the URLs, lets one failed page hide the others or panic the
//     cell, or leaves a failure out of the read ledger.
//  2. A page that never answers holds the cell for ever (no per-page guard).
//  3. The accessors on a Read result do not compile in a cell, or panic on a
//     result that is not a read result.
//  4. Cut panics, or splits a character, on an index past the end.
//  5. The overlay's worked cell does not compile or run, or sends a packet the
//     reducer's check refuses, so the prompt teaches a cell that fails.
//  6. A cell that reads and sends in one cell is still nudged to send.

type researchFixture struct {
	scope   *ChoirScope
	reads   atomic.Int32
	slow    map[string]time.Duration
	hang    map[string]bool
	failing map[string]string
	// searchCalls counts the searches the host was asked to start; urlsFor
	// fixes the URLs a query returns (default: five made from the query).
	searchCalls atomic.Int32
	urlsFor     map[string][]string
	// failFor makes a query's one provider fail (the text is its error);
	// hostErr makes the host refuse the query; runningFor leaves the search
	// started with no arrival and no end, for the test to finish.
	failFor    map[string]string
	hostErr    map[string]string
	runningFor map[string]bool
	searchMu   sync.Mutex
	// payloads are the search_start payloads the host was sent, in order (the
	// need state the guest reports: search_read_first_test.go).
	payloads []SearchStartPayload
	// corpus is the capture ids a corpus search (SearchWireCorpus) returns.
	corpus []string
	// captureErr makes ReadCapture of a capture id fail (a deleted capture, a
	// refused read); captureReads counts ReadCapture calls.
	captureErr   map[string]string
	captureReads atomic.Int32
	corpusCalls  atomic.Int32
}

func newResearchFixture(t *testing.T) *researchFixture {
	t.Helper()
	f := &researchFixture{slow: map[string]time.Duration{}, hang: map[string]bool{}, failing: map[string]string{}}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-choir", CurrentEpoch: 1, AllowedRoot: t.TempDir(),
		Egress: func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
			switch action {
			case ActionSearchStart:
				var in SearchStartPayload
				if err := json.Unmarshal(payload, &in); err != nil {
					return nil, err
				}
				f.searchCalls.Add(1)
				f.searchMu.Lock()
				f.payloads = append(f.payloads, in)
				f.searchMu.Unlock()
				if msg, bad := f.hostErr[in.Query]; bad {
					return nil, fmt.Errorf("%s", msg)
				}
				f.searchMu.Lock()
				defer f.searchMu.Unlock()
				f.scope.searches.mu.Lock()
				id := fmt.Sprintf("s%d", len(f.scope.searches.order)+1)
				f.scope.searches.mu.Unlock()
				urls := f.urlsFor[in.Query]
				for _, host := range []string{"a.example", "b.example", "a.example", "c.example", "d.example"} {
					if f.urlsFor[in.Query] == nil {
						urls = append(urls, fmt.Sprintf("https://%s/%s/%d", host, strings.ReplaceAll(in.Query, " ", "-"), len(urls)))
					}
				}
				f.scope.searches.apply(SearchFrame{ID: id, Kind: SearchFrameStart, Query: in.Query, Providers: []string{"keenable"}})
				if f.runningFor[in.Query] {
					return json.RawMessage(fmt.Sprintf(`{"id":%q,"query":%q,"providers":["keenable"]}`, id, in.Query)), nil
				}
				if _, bad := f.failFor[in.Query]; bad {
					arrival := providerFrame(id, "keenable", false)
					arrival.Arrival.Error = f.failFor[in.Query]
					f.scope.searches.apply(arrival)
					f.scope.searches.apply(SearchFrame{ID: id, Kind: SearchFrameDone})
					return json.RawMessage(fmt.Sprintf(`{"id":%q,"query":%q,"providers":["keenable"]}`, id, in.Query)), nil
				}
				f.scope.searches.apply(providerFrame(id, "keenable", true, urls...))
				f.scope.searches.apply(SearchFrame{ID: id, Kind: SearchFrameDone})
				return json.RawMessage(fmt.Sprintf(`{"id":%q,"query":%q,"providers":["keenable"]}`, id, in.Query)), nil
			case ActionSearchWireCorpus:
				f.corpusCalls.Add(1)
				var caps []map[string]string
				for _, id := range f.corpus {
					caps = append(caps, map[string]string{"capture_id": id, "url": strings.TrimPrefix(id, "cap:")})
				}
				return json.Marshal(map[string]any{"captures_matched_words": len(caps), "captures_near_by_meaning_only": 0, "words_note": "", "captures": caps})
			case ActionReadCapture:
				// A capture id "cap:<url>" names the capture of that page.
				var in ReadCapturePayload
				if err := json.Unmarshal(payload, &in); err != nil {
					return nil, err
				}
				f.captureReads.Add(1)
				if msg, bad := f.captureErr[in.CaptureID]; bad {
					return nil, fmt.Errorf("%s", msg)
				}
				return json.RawMessage(fmt.Sprintf(`{"capture_id":%q,"url":%q,"title":"t","blocks":[{"id":"b0","kind":"paragraph","text":"%s"}]}`, in.CaptureID, strings.TrimPrefix(in.CaptureID, "cap:"), strings.Repeat("The ministry said the cabinet changes took effect today. ", 3))), nil
			case ActionRead:
				var in ReadPayload
				if err := json.Unmarshal(payload, &in); err != nil {
					return nil, err
				}
				f.reads.Add(1)
				if f.hang[in.URL] {
					time.Sleep(2 * time.Second)
				}
				time.Sleep(f.slow[in.URL])
				if msg, bad := f.failing[in.URL]; bad {
					return nil, fmt.Errorf("%s", msg)
				}
				id := strings.NewReplacer("https://", "", "/", "-").Replace(in.URL)
				return json.RawMessage(fmt.Sprintf(`{"capture_id":"cap-%s","url":%q,"title":"Title of %s","block_count":2,"blocks":[{"id":"b0","kind":"heading","text":"Short"},{"id":"b1","kind":"paragraph","text":"%s"}]}`,
					id, in.URL, id, strings.Repeat("The ministry said the cabinet changes took effect today. ", 3))), nil
			}
			return nil, fmt.Errorf("unexpected action %s", action)
		}}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	f.scope, err = NewChoirScope(broker, issuer, "computer-choir", "activation-research", 1, "research", "")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// cell runs one cell source in a real yaegi session over the scope's exports.
func (f *researchFixture) session(t *testing.T) (cell func(string) SessionResult, hooks CellHooks) {
	t.Helper()
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), f.scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sess.Close)
	hooks = f.scope.BindCell()
	return func(source string) SessionResult {
		t.Helper()
		out, err := serveCell(sess, SessionFrame{ID: "cell", Source: source}, nil, &hooks)
		if err != nil {
			t.Fatalf("cell failed: %v\n%s", err, source)
		}
		if out.Error != "" {
			t.Fatalf("cell error: %s\n%s", out.Error, source)
		}
		return out
	}, hooks
}

func TestReadAllReadsInParallelInOrderAndReportsFailures(t *testing.T) {
	f := newResearchFixture(t)
	f.scope.BindCell().Begin(SessionFrame{})
	var urls []string
	for i := 0; i < 8; i++ {
		u := fmt.Sprintf("https://h%d.example/p", i)
		urls = append(urls, u)
		f.slow[u] = 300 * time.Millisecond
	}
	f.failing["https://h3.example/p"] = "HTTP 403"
	urls = append(urls, "  ", "https://h3.example/p")
	start := time.Now()
	pages := f.scope.ReadAll(urls)
	if took := time.Since(start); took > 1500*time.Millisecond {
		t.Fatalf("8 reads of 300ms took %s: they ran one after another", took)
	}
	if len(pages) != len(urls) {
		t.Fatalf("got %d pages for %d urls", len(pages), len(urls))
	}
	for i, p := range pages {
		switch {
		case i == 3 || i == 9:
			if p.OK() || !strings.Contains(p.Err, "403") || p.URL != urls[i] {
				t.Errorf("page %d should carry its failure: %+v", i, p)
			}
		case i == 8:
			if p.OK() || p.Err != "empty url" {
				t.Errorf("a blank url must be a failed page, not a skipped one: %+v", p)
			}
		default:
			if !p.OK() || p.URL != urls[i] || p.CaptureID == "" || len(p.Blocks) != 2 || p.Blocks[1].ID != "b1" || p.Title == "" {
				t.Errorf("page %d out of order or empty: %+v", i, p)
			}
		}
	}
	if caps := f.scope.Captures(); len(caps) != 7 {
		t.Errorf("ledger holds %d captures, want 7 (the failed page is a failure, not a capture)", len(caps))
	}
	failed := 0
	for _, e := range f.scope.Reads() {
		if !e.OK {
			failed++
		}
	}
	if failed != 2 {
		t.Errorf("ledger holds %d failed reads, want 2 (the 403 twice)", failed)
	}
}

func TestReadAllGuardsAPageThatNeverAnswers(t *testing.T) {
	old := readAllGuard
	readAllGuard = 100 * time.Millisecond
	defer func() { readAllGuard = old }()
	f := newResearchFixture(t)
	f.scope.BindCell().Begin(SessionFrame{})
	f.hang["https://slow.example/p"] = true
	start := time.Now()
	pages := f.scope.ReadAll([]string{"https://slow.example/p", "https://fast.example/p"})
	if took := time.Since(start); took > time.Second {
		t.Fatalf("a hung page held the cell for %s", took)
	}
	if pages[0].OK() || !strings.Contains(pages[0].Err, "timed out") || !pages[1].OK() {
		t.Fatalf("pages = %+v", pages)
	}
}

func TestReadAllRefusesTooManyUrlsWithoutDroppingTheRest(t *testing.T) {
	f := newResearchFixture(t)
	f.scope.BindCell().Begin(SessionFrame{})
	var urls []string
	for i := 0; i < readAllMax+2; i++ {
		urls = append(urls, fmt.Sprintf("https://h%d.example/p", i))
	}
	pages := f.scope.ReadAll(urls)
	if !pages[readAllMax-1].OK() || pages[readAllMax].OK() || !strings.Contains(pages[readAllMax].Err, "at most") {
		t.Fatalf("pages past the cap must say so: %+v", pages[readAllMax:])
	}
}

func TestCutClampsAndKeepsCharactersWhole(t *testing.T) {
	var s *ChoirScope
	for _, tc := range []struct {
		str  string
		a, b int
		want string
	}{
		{"hello", 0, 3, "hel"},
		{"hello", -5, 99, "hello"},
		{"hello", 4, 2, ""},
		{"hello", 99, 120, ""},
		{"", 0, 10, ""},
		{"héllo", 0, 2, "h"},    // b falls inside é
		{"héllo", 2, 99, "llo"}, // a falls inside é
	} {
		if got := s.Cut(tc.str, tc.a, tc.b); got != tc.want {
			t.Errorf("Cut(%q, %d, %d) = %q, want %q", tc.str, tc.a, tc.b, got, tc.want)
		}
	}
}

func TestReadResultAccessorsAndSearchWorkInACell(t *testing.T) {
	f := newResearchFixture(t)
	cell, _ := f.session(t)
	out := cell(`r, err := choir.Read("https://a.example/x")
fmt.Println(err, r.CaptureID(), "|", r.Title(), "|", r.BlockCount(), len(r.Blocks()), r.Blocks()[1].ID)`)
	if !strings.Contains(out.Stdout, "cap-a.example-x | Title of a.example-x | 2 2 b1") {
		t.Fatalf("accessors: %q", out.Stdout)
	}
	out = cell(`s, err := choir.Search("cabinet changes")
fmt.Println(err, len(s.Results), s.Done, choir.Cut("abc", 1, 99), choir.Cut("abc", 9, 99) == "")`)
	if !strings.Contains(out.Stdout, "<nil> 5 true bc true") {
		t.Fatalf("Search / Cut in a cell: %q", out.Stdout)
	}
}

// workedCell is the code block of the overlay's worked cell.
func workedCell(t *testing.T) string {
	t.Helper()
	overlay := runtimeprompts.RLMResearchOverlay()
	_, rest, ok := strings.Cut(overlay, "```go\n")
	if !ok {
		t.Fatal("the research overlay has no worked cell")
	}
	code, _, ok := strings.Cut(rest, "```")
	if !ok {
		t.Fatal("the worked cell is not closed")
	}
	return code
}

func TestOverlayWorkedCellIsGraphFirstThenTheWebForWhatIsMissing(t *testing.T) {
	// The graph holds two outlets: the cell reads them by capture id and sends no web search.
	f := newResearchFixture(t)
	f.corpus = []string{"cap:https://www.bbc.co.uk/a", "cap:https://www.itv.co.uk/b"}
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-1"})
	out := cell(workedCell(t))
	if !strings.Contains(out.Stdout, "interim packet: 2 sources") || strings.Contains(out.Stdout, "<error") || strings.Contains(out.Stdout, "refused") {
		t.Fatalf("the worked cell did not send its interim packet from the graph:\n%s", out.Stdout)
	}
	if f.corpusCalls.Load() != 1 || f.captureReads.Load() != 2 || f.reads.Load() != 0 || f.searchCalls.Load() != 0 {
		t.Errorf("graph searches %d, capture reads %d, web reads %d, web searches %d: want 1, 2, 0, 0", f.corpusCalls.Load(), f.captureReads.Load(), f.reads.Load(), f.searchCalls.Load())
	}
	if !strings.Contains(out.Stdout, "outlets read without error: 2") || !strings.Contains(out.Stdout, "graph captures matching your words:") {
		t.Errorf("the worked cell did not print its stop test:\n%s", out.Stdout)
	}

	// The graph holds one outlet: the cell goes to the web for the second, reads its first
	// five new pages, and says in the packet's notes what came from each.
	g := newResearchFixture(t)
	g.corpus = []string{"cap:https://www.bbc.co.uk/a"}
	cell, hooks = g.session(t)
	hooks.Begin(SessionFrame{RunID: "run-2"})
	out = cell(workedCell(t))
	if !strings.Contains(out.Stdout, "interim packet: 6 sources") || strings.Contains(out.Stdout, "<error") || strings.Contains(out.Stdout, "refused") {
		t.Fatalf("the worked cell did not go to the web for what the graph lacks:\n%s", out.Stdout)
	}
	if g.corpusCalls.Load() != 1 || g.captureReads.Load() != 1 || g.reads.Load() != 5 || g.searchCalls.Load() != 1 {
		t.Errorf("graph searches %d, capture reads %d, web reads %d, web searches %d: want 1, 1, 5, 1", g.corpusCalls.Load(), g.captureReads.Load(), g.reads.Load(), g.searchCalls.Load())
	}

	// The graph holds nothing: the web alone.
	h := newResearchFixture(t)
	cell, hooks = h.session(t)
	hooks.Begin(SessionFrame{RunID: "run-3"})
	out = cell(workedCell(t))
	if !strings.Contains(out.Stdout, "interim packet: 5 sources") || h.reads.Load() != 5 || h.searchCalls.Load() != 1 {
		t.Errorf("an empty graph: %d web reads, %d web searches:\n%s", h.reads.Load(), h.searchCalls.Load(), out.Stdout)
	}
	for _, l := range out.Searches {
		if strings.Contains(l, "interim packet") {
			t.Errorf("a cell that read and sent in one cell was nudged to send: %s", l)
		}
	}
	// And reads without a send are nudged, naming the captures.
	f2 := newResearchFixture(t)
	cell2, hooks2 := f2.session(t)
	hooks2.Begin(SessionFrame{RunID: "run-4"})
	res := cell2(`pages := choir.ReadAll([]string{"https://a.example/1", "https://b.example/2"})
fmt.Println(len(pages))`)
	nudged := strings.Join(res.Searches, "\n")
	if !strings.Contains(nudged, "cap-a.example-1") || !strings.Contains(nudged, "cap-b.example-2") {
		t.Errorf("reads with no packet must be nudged with their capture ids:\n%s", nudged)
	}
	// The nudge reports progress; it does not ask the run to read or search on.
	if strings.Contains(nudged, "read on") {
		t.Errorf("the interim nudge asks for more reading:\n%s", nudged)
	}
}

// The worked cell counts what was read WITHOUT error: a web page that failed is neither a
// page of the notes' "from the web" count nor an outlet read.
func TestOverlayWorkedCellCountsOnlyPagesReadWithoutError(t *testing.T) {
	f := newResearchFixture(t)
	f.corpus = []string{"cap:https://www.bbc.co.uk/a"}
	f.urlsFor = map[string][]string{"ministry cabinet changes": {"https://w1.example/p", "https://w2.example/p", "https://w3.example/p", "https://w4.example/p", "https://w5.example/p"}}
	f.failing["https://w2.example/p"] = "HTTP 403"
	f.failing["https://w4.example/p"] = "HTTP 500"
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-err"})
	out := cell(workedCell(t))
	if strings.Contains(out.Stdout, "<error") {
		t.Fatalf("the worked cell failed:\n%s", out.Stdout)
	}
	// 1 graph page and 3 web pages read; two web reads failed.
	if !strings.Contains(out.Stdout, "interim packet: 4 sources") || !strings.Contains(out.Stdout, "outlets read without error: 4") {
		t.Fatalf("the cell counted failed reads:\n%s", out.Stdout)
	}
	var notes []string
	for _, in := range out.Intents {
		var p struct {
			Notes []string `json:"notes"`
		}
		if json.Unmarshal([]byte(in.Packet), &p) == nil && len(p.Notes) > 0 {
			notes = p.Notes
		}
	}
	if len(notes) == 0 || notes[0] != "adds: 4 sources, 1 from the graph, 3 from the web" {
		t.Fatalf("notes = %q, want the adds line to count the 3 web pages read without error", notes)
	}
}

// The research prompts are bounded as the model sees them: the role prompt's content (not the
// YAML file, whose comments hold the version history) and the overlay's body, each with about
// 16% headroom over its size on 2026-11-23 (6959 and 9324), and the two together. The bounds
// are a sanity check against unnoticed growth, not a model limit.
func TestResearchRoleAndOverlayStayUnderTheirBound(t *testing.T) {
	overlay := runtimeprompts.RLMResearchOverlay()
	role, err := promptstore.New(t.TempDir()).Load("user-alice", agentprofile.Research)
	if err != nil {
		t.Fatal(err)
	}
	if len(overlay) > 10800 || len(role.Content) > 8100 || len(overlay)+len(role.Content) > 18900 {
		t.Fatalf("research overlay %d + role content %d chars: past the sanity bounds (10800, 8100, 18900 together)", len(overlay), len(role.Content))
	}
}

// What "held" means, and where the corpus count lives (2026-11-22: the prompts put
// "held" beside graph-first text, so a run took it for "the corpus holds it"; in code it
// means an earlier search of this run returned the page, and SearchHit.CaptureID, said to
// mark a page the corpus holds, was set nowhere). Failure list: a verb doc says held
// without saying what it means, or still describes it as the paper's pages; the
// the SearchWireCorpus doc does not say what zero
// word matches means or does not name the two counts.
func TestVerbDocsSayWhatHeldMeansAndWhatTheCorpusCountsMean(t *testing.T) {
	for _, name := range []string{"Search", "SearchAll"} {
		doc := strings.Join(strings.Fields(choirFunctionDocs[name].Summary), " ")
		if !strings.Contains(doc, "held means an earlier search of this run returned the page") || !strings.Contains(doc, "a web result is not compared with the corpus") {
			t.Errorf("%s doc does not say what held means: %s", name, doc)
		}
		if strings.Contains(doc, "pages you already hold") {
			t.Errorf("%s doc still calls held pages ones you hold", name)
		}
	}
	corpus := strings.Join(strings.Fields(choirFunctionDocs["SearchWireCorpus"].Summary), " ")
	for _, want := range []string{"captures_matched_words", "captures_near_by_meaning_only", "zero captures_matched_words on a search that answered in full means no page holds your words, so the graph likely lacks this"} {
		if !strings.Contains(corpus, want) {
			t.Errorf("SearchWireCorpus doc lacks %q", want)
		}
	}
}

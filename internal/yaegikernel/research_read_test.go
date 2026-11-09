package yaegikernel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
				id := fmt.Sprintf("s%d", len(f.scope.searches.order)+1)
				var urls []string
				for _, host := range []string{"a.example", "b.example", "a.example", "c.example", "d.example"} {
					urls = append(urls, fmt.Sprintf("https://%s/%s/%d", host, strings.ReplaceAll(in.Query, " ", "-"), len(urls)))
				}
				f.scope.searches.apply(SearchFrame{ID: id, Kind: SearchFrameStart, Query: in.Query, Providers: []string{"keenable"}})
				f.scope.searches.apply(providerFrame(id, "keenable", true, urls...))
				f.scope.searches.apply(SearchFrame{ID: id, Kind: SearchFrameDone})
				return json.RawMessage(fmt.Sprintf(`{"id":%q,"query":%q,"providers":["keenable"]}`, id, in.Query)), nil
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

func TestOverlayWorkedCellSearchesReadsAndSendsTheInterimInOneCell(t *testing.T) {
	f := newResearchFixture(t)
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-1"})
	out := cell(workedCell(t))
	if !strings.Contains(out.Stdout, "interim packet: 4 sources") {
		t.Fatalf("the worked cell did not send its interim packet:\n%s", out.Stdout)
	}
	if strings.Count(out.Stdout, "ok https://") != 4 {
		t.Errorf("the cell must print a line for every page it read:\n%s", out.Stdout)
	}
	if strings.Contains(out.Stdout, "<error") || strings.Contains(out.Stdout, "refused") {
		t.Errorf("the packet was refused:\n%s", out.Stdout)
	}
	if got := f.reads.Load(); got != 4 {
		t.Errorf("the cell read %d pages, want 4: one per host of 4 hosts", got)
	}
	for _, l := range out.Searches {
		if strings.Contains(l, "interim packet") {
			t.Errorf("a cell that read and sent in one cell was nudged to send: %s", l)
		}
	}
	// And the same reads without a send are nudged, naming the captures.
	f2 := newResearchFixture(t)
	cell2, hooks2 := f2.session(t)
	hooks2.Begin(SessionFrame{RunID: "run-2"})
	res := cell2(`pages := choir.ReadAll([]string{"https://a.example/1", "https://b.example/2"})
fmt.Println(len(pages))`)
	nudged := strings.Join(res.Searches, "\n")
	if !strings.Contains(nudged, "cap-a.example-1") || !strings.Contains(nudged, "cap-b.example-2") {
		t.Errorf("reads with no packet must be nudged with their capture ids:\n%s", nudged)
	}
}

// The overlay is checked against the bound it was rewritten to meet.
func TestResearchRoleAndOverlayStayUnderTheirBound(t *testing.T) {
	overlay := runtimeprompts.RLMResearchOverlay()
	raw, err := os.ReadFile("../promptstore/defaults/research.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// Before 2026-11-09: overlay 13,100 chars, role 5,586.
	if len(overlay) > 7000 || len(raw) > 5000 || len(overlay)+len(raw) > 11800 {
		t.Fatalf("research overlay %d + role %d chars: the dedupe grew back (bounds 7000, 5000, 11800 together)", len(overlay), len(raw))
	}
}

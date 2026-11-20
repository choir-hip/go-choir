package researchtools

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// Attribution on the corpus lines (search-reuse commit 4, from
// docs/problems/research-does-not-start-from-the-corpus-2026-11-18.md,
// "Remaining error"). Failure list, written before the code:
//  1. The corpus-search line carries no run, desk or conversation, so a
//     search of the corpus cannot be tied to the run that made it.
//  2. It counts captures only: published stories and events returned, and
//     whether the candidate cap was hit, are not on the line.
//  3. A Read of a capture the computer already held is not logged with the
//     run and capture id, so "share of a story's sources held before the
//     run began" is not countable.
//  5. held= is true for a capture this run made itself (its own earlier
//     Read), so the share of sources held before the run began is overstated:
//     held= is true only for a capture that existed before this run's first
//     Read of it, own for one this run made, false for the Read that made it.
//  6. capped= is parsed from the note's prose instead of coming from the
//     search; or the line repeats captures=.
//  7. The old fields (computer, captures_ms, captures, note, query,
//     capture_ids) change, or the line carries more than ids and counts
//     beyond the query it already logged.

var corpusLogMu sync.Mutex

func captureReadLog(t *testing.T) *syncLogBuffer {
	t.Helper()
	corpusLogMu.Lock()
	buf := &syncLogBuffer{}
	prev := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(prev); corpusLogMu.Unlock() })
	return buf
}

// syncLogBuffer is a log sink a test may read while a background call (a lead
// write or mark, search_leads.go) is still writing to it.
type syncLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncLogBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func TestCorpusSearchLineCarriesRunDeskAndCounts(t *testing.T) {
	srv := publishedServer(`{"query":"harbor strike","pieces":[{"publication_id":"p1","route_path":"/pub/texture/harbor-1","title":"Harbor strike ends","kind":"story","read_url":"https://choir.news/r","score":4},{"publication_id":"p2","route_path":"/pub/texture/harbor-2","title":"Harbor strike again","kind":"story","read_url":"https://choir.news/r2","score":3}]}`)
	defer srv.Close()
	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })
	logs := captureReadLog(t)
	tool := newSearchWireCorpusTool(Dependencies{Store: guest, Captures: NewCorpusdCaptureSink(srv.URL)})
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "run-5", OwnerID: "owner-1", ComputerID: "computer-1", Role: "research", ChannelID: "chan 7",
	})
	if _, err := tool.Func(ctx, json.RawMessage(`{"query":"harbor strike"}`)); err != nil {
		t.Fatal(err)
	}
	out := logs.String()
	for _, want := range []string{
		"corpus search computer=computer-1 captures_ms=",
		`query="harbor strike" capture_ids=`,
		" run=run-5 desk=research conv=chan7 published=2 events=0 capped=false",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("corpus search line lacks %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "corpus search ") != 1 {
		t.Fatalf("want one line per search:\n%s", out)
	}
	// A call that names no caller is a cell's verb call; caller is the last
	// field so parsers of the earlier fields keep working.
	if line := strings.TrimSpace(out); !strings.HasSuffix(line, " capped=false vector=unavailable vector_state=unavailable bm25_kept=0 caller=cell") {
		t.Fatalf("the line does not end with the caller of a verb call:\n%s", out)
	}
}

// The host's own search for a research run's brief is told apart from the
// desk's own corpus searches: a measure of "web search before any corpus read"
// would otherwise count the brief as the desk's first corpus read.
func TestCorpusSearchLineNamesTheBriefAsCaller(t *testing.T) {
	srv := publishedServer(`{"query":"harbor strike","pieces":[]}`)
	defer srv.Close()
	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })
	logs := captureReadLog(t)
	tool := newSearchWireCorpusTool(Dependencies{Store: guest, Captures: NewCorpusdCaptureSink(srv.URL)})
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "run-6", OwnerID: "owner-1", ComputerID: "computer-1", Role: "research",
	})
	if _, err := tool.Func(WithCorpusCaller(ctx, CorpusCallerBrief), json.RawMessage(`{"query":"harbor strike"}`)); err != nil {
		t.Fatal(err)
	}
	if line := strings.TrimSpace(logs.String()); !strings.HasSuffix(line, " vector=unavailable vector_state=unavailable bm25_kept=0 caller=brief") {
		t.Fatalf("the brief's search does not say so:\n%s", line)
	}
}

// capped comes from the search itself: the index said it ranked only its
// strongest candidates, or the window of newest captures was full.
type cappedSearcher struct {
	CaptureSink
	capped bool
}

func (c cappedSearcher) SearchCaptures(ctx context.Context, q CaptureSearchQuery) (CaptureSearchAnswer, error) {
	return CaptureSearchAnswer{Capped: c.capped, Searched: 900, Matched: 700}, nil
}

type windowSink struct {
	CaptureSink
	n int
}

func (w windowSink) ListCaptures(ctx context.Context, ownerID, computerID string, limit int) ([]objectgraph.Object, error) {
	return make([]objectgraph.Object, w.n), nil
}

func TestSearchCapturesReportsCappedAsABool(t *testing.T) {
	ctx := context.Background()
	for name, c := range map[string]struct {
		sink CaptureSink
		want bool
	}{
		"index capped":     {cappedSearcher{capped: true}, true},
		"index not capped": {cappedSearcher{capped: false}, false},
		"window full":      {windowSink{n: corpusCaptureScan}, true},
		"window not full":  {windowSink{n: 3}, false},
	} {
		t.Setenv(corpusSearchModeEnv, "")
		if name[:6] == "window" {
			t.Setenv(corpusSearchModeEnv, "window")
		}
		_, note, capped, err := searchCaptures(ctx, Dependencies{Captures: c.sink}, "owner-1", "computer-1", []string{"harbor"}, 10)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if capped != c.want {
			t.Errorf("%s: capped = %v, want %v (note %q)", name, capped, c.want, note)
		}
	}
}

func TestCorpusSearchLineHasOneCapturesField(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-5"})
	logCorpusSearch(ctx, "harbor", "computer-1", 0, nil, "", 1, 2, true)
	if got := strings.Count(buf.String(), "captures="); got != 1 { // captures_ms= is a different key
		t.Fatalf("captures= appears %d times, want exactly one:\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "capped=true") {
		t.Fatalf("line: %s", buf.String())
	}
}

func TestCorpusReadLineFormat(t *testing.T) {
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-5", Profile: "research"})
	if got := corpusReadLine(ctx, "cap-1", "true", "read_capture"); got != "corpus read run=run-5 desk=research conv=- capture=cap-1 held=true via=read_capture" {
		t.Fatalf("line = %q", got)
	}
	if got := corpusReadLine(context.Background(), "cap-2", "false", "read"); got != "corpus read run=- desk=- conv=- capture=cap-2 held=false via=read" {
		t.Fatalf("a call with no run = %q", got)
	}
}

func TestRunCaptureStateSeparatesOwnFromBefore(t *testing.T) {
	a := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-a"})
	b := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-b"})
	if got := runCaptureState(a, "c1", true); got != "false" {
		t.Fatalf("the read that made a capture = %q, want false", got)
	}
	if got := runCaptureState(a, "c1", false); got != "own" {
		t.Fatalf("this run's later read = %q, want own", got)
	}
	if got := runCaptureState(b, "c1", false); got != "true" {
		t.Fatalf("another run's read = %q, want true (held before it began)", got)
	}
	if got := runCaptureState(context.Background(), "c1", false); got != "true" {
		t.Fatalf("a call with no run = %q, want true", got)
	}
}

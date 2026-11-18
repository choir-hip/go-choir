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
//  4. The old fields (computer, captures_ms, captures, note, query,
//     capture_ids) change, or the line carries more than ids and counts
//     beyond the query it already logged.

var corpusLogMu sync.Mutex

func captureReadLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	corpusLogMu.Lock()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev); corpusLogMu.Unlock() })
	return &buf
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
		" run=run-5 desk=research conv=chan7 published=2 events=0 captures=",
		" capped=false",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("corpus search line lacks %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "corpus search ") != 1 {
		t.Fatalf("want one line per search:\n%s", out)
	}
}

func TestCorpusCapturesCappedFromTheNote(t *testing.T) {
	for note, want := range map[string]bool{
		"":                        false,
		"label hints not read: x": false,
		"searched all 900 captures of this computer; ranked the strongest, newest 500 of the 700 that match enough words": true,
		"searched this computer's newest 2000 captures; older captures were not searched":                                 true,
	} {
		if got := capturesCapped(note); got != want {
			t.Errorf("capturesCapped(%q) = %v, want %v", note, got, want)
		}
	}
}

func TestCorpusReadLineFormat(t *testing.T) {
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-5", Profile: "research"})
	if got := corpusReadLine(ctx, "cap-1", true, "read_capture"); got != "corpus read run=run-5 desk=research conv=- capture=cap-1 held=true via=read_capture" {
		t.Fatalf("line = %q", got)
	}
	if got := corpusReadLine(context.Background(), "cap-2", false, "read"); got != "corpus read run=- desk=- conv=- capture=cap-2 held=false via=read" {
		t.Fatalf("a call with no run = %q", got)
	}
}

package agentcore

// The sufficiency measurement of web searches (search_sufficiency.go, landing
// L2 of the one search verb). Failure list, written before the code:
//
//   M1 a search that reaches the host starts no ranked corpus question, asks
//      it more than once, or asks it with another query, owner, computer or
//      conversation
//   M2 the question blocks the caller (it must return before the corpus
//      answers) or its failure reaches the search
//   M3 the log line lacks a field a threshold needs (best cosine, strong
//      count, age of the newest strong page, unread strong, unread leads,
//      the decision and why), or does not carry the top entries' cosine, age
//      and read mark, or prints the query text
//   M4 a corpus that did not answer, or answered without a block, logs no line

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

type fakeRankSink struct {
	researchtools.CaptureSink
	calls   atomic.Int32
	got     chan researchtools.CaptureSearchQuery
	release chan struct{}
	answer  researchtools.CaptureSearchAnswer
	err     error
}

func (f *fakeRankSink) SearchCaptures(ctx context.Context, q researchtools.CaptureSearchQuery) (researchtools.CaptureSearchAnswer, error) {
	f.calls.Add(1)
	select {
	case f.got <- q:
	case <-time.After(10 * time.Second):
	}
	if f.release != nil {
		select {
		case <-f.release:
		case <-time.After(10 * time.Second):
		}
	}
	return f.answer, f.err
}

func waitLine(t *testing.T, logs *leadLogBuffer) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, l := range strings.Split(logs.String(), "\n") {
			if strings.Contains(l, "op=search_sufficiency") {
				return l
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no sufficiency line; log:\n%s", logs.String())
	return ""
}

func TestMeasureSearchAsksOneRankedQuestionOffThePathAndLogsOneLine(t *testing.T) {
	logs := &leadLogBuffer{}
	orig := log.Writer()
	log.SetOutput(logs)
	defer log.SetOutput(orig)
	now := time.Now().UTC()
	sink := &fakeRankSink{got: make(chan researchtools.CaptureSearchQuery, 4), release: make(chan struct{})}
	yes := true
	sink.answer.Ranked = &researchtools.CaptureRanking{
		VectorAnswered: true,
		Entries: []researchtools.RankedCapture{
			{CaptureID: "a", Cosine: 0.91, At: now.Add(-2 * time.Hour).Format(time.RFC3339)},
			{CaptureID: "b", Cosine: 0.55, At: now.Add(-48 * time.Hour).Format(time.RFC3339), Read: true},
			{CaptureID: "c"},
		},
		Sufficiency: &researchtools.Sufficiency{BestCosine: 0.91, Strong: 1, NewestStrongAgeSeconds: 7200, UnreadStrong: 1,
			LeadsMatched: 5, UnreadLeads: 4, UnreadLeadsConversation: 2, UnreadLeadsWords: 3, SameTerms: &yes, SameTermsAgeSeconds: 90,
			Thresholds: researchtools.SufficiencyThresholds{StrongCosine: 0.8, HorizonHours: 168, LagHours: 6, ReuseMinutes: 30}},
	}
	rt := &Runtime{researchDeps: &researchtools.Dependencies{Captures: sink}}
	exec := toolregistry.ExecutionContext{RunID: "run-9", Role: "research", OwnerID: "owner-1", ComputerID: "comp-1", ChannelID: "conv-1"}
	done := make(chan struct{})
	go func() {
		rt.measureSearchSufficiency(context.Background(), exec, "Secret harbor words")
		close(done)
	}()
	select {
	case <-done: // returned although the corpus has not answered (M2)
	case <-time.After(2 * time.Second):
		t.Fatal("the measurement blocked its caller")
	}
	var q researchtools.CaptureSearchQuery
	select {
	case q = <-sink.got:
	case <-time.After(5 * time.Second):
		t.Fatal("the corpus was not asked")
	}
	if q.Query != "Secret harbor words" || q.OwnerID != "owner-1" || q.ComputerID != "comp-1" || q.ConversationID != "conv-1" {
		t.Fatalf("question %+v", q)
	}
	close(sink.release)
	line := waitLine(t, logs)
	for _, want := range []string{"run=run-9", "desk=research", "conv=conv-1", "corpus=answered", "best=0.910", "strong=1", "newest_strong_age_s=7200",
		"unread_strong=1", "leads_matched=5", "unread_leads=4", "unread_leads_conv=2", "unread_leads_words=3", "reuse_min=30", "same_terms=true", "same_terms_age_s=90",
		"tau_s=0.80", "h_h=168", "lag_h=6", "decision=sent", "why=guest_gate_passed", "err=none", "words=3"} {
		if !strings.Contains(line, want) {
			t.Errorf("line lacks %q: %s", want, line)
		}
	}
	if !strings.Contains(line, "top=0.910:72") && !strings.Contains(line, "top=0.910:73") {
		t.Errorf("top list: %s", line)
	}
	if !strings.Contains(line, ":r,") || !strings.Contains(line, "0.000:-1:u") {
		t.Errorf("read mark or no-time entry missing: %s", line)
	}
	if strings.Contains(line, "Secret") || strings.Contains(line, "harbor") {
		t.Errorf("the line carries the query: %s", line)
	}
	if n := sink.calls.Load(); n != 1 {
		t.Fatalf("%d corpus questions, want 1", n)
	}
}

func TestMeasureSearchLogsAFailureAndAnAnswerWithNoBlock(t *testing.T) {
	logs := &leadLogBuffer{}
	orig := log.Writer()
	log.SetOutput(logs)
	defer log.SetOutput(orig)
	exec := toolregistry.ExecutionContext{RunID: "run-1", Role: "research", OwnerID: "o", ComputerID: "c"}
	sink := &fakeRankSink{got: make(chan researchtools.CaptureSearchQuery, 4), err: errors.New("corpusd down")}
	(&Runtime{researchDeps: &researchtools.Dependencies{Captures: sink}}).measureSearchSufficiency(context.Background(), exec, "harbor")
	line := waitLine(t, logs)
	if !strings.Contains(line, "corpus=failed") || !strings.Contains(line, "err=corpusd down") {
		t.Fatalf("failure line: %s", line)
	}
	if got := sufficiencyLine(exec, "harbor", &researchtools.CaptureRanking{}, 0, nil, time.Now()); !strings.Contains(got, "corpus=failed") {
		t.Fatalf("no block: %s", got)
	}
	// Unknown counts are logged as unknown, and a vector half that could not answer says why.
	unknown := &researchtools.CaptureRanking{VectorUnavailable: "the embedder is down", Entries: []researchtools.RankedCapture{{CaptureID: "a", At: time.Now().Format(time.RFC3339)}},
		Sufficiency: &researchtools.Sufficiency{UnreadStrong: -1, UnreadLeads: -1, UnreadLeadsConversation: -1, UnreadLeadsWords: -1, LeadsMatched: -1, Note: "leads_by_words:not_ready"}}
	got := sufficiencyLine(exec, "harbor", unknown, 0, nil, time.Now())
	for _, want := range []string{"corpus=vector_unavailable", "unread_strong=-1", "unread_leads=-1", "leads_matched=-1", "same_terms=unknown", ":?", "vector_unavailable:the_embedder_is_down"} {
		if !strings.Contains(got, want) {
			t.Errorf("unknown line lacks %q: %s", want, got)
		}
	}
	// No owner, no computer, no searcher: no question and no panic.
	(&Runtime{}).measureSearchSufficiency(context.Background(), exec, "harbor")
	(&Runtime{researchDeps: &researchtools.Dependencies{}}).measureSearchSufficiency(context.Background(), exec, "harbor")
	(&Runtime{researchDeps: &researchtools.Dependencies{Captures: sink}}).measureSearchSufficiency(context.Background(), toolregistry.ExecutionContext{RunID: "x"}, "harbor")
	time.Sleep(100 * time.Millisecond)
	if n := sink.calls.Load(); n != 1 {
		t.Fatalf("%d questions, want only the first", n)
	}
}

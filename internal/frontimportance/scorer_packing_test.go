package frontimportance

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway"
	"github.com/yusefmosiah/go-choir/internal/sectionlist"
)

// Ways scorer packing can fail (docs/design/jev-batched-questions-2026-11-10.md,
// section 7): a handle mismatch gives one story another's answers; a story
// larger than the bound; duplicate question names; a partial answer set; a
// cache key that changes; ordering; the byte bound. Each has a test here or in
// scorer_test.go (cache) with a fake judge.

func packingStories(n int) ([]frontStory, map[string]string) {
	var stories []frontStory
	answers := map[string]string{}
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("T%02d", i)
		stories = append(stories, frontStory{PublicationID: "pub-" + title, Title: title, LatestRevision: 1})
		answers[title] = fmt.Sprint(1 + i%10)
	}
	return stories, answers
}

// packAcross raises maxStoriesPerCall for one test (packing across stories
// is off in production until measured).
func packAcross(t *testing.T, n int) {
	t.Helper()
	old := maxStoriesPerCall
	maxStoriesPerCall = n
	t.Cleanup(func() { maxStoriesPerCall = old })
}

func runPacked(t *testing.T, stories []frontStory, jev *fakeJev, mutate func(*config)) (runStats, string) {
	t.Helper()
	srv := frontServer(t, http.StatusOK, stories)
	path := filepath.Join(t.TempDir(), "scores.json")
	cfg := testConfig(srv.URL, path)
	if mutate != nil {
		mutate(&cfg)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stats, err := run(ctx, cfg, srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return stats, path
}

// One call carries every small story, never two calls at once, and each
// story gets its own importance (handle attribution).
func TestScorerPacksSmallStoriesIntoOneCall(t *testing.T) {
	packAcross(t, 4)
	stories, answers := packingStories(3)
	jev := &fakeJev{answers: answers, delay: 5 * time.Millisecond}
	stats, path := runPacked(t, stories, jev, nil)
	if jev.nCalls != 1 || len(jev.perCall) != 1 || jev.perCall[0] != 3 {
		t.Fatalf("calls = %d (stories per call %v), want one call of 3", jev.nCalls, jev.perCall)
	}
	if stats.Scored != 3 || stats.Failed != 0 {
		t.Fatalf("stats = %+v", stats)
	}
	scores := readScores(path).Scores
	for _, st := range stories {
		if got := fmt.Sprint(scores[st.PublicationID].Importance); got != answers[st.Title] {
			t.Fatalf("%s importance = %s, want %s", st.Title, got, answers[st.Title])
		}
	}
}

// Twelve stories (handles s1..s12, where s1 is a prefix of s10) in calls one
// after another: never two at once, and no story takes another's answer.
func TestScorerCallsAreSerialAndHandlesDoNotCross(t *testing.T) {
	packAcross(t, 99)
	stories, answers := packingStories(12)
	jev := &fakeJev{answers: answers, delay: 5 * time.Millisecond}
	stats, path := runPacked(t, stories, jev, func(c *config) { c.RequestBytes = 1 << 30 })
	if jev.nCalls != 1 || jev.perCall[0] != 12 {
		t.Fatalf("calls = %d (%v), want one call of 12", jev.nCalls, jev.perCall)
	}
	scores := readScores(path).Scores
	for _, st := range stories {
		if got := fmt.Sprint(scores[st.PublicationID].Importance); got != answers[st.Title] {
			t.Fatalf("%s importance = %s, want %s (handle mismatch)", st.Title, got, answers[st.Title])
		}
	}
	// And split: one story a call still runs one at a time.
	jev2 := &fakeJev{answers: answers, delay: 5 * time.Millisecond}
	stats, _ = runPacked(t, stories, jev2, func(c *config) { c.RequestBytes = 1 })
	if jev2.nCalls != 12 || jev2.peak.Load() != 1 || stats.Scored != 12 {
		t.Fatalf("calls = %d peak in flight = %d stats = %+v, want 12 calls, never concurrent", jev2.nCalls, jev2.peak.Load(), stats)
	}
}

// The byte bound splits a run: no request is over the bound (every story is
// well under it alone), and the split uses more than one call.
func TestScorerSplitsAtTheByteBound(t *testing.T) {
	packAcross(t, 99)
	stories, answers := packingStories(10)
	jev := &fakeJev{answers: answers}
	stats, _ := runPacked(t, stories, jev, nil)
	if jev.nCalls < 2 || jev.nCalls > 5 {
		t.Fatalf("calls = %d (%v) for 10 stories, want a few packed calls", jev.nCalls, jev.perCall)
	}
	total := 0
	for i, n := range jev.perCall {
		total += n
		if jev.reqBytes[i] > requestByteBound {
			t.Fatalf("call %d is %d bytes, over the %d bound", i, jev.reqBytes[i], requestByteBound)
		}
	}
	if total != 10 || stats.Scored != 10 {
		t.Fatalf("stories asked %d, stats %+v", total, stats)
	}
	t.Logf("10 stories: %d calls %v, bytes %v", jev.nCalls, jev.perCall, jev.reqBytes)
}

// A partial answer set fails only the story it misses; a failed call fails
// only the stories in it and keeps their previous rows.
func TestScorerPartialAnswerAndFailedCall(t *testing.T) {
	packAcross(t, 4)
	stories, answers := packingStories(3)
	jev := &fakeJev{answers: answers, omit: map[string]bool{"T01": true}}
	stats, path := runPacked(t, stories, jev, nil)
	if stats.Scored != 2 || stats.Failed != 1 {
		t.Fatalf("stats = %+v, want 2 scored and 1 failed", stats)
	}
	scores := readScores(path).Scores
	if _, ok := scores["pub-T01"]; ok || scores["pub-T00"].Importance != 1 || scores["pub-T02"].Importance != 3 {
		t.Fatalf("scores = %+v", scores)
	}

	// A failed call: the stories in it fail, the next call's stories do not.
	stories, answers = packingStories(8)
	failing := &failOnFirst{fakeJev: fakeJev{answers: answers}}
	srv := frontServer(t, http.StatusOK, stories)
	path = filepath.Join(t.TempDir(), "scores.json")
	cfg := testConfig(srv.URL, path)
	cfg.RequestBytes = 20000
	stats, err := run(context.Background(), cfg, srv.Client(), failing, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Failed == 0 || stats.Scored == 0 || stats.Failed+stats.Scored != 8 {
		t.Fatalf("stats = %+v, want some failed (first call) and the rest scored", stats)
	}
}

type failOnFirst struct {
	fakeJev
	done bool
}

func (f *failOnFirst) Decide(ctx context.Context, id string, req gateway.JudgmentRequest) ([]byte, error) {
	if !f.done {
		f.done = true
		return nil, fmt.Errorf("jev unavailable")
	}
	return f.fakeJev.Decide(ctx, id, req)
}

// Question names are unique across stories and each names its own story.
func TestScorerQuestionNamesAreUniquePerStory(t *testing.T) {
	stories, _ := packingStories(4)
	var asked []storyAsk
	for _, st := range stories {
		asked = append(asked, plan(st, scoreRow{}, false, sectionlist.House()))
	}
	seen := map[string]bool{}
	for k, a := range asked {
		h := handleFor(k)
		for name, q := range storyQuestions(a, h) {
			if seen[name] || !strings.HasPrefix(name, h+"_") || !strings.Contains(string(q), "state.stories."+h+" ") || strings.Contains(string(q), "state.story ") {
				t.Fatalf("question %q duplicated or does not name %s", name, h)
			}
			seen[name] = true
		}
	}
	if len(seen) != 4*12 {
		t.Fatalf("questions = %d, want 48", len(seen))
	}
}

// By default a call carries one story, and calls never overlap.
func TestScorerDefaultsToOneStoryACallSerially(t *testing.T) {
	stories, answers := packingStories(10)
	jev := &fakeJev{answers: answers, delay: 3 * time.Millisecond}
	stats, _ := runPacked(t, stories, jev, nil)
	if jev.nCalls != 10 || jev.peak.Load() != 1 || stats.Scored != 10 {
		t.Fatalf("calls = %d peak = %d stats = %+v, want 10 serial calls", jev.nCalls, jev.peak.Load(), stats)
	}
}

// A pack of up to 4 with MaxCalls = 1: the first batch is asked, the rest are
// deferred and keep their old rows.
func TestScorerDeferredBatchKeepsOldRows(t *testing.T) {
	packAcross(t, 4)
	stories, answers := packingStories(8)
	srv := frontServer(t, http.StatusOK, stories)
	path := filepath.Join(t.TempDir(), "scores.json")
	prev := scoresFile{Version: 1, Scores: map[string]scoreRow{}}
	for _, st := range stories {
		prev.Scores[st.PublicationID] = scoreRow{Revision: 0, Importance: 9} // revised since: rescored
	}
	if err := writeScores(path, prev); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(srv.URL, path)
	cfg.MaxCalls, cfg.RequestBytes = 1, 1<<30
	jev := &fakeJev{answers: answers}
	stats, err := run(context.Background(), cfg, srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if jev.nCalls != 1 || jev.perCall[0] != 4 || stats.Scored != 4 || stats.Deferred != 4 {
		t.Fatalf("calls = %d (%v) stats = %+v", jev.nCalls, jev.perCall, stats)
	}
	scores := readScores(path).Scores
	deferred := 0
	for _, st := range stories {
		if scores[st.PublicationID].Importance == 9 {
			deferred++
		}
	}
	if deferred != 4 || len(scores) != 8 {
		t.Fatalf("%d rows kept their old score of %d rows, want 4 of 8", deferred, len(scores))
	}
}

// A story whose request is over the bound alone is sent alone and whole.
func TestScorerStoryOverTheBoundIsSentAlone(t *testing.T) {
	packAcross(t, 4)
	stories, answers := packingStories(3)
	jev := &fakeJev{answers: answers}
	stats, path := runPacked(t, stories, jev, func(c *config) { c.RequestBytes = 100 })
	if jev.nCalls != 3 || stats.Scored != 3 {
		t.Fatalf("calls = %d (%v) stats = %+v, want each story sent alone and scored", jev.nCalls, jev.perCall, stats)
	}
	for _, n := range jev.perCall {
		if n != 1 {
			t.Fatalf("stories per call = %v", jev.perCall)
		}
	}
	for i, b := range jev.reqBytes {
		if b < 7000 {
			t.Fatalf("call %d is %d bytes: the story was truncated", i, b)
		}
	}
	if len(readScores(path).Scores) != 3 {
		t.Fatal("a story was skipped")
	}
}

// Stories with no row are asked before rows that exist.
func TestScorerAsksStoriesWithNoRowFirst(t *testing.T) {
	stories, answers := packingStories(4)
	srv := frontServer(t, http.StatusOK, stories)
	path := filepath.Join(t.TempDir(), "scores.json")
	prev := scoresFile{Version: 1, Scores: map[string]scoreRow{"pub-T00": {Revision: 0, Importance: 9}, "pub-T01": {Revision: 0, Importance: 9}}}
	if err := writeScores(path, prev); err != nil {
		t.Fatal(err)
	}
	jev := &fakeJev{answers: answers}
	if _, err := run(context.Background(), testConfig(srv.URL, path), srv.Client(), jev, time.Now); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(jev.calls, ","); got != "T02,T03,T00,T01" {
		t.Fatalf("order = %s, want the unscored first", got)
	}
}

// Every call is recorded once, with each question joined to its own story.
func TestScorerRecordsEveryCallFailuresIncluded(t *testing.T) {
	stories := []frontStory{
		{PublicationID: "pub-ok", Title: "Fine", LatestRevision: 3},
		{PublicationID: "pub-bad", Title: "Bad", LatestRevision: 1},
	}
	srv := frontServer(t, http.StatusOK, stories)
	dir := t.TempDir()
	path := filepath.Join(dir, "scores.json")
	jev := &fakeJev{answers: map[string]string{"Fine": "7", "Bad": ""}}
	if _, err := run(context.Background(), testConfig(srv.URL, path), http.DefaultClient, jev, time.Now); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "jev-calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if got := strings.Count(text, "\n"); got != 2 {
		t.Fatalf("records = %d, want 2 (one call a story):\n%s", got, text)
	}
	for _, want := range []string{`pub-ok@3`, `pub-bad@1`, `"ok":true`, `"site":"front_importance"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("records lack %s:\n%s", want, text)
		}
	}
}

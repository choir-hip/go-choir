package frontimportance

// Ways the front-importance scorer can fail:
//
//  1. It rescores a story already scored at its latest revision (wasted
//     calls), or never rescores a revised story.
//  2. A failed or bad Jev answer drops a story's previous score, or writes
//     an out-of-range importance.
//  3. A front-list failure overwrites the scores file (losing scores).
//  4. Scores for stories no longer on the front list pile up forever.
//  5. The call bound is not honoured.
//  6. The request is not the gateway's Jev shape (choice questions with
//     the importance choices "1".."10").
//  7. Sections (LF1): the section question's choices are not exactly the
//     four sections; an answer outside them is written; rows scored before
//     sections existed are never rescored (the section switcher stays
//     hidden forever), or are rescored past the per-run call bound.
//  8. Criteria versions (LF, 2026-10-14): a row judged under older
//     criteria is never rescored (the old, miscalibrated score stays
//     forever), a row at the current criteria is rescored every run, or a
//     failed rescore drops the old score instead of keeping it.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway"
)

type fakeJev struct {
	mu      sync.Mutex
	calls   []string
	answers map[string]string // title -> importance choice ("" = fail)
	promo   map[string]string
	section map[string]string // title -> section choice (default "world")
}

func (f *fakeJev) Decide(_ context.Context, _ string, req gateway.JudgmentRequest) ([]byte, error) {
	var state struct {
		Story struct {
			Title string `json:"title"`
		} `json:"story"`
	}
	_ = json.Unmarshal(req.State, &state)
	var q struct {
		Type    string   `json:"type"`
		Choices []string `json:"choices"`
	}
	_ = json.Unmarshal(req.Questions["importance"], &q)
	if q.Type != "choice" || len(q.Choices) != 10 || q.Choices[0] != "1" || q.Choices[9] != "10" {
		return nil, errors.New("bad importance question shape")
	}
	if _, ok := req.Questions["promo"]; !ok {
		return nil, errors.New("missing promo question")
	}
	var sq struct {
		Type    string   `json:"type"`
		Choices []string `json:"choices"`
	}
	_ = json.Unmarshal(req.Questions["section"], &sq)
	if sq.Type != "choice" || strings.Join(sq.Choices, ",") != "world,us_politics,business,tech_ai" {
		return nil, errors.New("bad section question shape")
	}
	f.mu.Lock()
	f.calls = append(f.calls, state.Story.Title)
	f.mu.Unlock()
	choice := f.answers[state.Story.Title]
	if choice == "" {
		return nil, errors.New("jev unavailable")
	}
	section, ok := f.section[state.Story.Title]
	if !ok {
		section = "world"
	}
	body, _ := json.Marshal(map[string]any{
		"model": "typesafe/jev-1.13",
		"answers": map[string]any{
			"importance": map[string]any{"choice": choice},
			"promo":      map[string]any{"choice": f.promo[state.Story.Title]},
			"section":    map[string]any{"choice": section},
		},
	})
	return body, nil
}

func frontServer(t *testing.T, status int, stories []frontStory) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Caller") != "true" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"stories": stories})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testConfig(frontURL, path string) config {
	return config{FrontURL: frontURL, ScoresPath: path, MaxCalls: 80, Budget: time.Minute, CallTime: 5 * time.Second}
}

func TestScorerScoresKeepsAndPrunes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scores.json")
	writePrev := scoresFile{Version: 1, Scores: map[string]scoreRow{
		"pub-a":    {Revision: 2, Importance: 7, Section: "world", Criteria: scoringCriteria}, // current: kept, no call
		"pub-b":    {Revision: 1, Importance: 6},                                              // revised: rescored
		"pub-c":    {Revision: 1, Importance: 5},                                              // revised, Jev fails: old kept
		"pub-gone": {Revision: 1, Importance: 9},                                              // off the front: pruned
	}}
	if err := writeScores(path, writePrev); err != nil {
		t.Fatal(err)
	}
	stories := []frontStory{
		{PublicationID: "pub-a", Title: "A", LatestRevision: 2},
		{PublicationID: "pub-b", Title: "B", LatestRevision: 2},
		{PublicationID: "pub-c", Title: "C", LatestRevision: 3},
		{PublicationID: "pub-d", Title: "Open a story on widgets", LatestRevision: 1},
		{PublicationID: "pub-e", Title: "E", LatestRevision: 1},
		{PublicationID: "pub-f", Title: "Bad", LatestRevision: 1},
	}
	jev := &fakeJev{
		answers: map[string]string{"B": "9", "Open a story on widgets": "1", "E": "4", "Bad": "11"},
		promo:   map[string]string{"B": "no", "Open a story on widgets": "yes", "E": ""},
	}
	srv := frontServer(t, http.StatusOK, stories)
	stats, err := run(context.Background(), testConfig(srv.URL, path), srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got := readScores(path)
	want := map[string]scoreRow{
		"pub-a": {Revision: 2, Importance: 7},
		"pub-b": {Revision: 2, Importance: 9},
		"pub-c": {Revision: 1, Importance: 5},
		"pub-d": {Revision: 1, Importance: 1, Promo: true},
		"pub-e": {Revision: 1, Importance: 4, Promo: false},
	}
	if len(got.Scores) != len(want) {
		t.Fatalf("scores = %+v, want %+v", got.Scores, want)
	}
	for id, w := range want {
		g := got.Scores[id]
		if g.Revision != w.Revision || g.Importance != w.Importance || g.Promo != w.Promo {
			t.Fatalf("%s = %+v, want %+v", id, g, w)
		}
	}
	for _, title := range jev.calls {
		if title == "A" {
			t.Fatalf("story scored at its latest revision was rescored")
		}
	}
	if stats.Scored != 3 || stats.Kept != 1 || stats.Failed != 2 {
		t.Fatalf("stats = %+v", stats)
	}
	if got.ScoredAt == "" || got.Scores["pub-b"].ScoredAt == "" {
		t.Fatalf("scored_at missing: %+v", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".scores-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestScorerFrontFailureLeavesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	prev := scoresFile{Version: 1, Scores: map[string]scoreRow{"pub-a": {Revision: 1, Importance: 7}}}
	if err := writeScores(path, prev); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	srv := frontServer(t, http.StatusInternalServerError, nil)
	if _, err := run(context.Background(), testConfig(srv.URL, path), srv.Client(), &fakeJev{}, time.Now); err == nil {
		t.Fatalf("front failure returned no error")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatalf("front failure rewrote the scores file")
	}
}

func TestScorerHonoursCallBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	var stories []frontStory
	answers := map[string]string{}
	for i := 0; i < 10; i++ {
		title := string(rune('a' + i))
		stories = append(stories, frontStory{PublicationID: "pub-" + title, Title: title, LatestRevision: 1})
		answers[title] = "5"
	}
	jev := &fakeJev{answers: answers}
	srv := frontServer(t, http.StatusOK, stories)
	cfg := testConfig(srv.URL, path)
	cfg.MaxCalls = 3
	stats, err := run(context.Background(), cfg, srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(jev.calls) != 3 || stats.Deferred != 7 || len(readScores(path).Scores) != 3 {
		t.Fatalf("calls = %d, stats = %+v", len(jev.calls), stats)
	}
	// The next run scores the rest without rescoring the first three.
	stats, err = run(context.Background(), cfg, srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Kept != 3 || stats.Scored != 3 {
		t.Fatalf("second run stats = %+v", stats)
	}
}

func TestScorerFillsSectionsWithinTheCallBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	prev := scoresFile{Version: 1, Scores: map[string]scoreRow{}}
	var stories []frontStory
	answers := map[string]string{}
	for i := 0; i < 5; i++ {
		title := string(rune('a' + i))
		id := "pub-" + title
		stories = append(stories, frontStory{PublicationID: id, Title: title, LatestRevision: 1})
		answers[title] = "6"
		// Scored before sections existed: current revision, no section.
		prev.Scores[id] = scoreRow{Revision: 1, Importance: 3}
	}
	if err := writeScores(path, prev); err != nil {
		t.Fatal(err)
	}
	jev := &fakeJev{answers: answers, section: map[string]string{"a": "tech_ai", "b": "sports"}}
	srv := frontServer(t, http.StatusOK, stories)
	cfg := testConfig(srv.URL, path)
	cfg.MaxCalls = 3
	stats, err := run(context.Background(), cfg, srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(jev.calls) != 3 || stats.Deferred != 2 {
		t.Fatalf("first run: calls = %d, stats = %+v", len(jev.calls), stats)
	}
	got := readScores(path).Scores
	if got["pub-a"].Section != "tech_ai" || got["pub-a"].Importance != 6 {
		t.Fatalf("pub-a = %+v, want rescored with section tech_ai", got["pub-a"])
	}
	if got["pub-b"].Section != "" {
		t.Fatalf("pub-b section = %q, want unset for an answer outside the four", got["pub-b"].Section)
	}
	if got["pub-d"].Importance != 3 || got["pub-d"].Section != "" {
		t.Fatalf("deferred pub-d = %+v, want its old score kept", got["pub-d"])
	}
	// The next run reaches the deferred rows.
	jev.calls = nil
	if _, err := run(context.Background(), cfg, srv.Client(), jev, time.Now); err != nil {
		t.Fatal(err)
	}
	got = readScores(path).Scores
	for _, id := range []string{"pub-c", "pub-d", "pub-e"} {
		if got[id].Section != "world" {
			t.Fatalf("%s = %+v, want a section after the second run", id, got[id])
		}
	}
}

func TestScorerRescoresRowsUnderOlderCriteria(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	prev := scoresFile{Version: 1, Scores: map[string]scoreRow{
		// Current revision and section, judged before criteria versions.
		"pub-a": {Revision: 1, Importance: 5, Section: "tech_ai"},
		// The same, but Jev fails: the old score is kept.
		"pub-b": {Revision: 1, Importance: 6, Section: "world"},
		// Already under the current criteria: kept without a call.
		"pub-c": {Revision: 1, Importance: 8, Section: "world", Criteria: scoringCriteria},
	}}
	if err := writeScores(path, prev); err != nil {
		t.Fatal(err)
	}
	stories := []frontStory{
		{PublicationID: "pub-a", Title: "A", LatestRevision: 1},
		{PublicationID: "pub-b", Title: "B", LatestRevision: 1},
		{PublicationID: "pub-c", Title: "C", LatestRevision: 1},
	}
	jev := &fakeJev{answers: map[string]string{"A": "7", "C": "2"}, section: map[string]string{"A": "tech_ai"}}
	srv := frontServer(t, http.StatusOK, stories)
	cfg := testConfig(srv.URL, path)
	stats, err := run(context.Background(), cfg, srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	got := readScores(path).Scores
	if a := got["pub-a"]; a.Importance != 7 || a.Criteria != scoringCriteria || a.Section != "tech_ai" {
		t.Fatalf("pub-a = %+v, want rescored to 7 under the current criteria", a)
	}
	if b := got["pub-b"]; b.Importance != 6 || b.Criteria != 0 {
		t.Fatalf("pub-b = %+v, want its old score kept after a failed rescore", b)
	}
	if c := got["pub-c"]; c.Importance != 8 {
		t.Fatalf("pub-c = %+v, want kept", c)
	}
	for _, title := range jev.calls {
		if title == "C" {
			t.Fatalf("row under the current criteria was rescored")
		}
	}
	if stats.Scored != 1 || stats.Failed != 1 || stats.Kept != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	// The failed row is retried next run; the rescored one is not.
	jev.calls = nil
	jev.answers["B"] = "4"
	if _, err := run(context.Background(), cfg, srv.Client(), jev, time.Now); err != nil {
		t.Fatal(err)
	}
	if strings.Join(jev.calls, ",") != "B" {
		t.Fatalf("second run calls = %v, want only B", jev.calls)
	}
	if b := readScores(path).Scores["pub-b"]; b.Importance != 4 || b.Criteria != scoringCriteria {
		t.Fatalf("pub-b after retry = %+v", b)
	}
}

package frontimportance

// Ways the front-importance scorer can fail:
//
//  1. It rescores a story already scored at its latest revision (wasted
//     calls), or never rescores a revised story.
//  2. A failed or bad Jev answer drops a story's previous score, or writes
//     an out-of-range importance.
//  3. A front-list failure overwrites the scores file (losing scores).
//  4. Scores for stories outside the window pile up forever (they are
//     carried forward for a bounded retention), or are dropped as soon as
//     the story leaves the newest 100 (the story loses its section).
//  5. The call bound is not honoured.
//  6. The request is not the gateway's Jev shape (choice questions with
//     the importance choices "1".."10").
//  7. Sections (lane B, docs/design/sections-as-questions-2026-11-10.md):
//     rows without the current list's answers are never asked, or are
//     asked past the per-run call bound; the call carries more than the
//     host cap of questions. More in scorer_sections_test.go.
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
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway"
	"github.com/yusefmosiah/go-choir/internal/sectionlist"
)

type fakeJev struct {
	mu      sync.Mutex
	calls   []string
	asked   [][]string        // per call, the section question texts asked
	answers map[string]string // title -> importance choice ("" = fail)
	promo   map[string]string
	// sectionsDown makes a call with only section questions fail.
	sectionsDown bool
	// yes maps a title to the section ids it answers yes (P 0.9) to; a
	// title not in the map answers yes to world only. Others answer P 0.1.
	yes map[string][]string
	// prob overrides P(yes) for (title, section id).
	prob map[string]map[string]float64
	// omit makes the judge answer nothing for a title's questions (a
	// partial answer set); callErr fails every call.
	omit    map[string]bool
	callErr error
	delay   time.Duration
	// nCalls, perCall (stories in each call), reqBytes, and the peak of
	// calls in flight at once.
	nCalls         int
	perCall        []int
	reqBytes       []int
	inflight, peak atomic.Int64
}

func (f *fakeJev) Decide(_ context.Context, _ string, req gateway.JudgmentRequest) ([]byte, error) {
	cur := f.inflight.Add(1)
	defer f.inflight.Add(-1)
	for p := f.peak.Load(); cur > p && !f.peak.CompareAndSwap(p, cur); p = f.peak.Load() {
	}
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	var state struct {
		Stories map[string]struct {
			Title string `json:"title"`
		} `json:"stories"`
	}
	_ = json.Unmarshal(req.State, &state)
	if len(state.Stories) == 0 {
		return nil, errors.New("no stories in state")
	}
	if f.callErr != nil {
		return nil, f.callErr
	}
	house := sectionlist.House()
	answers := map[string]any{}
	asked := map[string][]string{}
	hasImportance := false
	for name, raw := range req.Questions {
		handle, rest, ok := strings.Cut(name, "_")
		st, known := state.Stories[handle]
		if !ok || !known {
			return nil, errors.New("question " + name + " names no story in state")
		}
		title := st.Title
		if !strings.Contains(string(raw), "state.stories."+handle+" ") {
			return nil, errors.New("question " + name + " does not name its own story")
		}
		if f.omit[title] {
			continue // a partial answer set: this story gets nothing
		}
		if rest == "importance" {
			hasImportance = true
			var q struct {
				Type    string   `json:"type"`
				Choices []string `json:"choices"`
			}
			_ = json.Unmarshal(raw, &q)
			if q.Type != "choice" || len(q.Choices) != 10 || q.Choices[0] != "1" || q.Choices[9] != "10" {
				return nil, errors.New("bad importance question shape")
			}
			if _, ok := req.Questions[handle+"_promo"]; !ok {
				return nil, errors.New("missing promo question")
			}
			answers[name] = map[string]any{"choice": f.answers[title]}
			continue
		}
		if rest == "promo" {
			answers[name] = map[string]any{"choice": f.promo[title]}
			continue
		}
		var sq struct {
			Type         string   `json:"type"`
			Choices      []string `json:"choices"`
			Instructions string   `json:"instructions"`
		}
		_ = json.Unmarshal(raw, &sq)
		if sq.Type != "choice" || strings.Join(sq.Choices, ",") != "yes,no" {
			return nil, errors.New("bad section question shape")
		}
		id := ""
		for _, sec := range house.Sections {
			if strings.Contains(sq.Instructions, sectionlist.Normalize(sec.Question)) {
				id = sec.ID
			}
		}
		if id == "" {
			id = "custom:" + sq.Instructions
		}
		asked[title] = append(asked[title], id)
		p := 0.1
		yes, ok := f.yes[title]
		if !ok {
			yes = []string{"world"}
		}
		for _, y := range yes {
			if y == id {
				p = 0.9
			}
		}
		if v, ok := f.prob[title][id]; ok {
			p = v
		}
		choice := "no"
		if p >= 0.5 {
			choice = "yes"
		}
		answers[name] = map[string]any{"choice": choice, "probabilities": map[string]float64{"yes": p, "no": 1 - p}}
	}
	if f.sectionsDown && !hasImportance {
		return nil, errors.New("jev unavailable")
	}
	var titles []string
	for _, st := range state.Stories {
		titles = append(titles, st.Title)
	}
	sort.Strings(titles)
	f.mu.Lock()
	f.nCalls++
	f.perCall = append(f.perCall, len(titles))
	f.reqBytes = append(f.reqBytes, len(req.State)+questionBytes(req.Questions))
	for _, t := range titles {
		f.calls = append(f.calls, t)
		f.asked = append(f.asked, asked[t])
	}
	f.mu.Unlock()
	body, _ := json.Marshal(map[string]any{"model": "typesafe/jev-1.13", "answers": answers})
	return body, nil
}

func questionBytes(qs map[string]json.RawMessage) int {
	n := 0
	for name, q := range qs {
		n += len(name) + len(q)
	}
	return n
}

// fullRow is a row current for the house list: every question answered, so
// a run asks it nothing (world yes).
func fullRow(rev, importance int) scoreRow {
	r := scoreRow{Revision: rev, Importance: importance, Criteria: scoringCriteria, Judge: sectionJudgeVersion, SectionP: map[string]float64{}}
	for _, s := range sectionlist.House().Sections {
		p := 0.1
		if s.ID == "world" {
			p = 0.9
		}
		r.SectionP[sectionlist.Hash(s.Question)] = p
	}
	r.Section = "world"
	return r
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
		"pub-a":    fullRow(2, 7),                                                  // current: kept, no call
		"pub-b":    {Revision: 1, Importance: 6},                                   // revised: rescored
		"pub-c":    {Revision: 1, Importance: 5},                                   // revised, Jev fails: old kept
		"pub-gone": {Revision: 1, Importance: 9, ScoredAt: "2000-01-01T00:00:00Z"}, // off the window, past retention: pruned
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
	cfg.MaxCalls, cfg.RequestBytes = 3, 1 // one story a call
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
	jev := &fakeJev{answers: answers, yes: map[string][]string{"a": {"tech_ai"}, "b": {"sports", "science"}}}
	srv := frontServer(t, http.StatusOK, stories)
	cfg := testConfig(srv.URL, path)
	cfg.MaxCalls, cfg.RequestBytes = 3, 1 // one story a call
	stats, err := run(context.Background(), cfg, srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(jev.calls) != 3 || stats.Deferred != 2 {
		t.Fatalf("first run: calls = %d, stats = %+v", len(jev.calls), stats)
	}
	got := readScores(path).Scores
	if got["pub-a"].Section != "tech_ai" || got["pub-a"].Importance != 6 || len(got["pub-a"].SectionP) != 10 {
		t.Fatalf("pub-a = %+v, want rescored with section tech_ai and ten answers", got["pub-a"])
	}
	pub := sectionlist.House().Published()
	if pl := pub.Place(got["pub-b"].SectionP); strings.Join(pl.Sections, ",") != "science,sports" {
		t.Fatalf("pub-b placement = %+v, want both science and sports (a story can be in several)", pl)
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
		"pub-a": withCriteria(fullRow(1, 5), 0),
		// The same, but Jev fails: the old score is kept.
		"pub-b": withCriteria(fullRow(1, 6), 0),
		// Already under the current criteria: kept without a call.
		"pub-c": fullRow(1, 8),
	}}
	if err := writeScores(path, prev); err != nil {
		t.Fatal(err)
	}
	stories := []frontStory{
		{PublicationID: "pub-a", Title: "A", LatestRevision: 1},
		{PublicationID: "pub-b", Title: "B", LatestRevision: 1},
		{PublicationID: "pub-c", Title: "C", LatestRevision: 1},
	}
	jev := &fakeJev{answers: map[string]string{"A": "7", "C": "2"}, yes: map[string][]string{"A": {"tech_ai"}}}
	srv := frontServer(t, http.StatusOK, stories)
	cfg := testConfig(srv.URL, path)
	stats, err := run(context.Background(), cfg, srv.Client(), jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	got := readScores(path).Scores
	if a := got["pub-a"]; a.Importance != 7 || a.Criteria != scoringCriteria {
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

func withCriteria(r scoreRow, c int) scoreRow { r.Criteria = c; return r }

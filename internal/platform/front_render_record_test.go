package platform

// The front render record (station R0 of
// docs/definitions/choir-records-and-measurement-2026-11-23.md): every change
// to what the front list serves is one append-only row of the ranked order
// with each story's score and the inputs to it. The offline measure of
// 2026-11-23 could not rebuild the page because only each story's latest
// importance was kept.
//
// Ways it can fail, each with its test:
//  1. a render writes no row, or a row whose order or inputs are not the
//     page's (the order the reader sees is the rank's, not the list's);
//  2. the same list served again writes another row (a row per request);
//  3. a changed importance rewrites the earlier row (history lost again) or
//     writes none;
//  4. a section page, a cursor page or an opinion page is recorded as the
//     front;
//  5. the route records the page it served, and a failure of the record
//     fails the response (TestAFailingRecordStillAnswersTheFrontPage) or
//     a slow record delays it (TestASlowRecordNeverHoldsThePage);
//  6. a stranger's request limit writes or spends anything, so that a sweep
//     of ?limit= can drop the canonical rows
//     (TestALimitSweepCannotTouchTheCanonicalRows);
//  7. the stored order is not what RankScored gives for the stored inputs.

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/frontrank"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func scoredFrontFixture(t *testing.T, harbor, bridge, canal int) (frontFixture, string) {
	t.Helper()
	f := newFrontFixture(t)
	page, _ := frontPageJSON(t, f.svc)
	ids := map[string]string{}
	for _, s := range page.Stories {
		ids[s.Title] = s.PublicationID
	}
	path := filepath.Join(t.TempDir(), "scores.json")
	f.svc.SetFrontImportancePath(path)
	writeFrontScores(t, path, ids, harbor, bridge, canal)
	return f, path
}

var scoresWrites int

func writeFrontScores(t *testing.T, path string, ids map[string]string, harbor, bridge, canal int) {
	t.Helper()
	body := `{"version":1,"scored_at":"2026-11-24T00:00:00Z","scores":{` +
		`"` + ids["Harbor reopens"] + `":{"revision":3,"importance":` + strconv.Itoa(harbor) + `},` +
		`"` + ids["Bridge closes"] + `":{"revision":1,"importance":` + strconv.Itoa(bridge) + `},` +
		`"` + ids["Canal dredged"] + `":{"revision":1,"importance":` + strconv.Itoa(canal) + `}}}`
	writeScores(t, path, body)
	// The scores cache is keyed on the file's mtime: every write gets a later
	// one, so a rewrite within one second is still reread.
	scoresWrites++
	future := time.Now().Add(time.Duration(scoresWrites) * time.Minute)
	_ = os.Chtimes(path, future, future)
}

func renderRows(t *testing.T, svc *Service) []types.DecisionRecord {
	t.Helper()
	svc.FlushFrontRenders()
	rows, err := svc.ListFrontRenders(context.Background(), time.Time{}, 100)
	if err != nil {
		t.Fatalf("list renders: %v", err)
	}
	return rows
}

func renderTitles(r types.DecisionRecord) []string {
	out := make([]string, 0, len(r.Shown))
	for _, s := range r.Shown {
		out = append(out, s.Title)
	}
	return out
}

// Failure 1.
func TestARenderRecordsTheRankedOrderWithEachScoreAndItsInputs(t *testing.T) {
	f, _ := scoredFrontFixture(t, 3, 9, 8)
	page, err := f.svc.ListFrontStories(context.Background(), 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := storyIDs(page); got[0] != "Harbor reopens" {
		t.Fatalf("the list is newest first, harbor on top: %v", got)
	}
	wrote, err := f.svc.RecordFrontRender(context.Background(), frontRenderCanonicalLimit, "", "", page)
	if err != nil || !wrote {
		t.Fatalf("record = %v, %v", wrote, err)
	}
	rows := renderRows(t, f.svc)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.Stage != types.DecisionStagePageRender || r.CaptureID != types.DecisionPageID || r.Decision != types.DecisionRenderedPage || r.DeciderKind != types.DeciderRule || r.Decider == "" || r.At.IsZero() {
		t.Fatalf("row = %+v", r)
	}
	// The page ranks by importance: bridge 9, canal 8, harbor 3 (thin, so
	// it cannot lead and carries no depth credit).
	if got := renderTitles(r); len(got) != 3 || got[0] != "Bridge closes" || got[1] != "Canal dredged" || got[2] != "Harbor reopens" {
		t.Fatalf("shown order = %v, want bridge, canal, harbor", got)
	}
	if len(r.Candidates) != 3 || r.Candidates[0] != r.Shown[0].PublicationID || r.CandidatesShown != 3 {
		t.Fatalf("candidates = %v shown %d", r.Candidates, r.CandidatesShown)
	}
	for i, s := range r.Shown {
		if s.Rank != i+1 || s.Score == nil || s.UpdatedAt == "" || s.PublicationID == "" {
			t.Fatalf("shown[%d] = %+v", i, s)
		}
	}
	assertShownIsRankScoredOfItsInputs(t, r)
	if s := r.Shown[0]; s.Importance == nil || *s.Importance != 9 {
		t.Fatalf("bridge inputs = %+v, want importance 9", s)
	}
	var harbor types.DecisionShown
	for _, s := range r.Shown {
		if s.Title == "Harbor reopens" {
			harbor = s
		}
	}
	if harbor.VersionCount != 3 || harbor.Importance == nil || *harbor.Importance != 3 || harbor.SourceCount == nil || harbor.Words == nil {
		t.Fatalf("harbor inputs = %+v (version count, importance, depth)", harbor)
	}
}

// Failures 2 and 3.
func TestAnUnchangedListWritesNoSecondRowAndAChangedImportanceAddsOneWithoutTouchingTheFirst(t *testing.T) {
	f, path := scoredFrontFixture(t, 3, 9, 8)
	ctx := context.Background()
	render := func() bool {
		page, err := f.svc.ListFrontStories(ctx, 0, "")
		if err != nil {
			t.Fatal(err)
		}
		wrote, err := f.svc.RecordFrontRender(ctx, frontRenderCanonicalLimit, "", "", page)
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return wrote
	}
	if !render() {
		t.Fatal("first render not recorded")
	}
	for i := 0; i < 3; i++ {
		if render() {
			t.Fatal("an unchanged list wrote another row")
		}
	}
	if n := len(renderRows(t, f.svc)); n != 1 {
		t.Fatalf("rows after repeats = %d, want 1", n)
	}

	// The scorer rescores: canal is now the most important story and bridge
	// falls behind it (harbor is thin, so it never leads and carries half credit).
	page, _ := frontPageJSON(t, f.svc)
	ids := map[string]string{}
	for _, s := range page.Stories {
		ids[s.Title] = s.PublicationID
	}
	writeFrontScores(t, path, ids, 3, 2, 9)
	if !render() {
		t.Fatal("a changed importance was not recorded")
	}
	rows := renderRows(t, f.svc)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	newer, older := rows[0], rows[1] // newest first
	if got := renderTitles(newer); got[0] != "Canal dredged" || got[1] != "Bridge closes" {
		t.Fatalf("newer row order = %v, want canal then bridge", got)
	}
	if got := renderTitles(older); got[0] != "Bridge closes" {
		t.Fatalf("the earlier row changed: %v", got)
	}
	for _, s := range older.Shown {
		if s.Title == "Canal dredged" && (s.Importance == nil || *s.Importance != 8) {
			t.Fatalf("the earlier row lost its importance history: %+v", s)
		}
	}

	// A list that goes back to the first state is a change again.
	writeFrontScores(t, path, ids, 3, 9, 8)
	if !render() {
		t.Fatal("a return to an earlier list was skipped")
	}
	if n := len(renderRows(t, f.svc)); n != 3 {
		t.Fatalf("rows = %d, want 3", n)
	}
}

// Failure 4, through the route: only the canonical first page (limit 60, no
// cursor, no section) is recorded.
func TestOnlyTheCanonicalFirstPageIsRecordedAndTheRouteStillAnswers(t *testing.T) {
	f, _ := scoredFrontFixture(t, 3, 9, 8)
	handler := NewHandler(f.svc)
	get := func(query string) int { return frontGet(handler, query).Code }
	cursor := func() string {
		page, _ := f.svc.ListFrontStories(context.Background(), 2, "")
		return page.NextCursor
	}()
	if cursor == "" {
		t.Fatal("fixture has no second page")
	}
	for _, q := range []string{"?limit=2", "?limit=100", "", "?limit=60&cursor=" + cursor, "?limit=60&section=world"} {
		if code := get(q); code != http.StatusOK {
			t.Fatalf("%q: %d", q, code)
		}
	}
	if n := len(renderRows(t, f.svc)); n != 0 {
		t.Fatalf("a request that is not the canonical first page wrote %d rows", n)
	}
	if code := get("?limit=60"); code != http.StatusOK {
		t.Fatalf("limit=60: %d", code)
	}
	rows := renderRows(t, f.svc)
	if len(rows) != 1 || len(rows[0].Shown) != 3 {
		t.Fatalf("rows = %d, want the one canonical first page of three stories", len(rows))
	}
	if code := get("?limit=60"); code != http.StatusOK {
		t.Fatalf("repeat: %d", code)
	}
	if n := len(renderRows(t, f.svc)); n != 1 {
		t.Fatalf("a repeated request wrote a row: %d", n)
	}
}

// assertShownIsRankScoredOfItsInputs is failure 7: the stored order is what
// the rank gives for the stored inputs (not "descending score": demoted rows
// sort last and a story that can lead is moved to the front).
func assertShownIsRankScoredOfItsInputs(t *testing.T, r types.DecisionRecord) {
	t.Helper()
	rows := make([]frontrank.Row, len(r.Shown))
	for i, s := range r.Shown {
		rows[i] = frontrank.Row{PublicationID: s.PublicationID, Title: s.Title, UpdatedAt: s.UpdatedAt, VersionCount: s.VersionCount,
			Promo: s.Promo, Section: s.Section, Importance: s.Importance, SourceCount: s.SourceCount, Words: s.Words}
	}
	got := frontrank.RankScored(rows)
	if len(got) != len(r.Shown) {
		t.Fatalf("replay has %d rows, shown %d", len(got), len(r.Shown))
	}
	for i, sc := range got {
		if sc.Row.PublicationID != r.Shown[i].PublicationID || sc.Demoted != r.Shown[i].Demoted {
			t.Fatalf("replay[%d] = %s demoted=%v, shown %s demoted=%v", i, sc.Row.PublicationID, sc.Demoted, r.Shown[i].PublicationID, r.Shown[i].Demoted)
		}
		if (r.Shown[i].Score == nil) == sc.Known && (r.Shown[i].Score == nil || *r.Shown[i].Score != sc.Score) {
			t.Fatalf("replay[%d] score differs from shown", i)
		}
	}
}

func frontGet(h *Handler, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/internal/platform/publications/front"+query, nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	h.HandleInternalFrontStories(w, req)
	return w
}

func captureRenderLog(t *testing.T) *syncBuf {
	t.Helper()
	b := &syncBuf{}
	log.SetOutput(b)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return b
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

// Failure 5, the failing half: the store refuses the row; the page is 200
// and the failure is logged.
func TestAFailingRecordStillAnswersTheFrontPage(t *testing.T) {
	f, _ := scoredFrontFixture(t, 3, 9, 8)
	logs := captureRenderLog(t)
	var puts atomic.Int32
	f.svc.frontRender.put = func(context.Context, objectgraph.Object) error {
		puts.Add(1)
		return errors.New("store down")
	}
	w := frontGet(NewHandler(f.svc), "?limit=60")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	f.svc.FlushFrontRenders()
	if puts.Load() != 1 || !strings.Contains(logs.String(), "front render record not written") || !strings.Contains(logs.String(), "store down") {
		t.Fatalf("puts=%d log=%q", puts.Load(), logs.String())
	}
	// The failed row is retried by the next request, not forgotten.
	f.svc.frontRender.put = nil
	if w := frontGet(NewHandler(f.svc), "?limit=60"); w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	if n := len(renderRows(t, f.svc)); n != 1 {
		t.Fatalf("rows after the retry = %d, want 1", n)
	}
}

// Failure 5, the slow half: a store that blocks never holds the page, and
// another request at the same time is not held behind it.
func TestASlowRecordNeverHoldsThePage(t *testing.T) {
	f, _ := scoredFrontFixture(t, 3, 9, 8)
	release := make(chan struct{})
	started := make(chan struct{}, 8)
	f.svc.frontRender.put = func(ctx context.Context, _ objectgraph.Object) error {
		started <- struct{}{}
		<-release
		return nil
	}
	h := NewHandler(f.svc)
	done := make(chan int, 3)
	go func() { done <- frontGet(h, "?limit=60").Code }()
	go func() { done <- frontGet(h, "?limit=100").Code }()
	go func() { done <- frontGet(h, "").Code }()
	for i := 0; i < 3; i++ {
		select {
		case code := <-done:
			if code != http.StatusOK {
				t.Fatalf("status %d", code)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a page waited on the record store")
		}
	}
	<-started
	close(release)
	f.svc.FlushFrontRenders()
}

// A full queue drops the row and says so; the page still answers.
func TestAFullRecordQueueDropsTheRowAndLogsIt(t *testing.T) {
	f, path := scoredFrontFixture(t, 3, 9, 8)
	logs := captureRenderLog(t)
	release := make(chan struct{})
	f.svc.frontRender.put = func(context.Context, objectgraph.Object) error { <-release; return nil }
	h := NewHandler(f.svc)
	page, _ := frontPageJSON(t, f.svc)
	ids := map[string]string{}
	for _, s := range page.Stories {
		ids[s.Title] = s.PublicationID
	}
	for i := 0; i < frontRenderQueue+6; i++ {
		writeFrontScores(t, path, ids, 1+i%9, 9-i%9, 2+i%7)
		if w := frontGet(h, "?limit=60"); w.Code != http.StatusOK {
			t.Fatal(w.Code)
		}
	}
	if !strings.Contains(logs.String(), "front render record dropped") {
		t.Fatalf("no drop logged: %q", logs.String())
	}
	close(release)
	f.svc.FlushFrontRenders()
}

// hundredStoryPage is a front page of n stories, newest first, each with an
// importance, the way the paper's first page is.
func hundredStoryPage(n int, bump int) *FrontPage {
	page := &FrontPage{Stories: []FrontStory{}}
	base := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		page.Stories = append(page.Stories, FrontStory{
			PublicationID: "pub-" + strconv.Itoa(i), Title: "Story " + strconv.Itoa(i), UpdatedAt: base.Add(-time.Duration(i) * time.Minute),
			Importance: 1 + (i*7+bump)%9, VersionCount: 1,
		})
	}
	return page
}

// Failure 6: a stranger's limit is never a key and never spends anything. On
// a 100-story page, a sweep of ?limit=1..100 (and of values past the maximum)
// writes nothing but the one canonical page, and the canonical rows are still
// written after the sweep, whatever the sweep did.
func TestALimitSweepCannotTouchTheCanonicalRows(t *testing.T) {
	f, _ := scoredFrontFixture(t, 3, 9, 8)
	svc := f.svc
	ctx := context.Background()
	for round := 0; round < 3; round++ {
		full := hundredStoryPage(100, round)
		for limit := 1; limit <= 100; limit++ {
			cut := &FrontPage{Stories: full.Stories[:limit]}
			wrote, err := svc.RecordFrontRender(ctx, limit, "", "", cut)
			if err != nil {
				t.Fatal(err)
			}
			if wrote != (limit == frontRenderCanonicalLimit) {
				t.Fatalf("limit %d: wrote = %v", limit, wrote)
			}
		}
		for _, limit := range []int{0, 101, 1000000, 99999999} {
			if wrote, _ := svc.RecordFrontRender(ctx, limit, "", "", full); wrote {
				t.Fatalf("limit %d wrote", limit)
			}
		}
	}
	svc.FlushFrontRenders()
	rows := renderRows(t, svc)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (the canonical page of each round, none from the sweep)", len(rows))
	}
	for _, r := range rows {
		if len(r.Shown) != frontRenderCanonicalLimit {
			t.Fatalf("a row shows %d stories, want %d", len(r.Shown), frontRenderCanonicalLimit)
		}
		assertShownIsRankScoredOfItsInputs(t, r)
	}
}

// A page that shrinks and returns to an earlier content is a change both ways
// (the dedupe is the last page's hash, not one hash per size).
func TestAShrinkAndARestoreOfTheCanonicalPageAreBothRecorded(t *testing.T) {
	f, _ := scoredFrontFixture(t, 3, 9, 8)
	ctx := context.Background()
	full := hundredStoryPage(60, 0)
	short := &FrontPage{Stories: full.Stories[:59]}
	for i, p := range []*FrontPage{full, short, full, full} {
		wrote, err := f.svc.RecordFrontRender(ctx, frontRenderCanonicalLimit, "", "", p)
		if err != nil {
			t.Fatal(err)
		}
		if want := i < 3; wrote != want {
			t.Fatalf("step %d: wrote = %v, want %v", i, wrote, want)
		}
	}
	f.svc.FlushFrontRenders()
	if n := len(renderRows(t, f.svc)); n != 3 {
		t.Fatalf("rows = %d, want 3", n)
	}
}

// A panic in the writer is recovered and logged; the service keeps writing.
func TestAPanickingWriterIsRecoveredAndLogged(t *testing.T) {
	f, _ := scoredFrontFixture(t, 3, 9, 8)
	logs := captureRenderLog(t)
	calls := 0
	f.svc.frontRender.put = func(context.Context, objectgraph.Object) error {
		calls++
		if calls == 1 {
			panic("store exploded")
		}
		return nil
	}
	ctx := context.Background()
	if _, err := f.svc.RecordFrontRender(ctx, frontRenderCanonicalLimit, "", "", hundredStoryPage(10, 0)); err != nil {
		t.Fatal(err)
	}
	f.svc.FlushFrontRenders()
	if !strings.Contains(logs.String(), "the writer panicked: store exploded") {
		t.Fatalf("log = %q", logs.String())
	}
	if wrote, _ := f.svc.RecordFrontRender(ctx, frontRenderCanonicalLimit, "", "", hundredStoryPage(10, 0)); !wrote {
		t.Fatal("the failed row was not retried")
	}
	f.svc.FlushFrontRenders()
	if calls != 2 {
		t.Fatalf("writer calls = %d, want 2", calls)
	}
}

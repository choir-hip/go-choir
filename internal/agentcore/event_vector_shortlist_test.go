package agentcore

// Station V5 (docs/archive/definitions/choir-night-exact-vectors-learning-2026-11-19.md;
// design docs/design/corpus-first-retrieval-and-research-2026-11-18.md, "The
// graph", station S7): every captured page reaches the event layer, research's
// reads included, and each capture's shortlist adds the events whose member
// captures are nearest to it by vector, where it had the recent events and a
// title-overlap slot. Jev still judges, in one call per flush.
//
// What goes wrong today (the failure list, written before the code):
//   - W1 a reworded report of a known event shares no title word with it, so the
//     title-overlap slot misses it, and it mints a new event (63 of 74 captures
//     minted a new event in the design's sample window);
//   - W2 a page research reads (choir.Read) never reaches the layer: only a
//     source schedule's poll queues captures (source_schedule.go), so the
//     paper's roughly 2,000 research reads in two hours are never joined;
//   - W3 a corroborated event past the 30 recent ones was reachable only by
//     title words (the corroborated overlap slot).
//
// What could go wrong with the change:
//   - X1 a vector neighbour that belongs to no event (another computer's
//     capture, a capture no flush has resolved, a sibling of this flush) puts an
//     event, or a blank, in the question;
//   - X2 a neighbour's event is offered twice (the recent slot and the vector
//     slot), or one capture's neighbours crowd every other capture's question;
//   - X3 the request names another owner or computer, or reaches back without
//     bound, so another computer's captures or a stale event are offered;
//   - X4 the unavailable embedder, a failed or slow neighbours request, or a
//     corpusd without the route stalls the flush or stops it resolving, or the
//     journal does not say the vector slot was missing;
//   - X5 neighbours cost one request per capture instead of one per flush, or
//     Jev one call per slot or per capture instead of one per flush;
//   - X6 a capture offered two events joins both (a capture has one membership);
//   - X7 the call grows past the vendor's limit (a flush of hundreds of
//     captures, each with its own neighbours);
//   - X8 a duplicate capture queued twice (a poll's and a read's) is judged
//     twice or minted twice;
//   - X9 the journal line drops a field the acceptance measurement reads, or
//     counts research's pages with the schedule's;
//   - X10 the runtime's research dependencies do not carry the hook, so the
//     tools announce nothing in production.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
)

// neighbourSink is a capture store that answers nearest-capture questions from
// a table, as corpusd would, and records each request.
type neighbourSink struct {
	mu          sync.Mutex
	requests    []researchtools.CaptureNeighboursQuery
	near        map[string][]string
	unembedded  int
	excluded    int
	cut         int
	unavailable string
	err         error
	delay       time.Duration
	state       string
}

func (s *neighbourSink) WriteCaptures(context.Context, string, string, []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	return sourcegraph.CaptureWriteResult{}, fmt.Errorf("not used")
}
func (s *neighbourSink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}
func (s *neighbourSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}

func (s *neighbourSink) CaptureNeighbours(ctx context.Context, q researchtools.CaptureNeighboursQuery) (researchtools.CaptureNeighboursAnswer, error) {
	s.mu.Lock()
	s.requests = append(s.requests, q)
	near, unavailable, err, delay := s.near, s.unavailable, s.err, s.delay
	s.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return researchtools.CaptureNeighboursAnswer{}, ctx.Err()
		}
	}
	if err != nil {
		return researchtools.CaptureNeighboursAnswer{}, err
	}
	ans := researchtools.CaptureNeighboursAnswer{Neighbours: map[string][]researchtools.CaptureNeighbour{}, Unembedded: s.unembedded, Excluded: s.excluded, Cut: s.cut, Unavailable: unavailable, State: "exact"}
	if s.state != "" {
		ans.State = s.state
	}
	for _, id := range q.CaptureIDs {
		for i, n := range near[id] {
			ans.Neighbours[id] = append(ans.Neighbours[id], researchtools.CaptureNeighbour{CaptureID: n, Score: 0.9 - float64(i)/100})
		}
	}
	return ans, nil
}

func (s *neighbourSink) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func vectorRuntime(t *testing.T, judge *eventJudge, sink *neighbourSink) (*Runtime, string, string) {
	t.Helper()
	rt, owner, computer := eventRuntime(t, judge)
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}
	return rt, owner, computer
}

// journal captures the process log while f runs.
func journal(t *testing.T, f func()) string {
	t.Helper()
	var b strings.Builder
	var mu sync.Mutex
	prev := log.Writer()
	log.SetOutput(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return b.Write(p) }))
	defer log.SetOutput(prev)
	f()
	mu.Lock()
	defer mu.Unlock()
	return b.String()
}

// eventOf is the event a capture is a member of.
func eventOf(t *testing.T, rt *Runtime, owner, computer, capture string) string {
	t.Helper()
	m, found, err := rt.store.EventMembershipOf(context.Background(), owner, computer, capture)
	if err != nil || !found {
		t.Fatalf("capture %s has no membership: %v", capture, err)
	}
	return m.EventID
}

// aged is a capture resolved at a past time, so its event is outside the
// recent slot.
func aged(id, title, url string, age time.Duration) eventInput {
	in := capItem(id, title, url, "")
	in.At = time.Now().UTC().Add(-age)
	return in
}

func eventAsked(j *eventJudge, call int, key string) string { return string(j.asked[call][key]) }

// W1, X6: a reworded report of an event five days old, which shares no title
// word with it and is far past the recent slot, joins it through its vector
// neighbour: one membership, one event, one neighbours request.
func TestEventVectorNeighbourJoinsAReWordedReportOfAnOlderEvent(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{
		"Bus and train collide at Channel Tunnel terminal": {target: "Nineteen injured in Eurotunnel accident"},
	}}
	sink := &neighbourSink{near: map[string][]string{"b1": {"a1"}}}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	ctx := context.Background()
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{aged("a1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", 5*24*time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var fill []eventInput
	for i := 0; i < eventShortlistRecent+5; i++ {
		fill = append(fill, capItem(fmt.Sprintf("f%d", i), fmt.Sprintf("Unrelated item %d", i), fmt.Sprintf("https://f%d.example.org/x", i), ""))
	}
	if _, err := rt.resolveEvents(ctx, owner, computer, fill); err != nil {
		t.Fatal(err)
	}
	sink.requests = nil
	report, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("b1", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", "")})
	if err != nil {
		t.Fatal(err)
	}
	if report.Joined != 1 || report.Minted != 0 {
		t.Fatalf("report = %+v; want the capture to join the Eurotunnel event", report)
	}
	if eventOf(t, rt, owner, computer, "b1") != eventOf(t, rt, owner, computer, "a1") {
		t.Fatal("b1 is not a member of a1's event")
	}
	if sink.requestCount() != 1 || len(sink.requests[0].CaptureIDs) != 1 || sink.requests[0].CaptureIDs[0] != "b1" {
		t.Fatalf("neighbour requests = %+v", sink.requests)
	}
	// X6: one membership, whatever the number of events offered.
	events, _ := rt.store.RecentEventMemberships(ctx, owner, computer, 100)
	n := 0
	for _, m := range events {
		if m.CaptureID == "b1" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("b1 has %d memberships", n)
	}
}

// The control of W1: with no vectors the same capture mints a rival event, so
// the test above is the vector slot's doing and not the recent slot's.
func TestEventWithoutVectorsTheReWordedReportMintsANewEvent(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{
		"Bus and train collide at Channel Tunnel terminal": {target: "Nineteen injured in Eurotunnel accident"},
	}}
	sink := &neighbourSink{unavailable: "the embedder did not answer"}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	ctx := context.Background()
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{aged("a1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", 5*24*time.Hour)}); err != nil {
		t.Fatal(err)
	}
	report, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("b1", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", "")})
	if err != nil || report.Minted != 1 || report.Joined != 0 {
		t.Fatalf("report = %+v, %v", report, err)
	}
}

// X1: neighbours that belong to no event offer nothing: another computer's or
// an unresolved capture, and a sibling queued in the same flush.
func TestEventVectorNeighboursWithNoEventOfferNothing(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	sink := &neighbourSink{near: map[string][]string{"b1": {"stranger", "unresolved", "b2"}, "b2": {"b1"}}}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	report, err := rt.resolveEvents(context.Background(), owner, computer, []eventInput{
		capItem("b1", "Harbour board approves dredging plan", "https://a.example.com/1", ""),
		capItem("b2", "Customs office moves to new building", "https://b.example.com/2", ""),
	})
	if err != nil || report.Calls != 1 || report.Minted != 2 {
		t.Fatalf("report = %+v, %v", report, err)
	}
	if got := eventAsked(judge, 0, "k1"); strings.Contains(got, `"e1"`) {
		t.Fatalf("a capture with no event neighbour was offered an event: %s", got)
	}
	if len(report.vectorOnlyEvts) != 0 {
		t.Fatalf("vector slot offered %v", report.vectorOnlyEvts)
	}
}

// X2: an event the recent slot already offers is not offered twice, and one
// capture's neighbours are offered to its own question only, at most
// eventShortlistVector of them.
func TestEventVectorSlotNeverDuplicatesAndIsPerCapture(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	sink := &neighbourSink{near: map[string][]string{}}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	ctx := context.Background()
	// Six old events, past the recent slot, and one recent event.
	var old []eventInput
	for i := 0; i < 6; i++ {
		old = append(old, aged(fmt.Sprintf("old%d", i), fmt.Sprintf("Old story number %d", i), fmt.Sprintf("https://o%d.example.com/x", i), time.Duration(5+i)*24*time.Hour))
	}
	old = append(old, capItem("recent0", "Recent story", "https://r.example.com/x", ""))
	if _, err := rt.resolveEvents(ctx, owner, computer, old); err != nil {
		t.Fatal(err)
	}
	sink.near["n1"] = []string{"recent0", "old0", "old1", "old2", "old3", "old4", "old5"}
	sink.near["n2"] = []string{"old5"}
	judge.calls, judge.asked = nil, nil
	report, err := rt.resolveEvents(ctx, owner, computer, []eventInput{
		capItem("n1", "First reworded report", "https://n1.example.com/x", ""),
		capItem("n2", "Second reworded report", "https://n2.example.com/x", ""),
	})
	if err != nil || report.Calls != 1 {
		t.Fatalf("report = %+v, %v", report, err)
	}
	var state struct {
		Events map[string]struct {
			Headline string `json:"headline"`
		} `json:"events"`
	}
	raw, _ := json.Marshal(judge.calls[0])
	_ = json.Unmarshal(raw, &state)
	alias := map[string]string{}
	for a, e := range state.Events {
		alias[e.Headline] = a
	}
	offered := func(key, headline string) bool {
		return strings.Contains(eventAsked(judge, 0, key), `"`+alias[headline]+`"`)
	}
	if !offered("k1", "Recent story") || !offered("k2", "Recent story") {
		t.Fatal("the recent event is not offered to both questions")
	}
	count := func(key string) int {
		n := 0
		for h := range alias {
			if strings.HasPrefix(h, "Old story") && offered(key, h) {
				n++
			}
		}
		return n
	}
	if got := count("k1"); got != eventShortlistVector {
		t.Fatalf("k1 was offered %d old events, want the %d nearest", got, eventShortlistVector)
	}
	if !offered("k1", "Old story number 0") || offered("k1", "Old story number 3") {
		t.Fatal("k1's neighbours are not the nearest first")
	}
	if got := count("k2"); got != 1 || !offered("k2", "Old story number 5") {
		t.Fatalf("k2 was offered %d old events; want only its own neighbour", got)
	}
	// The recent event counted once, in the recent slot: the vector slot reached
	// it but added the old events only.
	if len(report.vectorEvents) < len(report.vectorOnlyEvts) || len(report.vectorOnlyEvts) != 4 {
		t.Fatalf("vector events %d, new %d", len(report.vectorEvents), len(report.vectorOnlyEvts))
	}
}

// X3: the request names this owner and computer, only the waiting captures
// (not ones that already have a membership), and a window of 14 days.
func TestEventVectorRequestIsScopedAndWindowed(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	sink := &neighbourSink{near: map[string][]string{}}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	ctx := context.Background()
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("held", "Already resolved", "https://h.example.com/x", "")}); err != nil {
		t.Fatal(err)
	}
	sink.requests = nil
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("held", "Already resolved", "https://h.example.com/x", ""), capItem("n1", "New one", "https://n.example.com/x", "")}); err != nil {
		t.Fatal(err)
	}
	if sink.requestCount() != 1 {
		t.Fatalf("requests = %d", sink.requestCount())
	}
	q := sink.requests[0]
	want := time.Now().UTC().Add(-eventVectorHorizon).Format("2006-01-02")
	if q.OwnerID != owner || q.ComputerID != computer || len(q.CaptureIDs) != 1 || q.CaptureIDs[0] != "n1" || q.Since != want || q.Limit != eventNeighbourLimit {
		t.Fatalf("request = %+v; want owner %s computer %s ids [n1] since %s", q, owner, computer, want)
	}
}

// X4: the vector slot never stops the flush. Each way it can be missing (the
// capture store has no neighbours route, corpusd says unavailable, the request
// fails, the request is slow) leaves the flush resolving with one Jev call and
// the journal naming the slot unavailable.
func TestEventVectorSlotUnavailableNeverBlocksTheFlush(t *testing.T) {
	prevTimeout := eventNeighboursTimeout
	eventNeighboursTimeout = 100 * time.Millisecond
	defer func() { eventNeighboursTimeout = prevTimeout }()
	cases := map[string]*neighbourSink{
		"no embedder": {unavailable: "no embedder is configured"},
		"failed":      {err: fmt.Errorf("corpusd returned 503")},
		"slow":        {delay: 10 * time.Second},
	}
	for name, sink := range cases {
		t.Run(name, func(t *testing.T) {
			judge := &eventJudge{answers: map[string]eventAnswer{}}
			rt, owner, computer := vectorRuntime(t, judge, sink)
			var report eventResolveReport
			start := time.Now()
			out := journal(t, func() {
				var err error
				if report, err = rt.resolveEvents(context.Background(), owner, computer, []eventInput{capItem("c1", "Anything at all", "https://a.example.com/1", "")}); err != nil {
					t.Fatal(err)
				}
				log.Print(eventFlushLine(report))
			})
			if time.Since(start) > 5*time.Second {
				t.Fatalf("the flush waited %s for the vector slot", time.Since(start))
			}
			if report.Calls != 1 || report.Minted != 1 {
				t.Fatalf("report = %+v", report)
			}
			if !strings.Contains(out, "vector slot unavailable") || !strings.Contains(out, "vector=unavailable") {
				t.Fatalf("the journal does not say the vector slot was unavailable:\n%s", out)
			}
		})
	}
	// A capture store with no neighbours route at all (an older sink).
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	rt.researchDeps = nil
	if r, err := rt.resolveEvents(context.Background(), owner, computer, []eventInput{capItem("c1", "Anything at all", "https://a.example.com/1", "")}); err != nil || r.Minted != 1 || r.vec == nil || r.vec.Unavailable == "" {
		t.Fatalf("no sink: %+v, %v", r, err)
	}
}

// X5, X7: a flush of hundreds of captures makes one neighbours request, and
// Jev is asked in as few calls as the vendor's limits allow, each call inside
// them, the split named in the journal. (The design's sample is about 2,000
// research reads in two hours, which is about 17 a minute: one flush holds a
// few dozen. 600 is far past that and past the 255-choice limit.)
func TestEventVectorFlushOfManyCapturesIsOneNeighboursRequestAndFitsTheLimits(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	sink := &neighbourSink{near: map[string][]string{}}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	var inputs []eventInput
	for i := 0; i < 600; i++ {
		in := capItem(fmt.Sprintf("m%d", i), fmt.Sprintf("Distinct matter %d of the day", i), fmt.Sprintf("https://m%d.example.com/x", i), "")
		in.Research = i%2 == 0
		inputs = append(inputs, in)
		if i > 0 {
			sink.near[in.Item.CaptureID] = []string{fmt.Sprintf("m%d", i-1)}
		}
	}
	var report eventResolveReport
	out := journal(t, func() {
		var err error
		if report, err = rt.resolveEvents(context.Background(), owner, computer, inputs); err != nil {
			t.Fatal(err)
		}
	})
	if sink.requestCount() != 1 {
		t.Fatalf("neighbour requests = %d, want one for the flush", sink.requestCount())
	}
	if report.Unresolved != 0 || report.Minted != len(inputs) {
		t.Fatalf("report = %+v", report)
	}
	if report.Calls < 2 || report.Calls != judge.callCount() {
		t.Fatalf("calls = %d (judge %d)", report.Calls, judge.callCount())
	}
	for i, ok := range judge.fits {
		if !ok {
			t.Fatalf("call %d is over the vendor's limits", i)
		}
	}
	if !strings.Contains(out, "jev split: site=event_layer") || !strings.Contains(out, "forced_by=map[request_tokens_64k:") {
		t.Fatalf("the split does not name its limit:\n%s", out)
	}
}

// A flush that fits makes one Jev call whatever its slots offered (C of the
// station): neighbours do not add calls.
func TestEventVectorFlushIsOneJevCall(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	sink := &neighbourSink{near: map[string][]string{}}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	ctx := context.Background()
	var first []eventInput
	for i := 0; i < 20; i++ {
		first = append(first, aged(fmt.Sprintf("o%d", i), fmt.Sprintf("Older matter %d", i), fmt.Sprintf("https://o%d.example.com/x", i), 6*24*time.Hour))
	}
	if _, err := rt.resolveEvents(ctx, owner, computer, first); err != nil {
		t.Fatal(err)
	}
	judge.calls, judge.asked = nil, nil
	var second []eventInput
	for i := 0; i < 40; i++ {
		id := fmt.Sprintf("q%d", i)
		sink.near[id] = []string{fmt.Sprintf("o%d", i%20), fmt.Sprintf("o%d", (i+1)%20), fmt.Sprintf("o%d", (i+2)%20)}
		second = append(second, capItem(id, fmt.Sprintf("Newer report %d", i), fmt.Sprintf("https://q%d.example.com/x", i), ""))
	}
	report, err := rt.resolveEvents(ctx, owner, computer, second)
	if err != nil || report.Calls != 1 || judge.callCount() != 1 || sink.requestCount() < 1 {
		t.Fatalf("report = %+v, %v; jev calls %d", report, err, judge.callCount())
	}
}

// W2, X8, X9: a research read queued like a poll's capture is resolved with it
// in one flush, once, and the journal line counts it apart.
func TestEventResearchReadReachesTheLayerAndTheLineCountsIt(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	sink := &neighbourSink{near: map[string][]string{}}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	ctx := context.Background()
	read := []sourcegraph.CapturedItem{{CaptureID: "r1", Title: "Quake strikes off the coast", URL: "https://paper.example.com/quake", Summary: "A strong quake hit."}}
	polled := []sourcegraph.CapturedItem{{CaptureID: "p1", Title: "Customs office moves", URL: "https://news.example.org/customs"}, {CaptureID: "r1", Title: "Quake strikes off the coast", URL: "https://paper.example.com/quake"}}
	out := journal(t, func() {
		rt.enqueueEventCaptures(ctx, owner, computer, sourcegraph.ReadSourceID, read, true)
		rt.enqueueEventResolution(ctx, owner, computer, "rss:example", polled)
		rt.flushEventQueueForTest(ctx)
	})
	for _, id := range []string{"r1", "p1"} {
		eventOf(t, rt, owner, computer, id)
	}
	if judge.callCount() != 1 {
		t.Fatalf("Jev calls = %d, want one for the flush", judge.callCount())
	}
	// X8: r1 was queued twice and judged once (queued= counts the two questions).
	if n := strings.Count(string(judge.asked[0]["k1"])+string(judge.asked[0]["k2"])+string(judge.asked[0]["k3"]), `"type"`); n != 2 {
		t.Fatalf("the flush asked %d questions, want 2 (r1 once, p1 once)", n)
	}
	for _, want := range []string{"op=event_flush", "queued=2", "research_reads=1", "calls=1", "shortlist_recent=0", "shortlist_vector=0", "shortlist_vector_new=0", "vector=0 ", "vector_state=ok", "missing_unembedded=0", "missing_excluded=0", "vector_unmapped=0", "joined=0", "minted=2", "tentative=0", "unresolved=0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the flush line lacks %q:\n%s", want, out)
		}
	}
}

// The line's vector field is the neighbour rows; the captures with none are
// counted by reason; a deadline cut is the state, not a count of missing; the
// neighbours that mapped to no event are counted.
func TestEventFlushLineCountsVectorRowsStateAndMissingByReason(t *testing.T) {
	cases := []struct {
		name  string
		sink  *neighbourSink
		state string
		want  []string
	}{
		{"exact", &neighbourSink{near: map[string][]string{"b1": {"a1", "a2"}}, unembedded: 3, excluded: 2},
			"ok", []string{"vector=2 ", "vector_state=ok", "missing_unembedded=3", "missing_excluded=2", "vector_unmapped=2"}},
		{"approximate", &neighbourSink{near: map[string][]string{}, state: "approximate"},
			"approximate", []string{"vector=0 ", "vector_state=approximate", "missing_unembedded=0"}},
		{"partial", &neighbourSink{near: map[string][]string{}, state: "partial", cut: 2},
			"partial", []string{"vector_state=partial", "missing_unembedded=0", "missing_excluded=0"}},
		{"cut is partial", &neighbourSink{near: map[string][]string{}, cut: 1},
			"partial", []string{"vector_state=partial"}},
		{"tables not ready", &neighbourSink{unavailable: "the vector tables are not ready"},
			"unavailable", []string{"vector=unavailable", "vector_state=unavailable", "missing_unembedded=0"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			judge := &eventJudge{answers: map[string]eventAnswer{}}
			rt, owner, computer := vectorRuntime(t, judge, c.sink)
			report, err := rt.resolveEvents(context.Background(), owner, computer, []eventInput{
				capItem("b1", "First", "https://a.example.com/1", ""), capItem("b2", "Second", "https://b.example.com/2", "")})
			if err != nil {
				t.Fatal(err)
			}
			line := eventFlushLine(report)
			for _, want := range c.want {
				if !strings.Contains(line, want) {
					t.Fatalf("line lacks %q: %s", want, line)
				}
			}
		})
	}
}

// Review 7: a neighbour whose event the index does not hold (no membership, or
// past the load caps) is counted, and offers nothing.
func TestEventVectorUnmappedNeighboursAreCounted(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	sink := &neighbourSink{near: map[string][]string{"b1": {"ghost1", "ghost2"}, "b2": {"ghost2"}}}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	report, err := rt.resolveEvents(context.Background(), owner, computer, []eventInput{
		capItem("b1", "First", "https://a.example.com/1", ""), capItem("b2", "Second", "https://b.example.com/2", "")})
	if err != nil || !strings.Contains(eventFlushLine(report), "vector_unmapped=2") {
		t.Fatalf("%v %s", err, eventFlushLine(report))
	}
}

// Review 10: the neighbours request does not hold the index, so the management
// Stories read (eventSummary) is not made to wait for a slow request.
func TestEventSlowNeighboursRequestDoesNotBlockTheIndexReader(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	sink := &neighbourSink{delay: 600 * time.Millisecond}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	ctx := context.Background()
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("a1", "Opening", "https://a.example.com/1", "")}); err != nil {
		t.Fatal(err)
	}
	evID := eventOf(t, rt, owner, computer, "a1")
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_, _ = rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("a2", "Another", "https://b.example.com/2", "")})
		close(done)
	}()
	<-started
	time.Sleep(100 * time.Millisecond) // the request is in flight
	t0 := time.Now()
	if _, ok := rt.eventSummary(ctx, owner, computer, evID); !ok {
		t.Fatal("no summary")
	}
	if d := time.Since(t0); d > 200*time.Millisecond {
		t.Fatalf("eventSummary waited %s behind the neighbours request", d)
	}
	<-done
}

// Review: the capture hook returns at once, even while a flush is waiting on a
// slow neighbours request.
func TestEventCaptureHookReturnsPromptlyDuringAFlush(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	sink := &neighbourSink{delay: 600 * time.Millisecond}
	rt, owner, computer := vectorRuntime(t, judge, sink)
	ctx := context.Background()
	rt.enqueueEventCaptures(ctx, owner, computer, "web:read", []sourcegraph.CapturedItem{{CaptureID: "r1", Title: "First read", URL: "https://a.example.com/1"}}, true)
	rt.flushEventQueue(owner, computer)
	time.Sleep(100 * time.Millisecond)
	t0 := time.Now()
	rt.enqueueEventCaptures(ctx, owner, computer, "web:read", []sourcegraph.CapturedItem{{CaptureID: "r2", Title: "Second read", URL: "https://b.example.com/2"}}, true)
	if d := time.Since(t0); d > 100*time.Millisecond {
		t.Fatalf("the hook took %s", d)
	}
	rt.flushEventQueueForTest(ctx)
	eventOf(t, rt, owner, computer, "r2")
}

// X10: the runtime's research dependencies announce what the tools capture.
func TestEventRuntimeInstallsTheCaptureHook(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	if err := rt.InstallDefaultAgentTools(); err != nil {
		t.Fatal(err)
	}
	if rt.researchDeps == nil || rt.researchDeps.OnCaptured == nil {
		t.Fatal("the research dependencies carry no capture hook")
	}
	rt.researchDeps.OnCaptured(context.Background(), owner, computer, sourcegraph.ReadSourceID,
		[]sourcegraph.CapturedItem{{CaptureID: "r1", Title: "Quake strikes off the coast", URL: "https://paper.example.com/quake"}}, true)
	rt.flushEventQueueForTest(context.Background())
	eventOf(t, rt, owner, computer, "r1")
}

// The recent slot is bounded and holds only the recency horizon; older events
// are the vector slot's.
func TestEventRecentSlotIsBoundedByCountAndHorizon(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	var in []eventInput
	for i := 0; i < eventShortlistRecent+10; i++ {
		in = append(in, capItem(fmt.Sprintf("r%d", i), fmt.Sprintf("Recent %d", i), fmt.Sprintf("https://r%d.example.com/x", i), ""))
	}
	in = append(in, aged("old", "Old", "https://old.example.com/x", eventRecentHorizon+time.Hour))
	if _, err := rt.resolveEvents(ctx, owner, computer, in); err != nil {
		t.Fatal(err)
	}
	idx := rt.eventIndexFor(owner, computer)
	got := eventShortlist(idx, time.Now().UTC())
	if len(got) != eventShortlistRecent {
		t.Fatalf("recent slot = %d", len(got))
	}
	for _, v := range got {
		if v.ev.Headline == "Old" {
			t.Fatal("an event past the recency horizon is in the recent slot")
		}
	}
}

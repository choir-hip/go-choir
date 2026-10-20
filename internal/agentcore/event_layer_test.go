package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// The event layer's V1 (docs/reports/event-layer-at-ingestion-design-
// 2026-10-16.md): captures from every source are resolved together by Jev
// onto events, with no desk run, so corroboration accumulates across
// outlets before any story exists. Ways it could fail, written before the
// code (design §10, panel round 1):
//   - R1 a low-confidence match joins as a counted member (a false merge
//     counts another event's outlet), or mints a new event and splinters
//     the event (Gemini B1): it must be a tentative, uncounted member;
//   - R2 resolution runs per capture or per source, so calls scale with
//     sources; or chunks resolve independently, so an event minted in one
//     chunk is not a candidate in the next;
//   - R3 a retried resolution mints a second event or membership for one
//     capture;
//   - R4 two outlets that first meet in one flush (0 -> 2) never
//     corroborate, or corroboration fires again on the third outlet;
//   - R5 a slow second report misses an uncorroborated event older than
//     the recency horizon (Gemini B2), or the shortlist reaches back
//     without bound;
//   - R6 two outlets carrying the same wire story count as two origins
//     (Gemini B3);
//   - R7 a restart loses what the store holds: the index rebuilt from the
//     store must give the same outlets and corroboration, and nothing
//     queued before the restart is resolved after it;
//   - R8 a call's state grows past Jev's default-context bound;
//   - R9 the setting off still calls Jev;
//   - R10 a capture Jev's answer omits is minted as an event instead of
//     staying unresolved (code review, Gemini);
//   - R11 a capture unioned with a tentative member mints a rival event
//     (code review, Devin B1);
//   - R12 flushes run concurrently, so queued flushes pile on the index and
//     expire, and chunk order breaks (code review, Gemini);
//   - R13 a stopped runtime still flushes from a timer (code review, Gemini).
//
// Owner ruling 2026-10-20 (docs/problems/event-layer-false-merges-at-
// marginal-p-2026-10-16.md, "Owner ruling and plan"): one event is one
// incident, one place, one time; a later development is a new event linked
// to the earlier one. Ways that could fail, written before the code (the
// full list is F1-F8 in the problem doc):
//   - F1 the Jev request still tells it a development is the same event;
//   - F2 true duplicates (two outlets, one incident, updated details) stop
//     merging and corroborating;
//   - F3 a story-level pair from the false-merge tables joins as a counted
//     member and fakes corroboration instead of minting its own event;
//   - F4 the link lends one event's outlets to the other;
//   - F5 a weak, unknown or conflicting link answer sets a wrong link or
//     breaks the resolver;
//   - F6 the extra choices grow the request past the context bound;
//   - F7 the blind judge script's definition drifts from the resolver's.

// eventJudge answers each capture question by title: target is an event
// headline, a recent title of one, or another capture's title; "" is new.
type eventJudge struct {
	mu       sync.Mutex
	answers  map[string]eventAnswer
	omit     map[string]bool // capture titles whose answer is left out
	calls    []map[string]any
	asked    []map[string]json.RawMessage // each call's questions
	bytes    []int
	delay    time.Duration
	inFlight int
	maxIn    int
}

type eventAnswer struct {
	target string
	p      float64
	// after answers the target event as an earlier incident of the same
	// story: the capture is a new event that follows it. The fake picks
	// the after_ choice only when the question offered it, as Jev would.
	after bool
}

func (j *eventJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", state, questions)
}

func (j *eventJudge) JudgeScoped(_ context.Context, _ string, stateRaw json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	var state struct {
		Events map[string]struct {
			Headline string   `json:"headline"`
			Recent   []string `json:"recent_titles"`
		} `json:"events"`
		Captures map[string]struct {
			Title string `json:"title"`
		} `json:"captures"`
	}
	if err := json.Unmarshal(stateRaw, &state); err != nil {
		return nil, err
	}
	var generic map[string]any
	_ = json.Unmarshal(stateRaw, &generic)
	total := len(stateRaw)
	for _, q := range questions {
		total += len(q)
	}
	j.mu.Lock()
	j.calls = append(j.calls, generic)
	j.asked = append(j.asked, questions)
	j.bytes = append(j.bytes, total)
	j.inFlight++
	if j.inFlight > j.maxIn {
		j.maxIn = j.inFlight
	}
	j.mu.Unlock()
	defer func() {
		j.mu.Lock()
		j.inFlight--
		j.mu.Unlock()
	}()
	if j.delay > 0 {
		time.Sleep(j.delay)
	}
	answers := map[string]any{}
	for key := range questions {
		title := state.Captures[key].Title
		if j.omit[title] {
			continue
		}
		a := j.answers[title]
		choice := "new"
		if a.target != "" {
			for alias, ev := range state.Events {
				match := ev.Headline == a.target
				for _, r := range ev.Recent {
					if r == a.target {
						match = true
					}
				}
				switch {
				case match && a.after:
					if strings.Contains(string(questions[key]), `"after_`+alias+`"`) {
						choice = "after_" + alias
					}
				case match:
					choice = alias
				}
			}
			for k, c := range state.Captures {
				if c.Title == a.target && k != key && !a.after {
					choice = k
				}
			}
		}
		p := a.p
		if p == 0 {
			p = 0.95
		}
		answers[key] = map[string]any{"choice": choice, "probabilities": map[string]float64{choice: p}}
	}
	return json.Marshal(map[string]any{"model": "typesafe/jev-1.14-20261014", "answers": answers,
		"usage": map[string]any{"input_tokens": 400, "output_tokens": 30, "cost": 1.68e-05}})
}

func (j *eventJudge) callCount() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.calls)
}

func eventRuntime(t *testing.T, judge *eventJudge) (*Runtime, string, string) {
	t.Helper()
	rt, _ := testRuntime(t)
	rt.SetCandidateJudge(judge)
	return rt, "owner-events", rt.cfg.ComputerID
}

func capItem(id, title, url, summary string) eventInput {
	return eventInput{Item: sourcegraph.CapturedItem{CaptureID: id, Title: title, URL: url, Summary: summary}, SourceID: "src-" + id, At: time.Now().UTC()}
}

func TestEventLayerCorroboratesAcrossOutletsInOneBatchedCall(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{
		"Bus and train collide at Channel Tunnel terminal": {target: "Nineteen injured in Eurotunnel accident"},
	}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	report, err := rt.resolveEvents(ctx, owner, computer, []eventInput{
		capItem("c1", "Nineteen injured in Eurotunnel accident", "https://www.bbc.co.uk/news/1", ""),
		capItem("c2", "Bus and train collide at Channel Tunnel terminal", "https://www.dw.com/en/2", ""),
		capItem("c3", "Plex sells movie tickets", "https://arstechnica.com/3", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	// R2: three sources, one call.
	if judge.callCount() != 1 || report.Calls != 1 {
		t.Fatalf("calls = %d (report %d), want one call for the whole flush", judge.callCount(), report.Calls)
	}
	// R4: the two outlets first met in this flush and corroborate once.
	if len(report.Corroborated) != 1 {
		t.Fatalf("corroborated = %v, want the Eurotunnel event", report.Corroborated)
	}
	ev, ok := rt.eventSummary(ctx, owner, computer, report.Corroborated[0])
	if !ok || len(ev.Outlets) != 2 || ev.Members != 2 {
		t.Fatalf("event = %+v", ev)
	}
	// A third outlet does not corroborate again.
	judge.answers["Eurotunnel crash injures 19"] = eventAnswer{target: "Nineteen injured in Eurotunnel accident"}
	again, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("c4", "Eurotunnel crash injures 19", "https://www.euronews.com/4", "")})
	if err != nil || len(again.Corroborated) != 0 || again.Joined != 1 {
		t.Fatalf("third outlet: %+v, %v", again, err)
	}
	if ev, _ := rt.eventSummary(ctx, owner, computer, report.Corroborated[0]); len(ev.Outlets) != 3 {
		t.Fatalf("outlets after the third = %v", ev.Outlets)
	}
}

func TestEventLayerLowConfidenceMatchIsTentativeAndUncounted(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{
		"Ferry strike spreads": {target: "Harbor workers strike", p: 0.6},
		"Weather tomorrow":     {target: "Harbor workers strike", p: 0.3},
	}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	first, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("h1", "Harbor workers strike", "https://a.example.com/1", "")})
	if err != nil || first.Minted != 1 {
		t.Fatalf("first: %+v %v", first, err)
	}
	r, err := rt.resolveEvents(ctx, owner, computer, []eventInput{
		capItem("h2", "Ferry strike spreads", "https://b.example.net/2", ""),
		capItem("h3", "Weather tomorrow", "https://c.example.org/3", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	// R1: p 0.6 is tentative (no new event, no corroboration); p 0.3 is new.
	if r.Tentative != 1 || r.Minted != 1 || len(r.Corroborated) != 0 {
		t.Fatalf("report = %+v", r)
	}
	m, found, err := rt.store.EventMembershipOf(ctx, owner, computer, "h2")
	if err != nil || !found || !m.Tentative {
		t.Fatalf("h2 membership = %+v found=%v err=%v; want tentative", m, found, err)
	}
	ev, _ := rt.eventSummary(ctx, owner, computer, m.EventID)
	if len(ev.Outlets) != 1 || ev.Tentative != 1 {
		t.Fatalf("event = %+v; the tentative outlet must not count", ev)
	}
}

func TestEventLayerRetryIsIdempotentAndChunksSeeEarlierChunks(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	var inputs []eventInput
	for i := 0; i < eventChunkSize+3; i++ {
		inputs = append(inputs, capItem(fmt.Sprintf("x%d", i), fmt.Sprintf("Story number %d about topic", i), fmt.Sprintf("https://outlet%d.com/a", i), ""))
	}
	// The last capture reports the first chunk's first event.
	judge.answers[inputs[len(inputs)-1].Item.Title] = eventAnswer{target: inputs[0].Item.Title}
	r, err := rt.resolveEvents(ctx, owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	// R2: two sequential calls; the second saw the first's events.
	if r.Calls != 2 || len(r.Corroborated) != 1 {
		t.Fatalf("report = %+v; want 2 calls and the cross-chunk join corroborating", r)
	}
	// R3: resolving the same captures again writes nothing new.
	again, err := rt.resolveEvents(ctx, owner, computer, inputs)
	if err != nil || again.Minted != 0 || again.Joined != 0 || again.Calls != 0 {
		t.Fatalf("retry: %+v %v; want no calls and no writes", again, err)
	}
	events, _ := rt.store.RecentWorldEvents(ctx, owner, computer, 100)
	if len(events) != eventChunkSize+2 {
		t.Fatalf("events = %d, want %d", len(events), eventChunkSize+2)
	}
}

func TestEventLayerShortlistReachesUncorroboratedEventsWithinTheOverlapHorizon(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	now := time.Now().UTC()
	old := capItem("o1", "Regulator fines Acme over pipeline leak", "https://a.example.com/1", "")
	old.At = now.Add(-5 * 24 * time.Hour)
	ancient := capItem("o2", "Regulator fines Acme over earlier spill", "https://b.example.com/1", "")
	ancient.At = now.Add(-20 * 24 * time.Hour)
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{ancient}); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{old}); err != nil {
		t.Fatal(err)
	}
	// Fill the recency slots with newer, unrelated events.
	var fill []eventInput
	for i := 0; i < eventShortlistRecent+2; i++ {
		fill = append(fill, capItem(fmt.Sprintf("f%d", i), fmt.Sprintf("Unrelated item %d", i), fmt.Sprintf("https://f%d.example.org/x", i), ""))
	}
	if _, err := rt.resolveEvents(ctx, owner, computer, fill); err != nil {
		t.Fatal(err)
	}
	judge.calls = nil
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("n1", "Acme pipeline leak fine upheld", "https://c.example.net/1", "")}); err != nil {
		t.Fatal(err)
	}
	headlines := ""
	if len(judge.calls) == 1 {
		b, _ := json.Marshal(judge.calls[0]["events"])
		headlines = string(b)
	}
	// R5: the 5-day-old uncorroborated event is a candidate; the 20-day one is not.
	if !strings.Contains(headlines, "pipeline leak") || strings.Contains(headlines, "earlier spill") {
		t.Fatalf("shortlist = %s", headlines)
	}
}

func TestEventLayerWireCopyCountsAsOneOrigin(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{
		"Quake strikes off coast, tsunami warning issued": {target: "Strong quake hits off coast"},
	}}
	rt, owner, computer := eventRuntime(t, judge)
	r, err := rt.resolveEvents(context.Background(), owner, computer, []eventInput{
		capItem("w1", "Strong quake hits off coast", "https://www.thestar.com/1", "TOKYO (Reuters) - A strong earthquake struck off the coast on Thursday."),
		capItem("w2", "Quake strikes off coast, tsunami warning issued", "https://www.channelnewsasia.com/2", "TOKYO (Reuters) - A strong earthquake struck off the coast on Thursday, officials said."),
	})
	if err != nil {
		t.Fatal(err)
	}
	// R6: two outlets, one wire origin: not corroborated.
	if len(r.Corroborated) != 0 || r.Joined != 1 {
		t.Fatalf("report = %+v; wire copy on two outlets is one origin", r)
	}
}

func TestEventLayerIndexRebuildsFromTheStore(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{"B reports it": {target: "A reports it"}}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	r, err := rt.resolveEvents(ctx, owner, computer, []eventInput{
		capItem("a", "A reports it", "https://a.example.com/1", ""), capItem("b", "B reports it", "https://b.example.net/1", ""),
	})
	if err != nil || len(r.Corroborated) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	before, _ := rt.eventSummary(ctx, owner, computer, r.Corroborated[0])
	// R7: a fresh index (as after a restart) rebuilds from the store.
	rt.resetEventIndexForTest()
	after, ok := rt.eventSummary(ctx, owner, computer, r.Corroborated[0])
	if !ok || len(after.Outlets) != len(before.Outlets) || after.Corroborated != before.Corroborated || !after.Corroborated {
		t.Fatalf("before %+v after %+v", before, after)
	}
}

func TestEventLayerKeepsEachCallUnderTheContextBound(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	long := strings.Repeat("A long summary sentence about the matter. ", 200)
	// Fill the shortlist first: questions repeat every candidate's
	// criteria, so the bound must count them (code review, Devin B2).
	var fill []eventInput
	for i := 0; i < eventShortlistRecent+5; i++ {
		fill = append(fill, capItem(fmt.Sprintf("p%d", i), fmt.Sprintf("Prior event number %d with a fairly long headline about matters", i), fmt.Sprintf("https://p%d.example.com/x", i), ""))
	}
	if _, err := rt.resolveEvents(context.Background(), owner, computer, fill); err != nil {
		t.Fatal(err)
	}
	judge.bytes = nil
	var inputs []eventInput
	for i := 0; i < eventChunkSize*2; i++ {
		inputs = append(inputs, capItem(fmt.Sprintf("l%d", i), fmt.Sprintf("Long item %d", i), fmt.Sprintf("https://l%d.example.com/x", i), long))
	}
	r, err := rt.resolveEvents(context.Background(), owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if r.Unresolved != 0 || r.Minted != len(inputs) {
		t.Fatalf("report = %+v", r)
	}
	for i, n := range judge.bytes { // R8
		if n > eventStateMaxBytes {
			t.Fatalf("call %d state and questions = %d bytes, over %d", i, n, eventStateMaxBytes)
		}
	}
}

func TestEventLayerOffMakesNoCall(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	rt.enqueueEventResolution(context.Background(), owner, computer, "src", []sourcegraph.CapturedItem{{CaptureID: "z1", Title: "Anything", URL: "https://a.example.com/z"}})
	rt.flushEventQueueForTest(context.Background())
	if judge.callCount() != 0 { // R9
		t.Fatalf("intake.events is off but Jev was called %d times", judge.callCount())
	}
	if err := rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeEvents, Value: true}); err != nil {
		t.Fatal(err)
	}
	rt.enqueueEventResolution(context.Background(), owner, computer, "src", []sourcegraph.CapturedItem{{CaptureID: "z2", Title: "Anything else", URL: "https://a.example.com/z2"}})
	rt.flushEventQueueForTest(context.Background())
	if _, found, _ := rt.store.EventMembershipOf(context.Background(), owner, computer, "z2"); !found {
		t.Fatal("intake.events on: the queued capture was not resolved")
	}
}

func (rt *Runtime) resetEventIndexForTest() {
	rt.events.mu.Lock()
	rt.events.indexes = nil
	rt.events.mu.Unlock()
}

func (rt *Runtime) flushEventQueueForTest(context.Context) {
	rt.events.mu.Lock()
	var keys []string
	for k := range rt.events.queues {
		keys = append(keys, k)
	}
	rt.events.mu.Unlock()
	for _, k := range keys {
		owner, computer, _ := strings.Cut(k, "\x00")
		rt.flushEventQueue(owner, computer)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		busy := false
		rt.events.mu.Lock()
		for _, q := range rt.events.queues {
			busy = busy || q.inFlight || len(q.items) > 0
		}
		rt.events.mu.Unlock()
		if !busy {
			return
		}
	}
}

// docs/problems/event-layer-false-merges-at-marginal-p-2026-10-16.md:
// on staging every join from p 0.85 to 0.92 was a topic merge that faked a
// corroboration. A match below 0.92 is tentative, whether to a known event
// or to a sibling capture; 0.92 and above counts.
func TestEventLayerMarginalMatchIsTentativeNotCorroboration(t *testing.T) {
	judge := &eventJudge{answers: map[string]eventAnswer{
		"Middle East updates: Houthis target palace": {target: "Iran war live: Saudi blasts Houthis", p: 0.90},
		"Saudi coalition condemns Houthi strikes":    {target: "Iran war live: Saudi blasts Houthis", p: 0.92},
	}}
	rt, owner, computer := eventRuntime(t, judge)
	ctx := context.Background()
	if _, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("m1", "Iran war live: Saudi blasts Houthis", "https://a.example.com/1", "")}); err != nil {
		t.Fatal(err)
	}
	r, err := rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("m2", "Middle East updates: Houthis target palace", "https://b.example.net/2", "")})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tentative != 1 || len(r.Corroborated) != 0 {
		t.Fatalf("p 0.90 report = %+v; want a tentative member and no corroboration", r)
	}
	r, err = rt.resolveEvents(ctx, owner, computer, []eventInput{capItem("m3", "Saudi coalition condemns Houthi strikes", "https://c.example.org/3", "")})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Corroborated) != 1 {
		t.Fatalf("p 0.92 report = %+v; want the event corroborated", r)
	}
}

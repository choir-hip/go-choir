package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/jevrecord"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Intake labels (L2a A1 / R2-3 / R2-4,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). Ways they can
// fail, written before the code:
//  1. With intake.labels off, a poll still calls Jev (cost and a model on
//     the poll path before the fixture passed).
//  2. Jev fails, is slow or ignores its deadline, and the arrival waits on
//     it: labels must never gate arrival (5 s per call, 15 s per poll).
//  3. A label is written into the arrival record, so a relabel on retry
//     changes the command digest and the replay is refused.
//  4. Short aliases (c1, n1) are not mapped back, or mapped to the wrong
//     doc or capture: the label names a story Jev did not choose.
//  5. Two captures of one poll that report one event are labelled new
//     twice (no siblings), so management opens two stories.
//  6. Labels share the support judge's rate bucket and starve it.
//  7. More than four calls run at once, or calls run one at a time and
//     a large poll runs out of budget.
//  8. The label loses what it needs later: probability, model, cost from
//     the response's usage, the label version.
//  9. A relabel overwrites the version management already saw.
//
// Batched labels (docs/design/jev-batched-questions-2026-11-10.md, step 1:
// one call per candidate chunk carries the questions of up to
// intakeBatchCaptures captures). Ways that can fail, written before the code:
//
//	B1. Attribution shifts: capture i takes capture j's answer (a handle or
//	    question name maps to the wrong capture, or off by one).
//	B2. A capture is silently unlabelled, or labelled twice, when the batch
//	    splits at the capture cap or at the byte line: every capture must be
//	    in exactly one group per chunk it has candidates in.
//	B3. A request goes over the byte line (intakeRequestMaxBytes) instead of
//	    splitting the capture batch, or the split drops a capture.
//	B4. One bad answer (a missing event answer, an off-menu choice) voids the
//	    other captures of the call.
//	B5. A failed call labels its captures on part of the evidence, or voids
//	    captures of another call.
//	B6. The tape record (the jev-calls log) is missing a capture's questions,
//	    double-counts them, or cannot join a question to its capture.
//	B7. The site still fans out many calls: in-flight calls are bounded by
//	    intakeLabelConcurrency, and the call count is chunks x groups.
//	B8. Ordering changes: labels come back in the poll's order, the chunks
//	    merge in chunk order, and the choice order (newest story first, new /
//	    none last) is as before.
//	B9. The batched path labels differently from the per-capture path given
//	    the same answers (TestIntakeBatchEquivalentToPerCapture).
//	B11. The question's option set is not the keys of its criteria (the
//	    vendor reads options from them; a choices field is not in its
//	    schema), so Jev could pick only new / none, silently, with status 200.
//	B12. A bare handle option is several hops from its story: the state entry
//	    must start with exactly its handle, and the question must say so.
//	B13. A capture's options follow the poll-wide first-seen order of the
//	    shared state, not its own (window newest first, its own overlap
//	    picks in its own score order); the first option is what Jev leans to.
//	B14. A voided capture's cost share is on no label and nowhere else: the
//	    call's tape record must carry the whole call's cost.
//	B15. A slow Jev ends the budget, per-capture lines stop, and the poll
//	    leaves no trace: one counts-only line per poll.
//	B10. A capture's share of the call's cost and tokens is lost or counted
//	    whole by every capture of the call.

// labelFeedItem is one item of the test feed.
type labelFeedItem struct{ title, summary string }

// labelSink captures each URL once, reporting the feed's summary, the
// outlet and the language as sourcegraph does.
type labelSink struct {
	mu   sync.Mutex
	seen map[string]string
}

func (s *labelSink) WriteCaptures(_ context.Context, _, _ string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out sourcegraph.CaptureWriteResult
	for _, item := range items {
		if _, ok := s.seen[item.URL]; ok {
			out.Repeated++
			continue
		}
		id := fmt.Sprintf("choir.web_capture:label-%d", len(s.seen)+1)
		s.seen[item.URL] = id
		out.New++
		out.Captured = append(out.Captured, sourcegraph.CapturedItem{CaptureID: id, Title: item.Title, URL: item.URL,
			Language: item.Language, Script: sources.GuessScript(item.Title), Summary: firstNonEmpty(item.Summary, item.Body), Outlet: firstNonEmpty(item.Outlet, "Test Outlet")})
	}
	return out, nil
}

func (s *labelSink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}

func (s *labelSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}

// fakeIntakeJudge answers each capture by title: the headline of the story
// or the title of the sibling capture it reports, or "new".
type fakeIntakeJudge struct {
	mu       sync.Mutex
	answers  map[string]string // capture title -> story headline, sibling title, or "new"
	topics   map[string]string // capture title -> story headline sharing the topic
	calls    []fakeIntakeCall
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	delay    time.Duration
	fail     error
	block    bool
	model    string
	// dropEvent leaves out the event answer of the capture with this
	// title; offMenu answers it with a choice that was never offered.
	dropEvent map[string]bool
	offMenu   map[string]bool
}

type fakeIntakeCall struct {
	scope     string
	state     map[string]any
	questions map[string]map[string]any
	// bytes is the request's size: the state and every question.
	bytes int
	// raw is the questions as sent, key order intact.
	raw map[string]json.RawMessage
}

// storyEntries are state.stories, the list of "c12: headline (time) — lede".
func storyEntries(state map[string]any) []string {
	var out []string
	list, _ := state["stories"].([]any)
	for _, v := range list {
		s, _ := v.(string)
		out = append(out, s)
	}
	return out
}

// questionOptions are a question's options in the order sent: the keys of
// its criteria map, which is how Jev reads the option set. It fails a
// question with a choices field, which the vendor's schema does not have.
func questionOptions(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	if _, ok := shape["choices"]; ok {
		t.Fatalf("question carries a choices field; the option set is the keys of criteria: %s", raw)
	}
	crit := shape["criteria"]
	dec := json.NewDecoder(strings.NewReader(string(crit)))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("criteria is not an object: %s", crit)
	}
	var keys []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k.(string))
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
	}
	if len(keys) < 1 || len(keys) > 255 {
		t.Fatalf("criteria has %d entries; the vendor takes 1 to 255", len(keys))
	}
	return keys
}

func (j *fakeIntakeJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", state, questions)
}

func (j *fakeIntakeJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	n := j.inFlight.Add(1)
	defer j.inFlight.Add(-1)
	for {
		seen := j.maxSeen.Load()
		if n <= seen || j.maxSeen.CompareAndSwap(seen, n) {
			break
		}
	}
	call := fakeIntakeCall{scope: scope, questions: map[string]map[string]any{}}
	_ = json.Unmarshal(state, &call.state)
	call.bytes = len(state)
	call.raw = questions
	for name, q := range questions {
		call.bytes += len(q)
		var m map[string]any
		_ = json.Unmarshal(q, &m)
		call.questions[name] = m
	}
	j.mu.Lock()
	j.calls = append(j.calls, call)
	j.mu.Unlock()
	if j.block {
		time.Sleep(time.Hour) // ignores its deadline
	}
	if j.delay > 0 {
		time.Sleep(j.delay)
	}
	if j.fail != nil {
		return nil, j.fail
	}
	captures, _ := call.state["captures"].(map[string]any)
	alias := func(want string) string {
		if want == "" {
			return ""
		}
		for _, e := range storyEntries(call.state) {
			if h, rest, _ := strings.Cut(e, ": "); strings.HasPrefix(rest, want+" (") {
				return h
			}
		}
		for a, v := range captures {
			if m, _ := v.(map[string]any); m["title"] == want {
				return a
			}
		}
		return ""
	}
	answers := map[string]any{}
	for name := range questions {
		kind, handle, _ := strings.Cut(name, "_")
		subject, _ := captures[handle].(map[string]any)
		title, _ := subject["title"].(string)
		switch kind {
		case "event":
			if j.dropEvent[title] {
				continue
			}
			event := "new"
			if a := alias(j.answers[title]); a != "" {
				event = a
			}
			if j.offMenu[title] {
				event = "zzz"
			}
			answers[name] = map[string]any{"choice": event, "probabilities": map[string]float64{event: 0.93}, "confidence": 0.9}
		case "topic":
			topic := "none"
			if a := alias(j.topics[title]); a != "" {
				topic = a
			}
			answers[name] = map[string]any{"choice": topic, "probabilities": map[string]float64{topic: 0.81}}
		}
	}
	return json.Marshal(map[string]any{"model": firstNonEmpty(j.model, "typesafe/jev-1.13-20260917"), "answers": answers,
		"usage": map[string]any{"input_tokens": 685, "output_tokens": 47, "cost": 2.877e-05}})
}

// labelCalls are the calls that ask intake label questions (event_nX), not
// the event layer's.
func (j *fakeIntakeJudge) labelCalls() []fakeIntakeCall {
	j.mu.Lock()
	defer j.mu.Unlock()
	var out []fakeIntakeCall
	for _, c := range j.calls {
		for name := range c.questions {
			if strings.HasPrefix(name, "event_n") {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func (j *fakeIntakeJudge) callCount() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.calls)
}

// openTestStory opens a story the way management's Cast to texture:new
// does and returns its doc id.
func openTestStory(t *testing.T, s *store.Store, owner, computer, headline string) string {
	t.Helper()
	management := agentprofile.Management + ":" + owner
	id := fmt.Sprintf("cell-%x:cast:c1", headline)
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: headline},
		Cast:      &types.CommitmentCast{Publish: types.CastPublishAuto},
		Inbox:     types.StoryOpenInbox(), SenderInbox: management, EmitterKind: types.CommitmentEmitterAgent,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: management},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cell-records:" + id, Records: []types.CommitmentRecord{cast}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	receipt, err := s.CommitInboxRecords(context.Background(), req)
	if err != nil {
		t.Fatalf("open story %q: %v", headline, err)
	}
	return strings.TrimPrefix(receipt.Records[0].Inbox, "texture:")
}

type intakeFixture struct {
	rt       *Runtime
	s        *store.Store
	owner    string
	computer string
	sourceID string
	sink     *labelSink
	wakes    *[]armedWake
	items    *[]labelFeedItem
}

func newIntakeFixture(t *testing.T, owner string) *intakeFixture {
	t.Helper()
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() { sourcefetch.SetAllowPrivateNetworkForTests(prev) })
	items := &[]labelFeedItem{}
	var mu sync.Mutex
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var b strings.Builder
		for i, it := range *items {
			fmt.Fprintf(&b, `<item><title>%s</title><link>https://labels.example/%d</link><description>%s</description><guid>l-%d</guid></item>`, it.title, i, it.summary, i)
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Labels</title>%s</channel></rss>`, b.String())
	}))
	t.Cleanup(feed.Close)
	rt, s := testRuntime(t)
	f := &intakeFixture{rt: rt, s: s, owner: owner, computer: rt.cfg.ComputerID, sourceID: "rss:labels-" + owner, sink: &labelSink{seen: map[string]string{}}, items: items}
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: f.sourceID, Type: sources.SourceTypeRSS, Name: "Labels Daily", URL: feed.URL, Languages: []string{"en"}}}}
	rt.researchDeps = &researchtools.Dependencies{Sources: reg, Captures: f.sink}
	var wakes []armedWake
	f.wakes = &wakes
	rt.SetScheduleActor(func(_ context.Context, _, _, toAgentID, kind, content, _, _ string, notBefore time.Time) error {
		wakes = append(wakes, armedWake{mailbox: toAgentID, kind: kind, content: content, notBefore: notBefore})
		return nil
	})
	return f
}

func (f *intakeFixture) enable(t *testing.T) {
	t.Helper()
	if err := f.s.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: f.owner, ComputerID: f.computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
}

// poll starts the schedule if needed and runs one poll.
func (f *intakeFixture) poll(t *testing.T) store.SourceSchedule {
	t.Helper()
	ctx := context.Background()
	if len(*f.wakes) == 0 {
		if _, err := f.rt.StartSourceSchedule(ctx, f.owner, f.computer, f.sourceID, 10*time.Minute, 50); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.rt.HandleSourcePollDue(ctx, f.owner, f.computer, (*f.wakes)[len(*f.wakes)-1].content); err != nil {
		t.Fatal(err)
	}
	sched, err := f.s.GetSourceSchedule(ctx, f.owner, f.sourceID)
	if err != nil {
		t.Fatal(err)
	}
	return sched
}

// labelNow holds captured and runs the flush at once, returning its counts.
func labelNow(rt *Runtime, ctx context.Context, owner, computer, sourceID string, captured []sourcegraph.CapturedItem) (labelled, unlabelled int) {
	var held []intakeHeld
	for _, c := range captured {
		if strings.TrimSpace(c.CaptureID) != "" {
			held = append(held, intakeHeld{item: c, sourceID: sourceID, at: time.Now()})
		}
	}
	return rt.labelHeld(ctx, owner, computer, held)
}

func (f *intakeFixture) arrivals(t *testing.T) []types.CommitmentRecord {
	t.Helper()
	records, err := f.s.ListInboxRecords(context.Background(), f.owner, f.computer, agentprofile.Management+":"+f.owner, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func (f *intakeFixture) captureID(n int) string {
	return f.sink.seen[fmt.Sprintf("https://labels.example/%d", n)]
}

func TestIntakeLabelsOffMakesNoJevCall(t *testing.T) {
	f := newIntakeFixture(t, "owner-labels-off")
	storeIntakeLabelsOff(t, f.s, f.owner, f.computer)
	judge := &fakeIntakeJudge{}
	f.rt.SetCandidateJudge(judge)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	*f.items = []labelFeedItem{{"Papst an der Lunge operiert", "Der Vatikan meldet eine erfolgreiche Operation."}}
	f.poll(t)
	if judge.callCount() != 0 {
		t.Fatalf("intake.labels is off but Jev was called %d times", judge.callCount())
	}
	if got := f.arrivals(t); len(got) != 1 || got[0].Captures == nil || len(got[0].Captures.Items) != 1 {
		t.Fatalf("arrivals = %+v; want one arrival", got)
	}
	if labels, err := f.s.CaptureLabels(context.Background(), f.owner, f.computer, f.captureID(0)); err != nil || len(labels) != 0 {
		t.Fatalf("labels with the setting off = %+v, %v", labels, err)
	}
}

func TestIntakeLabelsMapAliasesBackLabelSiblingsAndStayOutOfTheArrival(t *testing.T) {
	ctx := context.Background()
	f := newIntakeFixture(t, "owner-labels-on")
	f.enable(t)
	pope := openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	nordStream := openTestStory(t, f.s, f.owner, f.computer, "Nord Stream trial opens in Hamburg")
	judge := &fakeIntakeJudge{
		answers: map[string]string{
			"Papst an der Lunge operiert":           "Pope has lung surgery",
			"Russian strikes hit Kyiv power plant":  "new",
			"Удар по электростанции в Киеве":        "Russian strikes hit Kyiv power plant",
			"Ukrainian goes on trial over pipeline": "new",
		},
		topics: map[string]string{"Ukrainian goes on trial over pipeline": "Nord Stream trial opens in Hamburg"},
	}
	f.rt.SetCandidateJudge(judge)
	*f.items = []labelFeedItem{
		{"Papst an der Lunge operiert", "Der Vatikan meldet eine erfolgreiche Operation."},
		{"Russian strikes hit Kyiv power plant", "Ballistic missiles struck the plant overnight."},
		{"Удар по электростанции в Киеве", "Ракеты поразили электростанцию."},
		{"Ukrainian goes on trial over pipeline", "A second suspect appears in court."},
	}
	f.poll(t)

	// B7: four captures and two stories are one chunk and one group, so one
	// call carries all eight questions.
	if judge.callCount() != 1 {
		t.Fatalf("Jev calls = %d; want one call for the poll's four captures", judge.callCount())
	}
	for _, call := range judge.calls {
		if call.scope != judgmentScopeIntakeLabels {
			t.Fatalf("label drew on scope %q; want %q, its own rate bucket", call.scope, judgmentScopeIntakeLabels)
		}
		if len(call.questions) != 8 {
			t.Fatalf("questions = %d; want an event and a topic question per capture", len(call.questions))
		}
		captures, _ := call.state["captures"].(map[string]any)
		if len(captures) != 4 {
			t.Fatalf("state.captures = %+v; want the poll's four captures", captures)
		}
		for handle, v := range captures {
			capture, _ := v.(map[string]any)
			if !strings.HasPrefix(handle, "n") || capture["summary"] == "" || capture["outlet"] != "Labels Daily" && capture["outlet"] != "Test Outlet" || capture["language"] == "" || capture["script"] == "" {
				t.Fatalf("Jev's capture state %s lacks summary, outlet, language or script: %+v", handle, capture)
			}
		}
		stories := storyEntries(call.state)
		if len(stories) != 2 {
			t.Fatalf("shortlist = %+v; want both live stories, once", stories)
		}
		for n, e := range stories {
			// Each story starts with exactly its handle, then headline and time.
			if !strings.HasPrefix(e, fmt.Sprintf("c%d: ", n+1)) || !strings.Contains(e, " (20") {
				t.Fatalf("story entry %q; want it to start with its handle, then the headline and update time", e)
			}
			if strings.Contains(e, "doc-") || strings.Contains(e, "texture:") {
				t.Fatalf("story entry %q shows its doc id; choices are short handles", e)
			}
		}
		for handle := range captures {
			// B8: stories first (newest first), then the siblings, then new.
			options := questionOptions(t, call.raw["event_"+handle])
			if len(options) != 2+3+1 || options[0] != "c1" || options[1] != "c2" || options[len(options)-1] != "new" {
				t.Fatalf("event options of %s = %v; want c1, c2, the three other captures and new last", handle, options)
			}
			for _, c := range options {
				if c == handle {
					t.Fatalf("event options of %s offer the capture itself: %v", handle, options)
				}
			}
			if to := questionOptions(t, call.raw["topic_"+handle]); len(to) != 3 || to[len(to)-1] != "none" {
				t.Fatalf("topic options of %s = %v; want c1, c2 and none", handle, to)
			}
		}
	}

	label := func(n int) store.CaptureLabel {
		t.Helper()
		l, found, err := f.s.LatestCaptureLabel(ctx, f.owner, f.computer, f.captureID(n))
		if err != nil || !found {
			t.Fatalf("label of capture %d: found %v, %v", n, found, err)
		}
		return l
	}
	if l := label(0); l.Event != store.CaptureLabelEventStory || l.EventDocID != pope || l.EventHeadline != "Pope has lung surgery" || l.EventP != 0.93 {
		t.Fatalf("label 0 = %+v; want same event as the pope story (p 0.93)", l)
	}
	if l := label(1); l.Event != store.CaptureLabelEventNew || l.EventDocID != "" {
		t.Fatalf("label 1 = %+v; want new", l)
	}
	if l := label(2); l.Event != store.CaptureLabelEventSibling || l.EventCaptureID != f.captureID(1) {
		t.Fatalf("label 2 = %+v; want same event as sibling capture %s", l, f.captureID(1))
	}
	l := label(3)
	if l.Event != store.CaptureLabelEventNew || l.TopicDocID != nordStream || l.TopicHeadline != "Nord Stream trial opens in Hamburg" {
		t.Fatalf("label 3 = %+v; want new, same topic as the Nord Stream story", l)
	}
	// B10: the call's usage is split by question count; four captures asked
	// two questions each take a quarter of 685 tokens and 2.877e-05.
	if l.Version != 1 || l.Model != "typesafe/jev-1.13-20260917" || math.Abs(l.CostUSD-2.877e-05/4) > 1e-12 || l.InputTokens != 171 || l.SourceID != f.sourceID || len(l.CandidateDocIDs) != 2 {
		t.Fatalf("label 3 lost version, model, cost share, tokens, source or candidates: %+v", l)
	}

	// The arrival names the captures and carries no label.
	arrivals := f.arrivals(t)
	if len(arrivals) != 1 || len(arrivals[0].Captures.Items) != 4 {
		t.Fatalf("arrivals = %+v", arrivals)
	}
	raw, _ := json.Marshal(arrivals[0])
	if strings.Contains(string(raw), "jev") || strings.Contains(string(raw), pope) {
		t.Fatalf("the arrival record carries a label: %s", raw)
	}
}

func TestIntakeLabelsFailingOrSlowJevLeavesCapturesUnlabelledWithinBudget(t *testing.T) {
	prevCall := intakeLabelCallTimeout
	intakeLabelCallTimeout = 100 * time.Millisecond
	t.Cleanup(func() { intakeLabelCallTimeout = prevCall })
	for _, c := range []struct {
		name  string
		judge *fakeIntakeJudge
	}{
		{"failing", &fakeIntakeJudge{fail: fmt.Errorf("gateway: 429 Jev judgment rate limit exceeded")}},
		{"ignores its deadline", &fakeIntakeJudge{block: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newIntakeFixture(t, "owner-labels-"+strings.ReplaceAll(c.name, " ", "-"))
			f.enable(t)
			openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
			f.rt.SetCandidateJudge(c.judge)
			for i := 0; i < 9; i++ {
				*f.items = append(*f.items, labelFeedItem{fmt.Sprintf("Story %d", i), "Summary"})
			}
			start := time.Now()
			sched := f.poll(t)
			if took := time.Since(start); took > 3*time.Second {
				t.Fatalf("the poll waited %v on Jev; labels must never gate arrival", took)
			}
			if got := f.arrivals(t); len(got) != 1 || len(got[0].Captures.Items) != 9 {
				t.Fatalf("arrival = %+v; want all 9 captures announced unlabelled", got)
			}
			for i := 0; i < 9; i++ {
				if labels, _ := f.s.CaptureLabels(context.Background(), f.owner, f.computer, f.captureID(i)); len(labels) != 0 {
					t.Fatalf("capture %d labelled by a failing Jev: %+v", i, labels)
				}
			}
			_ = sched
		})
	}
}

// B7: twelve captures against no stories are two calls of six, and the site never
// has more than one in flight.
func TestIntakeLabelsBatchTwelveCapturesIntoTwoCalls(t *testing.T) {
	f := newIntakeFixture(t, "owner-labels-parallel")
	f.enable(t)
	judge := &fakeIntakeJudge{delay: 40 * time.Millisecond}
	f.rt.SetCandidateJudge(judge)
	for i := 0; i < 12; i++ {
		*f.items = append(*f.items, labelFeedItem{fmt.Sprintf("Item %d", i), "Summary"})
	}
	f.poll(t)
	labelCalls := judge.labelCalls()
	if got := len(labelCalls); got != 2 {
		t.Fatalf("B7: %d label calls for 12 captures; want 2 of 6", got)
	}
	if max := judge.maxSeen.Load(); max != 1 {
		t.Fatalf("B7: %d calls ran at once; want 1", max)
	}
	for _, call := range labelCalls {
		for name := range call.questions {
			if strings.HasPrefix(name, "topic_") {
				t.Fatal("a topic question was asked with no candidate stories")
			}
		}
		if len(call.questions) != 6 {
			t.Fatalf("B2: a call has %d questions; want 6 (one per capture, 6 captures)", len(call.questions))
		}
	}
}

// storiesPerChunk is how many stories one chunk holds for the test library's
// stories, measured on the poll's own cutting.
func storiesPerChunk(t *testing.T, rt *Runtime) int {
	t.Helper()
	p, err := newIntakePoll(context.Background(), pollLib(rt, 2000), pollItems("probe"))
	if err != nil || len(p.cuts) < 2 {
		t.Fatalf("probe poll: %v", err)
	}
	return p.cuts[0]
}

// pollLib is a library of n stories updated a minute apart, newest first, and
// the stories in rest for title overlap.
func pollLib(rt *Runtime, n int, rest ...types.Document) *intakeLibrary {
	lib := &intakeLibrary{rt: rt, summary: map[string]store.StorySummary{}}
	now := time.Now().UTC()
	for i := 0; i < n; i++ {
		lib.recent = append(lib.recent, types.Document{DocID: fmt.Sprintf("doc-%03d", i), Title: fmt.Sprintf("Story headline %03d", i), UpdatedAt: now.Add(-time.Duration(i) * time.Minute)})
	}
	lib.rest = rest
	return lib
}

func pollItems(titles ...string) []sourcegraph.CapturedItem {
	var items []sourcegraph.CapturedItem
	for i, title := range titles {
		items = append(items, sourcegraph.CapturedItem{CaptureID: fmt.Sprintf("cap-%02d", i), Title: title, Summary: "Summary of " + title, Outlet: "Wire", Language: "en"})
	}
	return items
}

// labelOnly labels items[0] alone against lib.
func labelOnly(rt *Runtime, lib *intakeLibrary, items []sourcegraph.CapturedItem) (store.CaptureLabel, error) {
	r := rt.labelPoll(context.Background(), lib, "owner", "computer", "src", items[:1], intakeBatchCaptures)[0]
	return r.Value, r.Err
}

func mustLabels(t *testing.T, results []intakeLabelResult) []store.CaptureLabel {
	t.Helper()
	out := make([]store.CaptureLabel, len(results))
	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("capture %d unlabelled: %v", i, r.Err)
		}
		out[i] = r.Value
	}
	return out
}

// B9, B1, B8: the batched path labels a poll exactly as one call per capture
// does, given the same answers, over a window of one chunk and of two, with
// per-capture title-overlap picks in the last chunk.
func TestIntakeBatchEquivalentToPerCapture(t *testing.T) {
	for _, stories := range []int{6, 100} {
		t.Run(fmt.Sprintf("%d stories", stories), func(t *testing.T) {
			rt, _ := testRuntime(t)
			rest := []types.Document{
				{DocID: "doc-old-1", Title: "Hamburg pipeline trial opens", UpdatedAt: time.Now().Add(-30 * time.Hour)},
				{DocID: "doc-old-2", Title: "Vatican surgery update", UpdatedAt: time.Now().Add(-31 * time.Hour)},
			}
			items := pollItems("Pipeline suspect on trial in Hamburg", "Vatican surgery went well", "Quake shakes Peru", "Story headline 002 develops", "Peru quake toll rises", "Unrelated markets note")
			answers := map[string]string{
				items[0].Title: "Hamburg pipeline trial opens",
				items[1].Title: "Vatican surgery update",
				items[2].Title: "new",
				items[3].Title: "Story headline 002",
				items[4].Title: items[2].Title, // a sibling
				items[5].Title: "new",
			}
			topics := map[string]string{items[2].Title: "Story headline 001", items[5].Title: fmt.Sprintf("Story headline %03d", stories-1)}
			run := func(batch int) ([]store.CaptureLabel, *fakeIntakeJudge) {
				judge := &fakeIntakeJudge{answers: answers, topics: topics}
				rt.SetCandidateJudge(judge)
				lib := pollLib(rt, stories, rest...)
				return mustLabels(t, rt.labelPoll(context.Background(), lib, "owner", "computer", "src", items, batch)), judge
			}
			perCapture, single := run(1)
			batched, batch := run(intakeBatchCaptures)
			if batch.callCount() >= single.callCount() {
				t.Fatalf("batching did not cut the calls: %d batched against %d per capture", batch.callCount(), single.callCount())
			}
			for i := range items {
				a, b := perCapture[i], batched[i]
				if a.CaptureID != items[i].CaptureID || b.CaptureID != items[i].CaptureID {
					t.Fatalf("B8: label %d is for %s / %s; want %s", i, a.CaptureID, b.CaptureID, items[i].CaptureID)
				}
				if a.Event != b.Event || a.EventDocID != b.EventDocID || a.EventHeadline != b.EventHeadline || a.EventCaptureID != b.EventCaptureID || a.EventP != b.EventP ||
					a.TopicDocID != b.TopicDocID || a.TopicHeadline != b.TopicHeadline || a.TopicP != b.TopicP || a.Model != b.Model ||
					strings.Join(a.CandidateDocIDs, ",") != strings.Join(b.CandidateDocIDs, ",") || strings.Join(a.SiblingCaptureIDs, ",") != strings.Join(b.SiblingCaptureIDs, ",") {
					t.Fatalf("B9: capture %d labelled differently:\n per capture %+v\n batched     %+v", i, a, b)
				}
			}
			// The answers themselves are what the fixture says (B1).
			if batched[0].EventHeadline != "Hamburg pipeline trial opens" || batched[1].EventHeadline != "Vatican surgery update" ||
				batched[3].EventHeadline != "Story headline 002" || batched[4].EventCaptureID != items[2].CaptureID ||
				batched[2].Event != store.CaptureLabelEventNew || batched[2].TopicHeadline != "Story headline 001" ||
				batched[5].TopicHeadline != fmt.Sprintf("Story headline %03d", stories-1) {
				t.Fatalf("B1: attribution is off: %+v", batched)
			}
		})
	}
}

// B2: nine captures are groups of 4, 4 and 1, and every capture is labelled
// once.
func TestIntakeBatchSplitsAtTheCaptureCapAndDropsNone(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	var titles []string
	for i := 0; i < 9; i++ {
		titles = append(titles, fmt.Sprintf("Capture title %d", i))
	}
	items := pollItems(titles...)
	labels := mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", items, 4))
	if judge.callCount() != 3 {
		t.Fatalf("calls = %d; want groups of 4, 4 and 1", judge.callCount())
	}
	asked := map[string]int{}
	for _, call := range judge.calls {
		if n := len(call.questions); n != 8 && n != 2 {
			t.Fatalf("a call has %d questions; want 8, 8 and 2", n)
		}
		for name := range call.questions {
			asked[name]++
		}
	}
	if len(asked) != 18 {
		t.Fatalf("B2: %d distinct questions asked; want an event and a topic for each of 9", len(asked))
	}
	for name, n := range asked {
		if n != 1 {
			t.Fatalf("B2: question %s asked %d times", name, n)
		}
	}
	for i, l := range labels {
		if l.CaptureID != items[i].CaptureID {
			t.Fatalf("B8: label %d is for %s", i, l.CaptureID)
		}
	}
}

// B3: with lede-heavy candidates a chunk's state is big, so four captures'
// questions do not fit under the byte line and the batch splits; no request
// is over the line and no capture is dropped. The chunk of 80 candidates is
// the state; the byte line is on state plus questions.
func TestIntakeBatchRespectsTheByteLineBySplittingTheCaptureBatch(t *testing.T) {
	rt, _ := testRuntime(t)
	lib := pollLib(rt, 80)
	for _, d := range lib.recent {
		lib.summary[d.DocID] = store.StorySummary{DocID: d.DocID, Headline: d.Title, UpdatedAt: d.UpdatedAt, Lede: strings.Repeat("lede words ", 50)}
	}
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	long := strings.Repeat("a long capture summary sentence. ", 40)
	items := pollItems("one", "two", "three", "four", "five", "six")
	for i := range items {
		items[i].Summary = long
	}
	labels := mustLabels(t, rt.labelPoll(context.Background(), lib, "owner", "computer", "src", items, 4))
	if len(labels) != len(items) {
		t.Fatalf("B2: %d labels for %d captures", len(labels), len(items))
	}
	if judge.callCount() <= 2 {
		t.Fatalf("B3: %d calls for 6 captures; the byte line should have split the groups of 4", judge.callCount())
	}
	for _, call := range judge.calls {
		if call.bytes > intakeRequestMaxBytes {
			t.Fatalf("B3: a request is %d bytes, over the %d line", call.bytes, intakeRequestMaxBytes)
		}
	}
}

// B4: a missing event answer or an off-menu choice voids that capture alone;
// the others in the same call are labelled.
func TestIntakeBatchOneBadAnswerVoidsOnlyThatCapture(t *testing.T) {
	rt, _ := testRuntime(t)
	items := pollItems("Alpha", "Bravo", "Charlie", "Delta")
	judge := &fakeIntakeJudge{answers: map[string]string{"Alpha": "Story headline 001", "Charlie": "Story headline 002"}, dropEvent: map[string]bool{"Bravo": true}, offMenu: map[string]bool{"Delta": true}}
	rt.SetCandidateJudge(judge)
	results := rt.labelPoll(context.Background(), pollLib(rt, 4), "owner", "computer", "src", items, 4)
	if judge.callCount() != 1 {
		t.Fatalf("calls = %d; want one", judge.callCount())
	}
	if results[0].Err != nil || results[0].Value.EventHeadline != "Story headline 001" || results[2].Err != nil || results[2].Value.EventHeadline != "Story headline 002" {
		t.Fatalf("B4: the good answers were voided: %+v / %+v", results[0], results[2])
	}
	if results[1].Err == nil || results[3].Err == nil {
		t.Fatalf("B4: a capture with no event answer or an off-menu one was labelled: %+v / %+v", results[1], results[3])
	}
}

// B5: a failed call leaves its own captures unlabelled and no other.
func TestIntakeBatchFailedCallLeavesOnlyItsCapturesUnlabelled(t *testing.T) {
	rt, _ := testRuntime(t)
	var titles []string
	for i := 0; i < 9; i++ {
		titles = append(titles, fmt.Sprintf("Capture title %d", i))
	}
	items := pollItems(titles...)
	// Captures 4 to 7 are the second group; its calls carry the question of
	// capture 6 (handle n6), the others' do not.
	rt.SetCandidateJudge(&failingQuestionJudge{question: "event_n6"})
	results := rt.labelPoll(context.Background(), pollLib(rt, 3), "owner", "computer", "src", items, 4)
	for i, r := range results {
		failed := i >= 4 && i <= 7
		if failed != (r.Err != nil) {
			t.Fatalf("B5: capture %d error = %v; the second group (4 to 7) alone should fail", i, r.Err)
		}
	}
}

// B6: the jev-calls log has one row per call, whose questions name every
// capture's event and topic question once and join each to its capture.
func TestIntakeBatchRecordsEveryCaptureQuestionOnceWithItsDecision(t *testing.T) {
	f := newIntakeFixture(t, "owner-labels-record")
	f.enable(t)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	f.rt.SetCandidateJudge(&fakeIntakeJudge{})
	for i := 0; i < 6; i++ {
		*f.items = append(*f.items, labelFeedItem{fmt.Sprintf("Record item %d", i), "Summary"})
	}
	f.poll(t)
	f.rt.jevRecorder().Close()
	raw, err := os.ReadFile(jevCallsFile(f.rt))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	rows := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var rec jevrecord.Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		if rec.Site != "intake_labels" {
			continue
		}
		rows++
		if !rec.OK || len(rec.Answers) != len(rec.Questions) {
			t.Fatalf("B6: row ok=%v with %d answers for %d questions", rec.OK, len(rec.Answers), len(rec.Questions))
		}
		for _, q := range rec.Questions {
			if q.DecisionID == "" {
				t.Fatalf("B6: question %s has no decision id", q.Name)
			}
			counts[q.DecisionID+"/"+strings.SplitN(q.Name, "_", 2)[0]]++
		}
	}
	if rows != 1 {
		t.Fatalf("B6: %d rows; want one call carrying all six captures", rows)
	}
	for i := 0; i < 6; i++ {
		for _, kind := range []string{"event", "topic"} {
			if key := f.captureID(i) + "/" + kind; counts[key] != 1 {
				t.Fatalf("B6: %s recorded %d times; want once", key, counts[key])
			}
		}
	}
	if len(counts) != 12 {
		t.Fatalf("B6: %d capture-questions recorded; want 12: %v", len(counts), counts)
	}
}

func TestRelabelWritesTheNextVersionAndLeavesTheArrivalDigest(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer, sourceID := "owner-relabel", rt.cfg.ComputerID, "rss:relabel"
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	pope := openTestStory(t, s, owner, computer, "Pope has lung surgery")
	captured := []sourcegraph.CapturedItem{{CaptureID: "choir.web_capture:relabel-1", Title: "Papst operiert", URL: "https://relabel.example/1", Language: "de", Summary: "Vatikan", Outlet: "DW"}}
	before, err := captureArrivalRequest(owner, computer, sourceID, captureArrivalItems(captured), false)
	if err != nil {
		t.Fatal(err)
	}
	rt.SetCandidateJudge(&fakeIntakeJudge{answers: map[string]string{"Papst operiert": "Pope has lung surgery"}})
	if labelled, _ := labelNow(rt, ctx, owner, computer, sourceID, captured); labelled != 1 {
		t.Fatalf("first label: labelled %d", labelled)
	}
	rt.SetCandidateJudge(&fakeIntakeJudge{answers: map[string]string{"Papst operiert": "new"}, model: "typesafe/jev-1.14"})
	if labelled, _ := labelNow(rt, ctx, owner, computer, sourceID, captured); labelled != 1 {
		t.Fatalf("relabel: labelled %d", labelled)
	}
	labels, err := s.CaptureLabels(ctx, owner, computer, "choir.web_capture:relabel-1")
	if err != nil || len(labels) != 2 {
		t.Fatalf("labels = %+v, %v; want two versions", labels, err)
	}
	if labels[0].Version != 1 || labels[0].EventDocID != pope || labels[1].Version != 2 || labels[1].Event != store.CaptureLabelEventNew || labels[1].Model != "typesafe/jev-1.14" {
		t.Fatalf("versions = %+v; want v1 kept as written and v2 the relabel", labels)
	}
	after, err := captureArrivalRequest(owner, computer, sourceID, captureArrivalItems(captured), false)
	if err != nil {
		t.Fatal(err)
	}
	if before.CommandDigest != after.CommandDigest {
		t.Fatalf("a relabel changed the arrival digest: %s -> %s", before.CommandDigest, after.CommandDigest)
	}
}

func TestIntakeShortlistTakesTheWindowThenTenByTitleOverlap(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	var docs []types.Document
	for i := 0; i < 50; i++ {
		docs = append(docs, types.Document{DocID: fmt.Sprintf("doc-%02d", i), Title: fmt.Sprintf("Unrelated story %d", i), UpdatedAt: now.Add(-time.Duration(i) * 15 * time.Minute)})
	}
	docs[45].Title = "Nord Stream pipeline trial"
	docs[44].Title = "Hamburg court hears pipeline case"
	docs = append(docs, types.Document{DocID: "doc-old", Title: "Nord Stream pipeline sabotage", UpdatedAt: now.Add(-100 * time.Hour)})
	recent, rest := intakeShortlistSplit(docs, now)
	// D2: a window of time (8 hours: stories 0 to 32 at 15 minutes apart), not a count.
	if len(recent) != 33 || recent[0].DocID != "doc-00" || recent[32].DocID != "doc-32" {
		t.Fatalf("recent = %d, first %s; want the 33 updated within 8 hours", len(recent), recent[0].DocID)
	}
	for _, d := range rest {
		if d.DocID == "doc-old" {
			t.Fatal("a story past the 72-hour horizon is a candidate")
		}
	}
	picked := intakeOverlapPicks(rest, "Ukrainian on trial over Nord Stream pipeline", 10)
	if len(picked) < 2 || picked[0].DocID != "doc-45" || picked[1].DocID != "doc-44" {
		t.Fatalf("overlap picks = %+v; want the Nord Stream trial first", picked)
	}
	for _, d := range picked {
		if !strings.Contains(strings.ToLower(d.Title), "pipeline") && !strings.Contains(strings.ToLower(d.Title), "trial") {
			t.Fatalf("overlap picked %q, which shares no title token", d.Title)
		}
	}
	// A busy window is not cut to a count: 300 stories in the window all stay.
	var busy []types.Document
	for i := 0; i < 700; i++ {
		busy = append(busy, types.Document{DocID: fmt.Sprintf("busy-%03d", i), Title: "x", UpdatedAt: now.Add(-time.Duration(i) * time.Second)})
	}
	if recent, rest := intakeShortlistSplit(busy[:300], now); len(recent) != 300 || len(rest) != 0 {
		t.Fatalf("a 300-story window = %d recent, %d rest; want all 300 kept", len(recent), len(rest))
	}
	// A quiet stretch keeps the floor: 40 stories, all a day old, still give
	// the newest 30 (the old reach), and a story past the 72-hour horizon none.
	var quiet []types.Document
	for i := 0; i < 40; i++ {
		quiet = append(quiet, types.Document{DocID: fmt.Sprintf("quiet-%02d", i), Title: "x", UpdatedAt: now.Add(-24*time.Hour - time.Duration(i)*time.Minute)})
	}
	quiet = append(quiet, types.Document{DocID: "quiet-ancient", Title: "x", UpdatedAt: now.Add(-100 * time.Hour)})
	if recent, rest := intakeShortlistSplit(quiet, now); len(recent) != intakeShortlistFloor || len(rest) != 10 {
		t.Fatalf("a quiet stretch = %d recent, %d rest; want the newest %d and 10 for overlap", len(recent), len(rest), intakeShortlistFloor)
	}
	if recent, _ := intakeShortlistSplit(busy, now); len(recent) != intakeShortlistCeiling {
		t.Fatalf("a 700-story window = %d; want the ceiling %d", len(recent), intakeShortlistCeiling)
	}
}

// failingChunkJudge fails the call whose state includes the marker.
type failingChunkJudge struct {
	fakeIntakeJudge
	marker string
}

func (j *failingChunkJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	if strings.Contains(string(state), j.marker) {
		return nil, fmt.Errorf("simulated timeout")
	}
	return j.fakeIntakeJudge.JudgeScoped(ctx, scope, state, questions)
}

// failingQuestionJudge fails the call that carries the question.
type failingQuestionJudge struct {
	fakeIntakeJudge
	question string
}

func (j *failingQuestionJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	if _, ok := questions[j.question]; ok {
		return nil, fmt.Errorf("simulated timeout")
	}
	return j.fakeIntakeJudge.JudgeScoped(ctx, scope, state, questions)
}

// A window longer than one call carries is asked in chunks (D2). Ways it can
// fail: C1 the list is cut to one call's worth, so a story past the cut is
// invisible; C2 aliases collide across chunks and map to the wrong story; C3
// the chunks are asked one after another; C4 a chunk that failed is
// labelled "new" on part of the evidence; C5 a story in a later chunk loses
// to the "new" answers of the others.
func TestIntakeLabelsAskALongWindowInChunksAndKeepEveryCandidate(t *testing.T) {
	rt, _ := testRuntime(t)
	per := storiesPerChunk(t, rt)
	n := 2*per + 20
	now := time.Now().UTC()
	lib := &intakeLibrary{rt: rt, summary: map[string]store.StorySummary{}}
	for i := 0; i < n; i++ {
		d := types.Document{DocID: fmt.Sprintf("doc-%03d", i), Title: fmt.Sprintf("Story headline %03d", i), UpdatedAt: now.Add(-time.Duration(i) * time.Minute)}
		lib.recent = append(lib.recent, d)
	}
	target := fmt.Sprintf("Story headline %03d", n-3) // in the last chunk
	judge := &failingChunkJudge{fakeIntakeJudge: fakeIntakeJudge{answers: map[string]string{"Incoming report": target}, delay: 30 * time.Millisecond}, marker: "never"}
	rt.SetCandidateJudge(judge)
	items := []sourcegraph.CapturedItem{{CaptureID: "cap-chunk", Title: "Incoming report"}}
	label, err := labelOnly(rt, lib, items)
	if err != nil {
		t.Fatal(err)
	}
	if got := judge.callCount(); got != 3 {
		t.Fatalf("C1: %d calls for %d candidates; want 3 chunks of at most %d", got, n, per)
	}
	if len(label.CandidateDocIDs) != n {
		t.Fatalf("C1: label lists %d candidates; want all %d", len(label.CandidateDocIDs), n)
	}
	if label.Event != store.CaptureLabelEventStory || label.EventHeadline != target || label.EventDocID != fmt.Sprintf("doc-%03d", n-3) {
		t.Fatalf("C2/C5: label = %+v; want the story in the last chunk", label)
	}
	seen := map[string]bool{}
	for _, c := range judge.calls {
		stories := storyEntries(c.state)
		if len(stories) > per {
			t.Errorf("a call listed %d stories", len(stories))
		}
		for _, e := range stories {
			a, _, _ := strings.Cut(e, ": ")
			if seen[a] {
				t.Errorf("C2: alias %s is in two chunks", a)
			}
			seen[a] = true
		}
	}
	if judge.maxSeen.Load() != 1 {
		t.Errorf("C3: chunks ran %d at a time; want one after another", judge.maxSeen.Load())
	}

	// No chunk matches: new.
	rt.SetCandidateJudge(&fakeIntakeJudge{answers: map[string]string{}})
	if label, err = labelOnly(rt, lib, items); err != nil || label.Event != store.CaptureLabelEventNew {
		t.Fatalf("no match: %+v, %v", label, err)
	}

	// C4: a failed chunk leaves the capture unlabelled.
	rt.SetCandidateJudge(&failingChunkJudge{fakeIntakeJudge: fakeIntakeJudge{answers: map[string]string{}}, marker: fmt.Sprintf("Story headline %03d", n-1)})
	if _, err = labelOnly(rt, lib, items); err == nil {
		t.Fatal("C4: a capture is labelled on part of the window after a chunk failed")
	}
}

// B11: Jev reads a choice question's option set from the KEYS of its criteria
// (each key an option, each value its description or null); a choices field is
// not in the vendor's schema. B12: an option is a bare handle, so the state
// must list each story under exactly that handle at the start of its entry and
// the question must say the options are those handles. Ways this fails: an
// option the capture is offered is missing from criteria (the model cannot
// choose it, and every capture comes out new or none, silently); criteria
// carries an option the capture was not offered; a handle has a description
// that costs bytes (null is read by name alone); the option order changes;
// the story entry does not start with its handle.
func TestIntakeBatchQuestionsOfferExactlyTheCriteriaKeys(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	items := pollItems("Alpha report", "Bravo report", "Charlie report")
	per := storiesPerChunk(t, rt)
	rt.labelPoll(context.Background(), pollLib(rt, per+20), "owner", "computer", "src", items, intakeBatchCaptures)
	calls := judge.labelCalls()
	if len(calls) != 2 {
		t.Fatalf("calls = %d; want one per chunk", len(calls))
	}
	// The calls arrive in either order (the pool runs two); put chunk 0 first.
	if !strings.HasPrefix(storyEntries(calls[0].state)[0], "c1: ") {
		calls[0], calls[1] = calls[1], calls[0]
	}
	for k, call := range calls {
		var stories []string
		for n, e := range storyEntries(call.state) {
			h, _, ok := strings.Cut(e, ": ")
			if !ok || h != fmt.Sprintf("c%d", k*per+n+1) {
				t.Fatalf("B12: story entry %q does not start with its own handle", e)
			}
			stories = append(stories, h)
		}
		captures, _ := call.state["captures"].(map[string]any)
		for name, raw := range call.raw {
			kind, handle, _ := strings.Cut(name, "_")
			if captures[handle] == nil {
				t.Fatalf("question %s names %s, which is not in state.captures", name, handle)
			}
			want := append([]string(nil), stories...)
			if kind == "event" {
				if k == 0 { // the siblings ride the first chunk only
					for _, other := range []string{"n1", "n2", "n3"} {
						if other != handle {
							want = append(want, other)
						}
					}
				}
				want = append(want, "new")
			} else {
				want = append(want, "none")
			}
			got := questionOptions(t, raw)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("B11: %s criteria keys = %v; want the offered options %v in order", name, got, want)
			}
			var shape struct {
				Instructions string                      `json:"instructions"`
				Criteria     map[string]*json.RawMessage `json:"criteria"`
			}
			if err := json.Unmarshal(raw, &shape); err != nil {
				t.Fatal(err)
			}
			for _, option := range got {
				described := shape.Criteria[option] != nil && string(*shape.Criteria[option]) != "null"
				if described != (option == "new" || option == "none") {
					t.Fatalf("B11: %s option %s description present = %v; handles are null, new and none are described", name, option, described)
				}
			}
			if !strings.Contains(shape.Instructions, "handle") || !strings.Contains(shape.Instructions, "state.stories") {
				t.Fatalf("B12: %s instructions do not say the options are the handles listed in state: %q", name, shape.Instructions)
			}
		}
	}
}

// B13: the first option is the one Jev leans toward, so each question's
// option order is its own capture's, as the per-capture path offered it: the
// window newest first, then that capture's own overlap picks in its own score
// order (not the poll-wide first-seen order the shared state lists them in),
// then the siblings, then new / none.
func TestIntakeBatchOptionOrderIsEachCapturesOwn(t *testing.T) {
	rt, _ := testRuntime(t)
	rest := []types.Document{
		{DocID: "r1", Title: "Hamburg pipeline trial opens"},
		{DocID: "r2", Title: "Pipeline sabotage inquiry widens"},
		{DocID: "r3", Title: "Verdict appeal filed in Hamburg"},
		{DocID: "r4", Title: "Hamburg court verdict due"},
	}
	lib := pollLib(rt, 3, rest...)
	items := pollItems("Hamburg pipeline trial verdict", "Pipeline sabotage inquiry")
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	mustLabels(t, rt.labelPoll(context.Background(), lib, "owner", "computer", "src", items, intakeBatchCaptures))
	calls := judge.labelCalls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d; want one", len(calls))
	}
	handleOf := func(title string) string {
		for _, e := range storyEntries(calls[0].state) {
			if h, rest, _ := strings.Cut(e, ": "); strings.HasPrefix(rest, title+" (") {
				return h
			}
		}
		t.Fatalf("no story %q in the state", title)
		return ""
	}
	// What the per-capture path offered each capture: its own helper call.
	expect := func(i int) []string {
		var out []string
		for _, d := range lib.recent {
			out = append(out, handleOf(d.Title))
		}
		for _, d := range intakeOverlapPicks(lib.rest, items[i].Title, intakeShortlistOverlap) {
			out = append(out, handleOf(d.Title))
		}
		return out
	}
	a, b := expect(0), expect(1)
	if strings.Join(a, ",") == strings.Join(b, ",") || len(a) == len(b) {
		t.Fatalf("fixture: the two captures' picks must differ and overlap partly: %v / %v", a, b)
	}
	for i, sibling := range []string{"n2", "n1"} {
		handle := fmt.Sprintf("n%d", i+1)
		stories := expect(i)
		wantEvent := append(append(append([]string(nil), stories...), sibling), "new")
		wantTopic := append(append([]string(nil), stories...), "none")
		if got := questionOptions(t, calls[0].raw["event_"+handle]); strings.Join(got, ",") != strings.Join(wantEvent, ",") {
			t.Fatalf("B13: event_%s options %v; want its own order %v", handle, got, wantEvent)
		}
		if got := questionOptions(t, calls[0].raw["topic_"+handle]); strings.Join(got, ",") != strings.Join(wantTopic, ",") {
			t.Fatalf("B13: topic_%s options %v; want its own order %v", handle, got, wantTopic)
		}
	}
}

// B14: a capture whose answer is voided gets no label, so its share of the
// call's cost is on no label; the call's tape record (the jev-calls row)
// carries the whole call's cost and tokens whatever the captures' fates.
func TestIntakeBatchVoidedCaptureCostStaysOnTheCallRecord(t *testing.T) {
	f := newIntakeFixture(t, "owner-labels-void-cost")
	f.enable(t)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	f.rt.SetCandidateJudge(&fakeIntakeJudge{dropEvent: map[string]bool{"Void item 1": true}})
	for i := 0; i < 4; i++ {
		*f.items = append(*f.items, labelFeedItem{fmt.Sprintf("Void item %d", i), "Summary"})
	}
	f.poll(t)
	labelled := 0
	for i := 0; i < 4; i++ {
		if _, found, _ := f.s.LatestCaptureLabel(context.Background(), f.owner, f.computer, f.captureID(i)); found {
			labelled++
		}
	}
	if labelled != 3 {
		t.Fatalf("%d captures labelled; want 3 labelled and 1 voided", labelled)
	}
	var shares float64
	for i := 0; i < 4; i++ {
		if l, found, _ := f.s.LatestCaptureLabel(context.Background(), f.owner, f.computer, f.captureID(i)); found {
			shares += l.CostUSD
		}
	}
	// Four captures asked two questions each: three labelled take 3/4.
	if math.Abs(shares-2.877e-05*3/4) > 1e-12 {
		t.Fatalf("B14: labels carry %g of the cost; want three quarters of 2.877e-05", shares)
	}
	f.rt.jevRecorder().Close()
	raw, err := os.ReadFile(jevCallsFile(f.rt))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var rec jevrecord.Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		if rec.Site == "intake_labels" && (math.Abs(rec.CostUSD-2.877e-05) > 1e-12 || rec.InputTokens != 685) {
			t.Fatalf("B14: the call record carries cost %g and %d tokens; want the whole call's", rec.CostUSD, rec.InputTokens)
		}
	}
}

// B15: one line per poll with counts only, so a slow-call collapse shows even
// when the poll budget has ended and the per-capture lines are suppressed.
func TestIntakeLabelsLogOneLinePerPoll(t *testing.T) {
	var buf strings.Builder
	var mu sync.Mutex
	log.SetOutput(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(p) }))
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	lines := func() []string {
		mu.Lock()
		defer mu.Unlock()
		var out []string
		for _, l := range strings.Split(buf.String(), "\n") {
			if strings.Contains(l, "intake labels:") && strings.Contains(l, " flush ") {
				out = append(out, l)
			}
		}
		return out
	}
	f := newIntakeFixture(t, "owner-labels-pollline")
	f.enable(t)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	f.rt.SetCandidateJudge(&fakeIntakeJudge{})
	for i := 0; i < 5; i++ {
		*f.items = append(*f.items, labelFeedItem{fmt.Sprintf("Line item %d", i), "Summary"})
	}
	f.poll(t)
	got := lines()
	if len(got) != 1 {
		t.Fatalf("B15: %d poll lines; want one: %v", len(got), got)
	}
	for _, want := range []string{"captures=5", "calls=1", "questions=10", "unlabelled=0", "cancelled=false", "bytes_max="} {
		if !strings.Contains(got[0], want) {
			t.Fatalf("B15: poll line %q lacks %s", got[0], want)
		}
	}

	// A slow Jev that ends the budget still leaves the line.
	prevCall := intakeLabelCallTimeout
	intakeLabelCallTimeout = 100 * time.Millisecond
	t.Cleanup(func() { intakeLabelCallTimeout = prevCall })
	g := newIntakeFixture(t, "owner-labels-pollline-slow")
	g.enable(t)
	openTestStory(t, g.s, g.owner, g.computer, "Pope has lung surgery")
	g.rt.SetCandidateJudge(&fakeIntakeJudge{block: true})
	for i := 0; i < 9; i++ {
		*g.items = append(*g.items, labelFeedItem{fmt.Sprintf("Slow item %d", i), "Summary"})
	}
	g.poll(t)
	got = lines()
	if len(got) != 2 || !strings.Contains(got[1], "captures=9") || !strings.Contains(got[1], "unlabelled=9") {
		t.Fatalf("B15: slow poll line = %v; want 9 unlabelled", got)
	}
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

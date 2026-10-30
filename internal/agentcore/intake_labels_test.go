package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
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
}

type fakeIntakeCall struct {
	scope     string
	state     map[string]any
	questions map[string]map[string]any
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
	for name, q := range questions {
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
	capture, _ := call.state["capture"].(map[string]any)
	title, _ := capture["title"].(string)
	alias := func(want string) string {
		if want == "" {
			return ""
		}
		if stories, ok := call.state["stories"].(map[string]any); ok {
			for a, v := range stories {
				if m, _ := v.(map[string]any); m["headline"] == want {
					return a
				}
			}
		}
		if sibs, ok := call.state["new_captures"].(map[string]any); ok {
			for a, v := range sibs {
				if m, _ := v.(map[string]any); m["title"] == want {
					return a
				}
			}
		}
		return ""
	}
	event := "new"
	if a := alias(j.answers[title]); a != "" {
		event = a
	}
	answers := map[string]any{"event": map[string]any{"choice": event, "probabilities": map[string]float64{event: 0.93}, "confidence": 0.9}}
	if _, asked := questions["topic"]; asked {
		topic := "none"
		if a := alias(j.topics[title]); a != "" {
			topic = a
		}
		answers["topic"] = map[string]any{"choice": topic, "probabilities": map[string]float64{topic: 0.81}}
	}
	return json.Marshal(map[string]any{"model": firstNonEmpty(j.model, "typesafe/jev-1.13-20260917"), "answers": answers,
		"usage": map[string]any{"input_tokens": 685, "output_tokens": 47, "cost": 2.877e-05}})
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
		Inbox:     types.StoryOpenInbox, SenderInbox: management, EmitterKind: types.CommitmentEmitterAgent,
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
	judge := &fakeIntakeJudge{}
	f.rt.SetCandidateJudge(judge)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	*f.items = []labelFeedItem{{"Papst an der Lunge operiert", "Der Vatikan meldet eine erfolgreiche Operation."}}
	sched := f.poll(t)
	if judge.callCount() != 0 {
		t.Fatalf("intake.labels is off but Jev was called %d times", judge.callCount())
	}
	if got := f.arrivals(t); len(got) != 1 || got[0].Captures == nil || len(got[0].Captures.Items) != 1 {
		t.Fatalf("arrivals = %+v; want one arrival", got)
	}
	if labels, err := f.s.CaptureLabels(context.Background(), f.owner, f.computer, f.captureID(0)); err != nil || len(labels) != 0 {
		t.Fatalf("labels with the setting off = %+v, %v", labels, err)
	}
	if last := sched.Recent[len(sched.Recent)-1]; last.Labelled != 0 || last.Unlabelled != 0 {
		t.Fatalf("usage counts labels with the setting off: %+v", last)
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
	sched := f.poll(t)

	if judge.callCount() != 4 {
		t.Fatalf("Jev calls = %d; want one per new capture", judge.callCount())
	}
	for _, call := range judge.calls {
		if call.scope != judgmentScopeIntakeLabels {
			t.Fatalf("label drew on scope %q; want %q, its own rate bucket", call.scope, judgmentScopeIntakeLabels)
		}
		capture, _ := call.state["capture"].(map[string]any)
		if capture["summary"] == "" || capture["outlet"] != "Labels Daily" && capture["outlet"] != "Test Outlet" || capture["language"] != "en" || capture["script"] == "" {
			t.Fatalf("Jev's capture state lacks summary, outlet, language or script: %+v", capture)
		}
		stories, _ := call.state["stories"].(map[string]any)
		if len(stories) != 2 {
			t.Fatalf("shortlist = %+v; want both live stories", stories)
		}
		for alias, v := range stories {
			m, _ := v.(map[string]any)
			if !strings.HasPrefix(alias, "c") || m["headline"] == "" || m["updated_at"] == "" {
				t.Fatalf("story entry %s = %+v; want a short alias with headline and update time", alias, m)
			}
			if _, ok := m["doc_id"]; ok {
				t.Fatalf("story entry %s shows its doc id; choices are short aliases", alias)
			}
		}
		if sibs, _ := call.state["new_captures"].(map[string]any); len(sibs) != 3 {
			t.Fatalf("siblings = %+v; want the poll's other three new captures", sibs)
		}
		event := call.questions["event"]
		choices, _ := event["choices"].([]any)
		if len(choices) != 2+3+1 || choices[len(choices)-1] != "new" {
			t.Fatalf("event choices = %v; want c1, c2, n1-n3 and new", choices)
		}
		topic := call.questions["topic"]
		if tc, _ := topic["choices"].([]any); len(tc) != 3 || tc[len(tc)-1] != "none" {
			t.Fatalf("topic choices = %v; want c1, c2 and none", tc)
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
	if l.Version != 1 || l.Model != "typesafe/jev-1.13-20260917" || l.CostUSD != 2.877e-05 || l.InputTokens != 685 || l.SourceID != f.sourceID || len(l.CandidateDocIDs) != 2 {
		t.Fatalf("label 3 lost version, model, cost, tokens, source or candidates: %+v", l)
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
	if last := sched.Recent[len(sched.Recent)-1]; last.Labelled != 4 || last.Unlabelled != 0 {
		t.Fatalf("poll usage = %+v; want 4 labelled", last)
	}
}

func TestIntakeLabelsFailingOrSlowJevLeavesCapturesUnlabelledWithinBudget(t *testing.T) {
	prevCall, prevPoll := intakeLabelCallTimeout, intakeLabelPollBudget
	intakeLabelCallTimeout, intakeLabelPollBudget = 100*time.Millisecond, 300*time.Millisecond
	t.Cleanup(func() { intakeLabelCallTimeout, intakeLabelPollBudget = prevCall, prevPoll })
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
			if last := sched.Recent[len(sched.Recent)-1]; last.Labelled != 0 || last.Unlabelled != 9 {
				t.Fatalf("poll usage = %+v; want 9 unlabelled", last)
			}
		})
	}
}

func TestIntakeLabelsRunAtMostFourAtOnce(t *testing.T) {
	f := newIntakeFixture(t, "owner-labels-parallel")
	f.enable(t)
	judge := &fakeIntakeJudge{delay: 40 * time.Millisecond}
	f.rt.SetCandidateJudge(judge)
	for i := 0; i < 12; i++ {
		*f.items = append(*f.items, labelFeedItem{fmt.Sprintf("Item %d", i), "Summary"})
	}
	start := time.Now()
	sched := f.poll(t)
	if max := judge.maxSeen.Load(); max > 4 || max < 2 {
		t.Fatalf("at most %d calls ran at once; want parallel, at most 4", max)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("12 labels took %v", took)
	}
	if last := sched.Recent[len(sched.Recent)-1]; last.Labelled != 12 {
		t.Fatalf("poll usage = %+v; want 12 labelled", last)
	}
	// No stories: the capture is compared with its siblings only, and no
	// topic question is asked.
	if _, asked := judge.calls[0].questions["topic"]; asked {
		t.Fatal("a topic question was asked with no candidate stories")
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
	if labelled, _ := rt.labelCaptures(ctx, owner, computer, sourceID, captured); labelled != 1 {
		t.Fatalf("first label: labelled %d", labelled)
	}
	rt.SetCandidateJudge(&fakeIntakeJudge{answers: map[string]string{"Papst operiert": "new"}, model: "typesafe/jev-1.14"})
	if labelled, _ := rt.labelCaptures(ctx, owner, computer, sourceID, captured); labelled != 1 {
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

func TestIntakeShortlistTakesTheNewestThirtyThenTenByTitleOverlap(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	var docs []types.Document
	for i := 0; i < 50; i++ {
		docs = append(docs, types.Document{DocID: fmt.Sprintf("doc-%02d", i), Title: fmt.Sprintf("Unrelated story %d", i), UpdatedAt: now.Add(-time.Duration(i) * time.Hour)})
	}
	docs[45].Title = "Nord Stream pipeline trial"
	docs[44].Title = "Hamburg court hears pipeline case"
	docs = append(docs, types.Document{DocID: "doc-old", Title: "Nord Stream pipeline sabotage", UpdatedAt: now.Add(-100 * time.Hour)})
	recent, rest := intakeShortlistSplit(docs, now)
	if len(recent) != 30 || recent[0].DocID != "doc-00" || recent[29].DocID != "doc-29" {
		t.Fatalf("recent = %d, first %s; want the 30 newest", len(recent), recent[0].DocID)
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
}

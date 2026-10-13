package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Living stories (Cut 2 2b). Ways it can fail, written before the code:
//   - a poll that wrote new captures tells management nothing, or tells it
//     once per capture (no batching by the cursor), or its record is a
//     system record that wakes no one;
//   - a poll that wrote nothing new (a repeat, a 304) still appends a
//     record and wakes management for nothing;
//   - management's Cast to texture:new is refused by the route, lands under
//     "texture:new", or its receipt does not name the story it opened, so
//     management cannot feed it;
//   - a retried opening cell mints a second story;
//   - the arrival record's body is lost on the way into the cell.

// arrivalSink writes a capture per new URL and repeats a URL it has seen.
type arrivalSink struct {
	mu   sync.Mutex
	seen map[string]string
}

func (s *arrivalSink) WriteCaptures(_ context.Context, _, _ string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out sourcegraph.CaptureWriteResult
	for _, item := range items {
		if _, ok := s.seen[item.URL]; ok {
			out.Repeated++
			continue
		}
		id := fmt.Sprintf("choir.web_capture:capture-%d", len(s.seen)+1)
		s.seen[item.URL] = id
		out.New++
		out.CaptureIDs = append(out.CaptureIDs, id)
		out.Captured = append(out.Captured, sourcegraph.CapturedItem{CaptureID: id, Title: item.Title, URL: item.URL})
	}
	return out, nil
}

func (s *arrivalSink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}

func (s *arrivalSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}

func TestPollWithNewCapturesAppendsOneArrivalRecordAndWakesManagement(t *testing.T) {
	ctx := context.Background()
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() { sourcefetch.SetAllowPrivateNetworkForTests(prev) })
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Port</title>`+
			`<item><title>Harbor reopens</title><link>https://port.example/1</link><description>Body one</description><guid>g-1</guid></item>`+
			`<item><title>Ferries resume</title><link>https://port.example/2</link><description>Body two</description><guid>g-2</guid></item>`+
			`</channel></rss>`)
	}))
	t.Cleanup(feed.Close)

	rt, s := testRuntime(t)
	owner, computer, sourceID := "owner-arrival", rt.cfg.ComputerID, "rss:port"
	management := agentprofile.Management + ":" + owner
	sink := &arrivalSink{seen: map[string]string{}}
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: sourceID, Type: sources.SourceTypeRSS, Name: "Port", URL: feed.URL}}}
	rt.researchDeps = &researchtools.Dependencies{Sources: reg, Captures: sink}
	var wakes []armedWake
	rt.SetScheduleActor(func(_ context.Context, _, _, toAgentID, kind, content, _, _ string, notBefore time.Time) error {
		wakes = append(wakes, armedWake{mailbox: toAgentID, kind: kind, content: content, notBefore: notBefore})
		return nil
	})
	if _, err := rt.StartSourceSchedule(ctx, owner, computer, sourceID, 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	if err := rt.HandleSourcePollDue(ctx, owner, computer, wakes[0].content); err != nil {
		t.Fatal(err)
	}
	records, err := s.ListInboxRecords(ctx, owner, computer, management, 0, 0)
	if err != nil || len(records) != 1 {
		t.Fatalf("management inbox after a poll with two new captures = %+v, %v; want one record", records, err)
	}
	arrival := records[0]
	if arrival.EmitterKind != types.CommitmentEmitterSystem || arrival.Captures == nil || arrival.Captures.SourceID != sourceID ||
		len(arrival.Captures.Items) != 2 || arrival.SenderInbox != types.CaptureArrivalSenderInbox(sourceID) {
		t.Fatalf("arrival record = %+v", arrival)
	}
	got := map[string]string{}
	for _, item := range arrival.Captures.Items {
		got[item.URL] = item.CaptureID + "|" + item.Title
	}
	if got["https://port.example/1"] != sink.seen["https://port.example/1"]+"|Harbor reopens" || got["https://port.example/2"] != sink.seen["https://port.example/2"]+"|Ferries resume" {
		t.Fatalf("arrival items = %+v; want each capture's id, title and URL", arrival.Captures.Items)
	}
	pending, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	woke := 0
	for _, w := range pending {
		if w.TargetAgentID == management && w.Kind == store.InboxRecordWakeKind {
			woke++
		}
	}
	if woke != 1 {
		t.Fatalf("management wakes = %d (%+v); want the arrival's one wake", woke, pending)
	}

	// The next poll writes nothing new: no record, no wake.
	if err := rt.HandleSourcePollDue(ctx, owner, computer, wakes[len(wakes)-1].content); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.ListInboxRecords(ctx, owner, computer, management, 0, 0); len(again) != 1 {
		t.Fatalf("a poll with no new captures appended a record: %+v", again)
	}
	sched, _ := s.GetSourceSchedule(ctx, owner, sourceID)
	if sched.Totals.Polls != 2 {
		t.Fatalf("schedule ran %d polls, want 2", sched.Totals.Polls)
	}
}

var castReceiptRE = regexp.MustCompile(`rlm:cast:\d+ to (texture:[0-9a-f-]+)`)

func TestManagementCastToTextureNewOpensOneStory(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-open", rt.cfg.ComputerID
	management := agentprofile.Management + ":" + owner

	arrival := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		RecordID:    types.CaptureArrivalRecordID("rss:port", []string{"capture-1"}),
		Provenance:  types.CommitmentProvenance{AgentID: types.CaptureArrivalSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, Inbox: management, SenderInbox: types.CaptureArrivalSenderInbox("rss:port"),
		Captures: &types.CommitmentCaptures{SourceID: "rss:port", Items: []types.CaptureArrival{{CaptureID: "capture-1", Title: "Harbor reopens", URL: "https://port.example/1"}}},
	}
	seed := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: arrival.RecordID, Records: []types.CommitmentRecord{arrival}}
	seed.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(seed)
	if _, err := s.CommitInboxRecords(ctx, seed); err != nil {
		t.Fatalf("seed arrival: %v", err)
	}

	rec := types.RunRecord{RunID: "run-mgmt-open", AgentID: management, OwnerID: owner, ComputerID: computer, ChannelID: management,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunRunning,
		Metadata: map[string]any{"request_source": inboxRecordRequestSource}}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: management, ComputerID: computer,
		Profile: agentprofile.Management, Role: agentprofile.Management, RunRecord: &rec, ToolCallID: "call-open"}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Management)
	cell := func(src string) string {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(toolregistry.WithExecutionContext(ctx, execCtx), payload)
		if err != nil {
			out += err.Error()
		}
		return out
	}
	// The cell collects capture ids with a loop: a slice literal whose
	// element indexes a choir.Records() result trips a yaegi compile panic
	// (see the management prompt's example, which uses the same loop).
	open := `rs := choir.Records()
fmt.Println(len(rs), rs[0].Kind, rs[0].Captures.SourceID, rs[0].Captures.Items[0].CaptureID)
var ids []string
for _, c := range rs[0].Captures.Items {
	ids = append(ids, c.CaptureID)
}
choir.Cast("texture:new", "Harbor reopens after the storm", map[string]interface{}{
	"hypotheses": []interface{}{map[string]interface{}{"statement": "the harbor reopens Monday", "probability": 0.7}},
	"captures":   ids,
	"publish":    "auto",
})`
	out := cell(open)
	if !strings.Contains(out, "1 captures rss:port capture-1") {
		t.Fatalf("management did not read the arrival: %s", out)
	}
	m := castReceiptRE.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("the Cast's receipt does not name the story it opened: %s", out)
	}
	story := m[1]
	docID := strings.TrimPrefix(story, "texture:")
	records, err := s.ListInboxRecords(ctx, owner, computer, story, 0, 0)
	if err != nil || len(records) != 1 || records[0].Cast == nil || records[0].Cast.Publish != types.CastPublishAuto ||
		len(records[0].Cast.Captures) != 1 || records[0].Cast.Captures[0] != "capture-1" || records[0].SenderInbox != management {
		t.Fatalf("story inbox = %+v, %v", records, err)
	}
	doc, err := s.GetLifecycleDocument(ctx, owner, computer, docID)
	if err != nil || doc.Title != "Harbor reopens after the storm" {
		t.Fatalf("story document = %+v, %v", doc, err)
	}

	// The same opening retried under the same tool call commits no second
	// story: its record id is the first one's, and the store keys the story
	// on the command.
	retry := cell(`choir.Cast("texture:new", "Harbor reopens after the storm", map[string]interface{}{
	"hypotheses": []interface{}{map[string]interface{}{"statement": "the harbor reopens Monday", "probability": 0.7}},
	"captures":   []string{"capture-1"},
	"publish":    "auto",
})`)
	if m := castReceiptRE.FindStringSubmatch(retry); m != nil && m[1] != story {
		t.Fatalf("a retried opening opened %s beside %s", m[1], story)
	}
	docs, err := s.ListDocumentsByScope(ctx, owner, computer, 100)
	if err != nil {
		t.Fatal(err)
	}
	if stories := len(docs); stories != 1 {
		t.Fatalf("documents after a retried opening = %d, want 1", stories)
	}
}

// Management declines captures that open no story and feed none with a Note
// to the source that sent them. Ways it can fail: the Note is refused (no
// route to a source), it wakes the source's poll mailbox, or any other act
// to a source (a Cast, opening a round nobody closes) is accepted.
func TestManagementDeclinesCapturesWithANoteToTheSource(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-decline", rt.cfg.ComputerID
	management := agentprofile.Management + ":" + owner
	source := types.CaptureArrivalSenderInbox("rss:port")
	arrival := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		RecordID:    types.CaptureArrivalRecordID("rss:port", []string{"capture-9"}),
		Provenance:  types.CommitmentProvenance{AgentID: types.CaptureArrivalSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, Inbox: management, SenderInbox: source,
		Captures: &types.CommitmentCaptures{SourceID: "rss:port", Items: []types.CaptureArrival{{CaptureID: "capture-9", Title: "Port newsletter signup", URL: "https://port.example/signup"}}},
	}
	seed := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: arrival.RecordID, Records: []types.CommitmentRecord{arrival}}
	seed.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(seed)
	if _, err := s.CommitInboxRecords(ctx, seed); err != nil {
		t.Fatalf("seed arrival: %v", err)
	}
	before, _ := s.ListUnprojectedActorWakes(ctx)
	rec := types.RunRecord{RunID: "run-mgmt-decline", AgentID: management, OwnerID: owner, ComputerID: computer, ChannelID: management,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunRunning,
		Metadata: map[string]any{"request_source": inboxRecordRequestSource}}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: management, ComputerID: computer,
		Profile: agentprofile.Management, Role: agentprofile.Management, RunRecord: &rec}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Management)
	calls := 0
	cell := func(src string) string {
		t.Helper()
		calls++
		execCtx.ToolCallID = fmt.Sprintf("call-%d", calls)
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(toolregistry.WithExecutionContext(ctx, execCtx), payload)
		if err != nil {
			out += err.Error()
		}
		return out
	}
	if out := cell(`choir.Cast("` + source + `", "poll again", map[string]interface{}{})`); !strings.Contains(out, "decline") {
		t.Fatalf("a Cast to a source was not refused: %s", out)
	}
	out := cell(`rs := choir.Records()
choir.Note(rs[0].From, "declined capture-9: a newsletter signup page, no news event")`)
	if !strings.Contains(out, "rlm:note:") {
		t.Fatalf("the decline did not commit: %s", out)
	}
	got, err := s.ListInboxRecords(ctx, owner, computer, source, 0, 0)
	if err != nil || len(got) != 1 || got[0].Directive == nil || !strings.Contains(got[0].Directive.Body, "declined capture-9") || got[0].SenderInbox != management {
		t.Fatalf("source inbox = %+v, %v; want management's decline", got, err)
	}
	after, _ := s.ListUnprojectedActorWakes(ctx)
	for _, w := range after {
		if w.TargetAgentID == source {
			t.Fatalf("the decline woke the source's mailbox: %+v (before %d wakes)", w, len(before))
		}
	}
}

type verdictRecorder struct {
	got chan types.StoryVerdictsRequest
}

func (v *verdictRecorder) PublishVersion(context.Context, StoryVersion) error { return nil }
func (v *verdictRecorder) PublishVerdicts(_ context.Context, req types.StoryVerdictsRequest) error {
	v.got <- req
	return nil
}

// Verdicts reach Store B only for publish-auto stories. Ways it can fail:
// a verdict on a private document leaves the computer; a story's verdict
// never reaches the overlay.
func TestStoryVerdictsPublishOnlyForPublishAutoStories(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-verdicts", rt.cfg.ComputerID
	management := agentprofile.Management + ":" + owner
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-v:cast:c1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "Harbor"},
		Cast:      &types.CommitmentCast{Publish: types.CastPublishAuto},
		Inbox:     types.StoryOpenInbox, SenderInbox: management, EmitterKind: types.CommitmentEmitterAgent,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: management},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cell-records:v", Records: []types.CommitmentRecord{cast}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	receipt, err := s.CommitInboxRecords(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	storyDoc := strings.TrimPrefix(receipt.Records[0].Inbox, "texture:")
	rec := &verdictRecorder{got: make(chan types.StoryVerdictsRequest, 4)}
	rt.SetStoryPublisher(rec)
	rt.publishStoryVerdicts(owner, computer, []types.SupportVerdict{
		{Key: "sp-story", Verdict: types.SupportUnsupported, Why: "says otherwise", Judge: "management", Pair: &types.CitationPair{DocID: storyDoc}},
		{Key: "sp-private", Verdict: types.SupportSupported, Judge: "management", Pair: &types.CitationPair{DocID: "doc-private"}},
	})
	select {
	case got := <-rec.got:
		if got.OwnerID != owner || len(got.Verdicts) != 1 || got.Verdicts[0].Key != "sp-story" || got.Verdicts[0].Why != "says otherwise" {
			t.Fatalf("published verdicts = %+v; want the story's verdict alone", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the story's verdict was not published")
	}
	rt.publishStoryVerdicts(owner, computer, []types.SupportVerdict{{Key: "sp-private-2", Verdict: types.SupportSupported, Judge: "management", Pair: &types.CitationPair{DocID: "doc-private"}}})
	select {
	case got := <-rec.got:
		t.Fatalf("a private document's verdict left the computer: %+v", got)
	case <-time.After(300 * time.Millisecond):
	}
}

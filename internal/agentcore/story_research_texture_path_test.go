package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The fed-capture refusal, the research-Cast log and the research context on
// the path a story's Texture really takes (review round 3, finding 1). A
// Texture run's Cast to research is folded into its TextureTurn before the
// records route is reached and commits with the turn, so the first version of
// the refusal and the log (in commitRecordsRoute) never ran for it, and the
// tests that called them directly could not fail. Failure list, written
// before the fix:
//
//  P1  a Texture's research Cast after a feed it has not read commits;
//  P2  the refusal does not reach the cell as an error saying nothing was sent
//      and naming the captures, or leaves a turn behind;
//  P3  a story with no earlier research is refused;
//  P4  a capture the Texture read (choir.ReadCapture) or tried to read and
//      could not is still owed, or the Cast after it is refused;
//  P5  a committed research Cast leaves no "story research cast" line, or one
//      without its round, opened and cause (open, then feed after a feed);
//  P6  the research run the Cast wakes does not receive the story's context.

// storeTurnCommitter commits a folded Texture turn's inbox records through the
// real store (the records half of ApplyTextureTurn, the transaction the
// textureowner commits them in) and answers as the committer does: its
// receipt names the records' receipts.
type storeTurnCommitter struct {
	s    *store.Store
	mu   sync.Mutex
	seen []string
}

func (c *storeTurnCommitter) Commit(ctx context.Context, rec *types.RunRecord, body, identity string) (string, error) {
	c.mu.Lock()
	c.seen = append(c.seen, body)
	c.mu.Unlock()
	var turn yaegikernel.TextureTurn
	if err := json.Unmarshal([]byte(body), &turn); err != nil {
		return "", err
	}
	out := map[string]any{"turn": "wait", "done": false}
	if len(turn.InboxRecords) > 0 {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		c.mu.Lock()
		n := len(c.seen)
		c.mu.Unlock()
		for i := range turn.InboxRecords {
			// A real run's cells have distinct ids; this harness reuses one
			// cell id, so the record ids are made distinct here.
			turn.InboxRecords[i].RecordID += ":" + strconv.Itoa(n)
			turn.InboxRecords[i].Provenance.CommittedAt = now
			turn.InboxRecords[i].Prediction.CommittedAt = now
		}
		req := types.CommitInboxRecordsRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "texture-turn:" + identity + ":" + strconv.Itoa(n), Records: turn.InboxRecords}
		req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
		receipt, err := c.s.CommitInboxRecords(ctx, req)
		if err != nil {
			return "", err
		}
		out["inbox_records"] = receipt.Records
	}
	if turn.Write != nil {
		out["revision_id"] = "rev-written"
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func (c *storeTurnCommitter) turns() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.seen)
}

const textureResearchCast = `choir.Cast("research", "cover the strike", map[string]interface{}{"kind":"question","summary":"the strike","questions":[]string{"what now?"}}, map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"})`

func closeResearchRound(t *testing.T, s *store.Store, owner, computer, doc string) {
	t.Helper()
	texture, research := "texture:"+doc, "research:"+doc
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "seed-report-" + doc, Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "research-cell:report:" + doc, Kind: types.CommitmentKindReport,
		Report: &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "the talks resumed"}}, StopReason: "answered"}, Inbox: texture, SenderInbox: research, Addressee: texture,
		ConversationID: types.InboxConversationID(texture, research), Round: 1, Final: true,
		ComputerID: computer, EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: research, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
	}}, Cursor: &types.InboxCursorAdvance{ReaderAgentID: research, Inbox: research, ExpectedSeq: 0, Seq: 1}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("close the round: %v", err)
	}
}

func TestTextureResearchCastIsGatedLoggedAndFollowedByContextOnTheRealFold(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	com := &storeTurnCommitter{s: s}
	bindTextureCommitter(t, rt, com)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	ctx := context.Background()
	owner, computer := "owner-texture-path", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Dock strike", "Whether the dock strike ends and on what terms", "", []string{"c1"}, t0)
	sink := &ctxSink{
		urls:  map[string]string{"c1": "https://port.example/1", "c4": "https://port.example/4"},
		gone:  map[string]bool{"c5": true},
		pages: map[string][]researchtools.LeadPage{doc: {{CaptureID: "cap-r1", URL: "https://news.example/a", Title: "Talks resume"}}},
	}
	rt.researchDeps = &researchtools.Dependencies{Captures: sink}

	rec := types.RunRecord{
		RunID: "run-texture-path", AgentID: "texture:" + doc, OwnerID: owner, ComputerID: computer,
		ChannelID: doc, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, State: types.RunRunning,
		Metadata: map[string]any{"doc_id": doc},
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{
		RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: doc, ComputerID: computer,
		Profile: agentprofile.Texture, Role: agentprofile.Texture, RunRecord: &rec,
	}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Texture)
	cellCtx := toolregistry.WithExecutionContext(ctx, execCtx)
	run := func(src string) string {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(cellCtx, payload)
		if err != nil {
			return out + err.Error()
		}
		return out
	}
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev); _ = os.Stderr })
	lines := func(prefix string) []string {
		var got []string
		for _, l := range strings.Split(logs.String(), "\n") {
			if i := strings.Index(l, prefix); i >= 0 {
				got = append(got, l[i:])
			}
		}
		return got
	}

	// P3, P5: a story with no earlier research is never refused; the Cast logs its round, that it opened it, and why.
	out := run(textureResearchCast)
	if com.turns() != 1 {
		t.Fatalf("P3: the first research Cast of a story was refused or lost: turns=%d out=%s", com.turns(), out)
	}
	want := "story research cast run=run-texture-path story=" + doc + " round=1 opened=true cause=open"
	if got := lines("story research cast"); len(got) != 1 || got[0] != want {
		t.Fatalf("P5: lines = %q, want [%q]", got, want)
	}
	// The first round's Report comes back; management then feeds two captures.
	closeResearchRound(t, s, owner, computer, doc)
	feedCastAt(t, s, owner, computer, doc, "cell-2:cast:f1", []string{"c4", "c5"}, time.Now().UTC().Add(2*time.Second))

	// P1, P2: the next research Cast is refused, names both captures, says nothing was sent, and leaves no turn.
	logs.Reset()
	out = run(textureResearchCast)
	if com.turns() != 1 {
		t.Fatalf("P1: a research Cast after an unread feed committed a turn: turns=%d out=%s", com.turns(), out)
	}
	for _, w := range []string{"c4", "c5", "nothing was sent", "choir.ReadCapture"} {
		if !strings.Contains(out, w) {
			t.Fatalf("P2: the cell's error lacks %q: %s", w, out)
		}
	}
	if got := lines("story research refused"); len(got) != 1 || got[0] != "story research refused run=run-texture-path story="+doc+" unread=2" {
		t.Fatalf("P2: the refusal is not counted: %q", got)
	}
	if got := lines("story research cast"); len(got) != 0 {
		t.Fatalf("P5: a refused Cast logged as a Cast: %q", got)
	}

	// P4: one read, one attempt at a capture the store no longer has; neither is owed, so the next Cast commits.
	run(`choir.ReadCapture("c4", 0, 100)`)
	run(`choir.ReadCapture("c5", 0, 100)`)
	logs.Reset()
	out = run(textureResearchCast)
	if com.turns() != 2 {
		t.Fatalf("P4: a read capture and a tried one still refuse research: turns=%d out=%s", com.turns(), out)
	}
	want = "story research cast run=run-texture-path story=" + doc + " round=2 opened=true cause=feed"
	if got := lines("story research cast"); len(got) != 1 || got[0] != want {
		t.Fatalf("P5: lines = %q, want [%q]\nout=%s\nlogs=%s", got, want, out, logs.String())
	}

	// P6: the run the committed Cast wakes. The research desk's agent record is
	// minted by ApplyTextureTurn, which this harness does not run, so the run
	// record is built as WakeRecordsInbox builds it (identity in the metadata:
	// the desk's agent id and its channel, the story), and the section is built
	// from it as the run's first message builds it.
	if open, err := s.ConversationState(ctx, owner, computer, types.InboxConversationID("texture:"+doc, "research:"+doc)); err != nil || !open.Open {
		t.Fatalf("P6: the Cast left no open round to wake research for: %+v, %v", open, err)
	}
	woken := &types.RunRecord{RunID: "run-research-woken", OwnerID: owner, ComputerID: computer, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning,
		Metadata: map[string]any{runMetadataAgentProfile: agentprofile.Research, runMetadataAgentRole: agentprofile.Research, runMetadataAgentID: "research:" + doc, runMetadataChannelID: doc}}
	c := rt.researchDigestOpening(ctx, woken)
	if c == nil || c.text == "" {
		t.Fatal("P6: the research run a Texture's Cast woke got no story digest")
	}
	text := c.text
	// This harness commits the turn's records through CommitInboxRecords (the records half
	// of ApplyTextureTurn), so its Cast carries no story: the real turn commit attaches it
	// (store.attachStoryToResearchCasts, tested in the store) and the digest then has the
	// story section. Here the digest has the captures and what earlier rounds found, and
	// never goes to the document for the story.
	if strings.Contains(text, "[This story]") {
		t.Errorf("P6: a story section without a Cast that carries it:\n%s", text)
	}
	for _, w := range []string{"cap-r1", "capture_id c4 https://port.example/4"} {
		if !strings.Contains(text, w) {
			t.Errorf("P6: the context lacks %q:\n%s", w, text)
		}
	}
}

// A research refusal never loses a revision (review round 4, finding 1): a cell
// that writes its page and casts research after an unread feed commits the Write
// and not the Cast, and is told both. The activation's turn is what the rest
// made, so its trigger is consumed in the normal way (finding 2); a cell with no
// turn to commit without its Cast is refused whole and told so.
func TestAResearchRefusalCommitsThePageWriteAndDropsOnlyTheCast(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	com := &storeTurnCommitter{s: s}
	bindTextureCommitter(t, rt, com)
	ctx := context.Background()
	owner, computer := "owner-texture-write", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Dock strike", "Whether the dock strike ends and on what terms", "", []string{"c1"}, t0)
	rt.researchDeps = &researchtools.Dependencies{Captures: &ctxSink{urls: map[string]string{"c4": "https://port.example/4"}}}
	rec := types.RunRecord{
		RunID: "run-texture-write", AgentID: "texture:" + doc, OwnerID: owner, ComputerID: computer,
		ChannelID: doc, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, State: types.RunRunning,
		Metadata: map[string]any{"doc_id": doc},
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{
		RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: doc, ComputerID: computer,
		Profile: agentprofile.Texture, Role: agentprofile.Texture, RunRecord: &rec,
	}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Texture)
	cellCtx := toolregistry.WithExecutionContext(ctx, execCtx)
	run := func(src string) string {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(cellCtx, payload)
		if err != nil {
			return out + err.Error()
		}
		return out
	}
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })

	run(textureResearchCast)
	closeResearchRound(t, s, owner, computer, doc)
	feedCastAt(t, s, owner, computer, doc, "cell-2:cast:f1", []string{"c4"}, time.Now().UTC().Add(2*time.Second))
	before := com.turns()
	logs.Reset()
	out := run(`choir.Write("@insert end\nv2 of the page", "revise from what was fed")
` + textureResearchCast)
	if com.turns() != before+1 {
		t.Fatalf("W1: the page write was lost to the research refusal: turns %d -> %d: %s", before, com.turns(), out)
	}
	var turn yaegikernel.TextureTurn
	if err := json.Unmarshal([]byte(com.seen[len(com.seen)-1]), &turn); err != nil || turn.Write == nil || len(turn.InboxRecords) != 0 {
		t.Fatalf("W1: the committed turn = %+v (%v), want the Write and no research record", turn, err)
	}
	for _, w := range []string{"research_not_sent", "NOT sent", "revision rev-written", "c4", "choir.ReadCapture"} {
		if !strings.Contains(out, w) {
			t.Errorf("W2: the cell is not told %q: %s", w, out)
		}
	}
	if strings.Contains(logs.String(), "story research cast") {
		t.Errorf("W1: a dropped Cast logged as sent: %s", logs.String())
	}
	// The activation's turn is what the rest made: its trigger is consumed by
	// the Write's turn, not left for an idle turn or a re-wake (finding 2).
	if cells, ran := rt.takeTextureActivationCells(rec.RunID); !ran || !cells.applied {
		t.Fatalf("W3: the activation recorded cells=%+v ran=%v, want a turn applied", cells, ran)
	}
	// A cell whose only act is the research Cast has nothing to commit without it: refused whole, and it says so.
	before = com.turns()
	out = run(textureResearchCast)
	if com.turns() != before || !strings.Contains(out, "nothing was committed") || !strings.Contains(out, "c4") {
		t.Fatalf("W4: a Cast-only cell: turns %d -> %d: %s", before, com.turns(), out)
	}
	if cells, ran := rt.takeTextureActivationCells(rec.RunID); ran && cells.applied {
		t.Fatalf("W4: a refused cell counted as an applied turn: %+v", cells)
	}
}

// The refusal for one set of unread captures is bounded, and a store that cannot
// be read does not block a story but says so (finding 2's worst case; finding 3).
func TestUnreadFedCapturesRefusalIsBoundedAndAStoreErrorIsLogged(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-gate-bound", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Dock strike", "Whether the dock strike ends and on what terms", "", []string{"c1"}, t0)
	researchCastAt(t, s, owner, computer, doc, "texture-cell:cast:r1", t0.Add(10*time.Minute))
	feedCastAt(t, s, owner, computer, doc, "cell-2:cast:f1", []string{"c4"}, t0.Add(20*time.Minute))
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })
	ctx := context.Background()
	for i := 1; i <= storyResearchRefusalLimit; i++ {
		if got := rt.unreadFedCaptures(ctx, owner, computer, doc, "run-b"); len(got) != 1 {
			t.Fatalf("refusal %d of %d: unread = %v", i, storyResearchRefusalLimit, got)
		}
	}
	if got := rt.unreadFedCaptures(ctx, owner, computer, doc, "run-b"); got != nil || !strings.Contains(logs.String(), "story research gate released run=run-b story="+doc) {
		t.Fatalf("the refusal past its limit: unread = %v, log = %s", got, logs.String())
	}
	// A new feed is a new set: refused again.
	feedCastAt(t, s, owner, computer, doc, "cell-3:cast:f2", []string{"c5"}, time.Now().UTC().Add(2*time.Second))
	if got := rt.unreadFedCaptures(ctx, owner, computer, doc, "run-b"); len(got) != 2 {
		t.Fatalf("a new feed was not refused: %v", got)
	}
	// A store that cannot be read: the gate opens and a line says why.
	logs.Reset()
	dead, cancel := context.WithCancel(ctx)
	cancel()
	if got := rt.unreadFedCaptures(dead, owner, computer, doc, "run-b"); got != nil || !strings.Contains(logs.String(), "story research gate skipped run=run-b story="+doc+" err=") {
		t.Fatalf("a store error: unread = %v, log = %q", got, logs.String())
	}
}

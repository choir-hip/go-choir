package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Support judgement off management's turn
// (docs/archive/reports/support-judgement-to-jev-design-2026-10-14.md): a Texture
// commit lists its new pairs by their check memos, and the runtime judges
// them with Jev after the commit returns. Ways it can fail, written first
// (the note's list):
//   - the hand-off waits for Jev, so a slow gateway stalls the Texture;
//   - two commits judge one pair at once: Jev twice, the Texture failed
//     twice;
//   - Jev judges text other than the committed sentence and the captured
//     block;
//   - a failure does not reach texture:<doc>, or reaches it without waking
//     the Texture (system records wake no one, D5);
//   - a pass sends the Texture a record or mints a revision;
//   - management still receives the pairs, or a legacy support_pairs record
//     wedges it;
//   - one bad pair blocks the verdicts of the others;
//   - the Jev call's cost never reaches /api/costs.

type fakeSupportJudge struct {
	mu      sync.Mutex
	choice  string
	calls   int
	state   string
	block   chan struct{}
	started chan struct{}
	// omitQuote is a cited quote whose pair's answer Jev leaves out.
	omitQuote string
}

func (f *fakeSupportJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	f.mu.Lock()
	f.calls++
	f.state = string(state)
	block, started := f.block, f.started
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	var parsed struct {
		Pairs map[string]struct {
			Quote string `json:"quote"`
		} `json:"pairs"`
	}
	_ = json.Unmarshal(state, &parsed)
	yes := 0.1
	if f.choice == "yes" {
		yes = 0.9
	}
	answers := map[string]any{}
	for name := range questions {
		if f.omitQuote != "" && parsed.Pairs[name].Quote == f.omitQuote {
			continue
		}
		answers[name] = map[string]any{"type": "choice", "choice": f.choice, "probabilities": map[string]float64{"yes": yes, "no": 1 - yes}, "confidence": 0.9}
	}
	return json.Marshal(map[string]any{"model": "typesafe/jev-1.13", "answers": answers,
		"usage": map[string]any{"input_tokens": 812, "output_tokens": 14, "cost": 0.00004}})
}

func (f *fakeSupportJudge) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type supportCaptures map[string]texturewrite.Capture

func (c supportCaptures) CaptureBlocks(_ context.Context, _ string, handle string) (texturewrite.Capture, error) {
	if got, ok := c[handle]; ok {
		return got, nil
	}
	return texturewrite.Capture{}, texturewrite.ErrNotCaptured
}

const (
	supportCitedBlock  = "The court ruled that the merger may proceed."
	supportAppealBlock = "The regulator appealed."
)

// seededSupport is a committed Texture revision and the pairs its commit
// listed.
type seededSupport struct {
	RunID      string
	RevisionID string
	Pairs      []types.CitationPair
}

// seedListedSupportPairs commits a Texture revision whose citations (one
// per cited block id) the store lists, through the real Texture turn
// transaction. It does not judge them: the caller hands them to the runtime
// as the Texture commit path does.
func seedListedSupportPairs(t *testing.T, s *store.Store, owner, computer, docID string, citedCIDs ...string) seededSupport {
	t.Helper()
	ctx := context.Background()
	trajectoryID, texture := "trajectory-"+docID, agentprofile.Texture+":"+docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "start-" + docID, TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-" + docID, Objective: "write the story", AssignedAgentID: texture, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: owner, ComputerID: computer, TrajectoryID: trajectoryID, Title: "Merger", CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "rev-base-" + docID, DocID: docID, OwnerID: owner, ComputerID: computer, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: owner, Content: "Draft.", CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: texture, OwnerID: owner, ComputerID: computer, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	caller := types.RunRecord{RunID: "run-texture-" + docID, OwnerID: owner, ComputerID: computer, AgentID: texture,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: docID, TrajectoryID: trajectoryID,
		State: types.RunRunning, CreatedAt: now, UpdatedAt: now}
	project := types.ReplaceLifecycleActivationRequest{OwnerID: owner, ComputerID: computer, CommandID: "project-" + docID, TrajectoryID: trajectoryID, AgentID: texture, Run: caller}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project Texture activation: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, owner, computer, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := s.GetAgentByScope(ctx, owner, computer, texture)
	if err != nil {
		t.Fatal(err)
	}
	if len(citedCIDs) == 0 {
		citedCIDs = []string{"cid-1"}
	}
	revisionID := "rev-1-" + docID
	body := texturewrite.PlainDoc(docID, revisionID, "The merger may proceed.")
	block := texturewrite.DocBlocks(body)[0]
	citing := texturewrite.CitingText(block.Node)
	cited := map[string]types.Ref{
		"cid-1": {Doc: "cap-court", Version: "doc-hash-1", CID: "cid-1", Hash: texturewrite.TextHash(supportCitedBlock), Quote: "the merger may proceed"},
		"cid-2": {Doc: "cap-court", Version: "doc-hash-1", CID: "cid-2", Hash: texturewrite.TextHash(supportAppealBlock), Quote: "regulator appealed"},
		// cid-3 names a block whose captured text no longer hashes to it.
		"cid-3": {Doc: "cap-court", Version: "doc-hash-1", CID: "cid-3", Hash: texturewrite.TextHash("A block that was re-captured."), Quote: "re-captured"},
	}
	out := seededSupport{RunID: caller.RunID, RevisionID: revisionID}
	for _, cid := range citedCIDs {
		ref := cited[cid]
		pair := types.CitationPair{DocID: docID, RevisionID: revisionID, BlockID: block.ID, BlockHash: texturewrite.TextHash(citing), Text: citing, Cited: ref, Matcher: quotematch.Version}
		pair.Key = texturewrite.PairKey(pair.BlockHash, ref.Hash, ref.Quote)
		out.Pairs = append(out.Pairs, pair)
	}
	rawBody, _ := json.Marshal(body)
	req := types.ApplyTextureTurnRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "texture-turn-" + docID, DocumentID: docID, TrajectoryID: trajectoryID,
		CallerAgentID: texture, CallerRunID: caller.RunID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedCallerLifecycleVersion: agent.LifecycleVersion,
		ExpectedHeadRevisionID: start.InitialRevision.RevisionID, CallerWorkDisposition: types.WorkItemOpen, Outcome: types.TextureTurnRevision,
		Revision: types.Revision{RevisionID: revisionID, DocID: docID, ParentRevisionID: start.InitialRevision.RevisionID,
			AuthorKind: types.AuthorAppAgent, AuthorLabel: "appagent", BodyDoc: rawBody, SourceEntities: json.RawMessage("[]")},
		Reason: "state the ruling", Citations: out.Pairs,
	}
	req.CommandDigest, _ = store.ComputeApplyTextureTurnWithSourceGraphDigest(req, store.TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurnWithSourceGraph(ctx, req, store.TextureSourceGraphWriteSet{}); err != nil {
		t.Fatalf("Texture revision with a citation: %v", err)
	}
	return out
}

func supportRuntime(t *testing.T, judge CandidateJudge) (*Runtime, *store.Store) {
	t.Helper()
	rt, s := testRuntime(t)
	rt.SetTextureCaptures(supportCaptures{"cap-court": {Doc: "cap-court", Version: "doc-hash-1", URL: "https://example.org/court", Title: "Court ruling",
		Blocks: []quotematch.Block{{ID: "cid-1", Text: supportCitedBlock}, {ID: "cid-2", Text: supportAppealBlock}, {ID: "cid-3", Text: "A block that changed."}}}})
	if judge != nil {
		rt.SetCandidateJudge(judge)
	}
	return rt, s
}

func TestJevFailureReachesTheTextureOffTheCommit(t *testing.T) {
	jev := &fakeSupportJudge{choice: "no"}
	rt, s := supportRuntime(t, jev)
	ctx := context.Background()
	owner, computer, docID := "owner-support-fail", rt.cfg.ComputerID, "doc-support-fail"
	seeded := seedListedSupportPairs(t, s, owner, computer, docID)
	pair := seeded.Pairs[0]
	doc, _ := s.GetLifecycleDocument(ctx, owner, computer, docID)
	management := agentprofile.Management + ":" + owner
	if head, _ := s.InboxHead(ctx, owner, computer, management); head != 0 {
		t.Fatalf("management inbox head = %d after the commit; management must not receive support pairs", head)
	}
	if _, listed, _ := s.CitationCheck(ctx, owner, computer, pair.Key); !listed {
		t.Fatal("the commit did not list its pair (no check memo)")
	}

	rt.JudgeRevisionSupport(owner, computer, seeded.RunID, seeded.RevisionID, seeded.Pairs)
	rt.WaitSupportJudgements()

	if jev.callCount() != 1 || !strings.Contains(jev.state, supportCitedBlock) || !strings.Contains(jev.state, pair.Text) {
		t.Fatalf("Jev calls = %d, state %s; want one call on the committed sentence and the captured block", jev.callCount(), jev.state)
	}
	texture := agentprofile.Texture + ":" + docID
	got, err := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10)
	if err != nil || len(got) != 1 || got[0].Support == nil || len(got[0].Support.Failures) != 1 {
		t.Fatalf("texture inbox = %+v, %v; want one failure record", got, err)
	}
	failure := got[0]
	if failure.SenderInbox != types.SupportSystemAgentID || failure.EmitterKind != types.CommitmentEmitterSystem ||
		failure.Support.Failures[0].Key != pair.Key || failure.Support.Failures[0].Pair == nil ||
		failure.Support.Failures[0].Pair.BlockID != pair.BlockID || failure.Support.Failures[0].Why == "" {
		t.Fatalf("failure record = %+v; want a system record from %s naming the pair with why", failure, types.SupportSystemAgentID)
	}
	if inboxRecordKind(failure) != "support_failure" {
		t.Fatalf("failure kind = %q", inboxRecordKind(failure))
	}
	// The failure wakes the Texture: a system record that asks a desk to
	// act.
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	woken := false
	for _, w := range wakes {
		if w.TargetAgentID == texture && w.Kind == store.InboxRecordWakeKind {
			woken = true
		}
	}
	if !woken {
		t.Fatalf("no inbox wake for %s among %d wakes", texture, len(wakes))
	}
	if v, found, _ := s.SupportVerdict(ctx, owner, computer, pair.Key); !found || v.Verdict != types.SupportUnsupported || v.Judge != "jev:typesafe/jev-1.13" {
		t.Fatalf("verdict memo = %+v, %v", v, found)
	}
	if head, _ := s.InboxHead(ctx, owner, computer, management); head != 0 {
		t.Fatalf("management inbox head = %d; judging sent management a record", head)
	}
	if after, _ := s.GetLifecycleDocument(ctx, owner, computer, docID); after.CurrentRevisionID != doc.CurrentRevisionID {
		t.Fatal("a verdict minted a revision")
	}

	// The call's cost is on the Texture run that committed the pairs, where
	// /api/costs reads it.
	evs, err := s.ListEventsByOwner(ctx, owner, 200)
	if err != nil {
		t.Fatal(err)
	}
	var jevEntries []llmcost.CostEntry
	for _, e := range llmcost.ExtractCostEntries(evs) {
		if e.Model == "typesafe/jev-1.13" {
			jevEntries = append(jevEntries, e)
		}
	}
	if len(jevEntries) != 1 || jevEntries[0].InputTokens != 812 || jevEntries[0].OutputTokens != 14 || jevEntries[0].RunID != seeded.RunID || jevEntries[0].AgentID != types.SupportSystemAgentID {
		t.Fatalf("Jev cost entries = %+v; want one on run %s with the answer's usage", jevEntries, seeded.RunID)
	}
}

func TestJevPassMintsNothingAndIsMemoized(t *testing.T) {
	jev := &fakeSupportJudge{choice: "yes"}
	rt, s := supportRuntime(t, jev)
	ctx := context.Background()
	owner, computer, docID := "owner-support-pass", rt.cfg.ComputerID, "doc-support-pass"
	seeded := seedListedSupportPairs(t, s, owner, computer, docID)
	pair := seeded.Pairs[0]

	rt.JudgeRevisionSupport(owner, computer, seeded.RunID, seeded.RevisionID, seeded.Pairs)
	rt.WaitSupportJudgements()
	texture := agentprofile.Texture + ":" + docID
	if head, _ := s.InboxHead(ctx, owner, computer, texture); head != 0 {
		t.Fatalf("texture inbox head = %d; a pass sent Texture a record", head)
	}
	if v, found, _ := s.SupportVerdict(ctx, owner, computer, pair.Key); !found || v.Verdict != types.SupportSupported || v.PSupportedBP != 9000 {
		t.Fatalf("verdict memo = %+v, %v", v, found)
	}
	// A later commit citing the judged pair again is a memo hit.
	rt.JudgeRevisionSupport(owner, computer, seeded.RunID, seeded.RevisionID, seeded.Pairs)
	rt.WaitSupportJudgements()
	if jev.callCount() != 1 {
		t.Fatalf("Jev calls = %d; a judged pair was judged again", jev.callCount())
	}
}

func TestJudgeRevisionSupportNeverBlocksTheCommitAndJudgesAPairOnce(t *testing.T) {
	jev := &fakeSupportJudge{choice: "no", block: make(chan struct{}), started: make(chan struct{}, 1)}
	rt, s := supportRuntime(t, jev)
	ctx := context.Background()
	owner, computer, docID := "owner-support-async", rt.cfg.ComputerID, "doc-support-async"
	seeded := seedListedSupportPairs(t, s, owner, computer, docID)

	began := time.Now()
	rt.JudgeRevisionSupport(owner, computer, seeded.RunID, seeded.RevisionID, seeded.Pairs)
	if waited := time.Since(began); waited > 500*time.Millisecond {
		t.Fatalf("the hand-off took %s; it must return before Jev answers", waited)
	}
	select {
	case <-jev.started:
	case <-time.After(10 * time.Second):
		t.Fatal("Jev was never called")
	}
	// A second commit citing the pair while Jev is still judging it does
	// not ask again.
	rt.JudgeRevisionSupport(owner, computer, seeded.RunID, seeded.RevisionID, seeded.Pairs)
	time.Sleep(100 * time.Millisecond)
	close(jev.block)
	rt.WaitSupportJudgements()
	if jev.callCount() != 1 {
		t.Fatalf("Jev calls = %d; a pair in flight was judged twice", jev.callCount())
	}
	if head, _ := s.InboxHead(ctx, owner, computer, agentprofile.Texture+":"+docID); head != 1 {
		t.Fatalf("texture inbox head = %d; want exactly one failure", head)
	}
}

func TestOneBadPairLeavesItPendingAndRecordsTheRest(t *testing.T) {
	jev := &fakeSupportJudge{choice: "no", omitQuote: "regulator appealed"}
	rt, s := supportRuntime(t, jev)
	ctx := context.Background()
	owner, computer, docID := "owner-support-bad", rt.cfg.ComputerID, "doc-support-bad"
	seeded := seedListedSupportPairs(t, s, owner, computer, docID, "cid-1", "cid-2", "cid-3")
	rt.JudgeRevisionSupport(owner, computer, seeded.RunID, seeded.RevisionID, seeded.Pairs)
	rt.WaitSupportJudgements()

	judged, omitted, changed := seeded.Pairs[0], seeded.Pairs[1], seeded.Pairs[2]
	if v, found, _ := s.SupportVerdict(ctx, owner, computer, judged.Key); !found || v.Verdict != types.SupportUnsupported {
		t.Fatalf("judged pair verdict = %+v, %v; one bad pair blocked the others", v, found)
	}
	for _, p := range []types.CitationPair{omitted, changed} {
		if _, found, _ := s.SupportVerdict(ctx, owner, computer, p.Key); found {
			t.Fatalf("pair %s (%s) has a verdict; it must stay pending", p.Key, p.Cited.CID)
		}
	}
	if strings.Contains(jev.state, "re-captured") {
		t.Fatal("Jev was asked about a pair whose cited block changed")
	}
}

func TestJudgeWithoutJevLeavesPairsPending(t *testing.T) {
	rt, s := supportRuntime(t, nil)
	ctx := context.Background()
	owner, computer, docID := "owner-support-nojev", rt.cfg.ComputerID, "doc-support-nojev"
	seeded := seedListedSupportPairs(t, s, owner, computer, docID)
	rt.JudgeRevisionSupport(owner, computer, seeded.RunID, seeded.RevisionID, seeded.Pairs)
	rt.WaitSupportJudgements()
	if _, found, _ := s.SupportVerdict(ctx, owner, computer, seeded.Pairs[0].Key); found {
		t.Fatal("a verdict with no decision model bound")
	}
	if _, listed, _ := s.CitationCheck(ctx, owner, computer, seeded.Pairs[0].Key); !listed {
		t.Fatal("the pair is no longer listed")
	}
}

func TestManagementRecordsWakePromptNamesNoSupportJudgement(t *testing.T) {
	prompt := recordsInboxPrompts[agentprofile.Management]
	for _, absent := range []string{"support_pairs", "choir.Judge"} {
		if strings.Contains(prompt, absent) {
			t.Errorf("management's records-wake prompt still says %q", absent)
		}
	}
}

// managementSupportCell runs management cells on the owner's records route.
func managementSupportCell(t *testing.T, rt *Runtime, s *store.Store, owner, computer string) func(string) string {
	t.Helper()
	ctx := context.Background()
	management := agentprofile.Management + ":" + owner
	rec := types.RunRecord{RunID: "run-mgmt-support-" + owner, AgentID: management, OwnerID: owner, ComputerID: computer, ChannelID: management,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunRunning,
		Metadata: map[string]any{"request_source": inboxRecordRequestSource}}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: management, ComputerID: computer,
		Profile: agentprofile.Management, Role: agentprofile.Management, RunRecord: &rec}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Management)
	calls := 0
	return func(src string) string {
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
}

// A support_pairs record minted before this change still sits in
// management's inbox. Management sees it closed, with nothing to answer,
// and its pairs go to Jev.
func TestLegacySupportPairsRecordIsShownClosedAndJudged(t *testing.T) {
	deskTestWorkerBin(t)
	jev := &fakeSupportJudge{choice: "no"}
	rt, s := supportRuntime(t, jev)
	ctx := context.Background()
	owner, computer, docID := "owner-support-legacy", rt.cfg.ComputerID, "doc-support-legacy"
	seeded := seedListedSupportPairs(t, s, owner, computer, docID)
	management := agentprofile.Management + ":" + owner
	legacy := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "support-pairs:" + seeded.RevisionID, Kind: types.CommitmentKindReport,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: types.SupportSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computer,
		Inbox: management, SenderInbox: agentprofile.Texture + ":" + docID, Addressee: management,
		SubjectRefs: map[string]string{"doc_id": docID, "revision_id": seeded.RevisionID},
		Support:     &types.CommitmentSupport{Pairs: seeded.Pairs},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "legacy-list", Records: []types.CommitmentRecord{legacy}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(ctx, req); err != nil {
		t.Fatalf("seed legacy list: %v", err)
	}

	cell := managementSupportCell(t, rt, s, owner, computer)
	out := cell(`rs := choir.Records()
fmt.Println(len(rs), rs[0].Kind, rs[0].Support == nil)
fmt.Println(rs[0].Body)`)
	if !strings.Contains(out, "1 support_pairs true") || !strings.Contains(out, "Closed: Jev judges citation support") {
		t.Fatalf("management cell = %s; want the legacy list shown closed without pairs", out)
	}
	rt.WaitSupportJudgements()
	if v, found, _ := s.SupportVerdict(ctx, owner, computer, seeded.Pairs[0].Key); !found || v.Verdict != types.SupportUnsupported {
		t.Fatalf("legacy pair verdict = %+v, %v; want it judged by Jev", v, found)
	}
	// Management's turn marks what it was shown when it completes
	// (captures-undisposed-after-failed-management-runs-2026-10-14).
	rt.recheckRecordsInbox(ctx, &types.RunRecord{RunID: "run-mgmt-support-" + owner, AgentID: management, OwnerID: owner, ComputerID: computer,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunCompleted})
	if cursor, _ := s.ReaderCursor(ctx, owner, computer, management, management); cursor != 1 {
		t.Fatalf("management cursor = %d; the closed record was not marked read", cursor)
	}
	if out := cell(`_, err := choir.Judge(nil)
fmt.Println(err)`); !strings.Contains(out, "Judge") || !strings.Contains(strings.ToLower(out), "undefined") {
		t.Fatalf("choir.Judge in a management cell = %s; want it undefined", out)
	}
}

// A pair's text is the span its citation closes, with its paragraph as
// context (docs/problems/support-pair-judges-whole-paragraph-against-one-
// quote-2026-10-19.md). Way it can fail (S6): Jev is not given the
// paragraph, so a span's "it" or "the regulator" cannot be resolved; or it
// is given the paragraph and not told to judge only the span, so it judges
// the paragraph again.
func TestJevJudgesTheSpanWithItsParagraphAsContext(t *testing.T) {
	jev := &fakeSupportJudge{choice: "yes"}
	rt, _ := supportRuntime(t, jev)
	span, paragraph := "It appealed on Monday.", "The regulator lost in court. It appealed on Monday. Shares rose."
	pair := types.CitationPair{Key: "sp-span", DocID: "doc", RevisionID: "rev", BlockID: "p1", BlockHash: texturewrite.TextHash(span),
		Text: span, Context: paragraph, Matcher: "qm1",
		Cited: types.Ref{Doc: "cap-court", CID: "cid-2", Hash: texturewrite.TextHash(supportAppealBlock), Quote: supportAppealBlock}}
	judged, err := rt.askJevForSupport(context.Background(), "owner", "", []types.SupportVerdict{{Key: pair.Key, Pair: &pair}})
	if err != nil || len(judged) != 1 {
		t.Fatalf("judged = %+v, %v", judged, err)
	}
	var state struct {
		Task  string                       `json:"task"`
		Pairs map[string]map[string]string `json:"pairs"`
	}
	if err := json.Unmarshal([]byte(jev.state), &state); err != nil {
		t.Fatal(err)
	}
	got := state.Pairs["p1"]
	if got["sentence"] != span || got["paragraph"] != paragraph {
		t.Fatalf("pair state = %+v; want the span as sentence and the paragraph as context", got)
	}
	if !strings.Contains(state.Task, "only the sentence") {
		t.Fatalf("task = %q; Jev is not told to judge only the sentence", state.Task)
	}
}

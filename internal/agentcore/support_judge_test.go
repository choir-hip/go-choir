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
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Support judgement through a real management cell (Cut 2 2a (v)): the
// pairs list a Texture commit addressed to management reaches its cell,
// choir.Judge asks Jev (or takes management's own verdict), and the cell's
// commit records each verdict by its hash pair. Ways it can fail, written
// first:
//   - Jev judges text the model typed instead of the committed sentence and
//     the captured block, so a cell can launder a verdict;
//   - a failing verdict never reaches texture:<doc>, or reaches it with no
//     pair to correct;
//   - a passing verdict sends Texture a record, or mints a revision;
//   - judging a judged pair calls Jev again or fails Texture twice;
//   - with no decision model bound, Judge fails without saying management
//     can judge the pair itself.

type fakeSupportJudge struct {
	mu     sync.Mutex
	choice string
	calls  int
	state  string
}

func (f *fakeSupportJudge) Judge(_ context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.state = string(state)
	yes := 0.1
	if f.choice == "yes" {
		yes = 0.9
	}
	answers := map[string]any{}
	for name := range questions {
		answers[name] = map[string]any{"type": "choice", "choice": f.choice, "probabilities": map[string]float64{"yes": yes, "no": 1 - yes}, "confidence": 0.9}
	}
	return json.Marshal(map[string]any{"model": "typesafe/jev-1.13", "answers": answers})
}

type supportCaptures map[string]texturewrite.Capture

func (c supportCaptures) CaptureBlocks(_ context.Context, _ string, handle string) (texturewrite.Capture, error) {
	if got, ok := c[handle]; ok {
		return got, nil
	}
	return texturewrite.Capture{}, texturewrite.ErrNotCaptured
}

const supportCitedBlock = "The court ruled that the merger may proceed."

// seedListedSupportPair commits a Texture revision whose one citation the
// store lists to management, through the real Texture turn transaction.
func seedListedSupportPair(t *testing.T, s *store.Store, owner, computer, docID string) types.CitationPair {
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
	revisionID := "rev-1-" + docID
	body := texturewrite.PlainDoc(docID, revisionID, "The merger may proceed.")
	block := texturewrite.DocBlocks(body)[0]
	citing := texturewrite.CitingText(block.Node)
	cited := types.Ref{Doc: "cap-court", Version: "doc-hash-1", CID: "cid-1", Hash: texturewrite.TextHash(supportCitedBlock), Quote: "the merger may proceed"}
	pair := types.CitationPair{DocID: docID, RevisionID: revisionID, BlockID: block.ID, BlockHash: texturewrite.TextHash(citing), Text: citing, Cited: cited, Matcher: quotematch.Version}
	pair.Key = texturewrite.PairKey(pair.BlockHash, cited.Hash, cited.Quote)
	rawBody, _ := json.Marshal(body)
	req := types.ApplyTextureTurnRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "texture-turn-" + docID, DocumentID: docID, TrajectoryID: trajectoryID,
		CallerAgentID: texture, CallerRunID: caller.RunID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedCallerLifecycleVersion: agent.LifecycleVersion,
		ExpectedHeadRevisionID: start.InitialRevision.RevisionID, CallerWorkDisposition: types.WorkItemOpen, Outcome: types.TextureTurnRevision,
		Revision: types.Revision{RevisionID: revisionID, DocID: docID, ParentRevisionID: start.InitialRevision.RevisionID,
			AuthorKind: types.AuthorAppAgent, AuthorLabel: "appagent", BodyDoc: rawBody, SourceEntities: json.RawMessage("[]")},
		Reason: "state the ruling", Citations: []types.CitationPair{pair},
	}
	req.CommandDigest, _ = store.ComputeApplyTextureTurnWithSourceGraphDigest(req, store.TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurnWithSourceGraph(ctx, req, store.TextureSourceGraphWriteSet{}); err != nil {
		t.Fatalf("Texture revision with a citation: %v", err)
	}
	return pair
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
		// Each cell is its own tool call, as in a run: two committing cells
		// must not share a cell id.
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

func supportRuntime(t *testing.T, judge CandidateJudge) (*Runtime, *store.Store) {
	t.Helper()
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	rt.SetTextureCaptures(supportCaptures{"cap-court": {Doc: "cap-court", Version: "doc-hash-1", URL: "https://example.org/court", Title: "Court ruling",
		Blocks: []quotematch.Block{{ID: "cid-1", Text: supportCitedBlock}, {ID: "cid-2", Text: "The regulator appealed."}}}})
	if judge != nil {
		rt.SetCandidateJudge(judge)
	}
	return rt, s
}

func TestJudgeFailureReachesTheTextureAndMintsNoRevision(t *testing.T) {
	jev := &fakeSupportJudge{choice: "no"}
	rt, s := supportRuntime(t, jev)
	ctx := context.Background()
	owner, computer, docID := "owner-support-fail", rt.cfg.ComputerID, "doc-support-fail"
	pair := seedListedSupportPair(t, s, owner, computer, docID)
	doc, _ := s.GetLifecycleDocument(ctx, owner, computer, docID)
	cell := managementSupportCell(t, rt, s, owner, computer)

	out := cell(`rs := choir.Records()
fmt.Println(len(rs), rs[0].Kind, len(rs[0].Support.Pairs))
vs, err := choir.Judge(rs[0].Support.Pairs)
fmt.Println("judged", err == nil, len(vs), vs[0].Verdict, vs[0].Judge)`)
	if !strings.Contains(out, "1 support_pairs 1") || !strings.Contains(out, "judged true 1 unsupported jev:typesafe/jev-1.13") {
		t.Fatalf("management cell = %s", out)
	}
	if jev.calls != 1 || !strings.Contains(jev.state, supportCitedBlock) || !strings.Contains(jev.state, pair.Text) {
		t.Fatalf("Jev calls = %d, state %s; want one call on the committed sentence and the captured block", jev.calls, jev.state)
	}
	texture := agentprofile.Texture + ":" + docID
	got, err := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10)
	if err != nil || len(got) != 1 || got[0].Support == nil || len(got[0].Support.Failures) != 1 || got[0].Support.Failures[0].Key != pair.Key ||
		got[0].Support.Failures[0].Pair == nil || got[0].Support.Failures[0].Pair.BlockID != pair.BlockID {
		t.Fatalf("texture inbox = %+v, %v; want management's failure naming the pair", got, err)
	}
	if v, found, _ := s.SupportVerdict(ctx, owner, computer, pair.Key); !found || v.Verdict != types.SupportUnsupported {
		t.Fatalf("verdict memo = %+v, %v", v, found)
	}
	if cursor, _ := s.ReaderCursor(ctx, owner, computer, agentprofile.Management+":"+owner, agentprofile.Management+":"+owner); cursor != 1 {
		t.Fatalf("management cursor = %d; the list was not marked read with the verdict", cursor)
	}
	if after, _ := s.GetLifecycleDocument(ctx, owner, computer, docID); after.CurrentRevisionID != doc.CurrentRevisionID {
		t.Fatal("a verdict minted a revision")
	}
}

func TestJudgePassMintsNothingAndIsMemoized(t *testing.T) {
	jev := &fakeSupportJudge{choice: "yes"}
	rt, s := supportRuntime(t, jev)
	ctx := context.Background()
	owner, computer, docID := "owner-support-pass", rt.cfg.ComputerID, "doc-support-pass"
	pair := seedListedSupportPair(t, s, owner, computer, docID)
	cell := managementSupportCell(t, rt, s, owner, computer)

	if out := cell(`rs := choir.Records()
vs, err := choir.Judge(rs[0].Support.Pairs)
fmt.Println("judged", err == nil, vs[0].Verdict)`); !strings.Contains(out, "judged true supported") {
		t.Fatalf("management cell = %s", out)
	}
	texture := agentprofile.Texture + ":" + docID
	if head, _ := s.InboxHead(ctx, owner, computer, texture); head != 0 {
		t.Fatalf("texture inbox head = %d; a pass sent Texture a record", head)
	}
	if v, found, _ := s.SupportVerdict(ctx, owner, computer, pair.Key); !found || v.Verdict != types.SupportSupported || v.PSupportedBP != 9000 {
		t.Fatalf("verdict memo = %+v, %v", v, found)
	}
	// Judging the judged pair again, by Jev or by management's own verdict,
	// is a memo hit.
	out := cell(`again, err := choir.Judge(rs[0].Support.Pairs)
own, err2 := choir.Judge([]map[string]any{{"key": rs[0].Support.Pairs[0].Key, "verdict": "unsupported", "why": "on reflection"}})
fmt.Println("again", err == nil, again[0].Memo, again[0].Verdict, err2 == nil, own[0].Memo, own[0].Verdict)`)
	if !strings.Contains(out, "again true true supported true true supported") {
		t.Fatalf("second Judge = %s", out)
	}
	if jev.calls != 1 {
		t.Fatalf("Jev calls = %d; a judged pair was judged again", jev.calls)
	}
	if head, _ := s.InboxHead(ctx, owner, computer, texture); head != 0 {
		t.Fatalf("texture inbox head = %d after a memo hit", head)
	}
}

func TestManagementJudgesAPairItselfWithoutJev(t *testing.T) {
	rt, s := supportRuntime(t, nil)
	ctx := context.Background()
	owner, computer, docID := "owner-support-own", rt.cfg.ComputerID, "doc-support-own"
	pair := seedListedSupportPair(t, s, owner, computer, docID)
	cell := managementSupportCell(t, rt, s, owner, computer)

	if out := cell(`rs := choir.Records()
_, err := choir.Judge(rs[0].Support.Pairs)
fmt.Println("jev", err)`); !strings.Contains(out, "set Verdict and Why") {
		t.Fatalf("Judge with no decision model = %s; want a refusal saying how to judge it yourself", out)
	}
	out := cell(`vs, err := choir.Judge(map[string]any{"key": rs[0].Support.Pairs[0].Key, "verdict": "unsupported", "why": "the ruling is about a different merger"})
fmt.Println("own", err == nil, vs[0].Verdict, vs[0].Judge)`)
	if !strings.Contains(out, "own true unsupported management") {
		t.Fatalf("management's own verdict = %s", out)
	}
	got, err := s.ListInboxRecords(ctx, owner, computer, agentprofile.Texture+":"+docID, 0, 10)
	if err != nil || len(got) != 1 || got[0].Support == nil || got[0].Support.Failures[0].Why != "the ruling is about a different merger" || got[0].Support.Failures[0].Key != pair.Key {
		t.Fatalf("texture inbox = %+v, %v", got, err)
	}
}

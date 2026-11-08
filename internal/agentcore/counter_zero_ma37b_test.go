package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA3.7b counter-zero. A representative day driven in process: a management
// intake with a story open and a delegation, a Texture cell's Cast to research
// (folded into its turn), a research cell reading its records and reporting
// back, a research corpus search. The writer, reader and disclosure gates
// then show no wrong_writer, wrong_reader or undeclared entry for a built-in
// kind, except the named list below.
//
// What the list holds and why:
//   - Entries keyed "undeclared" in the counters are the fixtures' own seeding
//     (they write and read through the store from test code); the oracles
//     (UndeclaredReads, UndeclaredWrites) skip a stack with a test frame and
//     see the product's call sites, so those are asserted empty instead.
//   - Nothing else is excused. A key landing here is a crossing the enforce
//     flip would block. MA3.7c closed the corpus search: world events and
//     capture labels are admitted to research (E4, reader table), and the
//     owner's stories are not part of it: they are the research Texture
//     child's SearchTexture under its own texture.search lease (MA4.6).
//   - The one residual a representative day may show is content_item (a
//     desk-authored content ref Texture resolves, counted as Texture's read; a
//     content lease replaces it). It does not occur in this day.
var counterZeroJustified = map[string]string{}

func TestRepresentativeDayCountsNoCrossingOnBuiltInKinds(t *testing.T) {
	// The package runs its fixtures in observe (TestMain); the counters are
	// the oracle of what enforce would refuse.
	runRepresentativeDay(t, false)
}

// F6: the same day with every gate enforcing. The fixtures' own seeding
// (documents, runs, inbox records written from test code with no declared
// writer) runs in observe, because a fixture is not a product call site; the
// product's calls between the seeds run in enforce and must cross nothing, so
// the day completes with zero refusals.
func TestRepresentativeDayPassesUnderEnforceWithZeroRefusals(t *testing.T) {
	runRepresentativeDay(t, true)
}

func runRepresentativeDay(t *testing.T, enforce bool) {
	f := newDelegationFixture(t, "owner-counter-zero")
	ctx := context.Background()
	rt, s := f.rt, f.s
	auth := &stubCellAuthorizer{}
	bindTextureCommitter(t, rt, auth)
	// seed runs a fixture write: observe around it when the day enforces.
	seed := func(fn func()) {
		t.Helper()
		if !enforce {
			fn()
			return
		}
		restoreSeed := objectgraph.SetModeForTest(objectgraph.WriterGateObserve)
		defer restoreSeed()
		fn()
	}

	seed(func() {
		seedArrival(t, s, f.owner, f.computer, "rss:cz", []types.CaptureArrival{
			{CaptureID: "cap-cz1", Title: "Harbor reopens", URL: "https://cz.example/1"},
		})
	})
	objectgraph.ResetWriterGateCounts()
	objectgraph.ResetReaderGateCounts()
	objectgraph.ResetUndeclaredWrites()
	objectgraph.ResetUndeclaredReads()
	// MA3.7c: attribute a read to its direct caller, so the product paths the
	// day drives synchronously are seen (d@e showed texture_document and
	// texture_revision reads with no reader that the exempt-any-test-frame
	// oracle hid).
	stopReads := objectgraph.TrackUndeclaredReadsFromProduct()
	t.Cleanup(stopReads)
	disclosure.ResetCounts()
	t.Cleanup(disclosure.ResetCounts)
	if enforce {
		t.Cleanup(objectgraph.SetModeForTest(objectgraph.WriterGateEnforce))
		t.Cleanup(disclosure.SetModeForTest(disclosure.ModeEnforce))
	}

	// Management intake: the story is open (the fixture opened one), the
	// parent delegates the arrival.
	f.parentTurn()
	if out := f.model.lastOutput(); !strings.Contains(out, "rlm:delegate:") {
		t.Fatalf("management did not delegate: %s", out)
	}

	// 7b: Delegate's synthesized per-group Casts went through the gate.
	delegated := uint64(0)
	for key, n := range disclosure.Counts() {
		if strings.HasPrefix(key, "desk_message|") {
			delegated += n
		}
	}
	if delegated == 0 {
		t.Errorf("Delegate's per-group Casts were not gated: disclosure counts %v", disclosure.Counts())
	}

	// Texture's cell Casts to research; the cast folds into the turn.
	owner, doc := "owner-cz-texture", "doc-cz"
	now := time.Now().UTC()
	seed(func() {
		if err := s.CreateDocument(ctx, types.Document{DocID: doc, OwnerID: owner, Title: "Chips", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateRevision(ctx, types.Revision{RevisionID: "rev-cz-1", DocID: doc, OwnerID: owner, AuthorKind: types.AuthorUser,
			Content: "# Chips\n\nWhat shipped this week?", Citations: json.RawMessage("[]"), CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	})
	trec := types.RunRecord{RunID: "run-cz-texture", AgentID: "texture:" + doc, OwnerID: owner, ComputerID: f.computer,
		ChannelID: doc, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, State: types.RunRunning,
		Metadata: map[string]any{"doc_id": doc}}
	seed(func() {
		if err := s.CreateRun(ctx, trec); err != nil {
			t.Fatal(err)
		}
	})
	// The Texture run read the owner's document: its label rides the run.
	rt.taintRun(trec.RunID, types.NewLabelSet(types.OwnerPrivateLabel("document")))
	if _, err := rt.flushRunLabels(ctx, owner, f.computer, trec.RunID); err != nil {
		t.Fatal(err)
	}
	texec := toolregistry.ExecutionContext{RunID: trec.RunID, AgentID: trec.AgentID, OwnerID: owner, ChannelID: doc, ComputerID: f.computer,
		Profile: agentprofile.Texture, Role: agentprofile.Texture, RunRecord: &trec}
	ttool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Texture)
	runCell := func(tool toolregistry.Tool, exec toolregistry.ExecutionContext, src string) string {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		// The run's execution context carries its desk as the reader
		// (runtime.go), which an in-process cell must do as the run does.
		cellCtx := withDeskReader(withDeskWriter(ctx, exec.Profile), exec.Profile)
		cellCtx = objectgraph.WithLeases(cellCtx, rt.leaseAuthority(), exec.RunID)
		out, err := tool.Func(toolregistry.WithExecutionContext(cellCtx, exec), payload)
		if err != nil {
			out += err.Error()
		}
		return out
	}
	out := runCell(ttool, texec, `choir.Write("@replace b2\nWe are finding out what shipped.", "acknowledge")
choir.Cast("research", "cover chips", map[string]interface{}{"kind":"question","summary":"chips","questions":[]string{"what shipped?"}}, map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"})`)
	if len(auth.seen) != 1 {
		t.Fatalf("the Texture cell's turn was not committed (calls %d): %s", len(auth.seen), out)
	}
	// 7a: the folded Cast reached the disclosure gate and was allowed: Texture
	// to research is on the owner's delegation chain (enforce flip D1), so it
	// is a counted allow, not an ask.
	if n := disclosure.Counts()["desk_message|-|allow|allowed"]; n == 0 {
		t.Errorf("the folded Cast to research was not gated and allowed: disclosure counts %v", disclosure.Counts())
	}

	// Research reads the Cast on its records and reports to Texture; it also
	// searches the corpus.
	texture, research := "texture:"+doc, "research:"+doc
	cast := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: f.computer, CommandID: "seed-cz-cast", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:cz", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "cover chips", Body: `{"kind":"question","summary":"chips"}`},
		Cast:      &types.CommitmentCast{},
		Inbox:     research, SenderInbox: texture, Addressee: research,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: texture},
	}}}
	cast.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(cast)
	seed(func() {
		if _, err := s.CommitInboxRecords(ctx, cast); err != nil {
			t.Fatal(err)
		}
	})
	rrec := types.RunRecord{RunID: "run-cz-research", AgentID: research, OwnerID: owner, ComputerID: f.computer,
		ChannelID: doc, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning}
	seed(func() {
		if err := s.CreateRun(ctx, rrec); err != nil {
			t.Fatal(err)
		}
	})
	rexec := toolregistry.ExecutionContext{RunID: rrec.RunID, AgentID: rrec.AgentID, OwnerID: owner, ChannelID: doc, ComputerID: f.computer,
		Profile: agentprofile.Research, Role: agentprofile.Research, RunRecord: &rrec}
	rtool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Research)
	out = runCell(rtool, rexec, `rs := choir.Records()
fmt.Println(len(rs))
r, err := choir.SearchWireCorpus("harbor", 5)
fmt.Println(r, err)
choir.ReportPacket("texture", map[string]interface{}{"kind":"evidence_update","summary":"it shipped","claims":[]interface{}{map[string]interface{}{"text":"it shipped"}},"work_disposition":"completed"}, "")`)
	got, err := s.ListInboxRecords(ctx, owner, f.computer, texture, 0, 10)
	if err != nil || len(got) == 0 {
		t.Fatalf("research did not report to Texture: %+v, %v\n%s", got, err, out)
	}

	// MA3.7c: the host paths a real Texture and research trajectory takes
	// outside any desk context. d@e counted texture_document and
	// texture_revision reads with no reader there; each is driven here with a
	// bare context and must read as its custodian (or the owner on a route).
	host := context.Background()
	if _, _, _, err := rt.textureRunRevision(host, &trec, ""); err != nil {
		t.Fatalf("texture frame read: %v", err)
	}
	if _, _, _, err := rt.rebaseRoot(host, owner, f.computer); err != nil {
		t.Fatalf("rebase root: %v", err)
	}
	if _, err := rt.calibrationSnapshotFor(host, owner, f.computer, 0); err != nil {
		t.Fatalf("calibration snapshot: %v", err)
	}
	// The slot starter reads the management task kind on a bare context
	// (d@e 847ad2417 counted it undeclared).
	if _, err := rt.startQueuedManagementTasks(host, f.owner, f.computer); err != nil {
		t.Fatalf("start queued tasks: %v", err)
	}
	for _, method := range []string{http.MethodGet} {
		req := httptest.NewRequest(method, "/api/computer-settings", nil)
		req.Header.Set("X-Authenticated-User", owner)
		(&APIHandler{rt: rt}).HandleComputerSettings(httptest.NewRecorder(), req)
	}

	// The gates.
	var crossings []string
	for key, n := range objectgraph.WriterGateCounts() {
		if strings.Contains(key, "|undeclared|") {
			continue // the fixtures' own seeding
		}
		crossings = append(crossings, "writer "+key+" x"+countStr(n))
	}
	for key, n := range objectgraph.ReaderGateCounts() {
		if strings.Contains(key, "|undeclared|") {
			continue // the fixtures' own seeding; the oracles below see the product
		}
		crossings = append(crossings, "reader "+key+" x"+countStr(n))
	}
	for key, n := range objectgraph.UndeclaredReads() {
		crossings = append(crossings, "undeclared read "+key+" x"+countStr(n))
	}
	for key, n := range objectgraph.UndeclaredWrites() {
		crossings = append(crossings, "undeclared write "+key+" x"+countStr(n))
	}
	sort.Strings(crossings)
	for _, c := range crossings {
		justified := false
		for allowed := range counterZeroJustified {
			if strings.Contains(c, allowed) {
				justified = true
			}
		}
		if !justified {
			t.Errorf("crossing on a built-in kind: %s", c)
		}
	}
	// An intake task's report to its parent: the outward message is stamped
	// with the parent, so it reads as management to management. It carries the
	// capture, computers and story labels the day showed asking (d@e e27a0522c).
	mgmtRec := types.RunRecord{RunID: "run-cz-task-report", AgentID: "agent-task-report", OwnerID: f.owner, ComputerID: f.computer, ChannelID: "ch-task-report",
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunRunning, Prompt: "report"}
	seed(func() {
		if err := s.CreateRun(ctx, mgmtRec); err != nil {
			t.Fatal(err)
		}
	})
	rt.taintRun(mgmtRec.RunID, types.NewLabelSet(types.OwnerPrivateLabel("capture"), types.OwnerPrivateLabel("computers"), types.OwnerPrivateLabel("story")))
	if _, err := rt.flushRunLabels(ctx, f.owner, f.computer, mgmtRec.RunID); err != nil {
		t.Fatal(err)
	}
	taskRun := toolregistry.ExecutionContext{RunID: mgmtRec.RunID, AgentID: mgmtRec.AgentID, OwnerID: f.owner, ComputerID: f.computer, RunRecord: &mgmtRec}
	if err := rt.gateDisclosure(ctx, disclosureCall{exec: taskRun, deskRole: agentprofile.Management, sink: disclosure.SinkDeskMessage, target: agentprofile.Management + ":" + f.owner, kind: "report"}); err != nil {
		t.Errorf("the task's report to its parent was refused: %v", err)
	}
	// Management declines a capture to its source's inbox (d@e 847ad2417 asked
	// on management->source|decline with the capture and story labels).
	if err := rt.gateDisclosure(ctx, disclosureCall{exec: taskRun, deskRole: agentprofile.Management, sink: disclosure.SinkDeskMessage, target: captureSourceDesk + ":rss:cz", kind: "decline"}); err != nil {
		t.Errorf("management's decline to the source inbox was refused: %v", err)
	}
	// Enforce flip S1: the day leaves no disclosure ask site (the intake
	// delegation to the task role included).
	for site, n := range disclosure.Sites() {
		t.Errorf("disclosure ask site remains: %s x%d", site, n)
	}
	for key, n := range disclosure.Counts() {
		if strings.Contains(key, "|ask|") {
			t.Errorf("disclosure ask remains: %s x%d", key, n)
		}
	}
}

func countStr(n uint64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// MA3.7b 7b: Resolve, Disagreement and Cancel are addressed non-envelope acts;
// when they name a desk (or, for a retract, the desk the retracted act
// addressed) a built-in desk's run carrying owner-private labels is counted at
// the desk-message sink and not blocked. The addressee is management and the
// sender research: Texture's messages to research are admitted by the
// texture.brief lease, and its messages to management are on the owner flow's
// delegation chain (enforce flip D1), so neither is an ask.
func TestResolveDisagreementAndCancelAreGatedAndCounted(t *testing.T) {
	f := newDisclosureFixture(t, "unusedgate7b", 62841)
	exec := f.run(t, "run-7b", agentprofile.Research, "write", types.OwnerPrivateLabel("document"))
	r := &rlmCallReduction{mb: f.rt, rec: exec.RunRecord, scope: ReductionScope{RunID: exec.RunID, OwnerID: f.owner, ComputerID: f.computer, DeskAgentID: exec.AgentID}}
	for i, in := range []yaegikernel.StagedIntent{
		{LocalID: "r1", Kind: yaegikernel.IntentResolve, ToDesk: agentprofile.Management, TargetRef: "rec-1", Resolve: `{"verdict":"confirmed"}`},
		{LocalID: "d1", Kind: yaegikernel.IntentDisagreement, ToDesk: agentprofile.Management, Disagreement: `{"commitment_id":"rec-1"}`},
		{LocalID: "c1", Kind: yaegikernel.IntentCancel, ToDesk: agentprofile.Management, TargetRef: "rec-1"},
	} {
		disclosure.ResetCounts()
		if err := r.gateMessageDisclosure(context.Background(), in); err != nil {
			t.Fatalf("%s was blocked on a built-in desk: %v", in.Kind, err)
		}
		if n := disclosure.Counts()["desk_message|owner_private|ask|observed"]; n != 1 {
			t.Errorf("%d: %s was not counted at the desk-message sink: %v", i, in.Kind, disclosure.Counts())
		}
	}
	// A retract names no desk: it reaches the desk the retracted act addressed.
	target := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:7b", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "cover chips"},
		Cast:      &types.CommitmentCast{}, Inbox: "management:doc-7b", SenderInbox: "texture:doc-7b", Addressee: "management:doc-7b",
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: "texture:doc-7b"},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "seed-7b", Records: []types.CommitmentRecord{target}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := f.s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	disclosure.ResetCounts()
	if err := r.gateMessageDisclosure(context.Background(), yaegikernel.StagedIntent{LocalID: "c2", Kind: yaegikernel.IntentCancel, TargetRef: target.RecordID}); err != nil {
		t.Fatal(err)
	}
	if n := disclosure.Counts()["desk_message|owner_private|ask|observed"]; n != 1 {
		t.Errorf("an unaddressed retract was not gated at the desk it stands down: %v", disclosure.Counts())
	}
}

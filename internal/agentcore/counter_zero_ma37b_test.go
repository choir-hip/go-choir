package agentcore

import (
	"context"
	"encoding/json"
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
//   - The research corpus search (choir.SearchWireCorpus) is the one named
//     residual: it lists Texture documents and revisions (X1, a texture.search
//     lease held by a research child, MA4.6) and reads world events and capture
//     labels (X15 and E4, shared world content that the reader gate does not
//     yet admit to research). They are counted, not blocked, and the enforce
//     flip must close them first.
//   - Nothing else is excused. A key landing here is a crossing the next slice
//     (the enforce flip) would block.
var counterZeroJustified = map[string]string{
	"choir.texture_document|desk:research|wrong_reader": "X1: corpus search lists Texture documents; the texture.search lease (MA4.6)",
	"choir.texture_revision|desk:research|wrong_reader": "X1: corpus search reads a story's head revision; the texture.search lease (MA4.6)",
	"choir.world_event|desk:research|wrong_reader":      "X15/E4: world events are shared world content; admit research to the kind before the flip",
	"choir.capture_label|desk:research|wrong_reader":    "X15/E4: capture labels ride the corpus search; admit research before the flip",
}

func TestRepresentativeDayCountsNoCrossingOnBuiltInKinds(t *testing.T) {
	f := newDelegationFixture(t, "owner-counter-zero")
	ctx := context.Background()
	rt, s := f.rt, f.s
	auth := &stubCellAuthorizer{}
	rt.SetTextureCellAuthorizer(auth)

	seedArrival(t, s, f.owner, f.computer, "rss:cz", []types.CaptureArrival{
		{CaptureID: "cap-cz1", Title: "Harbor reopens", URL: "https://cz.example/1"},
	})
	objectgraph.ResetWriterGateCounts()
	objectgraph.ResetReaderGateCounts()
	objectgraph.ResetUndeclaredWrites()
	objectgraph.ResetUndeclaredReads()
	disclosure.ResetCounts()
	t.Cleanup(disclosure.ResetCounts)

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
	if err := s.CreateDocument(ctx, types.Document{DocID: doc, OwnerID: owner, Title: "Chips", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateRevision(ctx, types.Revision{RevisionID: "rev-cz-1", DocID: doc, OwnerID: owner, AuthorKind: types.AuthorUser,
		Content: "# Chips\n\nWhat shipped this week?", Citations: json.RawMessage("[]"), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	trec := types.RunRecord{RunID: "run-cz-texture", AgentID: "texture:" + doc, OwnerID: owner, ComputerID: f.computer,
		ChannelID: doc, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, State: types.RunRunning,
		Metadata: map[string]any{"doc_id": doc}}
	if err := s.CreateRun(ctx, trec); err != nil {
		t.Fatal(err)
	}
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
	// 7a: the folded Cast reached the disclosure gate, counted and not blocked.
	if n := disclosure.Counts()["desk_message|owner_private|ask|observed"]; n == 0 {
		t.Errorf("the folded Cast to research was not gated: disclosure counts %v", disclosure.Counts())
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
	if _, err := s.CommitInboxRecords(ctx, cast); err != nil {
		t.Fatal(err)
	}
	rrec := types.RunRecord{RunID: "run-cz-research", AgentID: research, OwnerID: owner, ComputerID: f.computer,
		ChannelID: doc, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning}
	if err := s.CreateRun(ctx, rrec); err != nil {
		t.Fatal(err)
	}
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
}

func countStr(n uint64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// MA3.7b 7b: Resolve, Disagreement and Cancel are addressed non-envelope acts;
// when they name a desk (or, for a retract, the desk the retracted act
// addressed) a built-in desk's run carrying owner-private labels is counted at
// the desk-message sink and not blocked.
func TestResolveDisagreementAndCancelAreGatedAndCounted(t *testing.T) {
	f := newDisclosureFixture(t, "unusedgate7b", 62841)
	exec := f.run(t, "run-7b", agentprofile.Texture, "write", types.OwnerPrivateLabel("document"))
	r := &rlmCallReduction{mb: f.rt, rec: exec.RunRecord, scope: ReductionScope{RunID: exec.RunID, OwnerID: f.owner, ComputerID: f.computer, DeskAgentID: exec.AgentID}}
	for i, in := range []yaegikernel.StagedIntent{
		{LocalID: "r1", Kind: yaegikernel.IntentResolve, ToDesk: agentprofile.Research, TargetRef: "rec-1", Resolve: `{"verdict":"confirmed"}`},
		{LocalID: "d1", Kind: yaegikernel.IntentDisagreement, ToDesk: agentprofile.Research, Disagreement: `{"commitment_id":"rec-1"}`},
		{LocalID: "c1", Kind: yaegikernel.IntentCancel, ToDesk: agentprofile.Research, TargetRef: "rec-1"},
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
		Cast:      &types.CommitmentCast{}, Inbox: "research:doc-7b", SenderInbox: "texture:doc-7b", Addressee: "research:doc-7b",
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

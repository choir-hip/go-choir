package agentcore

// Record-native lifecycle act reductions (the staged Message intent and its
// reducer branches were deleted with the emit and message channels, MA1.1).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func TestCommitTrayLifecycleProducerReportMintsRecordNativePacket(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := bindResearchControlFixture(t, rt, s, "owner-report", "report")
	run := fixture.run
	work, err := s.GetLifecycleWorkItem(context.Background(), run.OwnerID, run.ComputerID, fixture.workID)
	if err != nil {
		t.Fatal(err)
	}
	run.RequestedByRunID, _ = work.Details["requested_by_run_id"].(string)
	run.Metadata["requested_by_run_id"] = work.Details["requested_by_run_id"]
	run.Metadata["requested_by_agent_id"] = work.Details["requested_by_agent_id"]
	run.Metadata["requested_by_profile"] = work.Details["requested_by_profile"]
	reproject := types.ReplaceLifecycleActivationRequest{
		OwnerID: run.OwnerID, ComputerID: run.ComputerID,
		CommandID: "reproject-report", TrajectoryID: run.TrajectoryID,
		AgentID: run.AgentID, Run: run,
	}
	reproject.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(reproject)
	if _, err := s.ReplaceLifecycleActivation(context.Background(), reproject); err != nil {
		t.Fatal(err)
	}

	textureAgentID := fixture.control.AgentID
	scope := ReductionScope{
		FromAgentID: run.AgentID, DeskAgentID: run.AgentID,
		FromRole: agentprofile.Research, ChannelID: run.ChannelID,
		RunID: run.RunID, OwnerID: run.OwnerID, CellID: "cell-report",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: run.RunID, AgentID: run.AgentID, OwnerID: run.OwnerID,
		ChannelID: run.ChannelID, ComputerID: run.ComputerID,
		Profile: agentprofile.Research, Role: agentprofile.Research,
		RunRecord: &run,
	})
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &run}

	packet := types.SourcePacket{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          "evidence_update",
		Summary:       "report-path finding",
		Claims:        []types.CoagentPacketClaim{{Text: "claim"}},
		Sources:       []types.CoagentPacketSource{{SourceID: "s1", Kind: "web_page", Target: types.CoagentPacketSourceTarget{URI: "https://example.com/c", Title: "C"}}},
	}
	packetJSON, _ := json.Marshal(packet)

	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "report-1", Kind: yaegikernel.IntentReport, ToDesk: textureAgentID, Packet: string(packetJSON), Claim: "report-path finding"},
	}, 1); err != nil {
		t.Fatalf("commitTray lifecycle producer report failed: %v", err)
	}

	pending, err := s.ListAllPendingLifecycleUpdates(ctx, run.OwnerID, run.ComputerID, textureAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("lifecycle producer report did not queue: pending=%d", len(pending))
	}
	if pending[0].ProducerUpdateID == "" || pending[0].WorkItemID != fixture.workID ||
		pending[0].Direction != types.LifecyclePacketDirectionProducerReport ||
		pending[0].SourceRecordID != "cell-report:report:report-1" {
		t.Fatalf("record-native report packet malformed: %+v", pending[0])
	}
	messages, err := s.ListChannelMessages(ctx, run.OwnerID, run.ChannelID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		if strings.Contains(message.Content, "report-path finding") {
			t.Fatalf("record-native report leaked channel envelope: %+v", message)
		}
	}
	if !reduction.receipt.Committed || len(reduction.receipt.Intents) != 1 {
		t.Fatalf("reduction receipt = %+v", reduction.receipt)
	}
}

// Desk-name ToDesk ("texture", as overlays document it) resolves to the
// document's Texture inbox — the record-native report must
// not require the cell to spell texture:<doc_id>. (s0m-report-desk-target-resolution)
func TestCommitTrayLifecycleProducerReportResolvesDeskName(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := bindResearchControlFixture(t, rt, s, "owner-desk-report", "desk-report")
	run := fixture.run
	work, err := s.GetLifecycleWorkItem(context.Background(), run.OwnerID, run.ComputerID, fixture.workID)
	if err != nil {
		t.Fatal(err)
	}
	run.RequestedByRunID, _ = work.Details["requested_by_run_id"].(string)
	run.Metadata["requested_by_run_id"] = work.Details["requested_by_run_id"]
	run.Metadata["requested_by_agent_id"] = work.Details["requested_by_agent_id"]
	run.Metadata["requested_by_profile"] = work.Details["requested_by_profile"]
	reproject := types.ReplaceLifecycleActivationRequest{
		OwnerID: run.OwnerID, ComputerID: run.ComputerID,
		CommandID: "reproject-desk-report", TrajectoryID: run.TrajectoryID,
		AgentID: run.AgentID, Run: run,
	}
	reproject.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(reproject)
	if _, err := s.ReplaceLifecycleActivation(context.Background(), reproject); err != nil {
		t.Fatal(err)
	}

	textureAgentID := fixture.control.AgentID
	scope := ReductionScope{
		FromAgentID: run.AgentID, DeskAgentID: run.AgentID,
		FromRole: agentprofile.Research, ChannelID: run.ChannelID,
		RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID,
		CellID: "cell-desk-report",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: run.RunID, AgentID: run.AgentID, OwnerID: run.OwnerID,
		ChannelID: run.ChannelID, ComputerID: run.ComputerID,
		Profile: agentprofile.Research, Role: agentprofile.Research,
		RunRecord: &run,
	})
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &run}

	packet := types.SourcePacket{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          "evidence_update",
		Summary:       "desk-name report finding",
		Claims:        []types.CoagentPacketClaim{{Text: "claim"}},
		Sources:       []types.CoagentPacketSource{{SourceID: "s1", Kind: "web_page", Target: types.CoagentPacketSourceTarget{URI: "https://example.com/c", Title: "C"}}},
	}
	packetJSON, _ := json.Marshal(packet)

	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "report-1", Kind: yaegikernel.IntentReport, ToDesk: "texture", Packet: string(packetJSON), Claim: "desk-name report finding"},
	}, 1); err != nil {
		t.Fatalf("commitTray desk-name producer report failed: %v", err)
	}

	// Cut 1 L3a: research's conversation with Texture is on records, so the
	// report lands in Texture's inbox as a record and queues no packet.
	pending, err := s.ListAllPendingLifecycleUpdates(ctx, run.OwnerID, run.ComputerID, textureAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("research report queued a packet on the records route: %+v", pending)
	}
	inbox, ok, _ := recordsRoute(run.AgentID, run.OwnerID, "texture")
	if !ok {
		t.Fatalf("no Texture inbox for research desk %q", run.AgentID)
	}
	recs, err := s.ListInboxRecords(ctx, run.OwnerID, run.ComputerID, inbox, 0, 10)
	if err != nil || len(recs) != 1 || recs[0].RecordID != "cell-desk-report:report:report-1" || recs[0].SenderInbox != run.AgentID {
		t.Fatalf("desk-name report record = %+v, %v", recs, err)
	}
	_ = textureAgentID
}

// Ledger-only kinds (resolve, disagreement) mint through CommitLifecycleAct
// under one replay-idempotent authority — no packet, no envelope, and the
// caller lifecycle version bump marks the record-native path (the legacy
// append arm never touches agent versions).
func TestCommitTrayLifecycleResolveDisagreementMintRecordNative(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := seedTextureLifecycleControl(t, s, "owner-resolve", "resv", "research:control-resv", agentprofile.Research)
	caller, err := s.GetLifecycleRun(context.Background(), fixture.run.OwnerID, fixture.run.ComputerID, "texture-run-resv")
	if err != nil {
		t.Fatal(err)
	}
	callerAgent, err := s.GetAgentByScope(context.Background(), caller.OwnerID, caller.ComputerID, caller.AgentID)
	if err != nil {
		t.Fatal(err)
	}

	scope := ReductionScope{
		FromAgentID: caller.AgentID, DeskAgentID: caller.AgentID,
		FromRole: agentprofile.Texture, ChannelID: caller.ChannelID,
		RunID: caller.RunID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		CellID: "cell-resolve",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: caller.RunID, AgentID: caller.AgentID, OwnerID: caller.OwnerID,
		ChannelID: caller.ChannelID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture,
		RunRecord: &caller,
	})
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &caller}

	resolveBody, _ := json.Marshal(types.CommitmentResolve{Verdict: "confirmed", EvidenceRefs: []string{"evidence://x"}})
	disagreementBody, _ := json.Marshal(types.CommitmentDisagreement{CommitmentID: "cell-resolve:resolve:res-1", ScorerVerdict: "contradicted", ResolverVerdict: "confirmed"})
	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "res-1", Kind: yaegikernel.IntentResolve, TargetRef: "some-ask", Resolve: string(resolveBody)},
		{LocalID: "dis-1", Kind: yaegikernel.IntentDisagreement, Disagreement: string(disagreementBody)},
	}, 1); err != nil {
		t.Fatalf("commitTray resolve+disagreement failed: %v", err)
	}

	records, err := s.ListCommitmentRecords(ctx, caller.OwnerID, caller.ComputerID, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]types.CommitmentRecord{}
	for _, rec := range records {
		byID[rec.RecordID] = rec
	}
	if rec := byID["cell-resolve:resolve:res-1"]; rec.Resolve == nil || rec.Resolve.Verdict != "confirmed" {
		t.Fatalf("resolve record malformed: %+v", rec)
	}
	if rec := byID["cell-resolve:disagreement:dis-1"]; rec.Disagreement == nil || rec.Disagreement.ScorerVerdict != "contradicted" {
		t.Fatalf("disagreement record malformed: %+v", rec)
	}
	// Discriminator: CommitLifecycleAct bumps the caller agent version; the
	// legacy append+envelope arm does not.
	after, err := s.GetAgentByScope(context.Background(), caller.OwnerID, caller.ComputerID, caller.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if after.LifecycleVersion != callerAgent.LifecycleVersion+2 {
		t.Fatalf("caller lifecycle version %d → %d, want +2 (two record-native acts)", callerAgent.LifecycleVersion, after.LifecycleVersion)
	}
	// No envelope leaked: the channel gains no message for ledger-only acts.
	messages, err := s.ListChannelMessages(ctx, caller.OwnerID, caller.ChannelID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range messages {
		if strings.Contains(m.Content, "resolve") || strings.Contains(m.Content, "disagreement") {
			t.Fatalf("ledger-only act leaked channel envelope: %+v", m)
		}
	}
}

// Addressed directive subtypes (escalate, cast, retract) mint record+directive
// packet atomically via CommitLifecycleAct. Escalate's addressee is the
// persistent management desk; retract's addressee derives from the retracted
// record (the intent itself is unaddressed).
func TestCommitTrayLifecycleEscalateCastRetractMintRecordNative(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := seedTextureLifecycleControl(t, s, "owner-esc", "esc", "research:control-esc", agentprofile.Research)
	caller, err := s.GetLifecycleRun(context.Background(), fixture.run.OwnerID, fixture.run.ComputerID, "texture-run-esc")
	if err != nil {
		t.Fatal(err)
	}
	managementID := persistentManagementAgentID(caller.OwnerID)
	now := time.Now().UTC()
	if err := s.UpsertAgent(context.Background(), types.AgentRecord{
		AgentID: managementID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Management, Role: agentprofile.Management,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	scope := ReductionScope{
		FromAgentID: caller.AgentID, DeskAgentID: caller.AgentID,
		FromRole: agentprofile.Texture, ChannelID: caller.ChannelID,
		RunID: caller.RunID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		CellID: "cell-directives",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: caller.RunID, AgentID: caller.AgentID, OwnerID: caller.OwnerID,
		ChannelID: caller.ChannelID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture,
		RunRecord: &caller,
	})
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &caller}

	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "esc-1", Kind: yaegikernel.IntentEscalate, ToDesk: "management", Body: "evidence window expired"},
	}, 1); err != nil {
		t.Fatalf("commitTray escalate failed: %v", err)
	}

	pending, err := s.ListAllPendingLifecycleUpdates(ctx, caller.OwnerID, caller.ComputerID, managementID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("escalate directive did not queue on management: pending=%d", len(pending))
	}
	if pending[0].Direction != types.LifecyclePacketDirectionDirective || pending[0].SourceRecordID != "cell-directives:escalate:esc-1" {
		t.Fatalf("escalate packet malformed: %+v", pending[0])
	}

	// No envelope for the escalate act.
	messages, _ := s.ListChannelMessages(ctx, caller.OwnerID, caller.ChannelID, 0, 100)
	for _, m := range messages {
		if strings.Contains(m.Content, "evidence window expired") {
			t.Fatalf("escalate leaked channel envelope: %+v", m)
		}
	}

	// Retract the escalation: the unaddressed cancel resolves its stand-down
	// target from the retracted record's addressee (management).
	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "cancel-1", Kind: yaegikernel.IntentCancel, TargetRef: "cell-directives:escalate:esc-1"},
	}, 1); err != nil {
		t.Fatalf("commitTray retract failed: %v", err)
	}
	pending, err = s.ListAllPendingLifecycleUpdates(ctx, caller.OwnerID, caller.ComputerID, managementID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Fatalf("retract stand-down did not queue on management: pending=%d", len(pending))
	}
	retract := pending[1]
	if retract.Direction != types.LifecyclePacketDirectionDirective ||
		retract.SourceRecordID != "cell-directives:cancel:cancel-1" ||
		!strings.Contains(strings.Join(retract.Packet.Notes, " "), "retract_target:cell-directives:escalate:esc-1") {
		t.Fatalf("retract packet malformed: %+v", retract)
	}
}

// C1e B1: the packet path no longer casts engineering. An act-path Cast to
// engineering (no records parent) is refused before anything commits: no
// record, no directive packet, no assignment.
func TestCommitTrayPacketPathEngineeringCastIsRefused(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := seedTextureLifecycleControl(t, s, "owner-cast", "cast", "research:control-cast", agentprofile.Research)
	caller, err := s.GetLifecycleRun(context.Background(), fixture.run.OwnerID, fixture.run.ComputerID, "texture-run-cast")
	if err != nil {
		t.Fatal(err)
	}
	engineeringID := agentprofile.Engineering + ":" + caller.ChannelID
	now := time.Now().UTC()
	if err := s.UpsertAgent(context.Background(), types.AgentRecord{
		AgentID: engineeringID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Engineering, Role: agentprofile.Engineering, ChannelID: caller.ChannelID,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	scope := ReductionScope{
		FromAgentID: caller.AgentID, DeskAgentID: caller.AgentID,
		FromRole: agentprofile.Texture, ChannelID: caller.ChannelID,
		RunID: caller.RunID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		CellID: "cell-cast",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: caller.RunID, AgentID: caller.AgentID, OwnerID: caller.OwnerID,
		ChannelID: caller.ChannelID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture,
		RunRecord: &caller,
	})
	// The act path under test is the generic desk one; since protocol P2a a
	// Texture run's Cast or Ask folds into its turn and may not address
	// engineering, so the reducer runs this fixture's caller as a desk.
	deskCaller := caller
	deskCaller.AgentProfile = agentprofile.Management
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &deskCaller}

	err = reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "cast-1", Kind: yaegikernel.IntentCast, ToDesk: "engineering", Objective: "build the widget", Statement: "spec body"},
	}, 1)
	if err == nil || !strings.Contains(err.Error(), "naming the Texture request it serves") {
		t.Fatalf("packet-path engineering Cast was not refused: %v", err)
	}
	pending, err := s.ListAllPendingLifecycleUpdates(ctx, caller.OwnerID, caller.ComputerID, engineeringID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("refused engineering Cast queued a directive: %+v", pending)
	}
	if assignments, err := s.ListEngineeringAssignments(ctx, caller.OwnerID, caller.ComputerID, fixture.trajectoryID); err != nil || len(assignments) != 0 {
		t.Fatalf("refused engineering Cast opened an assignment: %+v, %v", assignments, err)
	}
}

// Protocol P2d1: only a Cast to engineering opens an engineering assignment.
// A management Cast to research commits its record and directive packet and
// opens nothing (before P2d1 every act-path Cast ran delegated-cast admission,
// which in this runtime fails on the missing capsule executor).
func TestCommitTrayLifecycleCastToResearchOpensNoAssignment(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := seedTextureLifecycleControl(t, s, "owner-cast-research", "castr", "research:control-castr", agentprofile.Research)
	caller, err := s.GetLifecycleRun(context.Background(), fixture.run.OwnerID, fixture.run.ComputerID, "texture-run-castr")
	if err != nil {
		t.Fatal(err)
	}
	researchID := agentprofile.Research + ":" + caller.ChannelID
	now := time.Now().UTC()
	if err := s.UpsertAgent(context.Background(), types.AgentRecord{
		AgentID: researchID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Research, Role: agentprofile.Research, ChannelID: caller.ChannelID,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	scope := ReductionScope{
		FromAgentID: caller.AgentID, DeskAgentID: caller.AgentID,
		FromRole: agentprofile.Management, ChannelID: caller.ChannelID,
		RunID: caller.RunID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		CellID: "cell-castr",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: caller.RunID, AgentID: caller.AgentID, OwnerID: caller.OwnerID,
		ChannelID: caller.ChannelID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Management, Role: agentprofile.Management,
		RunRecord: &caller,
	})
	deskCaller := caller
	deskCaller.AgentProfile = agentprofile.Management
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &deskCaller}

	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "cast-r", Kind: yaegikernel.IntentCast, ToDesk: "research", Objective: "find the filing", Statement: "spec body"},
	}, 1); err != nil {
		t.Fatalf("a Cast to research ran delegated-cast admission: %v", err)
	}
	pending, err := s.ListAllPendingLifecycleUpdates(ctx, caller.OwnerID, caller.ComputerID, researchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Direction != types.LifecyclePacketDirectionDirective ||
		pending[0].SourceRecordID != "cell-castr:cast:cast-r" {
		t.Fatalf("cast to research did not queue its directive: %+v", pending)
	}
	assignments, err := s.ListEngineeringAssignmentsForComputer(ctx, caller.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 0 {
		t.Fatalf("a Cast to research opened %d engineering assignment(s)", len(assignments))
	}
}

func TestCastOpensEngineeringAssignmentOnlyForEngineering(t *testing.T) {
	for desk, want := range map[string]bool{
		"engineering": true, "engineering:doc-1": true, " Engineering ": true,
		"research": false, "research:doc-1": false, "management": false,
		"texture:doc-1": false, "": false, "not-a-desk": false,
	} {
		if got := castOpensEngineeringAssignment(desk); got != want {
			t.Errorf("castOpensEngineeringAssignment(%q) = %v, want %v", desk, got, want)
		}
	}
}

// Ask/precommit cutover: an addressed staked question mints record+directive
// packet (Kind=question) in one CommitLifecycleAct; an unaddressed precommit
// mints the record ledger-only.
func TestCommitTrayLifecycleAskPrecommitMintRecordNative(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := seedTextureLifecycleControl(t, s, "owner-ask", "ask", "research:control-ask", agentprofile.Research)
	caller, err := s.GetLifecycleRun(context.Background(), fixture.run.OwnerID, fixture.run.ComputerID, "texture-run-ask")
	if err != nil {
		t.Fatal(err)
	}
	engineeringID := agentprofile.Engineering + ":" + caller.ChannelID
	now := time.Now().UTC()
	if err := s.UpsertAgent(context.Background(), types.AgentRecord{
		AgentID: engineeringID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Engineering, Role: agentprofile.Engineering, ChannelID: caller.ChannelID,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	scope := ReductionScope{
		FromAgentID: caller.AgentID, DeskAgentID: caller.AgentID,
		FromRole: agentprofile.Texture, ChannelID: caller.ChannelID,
		RunID: caller.RunID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		CellID: "cell-ask",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: caller.RunID, AgentID: caller.AgentID, OwnerID: caller.OwnerID,
		ChannelID: caller.ChannelID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture,
		RunRecord: &caller,
	})
	deskCaller := caller
	deskCaller.AgentProfile = agentprofile.Management
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &deskCaller}

	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "ask-1", Kind: yaegikernel.IntentAsk, ToDesk: "engineering", Question: "does the build pass?"},
		{LocalID: "pre-1", Kind: yaegikernel.IntentPrecommit, Precommit: `{"question":"will deploy land today?"}`},
	}, 1); err != nil {
		t.Fatal(err)
	}

	pending, err := s.ListAllPendingLifecycleUpdates(ctx, caller.OwnerID, caller.ComputerID, engineeringID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("ask did not queue on engineering: pending=%d", len(pending))
	}
	if pending[0].Packet.Kind != "question" ||
		pending[0].SourceRecordID != "cell-ask:ask:ask-1" ||
		!strings.Contains(strings.Join(pending[0].Packet.Notes, " "), "question:does the build pass?") {
		t.Fatalf("ask packet malformed: %+v", pending[0])
	}

	// The unaddressed precommit minted ledger-only: record present, no second
	// packet, no envelope.
	rec, err := s.GetCommitmentRecord(ctx, caller.OwnerID, caller.ComputerID, "cell-ask:precommit:pre-1")
	if err != nil || rec == nil || rec.RecordKind() != types.CommitmentKindPrecommit {
		t.Fatalf("unaddressed precommit record missing: rec=%v err=%v", rec, err)
	}
	messages, err := s.ListChannelMessages(ctx, caller.OwnerID, caller.ChannelID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range messages {
		if strings.Contains(m.Content, "ask-1") || strings.Contains(m.Content, "pre-1") {
			t.Fatalf("lifecycle act leaked channel envelope: %+v", m)
		}
	}
}

// Reply cutover: a desk answering a staked question mints a report-kind
// record plus a directive packet (Kind=evidence_update, reply_to: link) in
// one CommitLifecycleAct — no envelope.
func TestCommitTrayLifecycleReplyMintsRecordNative(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := seedTextureLifecycleControl(t, s, "owner-reply", "reply", "research:control-reply", agentprofile.Research)
	caller, err := s.GetLifecycleRun(context.Background(), fixture.run.OwnerID, fixture.run.ComputerID, "texture-run-reply")
	if err != nil {
		t.Fatal(err)
	}
	researchID := agentprofile.Research + ":" + caller.ChannelID
	now := time.Now().UTC()
	if err := s.UpsertAgent(context.Background(), types.AgentRecord{
		AgentID: researchID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Research, Role: agentprofile.Research, ChannelID: caller.ChannelID,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	askID := "cell-ask:ask:ask-1"
	if _, err := s.AppendCommitmentRecord(context.Background(), caller.OwnerID, caller.ComputerID, types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    askID,
		Kind:        types.CommitmentKindPrecommit,
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: researchID, CommittedAt: now.Format(time.RFC3339Nano)},
		Precommit:   &types.CommitmentPrecommit{Question: "does the build pass?", Resolver: agentprofile.Texture},
		Prediction:  types.CommitmentPrediction{Hypothesis: "does the build pass?", CommittedAt: now.Format(time.RFC3339Nano)},
	}); err != nil {
		t.Fatal(err)
	}

	scope := ReductionScope{
		FromAgentID: caller.AgentID, DeskAgentID: caller.AgentID,
		FromRole: agentprofile.Texture, ChannelID: caller.ChannelID,
		RunID: caller.RunID, OwnerID: caller.OwnerID, ComputerID: caller.ComputerID,
		CellID: "cell-reply",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: caller.RunID, AgentID: caller.AgentID, OwnerID: caller.OwnerID,
		ChannelID: caller.ChannelID, ComputerID: caller.ComputerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture,
		RunRecord: &caller,
	})
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &caller}

	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "reply-1", Kind: yaegikernel.IntentReply, ToDesk: "research", Answer: "the build passes", TargetRef: askID},
	}, 1); err != nil {
		t.Fatal(err)
	}

	pending, err := s.ListAllPendingLifecycleUpdates(ctx, caller.OwnerID, caller.ComputerID, researchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("reply did not queue on research: pending=%d", len(pending))
	}
	if pending[0].SourceRecordID != "cell-reply:reply:reply-1" ||
		pending[0].Packet.Kind != "evidence_update" ||
		!strings.Contains(strings.Join(pending[0].Packet.Notes, " "), "reply_to:cell-ask:ask:ask-1") {
		t.Fatalf("reply packet malformed: %+v", pending[0])
	}
	rec, err := s.GetCommitmentRecord(ctx, caller.OwnerID, caller.ComputerID, "cell-reply:reply:reply-1")
	if err != nil || rec == nil || rec.RecordKind() != types.CommitmentKindReport ||
		len(rec.RelatedIDs) != 1 || rec.RelatedIDs[0] != askID {
		t.Fatalf("reply record malformed: rec=%+v err=%v", rec, err)
	}
	derived, err := s.GetCommitmentRecord(ctx, caller.OwnerID, caller.ComputerID, "cell-reply:reply:reply-1:resolve:answered")
	if err != nil || derived == nil || derived.Resolve == nil || derived.Resolve.Verdict != "answered" ||
		derived.Discrepancy != types.DiscrepancyAnswered ||
		derived.Provenance.AgentID != "system:reducer" || derived.Provenance.ResolvedAt == "" ||
		len(derived.Scores) != 0 || len(derived.RelatedIDs) != 1 || derived.RelatedIDs[0] != askID {
		t.Fatalf("reply mechanical resolve malformed: rec=%+v err=%v", derived, err)
	}
	messages, err := s.ListChannelMessages(ctx, caller.OwnerID, caller.ChannelID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range messages {
		if strings.Contains(m.Content, "reply-1") {
			t.Fatalf("reply leaked channel envelope: %+v", m)
		}
	}
}

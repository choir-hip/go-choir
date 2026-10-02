package agentcore

// Regression: a lifecycle desk (research opened via ApplyTexture controls)
// delivering findings through choir.Message used to fail twice at the queue
// boundary — commitMessageIntent never derived ProducerUpdateID (required by
// QueueLifecycleUpdate) and left WorkDisposition empty while carrying a bound
// work_item_id (violating validateUpdateWorkConsequence). Both defects dropped
// research evidence silently, which is what produced the observed hollow
// "research pending" revision accumulation on staging.

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

func TestCommitMessageIntentLifecycleResearchQueuesProducerReport(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := bindResearchControlFixture(t, rt, s, "owner-msg-queue", "msgqueue")
	run := fixture.run
	if run.AgentID == "" || run.OwnerID == "" || run.ComputerID == "" {
		t.Fatalf("bound research run = %+v, want lifecycle-projected run", run)
	}
	// Production stamps requested_by_run_id on the spawned research run; the
	// fixture proves provenance on the work item instead — lift it onto the
	// run so exactRequesterRunID resolves the way a real spawn does.
	work, err := s.GetLifecycleWorkItem(context.Background(), run.OwnerID, run.ComputerID, fixture.workID)
	if err != nil {
		t.Fatal(err)
	}
	requester, _ := work.Details["requested_by_run_id"].(string)
	if requester == "" {
		t.Fatalf("work item %s missing requested_by_run_id: %+v", fixture.workID, work.Details)
	}
	run.RequestedByRunID = requester
	run.Metadata["requested_by_run_id"] = requester
	// loadLifecycleRequesterRun also requires the requester's agent id and
	// profile on run metadata — the spawn stamps all three together.
	run.Metadata["requested_by_agent_id"] = work.Details["requested_by_agent_id"]
	run.Metadata["requested_by_profile"] = work.Details["requested_by_profile"]
	// authority.callerRun loads from the lifecycle store, not the in-memory
	// record — re-project the run so exactRequesterRunID sees the stamp.
	reproject := types.ReplaceLifecycleActivationRequest{
		OwnerID: run.OwnerID, ComputerID: run.ComputerID,
		CommandID: "reproject-msgqueue", TrajectoryID: run.TrajectoryID,
		AgentID: run.AgentID, Run: run,
	}
	reproject.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(reproject)
	if _, err := s.ReplaceLifecycleActivation(context.Background(), reproject); err != nil {
		t.Fatal(err)
	}

	textureAgentID := fixture.control.AgentID // control issuer is texture:<doc>
	if textureAgentID == "" {
		t.Fatalf("fixture control missing issuer agent: %+v", fixture.control)
	}

	scope := ReductionScope{
		FromAgentID: run.AgentID,
		DeskAgentID: run.AgentID,
		FromRole:    agentprofile.Research,
		ChannelID:   run.ChannelID,
		RunID:       run.RunID,
		OwnerID:     run.OwnerID,
		CellID:      "cell-msg-queue",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: run.RunID, AgentID: run.AgentID, OwnerID: run.OwnerID,
		ChannelID: run.ChannelID, ComputerID: run.ComputerID,
		Profile: agentprofile.Research, Role: agentprofile.Research,
		RunRecord: &run,
	})
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &run}

	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          "evidence_update",
		Summary:       "grounded finding for msg-queue",
		Claims:        []types.CoagentPacketClaim{{Text: "evidence-bound claim"}},
		Sources:       []types.CoagentPacketSource{{SourceID: "s1", Kind: "web_page", Target: types.CoagentPacketSourceTarget{URI: "https://example.com/a", Title: "A"}}},
	}
	body, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}

	seq, err := reduction.commitMessageIntent(ctx, yaegikernel.StagedIntent{
		LocalID: "msg-1", Kind: yaegikernel.IntentMessage,
		ToDesk: textureAgentID, MsgKind: "evidence_update", Body: string(body),
	})
	if err != nil {
		t.Fatalf("lifecycle research Message to texture failed: %v", err)
	}

	// The queued producer update must be durable and addressed at the texture
	// target — this is what wakes the Texture actor into an incorporation turn.
	pending, err := s.ListAllPendingLifecycleUpdates(ctx, run.OwnerID, run.ComputerID, textureAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending updates for %s = %d, want 1", textureAgentID, len(pending))
	}
	stored := pending[0]
	if stored.ProducerUpdateID == "" {
		t.Fatalf("queued update missing producer_update_id: %+v", stored)
	}
	if stored.AgentID != run.AgentID || stored.TargetAgentID != textureAgentID {
		t.Fatalf("queued update producer/target = %s -> %s, want %s -> %s", stored.AgentID, stored.TargetAgentID, run.AgentID, textureAgentID)
	}
	if stored.SourceRunID != run.RunID {
		t.Fatalf("queued update source_run = %q, want %q", stored.SourceRunID, run.RunID)
	}
	if stored.WorkItemID != fixture.workID {
		t.Fatalf("queued update work_item_id = %q, want bound %q", stored.WorkItemID, fixture.workID)
	}
	if stored.WorkDisposition != types.WorkItemOpen {
		t.Fatalf("queued update work_disposition = %q, want open", stored.WorkDisposition)
	}
	if seq != uint64(stored.MessageSeq) {
		t.Fatalf("returned seq %d != stored message_seq %d", seq, stored.MessageSeq)
	}

	// Replay safety: the same cell re-reduce must dedup to the same durable
	// update rather than minting a second.
	seq2, err := reduction.commitMessageIntent(ctx, yaegikernel.StagedIntent{
		LocalID: "msg-1", Kind: yaegikernel.IntentMessage,
		ToDesk: textureAgentID, MsgKind: "evidence_update", Body: string(body),
	})
	if err != nil {
		t.Fatalf("replayed lifecycle Message failed: %v", err)
	}
	pending2, err := s.ListAllPendingLifecycleUpdates(ctx, run.OwnerID, run.ComputerID, textureAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending2) != 1 || seq2 != seq {
		t.Fatalf("replay minted duplicate: seq2=%d pending=%d", seq2, len(pending2))
	}
}

// TestCommitTrayLifecycleProducerMessageRoutesToQueue is the dispatch-level
// regression for the hollow-revision defect: commitTray routed IntentMessage
// to commitMessageIntent only when isAssignedDesk() held (assignment_id), so
// a lifecycle producer's addressed Message fell to castStagedIntent — a
// channel row whose wake no-ops on texture:* — and the evidence never woke
// the Texture actor. The widened gate must queue the update durably.
func TestCommitTrayLifecycleProducerMessageRoutesToQueue(t *testing.T) {
	rt, s := testRuntime(t)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	fixture := bindResearchControlFixture(t, rt, s, "owner-dispatch", "dispatch")
	run := fixture.run
	work, err := s.GetLifecycleWorkItem(context.Background(), run.OwnerID, run.ComputerID, fixture.workID)
	if err != nil {
		t.Fatal(err)
	}
	// Provenance stamping on the run (the RN0 fix to the activation path).
	run.RequestedByRunID, _ = work.Details["requested_by_run_id"].(string)
	run.Metadata["requested_by_run_id"] = work.Details["requested_by_run_id"]
	run.Metadata["requested_by_agent_id"] = work.Details["requested_by_agent_id"]
	run.Metadata["requested_by_profile"] = work.Details["requested_by_profile"]
	reproject := types.ReplaceLifecycleActivationRequest{
		OwnerID: run.OwnerID, ComputerID: run.ComputerID,
		CommandID: "reproject-dispatch", TrajectoryID: run.TrajectoryID,
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
		RunID: run.RunID, OwnerID: run.OwnerID, CellID: "cell-dispatch",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: run.RunID, AgentID: run.AgentID, OwnerID: run.OwnerID,
		ChannelID: run.ChannelID, ComputerID: run.ComputerID,
		Profile: agentprofile.Research, Role: agentprofile.Research,
		RunRecord: &run,
	})
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &run}

	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          "evidence_update",
		Summary:       "dispatch-path finding",
		Claims:        []types.CoagentPacketClaim{{Text: "claim"}},
		Sources:       []types.CoagentPacketSource{{SourceID: "s1", Kind: "web_page", Target: types.CoagentPacketSourceTarget{URI: "https://example.com/b", Title: "B"}}},
	}
	body, _ := json.Marshal(packet)

	if err := reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "msg-dispatch", Kind: yaegikernel.IntentMessage, ToDesk: textureAgentID, MsgKind: "evidence_update", Body: string(body)},
	}, 1); err != nil {
		t.Fatalf("commitTray lifecycle producer message failed: %v", err)
	}
	pending, err := s.ListAllPendingLifecycleUpdates(ctx, run.OwnerID, run.ComputerID, textureAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("lifecycle producer message did not queue: pending=%d", len(pending))
	}
}

// TestCommitTrayLifecycleProducerReportMintsRecordNativePacket proves the
// record-native report path: a lifecycle producer mints its commitment record
// and producer_report packet in one commit, then wakes Texture without
// mailing a dead-letter report envelope on the document channel.
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

	packet := types.CoagentSourcePacketPayload{
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
// caller trajectory's current Texture agent — the record-native report must
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

	packet := types.CoagentSourcePacketPayload{
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

	pending, err := s.ListAllPendingLifecycleUpdates(ctx, run.OwnerID, run.ComputerID, textureAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("desk-name producer report did not queue: pending=%d", len(pending))
	}
	if pending[0].Direction != types.LifecyclePacketDirectionProducerReport ||
		pending[0].SourceRecordID != "cell-desk-report:report:report-1" {
		t.Fatalf("desk-name report packet malformed: %+v", pending[0])
	}
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

// Cast cutover mints record+directive packet via CommitLifecycleAct and then
// invokes the delegated-cast admission hook against the record's canonical id
// (LifecycleResult.RecordCanonicalID). testRuntime has no capsule executor, so
// admission fails post-mint — the record and packet are durable regardless.
func TestCommitTrayLifecycleCastMintsBeforeAdmission(t *testing.T) {
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
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &caller}

	// The cast record+packet mint atomically; admission fails on the missing
	// capsule executor in the test runtime — the same legacy-ordering defect,
	// but now the record mint is inside the lifecycle transaction.
	err = reduction.commitTray(ctx, []yaegikernel.StagedIntent{
		{LocalID: "cast-1", Kind: yaegikernel.IntentCast, ToDesk: "engineering", Objective: "build the widget", Statement: "spec body"},
	}, 1)
	if err == nil || !strings.Contains(err.Error(), "capsule authority unavailable") {
		t.Fatalf("expected delegated-cast admission failure, got: %v", err)
	}

	pending, err := s.ListAllPendingLifecycleUpdates(ctx, caller.OwnerID, caller.ComputerID, engineeringID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("cast directive did not queue on engineering: pending=%d", len(pending))
	}
	if pending[0].Direction != types.LifecyclePacketDirectionDirective ||
		pending[0].SourceRecordID != "cell-cast:cast:cast-1" ||
		!strings.Contains(strings.Join(pending[0].Packet.Notes, " "), "cast_objective:build the widget") {
		t.Fatalf("cast packet malformed: %+v", pending[0])
	}
}

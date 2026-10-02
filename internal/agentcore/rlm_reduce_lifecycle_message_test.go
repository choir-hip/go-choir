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
	"testing"

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
